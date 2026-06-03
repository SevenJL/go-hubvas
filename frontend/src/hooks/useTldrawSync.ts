import { useRef, useCallback, useEffect } from 'react';
import type { Editor } from '@tldraw/tldraw';
import { getAccessToken } from '../services/api';

const BASE_URL = '/api';

/**
 * useTldrawSync provides save/load persistence for tldraw stores
 * via the REST API (PUT/GET /api/canvases/:id/snapshot).
 *
 * How it works:
 *   1. On mount, load the saved snapshot and apply it to the editor.
 *   2. On store changes, debounce-save the full snapshot every 3 seconds.
 *   3. Snapshot is stored in PostgreSQL via the api-server.
 */

interface UseTldrawSyncOptions {
  canvasId: string;
}

export function useTldrawSync({ canvasId }: UseTldrawSyncOptions) {
  const editorRef = useRef<Editor | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout>>();
  const lastSaved = useRef<string>('');

  /** Save the current store snapshot to the server. */
  const save = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor) return;

    const token = getAccessToken();
    if (!token) return;

    try {
      const snapshot = editor.store.getStoreSnapshot();
      const json = JSON.stringify(snapshot);

      // Skip if nothing changed.
      if (json === lastSaved.current) return;
      lastSaved.current = json;

      await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: json,
      });
    } catch {
      // Silently retry on next change.
    }
  }, [canvasId]);

  /** Load the saved snapshot from the server and apply to the editor. */
  const load = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor) return;

    const token = getAccessToken();
    if (!token) return;

    try {
      const res = await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const body = await res.json();
      if (body.code === 0 && body.data) {
        editor.store.loadStoreSnapshot(body.data);
        lastSaved.current = JSON.stringify(body.data);
      }
    } catch {
      // Start with empty canvas if load fails.
    }
  }, [canvasId]);

  /** Called by tldraw's onMount — registers the editor and loads saved data. */
  const onMount = useCallback(
    (editor: Editor) => {
      editorRef.current = editor;

      // Load saved snapshot (if any).
      load();

      // Listen for store changes and debounce-save.
      const unlisten = editor.store.listen(
        () => {
          if (saveTimer.current) clearTimeout(saveTimer.current);
          saveTimer.current = setTimeout(save, 3000);
        },
        { source: 'user', scope: 'document' },
      );

      // Save on page close / refresh.
      const handleUnload = () => save();
      window.addEventListener('beforeunload', handleUnload);

      return () => {
        unlisten();
        window.removeEventListener('beforeunload', handleUnload);
        if (saveTimer.current) clearTimeout(saveTimer.current);
        save();
      };
    },
    [load, save],
  );

  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
  }, []);

  return { onMount, save, load };
}

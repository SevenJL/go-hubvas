import { useRef, useCallback, useEffect } from 'react';
import type { Editor } from '@tldraw/tldraw';
import { getAccessToken } from '../services/api';

const BASE_URL = '/api';

/**
 * useTldrawSync provides save/load persistence for tldraw stores
 * via the REST API (PUT/GET /api/canvases/:id/snapshot).
 *
 * Key behaviors:
 *   - On mount: load saved snapshot FIRST, then enable saving.
 *   - On draw: debounce-save 1 second after the last change.
 *   - On unload: save immediately (covers page close/refresh).
 */

interface UseTldrawSyncOptions {
  canvasId: string;
}

export function useTldrawSync({ canvasId }: UseTldrawSyncOptions) {
  const editorRef = useRef<Editor | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout>>();
  const lastSaved = useRef<string>('');
  const loaded = useRef(false);
  // Store the cleanup function returned by onMount so we can call it later.
  const cleanupRef = useRef<(() => void) | null>(null);

  const save = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor || !loaded.current) return;

    const token = getAccessToken();
    if (!token) return;

    try {
      const snapshot = editor.store.getStoreSnapshot();
      const json = JSON.stringify(snapshot);

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

  const load = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor) return;

    const token = getAccessToken();
    if (!token) {
      loaded.current = true;
      return;
    }

    try {
      const res = await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const body = await res.json();

      if (body.code === 0 && body.data) {
        // Use mergeRemoteChanges so applying the snapshot doesn't trigger
        // the store.listen callback (avoids unnecessary save after load).
        editor.store.mergeRemoteChanges(() => {
          editor.store.loadStoreSnapshot(body.data);
        });
        lastSaved.current = JSON.stringify(body.data);
      }
    } catch {
      // Start with empty canvas if load fails.
    } finally {
      loaded.current = true;
    }
  }, [canvasId]);

  const onMount = useCallback(
    (editor: Editor) => {
      editorRef.current = editor;
      loaded.current = false;

      // 1. Load saved snapshot first (prevents empty-overwrite race).
      load().then(() => {
        // 2. Only AFTER load completes, start listening for changes.
        const unlisten = editor.store.listen(
          () => {
            if (saveTimer.current) clearTimeout(saveTimer.current);
            saveTimer.current = setTimeout(save, 1000);
          },
          { source: 'user', scope: 'document' },
        );

        const handleUnload = () => save();
        window.addEventListener('beforeunload', handleUnload);

        cleanupRef.current = () => {
          unlisten();
          window.removeEventListener('beforeunload', handleUnload);
          if (saveTimer.current) clearTimeout(saveTimer.current);
          save();
        };
      });

      return () => {
        cleanupRef.current?.();
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

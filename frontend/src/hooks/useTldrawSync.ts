import { useRef, useCallback, useEffect } from 'react';
import type { Editor } from '@tldraw/tldraw';
import { getAccessToken } from '../services/api';

const BASE_URL = '/api';

interface UseTldrawSyncOptions {
  canvasId: string;
  /** Called after a successful save with the JSON snapshot — use this to broadcast to peers. */
  onSnapshotSaved?: (snapshot: unknown) => void;
}

export function useTldrawSync({ canvasId, onSnapshotSaved }: UseTldrawSyncOptions) {
  const editorRef = useRef<Editor | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const lastSaved = useRef<string>('');
  const loaded = useRef(false);
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

      // Notify peers via the callback (Editor wires this to WebSocket).
      onSnapshotSaved?.(JSON.parse(json));
    } catch {
      // Silently retry on next change.
    }
  }, [canvasId, onSnapshotSaved]);

  /** Apply a remote snapshot received from a peer via WebSocket. */
  const applyRemoteSnapshot = useCallback((snapshot: unknown) => {
    const editor = editorRef.current;
    if (!editor) return;

    try {
      editor.store.mergeRemoteChanges(() => {
        editor.store.loadStoreSnapshot(snapshot as Parameters<typeof editor.store.loadStoreSnapshot>[0]);
      });
      lastSaved.current = JSON.stringify(snapshot);
    } catch {
      // Skip malformed snapshots.
    }
  }, []);

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

      load().then(() => {
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

  return { onMount, save, load, applyRemoteSnapshot };
}

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

      // Generate thumbnail asynchronously (best-effort, with timeout).
      let thumbnail = '';
      try {
        const shapeIds = [...editor.getCurrentPageShapeIds()];
        if (shapeIds.length > 0) {
          const result = await Promise.race([
            editor.toImageDataUrl(shapeIds, {
              format: 'png',
              scale: 0.25,
              background: true,
            }),
            new Promise<null>((resolve) => setTimeout(() => resolve(null), 3000)),
          ]);
          if (result) thumbnail = result.url;
        }
      } catch {
        // Thumbnail generation is best-effort. Don't block save.
      }

      console.log('[tldraw] saving...', { size: json.length, hasThumbnail: !!thumbnail });
      await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ data: JSON.parse(json), thumbnail }),
      });
      console.log('[tldraw] saved');

      onSnapshotSaved?.(JSON.parse(json));
    } catch (e) {
      console.error('[tldraw] save failed:', e);
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

      // Response format: { data: <tldraw snapshot>, thumbnail: "<base64>" }
      const snapshot = body.data?.data || body.data;
      if (body.code === 0 && snapshot) {
        console.log('[tldraw] loading saved snapshot...');
        editor.store.mergeRemoteChanges(() => {
          editor.store.loadStoreSnapshot(snapshot);
        });
        lastSaved.current = JSON.stringify(snapshot);
        console.log('[tldraw] loaded');
      } else {
        console.log('[tldraw] no saved snapshot');
      }
    } catch (e) {
      console.error('[tldraw] load failed:', e);
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

import { useRef, useCallback, useEffect } from 'react';
import type { Editor } from '@tldraw/tldraw';
import { getAccessToken } from '../services/api';

const BASE_URL = '/api';
const SAVE_DEBOUNCE_MS = 1000;
const MAX_QUEUED_REMOTE_MESSAGES = 500;

type StoreDiff = Parameters<Editor['store']['applyDiff']>[0];

export interface TldrawDiffBatch {
  kind: 'tldraw-diff-v1';
  diffs: StoreDiff[];
}

interface UseTldrawSyncOptions {
  canvasId: string;
  /** Broadcast a full snapshot after durable persistence for reconnect/catch-up compatibility. */
  onSnapshotSaved?: (snapshot: unknown) => void;
  /** Broadcast local document changes immediately; batches are flushed once per animation frame. */
  onRealtimeChange?: (batch: TldrawDiffBatch) => void;
}

function isDiffBatch(value: unknown): value is TldrawDiffBatch {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<TldrawDiffBatch>;
  return candidate.kind === 'tldraw-diff-v1' && Array.isArray(candidate.diffs);
}

export function useTldrawSync({ canvasId, onSnapshotSaved, onRealtimeChange }: UseTldrawSyncOptions) {
  const editorRef = useRef<Editor | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const realtimeFrame = useRef<number | undefined>(undefined);
  const pendingDiffs = useRef<StoreDiff[]>([]);
  const pendingRemote = useRef<unknown[]>([]);
  const lastSaved = useRef<string>('');
  const loaded = useRef(false);
  const cleanupRef = useRef<(() => void) | null>(null);
  const onRealtimeChangeRef = useRef(onRealtimeChange);

  useEffect(() => {
    onRealtimeChangeRef.current = onRealtimeChange;
  }, [onRealtimeChange]);

  const flushRealtimeChanges = useCallback(() => {
    realtimeFrame.current = undefined;
    if (pendingDiffs.current.length === 0) return;
    const diffs = pendingDiffs.current;
    pendingDiffs.current = [];
    onRealtimeChangeRef.current?.({ kind: 'tldraw-diff-v1', diffs });
  }, []);

  const queueRealtimeChange = useCallback((diff: StoreDiff) => {
    pendingDiffs.current.push(diff);
    if (realtimeFrame.current === undefined) {
      realtimeFrame.current = window.requestAnimationFrame(flushRealtimeChanges);
    }
  }, [flushRealtimeChanges]);

  const save = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor || !loaded.current) return;

    const token = getAccessToken();
    if (!token) return;

    try {
      const snapshot = editor.store.getStoreSnapshot();
      const json = JSON.stringify(snapshot);
      if (json === lastSaved.current) return;

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

      const res = await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ data: snapshot, thumbnail }),
      });

      if (!res.ok) {
        if (res.status === 403) return;
        throw new Error(`Save failed: ${res.status}`);
      }

      lastSaved.current = json;
      onSnapshotSaved?.(snapshot);
    } catch (e) {
      console.error('[tldraw] save failed:', e);
    }
  }, [canvasId, onSnapshotSaved]);

  const applyRemoteMessageNow = useCallback((message: unknown) => {
    const editor = editorRef.current;
    if (!editor) return;

    try {
      editor.store.mergeRemoteChanges(() => {
        if (isDiffBatch(message)) {
          for (const diff of message.diffs) editor.store.applyDiff(diff);
        } else {
          editor.store.loadStoreSnapshot(
            message as Parameters<typeof editor.store.loadStoreSnapshot>[0],
          );
          lastSaved.current = JSON.stringify(message);
        }
      });
    } catch (error) {
      console.warn('[tldraw] ignored malformed remote change:', error);
    }
  }, []);

  /** Apply an immediate diff batch or a backward-compatible full snapshot. */
  const applyRemoteSnapshot = useCallback((message: unknown) => {
    if (!editorRef.current || !loaded.current) {
      pendingRemote.current.push(message);
      if (pendingRemote.current.length > MAX_QUEUED_REMOTE_MESSAGES) pendingRemote.current.shift();
      return;
    }
    applyRemoteMessageNow(message);
  }, [applyRemoteMessageNow]);

  const load = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor) return;

    const token = getAccessToken();
    if (token) {
      try {
        const res = await fetch(`${BASE_URL}/canvases/${canvasId}/snapshot`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const body = await res.json();
        const snapshot = body.data?.data || body.data;
        if (body.code === 0 && snapshot) {
          editor.store.mergeRemoteChanges(() => editor.store.loadStoreSnapshot(snapshot));
          lastSaved.current = JSON.stringify(snapshot);
        }
      } catch {
        // Start with an empty canvas if load fails.
      }
    }

    loaded.current = true;
    const queued = pendingRemote.current;
    pendingRemote.current = [];
    for (const message of queued) applyRemoteMessageNow(message);
  }, [canvasId, applyRemoteMessageNow]);

  const onMount = useCallback((editor: Editor) => {
    cleanupRef.current?.();
    editorRef.current = editor;
    loaded.current = false;

    void load().then(() => {
      if (editorRef.current !== editor) return;
      const unlisten = editor.store.listen(
        ({ changes }) => {
          queueRealtimeChange(changes);
          if (saveTimer.current) clearTimeout(saveTimer.current);
          saveTimer.current = setTimeout(save, SAVE_DEBOUNCE_MS);
        },
        { source: 'user', scope: 'document' },
      );

      const handleUnload = () => { void save(); };
      window.addEventListener('beforeunload', handleUnload);

      cleanupRef.current = () => {
        unlisten();
        window.removeEventListener('beforeunload', handleUnload);
        if (saveTimer.current) clearTimeout(saveTimer.current);
        saveTimer.current = undefined;
        if (realtimeFrame.current !== undefined) cancelAnimationFrame(realtimeFrame.current);
        realtimeFrame.current = undefined;
        flushRealtimeChanges();
        void save();
        if (editorRef.current === editor) editorRef.current = null;
      };
    });
  }, [load, queueRealtimeChange, save, flushRealtimeChanges]);

  useEffect(() => () => {
    cleanupRef.current?.();
    cleanupRef.current = null;
  }, []);

  return { onMount, save, load, applyRemoteSnapshot };
}

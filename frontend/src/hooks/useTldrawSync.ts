import { useRef, useCallback, useEffect } from 'react';
import type { Editor, TLRecord } from '@tldraw/tldraw';
import * as Y from 'yjs';
import { getAccessToken } from '../services/api';

const BASE_URL = '/api';
const SAVE_DEBOUNCE_MS = 1000;
const LOCAL_TLDRAW_ORIGIN = 'tldraw-local';
const RECORDS_MAP = 'tldraw-records';
const META_MAP = 'tldraw-meta';
const SCHEMA_KEY = 'schema';

type StoreSnapshot = ReturnType<Editor['store']['getStoreSnapshot']>;

interface UseTldrawSyncOptions {
  canvasId: string;
  doc: Y.Doc;
  canEdit: boolean;
}

/**
 * Synchronizes tldraw's document records through Yjs.
 *
 * Each tldraw record is stored under its stable record id in a Y.Map. Local
 * tldraw diffs become Yjs transactions, while remote Yjs transactions are
 * applied through tldraw's remote-change path so they do not loop back.
 * PostgreSQL snapshots remain as a durable/read-model fallback and thumbnail
 * source, but Yjs is the authoritative real-time collaboration state.
 */
export function useTldrawSync({ canvasId, doc, canEdit }: UseTldrawSyncOptions) {
  const editorRef = useRef<Editor | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const lastSaved = useRef<string>('');
  const loaded = useRef(false);
  const cleanupRef = useRef<(() => void) | null>(null);

  const save = useCallback(async () => {
    const editor = editorRef.current;
    if (!editor || !loaded.current || !canEdit) return;

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
        // Thumbnail generation is best-effort. Don't block snapshot persistence.
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
    } catch (error) {
      console.error('[tldraw] save failed:', error);
    }
  }, [canvasId, canEdit]);

  const loadHTTPFallback = useCallback(async (editor: Editor) => {
    const token = getAccessToken();
    if (!token) return;
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
      // New canvases and temporarily unavailable snapshot storage start empty.
    }
  }, [canvasId]);

  const bindYjs = useCallback((editor: Editor) => {
    const records = doc.getMap<TLRecord>(RECORDS_MAP);
    const metadata = doc.getMap<StoreSnapshot['schema']>(META_MAP);

    const applyCompleteYjsState = () => {
      if (records.size === 0) return;
      const current = editor.store.getStoreSnapshot();
      const schema = metadata.get(SCHEMA_KEY) ?? current.schema;
      const store = Object.fromEntries(records.entries()) as StoreSnapshot['store'];
      editor.store.mergeRemoteChanges(() => {
        editor.store.loadStoreSnapshot({ store, schema });
      });
      lastSaved.current = JSON.stringify(editor.store.getStoreSnapshot());
    };

    const seedYjsFromEditor = () => {
      if (records.size > 0 || !canEdit) return;
      const snapshot = editor.store.getStoreSnapshot();
      doc.transact(() => {
        metadata.set(SCHEMA_KEY, snapshot.schema);
        for (const [id, record] of Object.entries(snapshot.store)) {
          records.set(id, record as TLRecord);
        }
      }, LOCAL_TLDRAW_ORIGIN);
    };

    if (records.size > 0) applyCompleteYjsState();
    else seedYjsFromEditor();

    const onYjsRecords = (event: Y.YMapEvent<TLRecord>, transaction: Y.Transaction) => {
      if (transaction.origin === LOCAL_TLDRAW_ORIGIN) return;
      const put: TLRecord[] = [];
      const remove: TLRecord['id'][] = [];
      for (const key of event.keysChanged) {
        const record = records.get(key);
        if (record) put.push(record);
        else remove.push(key as TLRecord['id']);
      }
      editor.store.mergeRemoteChanges(() => {
        if (remove.length > 0) editor.store.remove(remove);
        if (put.length > 0) editor.store.put(put);
      });
    };
    records.observe(onYjsRecords);

    const unlisten = canEdit
      ? editor.store.listen(({ changes }) => {
          doc.transact(() => {
            for (const [id, record] of Object.entries(changes.added)) records.set(id, record);
            for (const [id, pair] of Object.entries(changes.updated)) records.set(id, pair[1]);
            for (const id of Object.keys(changes.removed)) records.delete(id);
          }, LOCAL_TLDRAW_ORIGIN);

          if (saveTimer.current) clearTimeout(saveTimer.current);
          saveTimer.current = setTimeout(save, SAVE_DEBOUNCE_MS);
        }, { source: 'user', scope: 'document' })
      : () => undefined;

    return () => {
      records.unobserve(onYjsRecords);
      unlisten();
    };
  }, [canEdit, doc, save]);

  const onMount = useCallback((editor: Editor) => {
    cleanupRef.current?.();
    editorRef.current = editor;
    loaded.current = false;

    void loadHTTPFallback(editor).then(() => {
      if (editorRef.current !== editor) return;
      const unbindYjs = bindYjs(editor);
      loaded.current = true;

      const handleUnload = () => { void save(); };
      window.addEventListener('beforeunload', handleUnload);
      cleanupRef.current = () => {
        unbindYjs();
        window.removeEventListener('beforeunload', handleUnload);
        if (saveTimer.current) clearTimeout(saveTimer.current);
        saveTimer.current = undefined;
        void save();
        if (editorRef.current === editor) editorRef.current = null;
      };
    });
  }, [bindYjs, loadHTTPFallback, save]);

  useEffect(() => () => {
    cleanupRef.current?.();
    cleanupRef.current = null;
  }, []);

  return { onMount, save };
}

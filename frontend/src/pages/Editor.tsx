import { useEffect, useState, useCallback, useRef, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Tldraw } from '@tldraw/tldraw';
import type { Editor as TldrawEditor, TLComponents } from '@tldraw/tldraw';
import '@tldraw/tldraw/tldraw.css';
import { canvasService } from '../services/canvas';
import { getAccessToken } from '../services/api';
import { useYjsProvider } from '../hooks/useYjsProvider';
import { useTldrawSync } from '../hooks/useTldrawSync';
import { useAuth } from '../store/AuthContext';
import { Layout } from '../components/layout/Layout';
import { OnlineUsers } from '../components/canvas/OnlineUsers';
import type { CanvasInfo } from '../types';
import { ArrowLeft, Globe, EyeOff } from 'lucide-react';

function RemoteCursors({
  cursors,
  editor,
  viewportRevision,
}: {
  cursors: Map<string, { x: number; y: number; pageId?: string; username: string; color: string }>;
  editor: TldrawEditor | null;
  viewportRevision: number;
}) {
  // viewportRevision intentionally invalidates this projection after pan/zoom.
  void viewportRevision;
  if (cursors.size === 0 || !editor) return null;

  const containerBounds = editor.getContainer().getBoundingClientRect();
  const currentPageId = editor.getCurrentPageId();
  const projected = Array.from(cursors.entries()).flatMap(([uid, cursor]) => {
    if (cursor.pageId && cursor.pageId !== currentPageId) return [];
    const screenPoint = editor.pageToScreen(cursor);
    return [{
      uid,
      ...cursor,
      x: screenPoint.x - containerBounds.left,
      y: screenPoint.y - containerBounds.top,
    }];
  });

  return (
    <>
      {projected.map((pos) => (
        <div key={pos.uid} className="absolute pointer-events-none" style={{ left: pos.x, top: pos.y, zIndex: 1000, transition: 'left 0.04s linear, top 0.04s linear' }}>
          <svg width="18" height="18" viewBox="0 0 18 18">
            <path d="M3 1l12 12l-5 1l-3 4l-2-1l3-5z" fill={pos.color} stroke="white" strokeWidth="0.5" />
          </svg>
          <span className="text-[10px] text-white px-1.5 py-0.5 rounded ml-0.5 whitespace-nowrap" style={{ backgroundColor: pos.color }}>{pos.username}</span>
        </div>
      ))}
    </>
  );
}

export function Editor() {
  const { id } = useParams<{ id: string }>();
  const canvasId = id!;
  const { user } = useAuth();
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [loadError, setLoadError] = useState('');
  const [publishing, setPublishing] = useState(false);
  const [editorInstance, setEditorInstance] = useState<TldrawEditor | null>(null);
  const [viewportRevision, setViewportRevision] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas).catch(err => setLoadError(err.message));
  }, [canvasId]);

  // Determine if current user can edit this canvas.
  const canEdit = user && canvas
    ? canvas.current_role === 'owner' || canvas.current_role === 'editor'
      || Number(canvas.owner_id) === Number(user.id)
    : false;

  // ---- WebSocket (awareness + real-time sync for editors) ----
  const token = getAccessToken() || '';
  const applyRemoteRef = useRef<(snapshot: unknown) => void>(() => {});
  const handleSyncMessage = useCallback((payload: unknown) => { applyRemoteRef.current(payload); }, []);

  const { connected, awareness, onlineUsers, sendAwareness, sendTextMessage } = useYjsProvider({
    canvasId, token,
    username: user?.username || 'Anonymous',
    userId: user?.id || '0',
    onSyncMessage: handleSyncMessage,
  });

  // ---- tldraw persistence (only for editors) ----
  const handleSnapshotSaved = useCallback((snapshot: unknown) => {
    sendTextMessage('sync', snapshot);
  }, [sendTextMessage]);
  const handleRealtimeChange = useCallback((batch: unknown) => {
    sendTextMessage('sync', batch);
  }, [sendTextMessage]);

  const { onMount: onTldrawMount, applyRemoteSnapshot } = useTldrawSync(
    canEdit
      ? { canvasId, onSnapshotSaved: handleSnapshotSaved, onRealtimeChange: handleRealtimeChange }
      : { canvasId },
  );

  useEffect(() => { applyRemoteRef.current = applyRemoteSnapshot; }, [applyRemoteSnapshot]);

  // ---- Awareness (cursor tracking — only for editors) ----
  const awarenessThrottle = useRef<number>(0);
  const sendAwarenessRef = useRef(sendAwareness);
  useEffect(() => { sendAwarenessRef.current = sendAwareness; }, [sendAwareness]);

  const handleCursorMove = useCallback((cursor: { x: number; y: number; pageId?: string } | null) => {
    if (!canEdit) return;
    const now = Date.now();
    if (cursor && now - awarenessThrottle.current < 24) return;
    awarenessThrottle.current = now;
    sendAwarenessRef.current(cursor);
  }, [canEdit]);

  // ---- Publish ----
  const handlePublish = async () => {
    if (!canvas || publishing) return;
    setPublishing(true);
    try { await canvasService.publish(canvas.id); setCanvas({ ...canvas, visibility: 'published' }); }
    catch (err) { setLoadError(err instanceof Error ? err.message : 'Publish failed'); }
    finally { setPublishing(false); }
  };

  // ---- Mount ----
  // Hide toolbar in read-only mode.
  const tldrawComponents = useMemo<TLComponents>(
    () => (canEdit ? {} : { Toolbar: null, StylePanel: null, QuickActions: null }),
    [canEdit],
  );

  const handleMount = useCallback(
    (editor: Parameters<typeof onTldrawMount>[0]) => {
      onTldrawMount(editor);
      setEditorInstance(editor);

      let viewportFrame: number | undefined;
      let pointerInside = false;
      const unlistenViewport = editor.store.listen(({ changes }) => {
        const changedIds = [
          ...Object.keys(changes.added),
          ...Object.keys(changes.updated),
          ...Object.keys(changes.removed),
        ];
        if (!changedIds.some((recordId) => recordId.startsWith('camera:') || recordId.startsWith('instance:'))) return;
        if (viewportFrame === undefined) {
          viewportFrame = requestAnimationFrame(() => {
            viewportFrame = undefined;
            setViewportRevision((revision) => revision + 1);
            if (canEdit && pointerInside) {
              const pagePoint = editor.inputs.getCurrentPagePoint();
              handleCursorMove({ x: pagePoint.x, y: pagePoint.y, pageId: editor.getCurrentPageId() });
            }
          });
        }
      }, { scope: 'session' });

      // Read-only mode: lock editing + hide UI.
      if (!canEdit) {
        editor.updateInstanceState({ isReadonly: true });
        editor.setCurrentTool('select');
      }

      // Pointer tracking for awareness (editors only).
      if (canEdit) {
        const container = editor.getContainer();
        const onPointerMove = (event: PointerEvent) => {
          pointerInside = true;
          const pagePoint = editor.screenToPage({ x: event.clientX, y: event.clientY });
          handleCursorMove({ x: pagePoint.x, y: pagePoint.y, pageId: editor.getCurrentPageId() });
        };
        const onPointerLeave = () => {
          pointerInside = false;
          handleCursorMove(null);
        };
        container.addEventListener('pointermove', onPointerMove);
        container.addEventListener('pointerleave', onPointerLeave);
        const origDispose = editor.dispose.bind(editor);
        editor.dispose = () => {
          container.removeEventListener('pointermove', onPointerMove);
          container.removeEventListener('pointerleave', onPointerLeave);
          unlistenViewport();
          if (viewportFrame !== undefined) cancelAnimationFrame(viewportFrame);
          setEditorInstance(null);
          origDispose();
        };
      } else {
        const origDispose = editor.dispose.bind(editor);
        editor.dispose = () => {
          unlistenViewport();
          if (viewportFrame !== undefined) cancelAnimationFrame(viewportFrame);
          setEditorInstance(null);
          origDispose();
        };
      }
    },
    [onTldrawMount, handleCursorMove, canEdit],
  );

  if (!canvas) {
    return <Layout><div className="max-w-5xl mx-auto px-4 py-8 text-center text-gray-400">{loadError || 'Loading canvas...'}</div></Layout>;
  }

  return (
    <Layout>
      <div className="h-[calc(100vh-3.5rem)] flex flex-col">
        {/* Toolbar */}
        <div className="h-11 bg-white border-b border-gray-200 flex items-center justify-between px-4 z-50 shrink-0">
          <div className="flex items-center gap-3">
            <Link to="/dashboard" className="text-gray-400 hover:text-gray-600 transition-colors" title="Back to dashboard">
              <ArrowLeft size={20} />
            </Link>
            <h2 className="font-semibold text-gray-900 text-sm">{canvas.title}</h2>

            {/* Read-only badge */}
            {!canEdit && (
              <span className="flex items-center gap-1 text-xs text-amber-600 bg-amber-50 px-2 py-0.5 rounded-full">
                <EyeOff size={12} /> Read-only
              </span>
            )}

            {/* Publish button (owner only) */}
            {canEdit && canvas.visibility !== 'published' && (
              <button onClick={handlePublish} disabled={publishing}
                      className="flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium bg-indigo-50 text-indigo-600 hover:bg-indigo-100 transition-colors disabled:opacity-50">
                <Globe size={13} /> {publishing ? 'Publishing...' : 'Publish'}
              </button>
            )}
            {canvas.visibility === 'published' && (
              <span className="text-xs text-green-600 bg-green-50 px-2 py-0.5 rounded-full flex items-center gap-1">
                <Globe size={12} /> Published
              </span>
            )}
          </div>

          <OnlineUsers users={onlineUsers} connected={connected} currentUsername={user?.username || 'You'} />
        </div>

        {/* tldraw canvas */}
        <div className="flex-1 relative" ref={containerRef}>
          <Tldraw onMount={handleMount} components={tldrawComponents} />
          <RemoteCursors cursors={awareness} editor={editorInstance} viewportRevision={viewportRevision} />
        </div>
      </div>
    </Layout>
  );
}

import { useEffect, useState, useCallback, useRef, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Tldraw } from '@tldraw/tldraw';
import type { Editor as TldrawEditor, TLComponents } from '@tldraw/tldraw';
import '@tldraw/tldraw/tldraw.css';
import { canvasService } from '../services/canvas';
import { getAccessToken } from '../services/api';
import { useYjsProvider } from '../hooks/useYjsProvider';
import { useTldrawSync } from '../hooks/useTldrawSync';
import { useAuth } from '../store/authStore';
import { Layout } from '../components/layout/Layout';
import { OnlineUsers } from '../components/canvas/OnlineUsers';
import { MemberManager } from '../components/canvas/MemberManager';
import type { CanvasInfo } from '../types';
import { ArrowLeft, Globe, EyeOff, UserPlus } from 'lucide-react';
import { ErrorState, InlineLoader } from '../components/ui';
import { useI18n } from '../i18n';

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
  const { language, t } = useI18n();
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [loadError, setLoadError] = useState('');
  const [publishing, setPublishing] = useState(false);
  const [showMembers, setShowMembers] = useState(false);
  const [editorInstance, setEditorInstance] = useState<TldrawEditor | null>(null);
  const [viewportRevision, setViewportRevision] = useState(0);
  const [lockNotice, setLockNotice] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas).catch(err => setLoadError(err.message));
  }, [canvasId]);

  const isOwner = user && canvas ? canvas.owner_id === user.id : false;

  // Determine if current user can edit this canvas.
  const canEdit = user && canvas
    ? canvas.current_role === 'owner' || canvas.current_role === 'editor'
      || canvas.owner_id === user.id
    : false;

  // ---- WebSocket (awareness + real-time sync for editors) ----
  const token = getAccessToken() || '';
  const {
    doc, connected, awareness, onlineUsers, locks,
    sendAwareness, lockObject, unlockObject,
  } = useYjsProvider({
    canvasId, token,
    username: user?.username || t('Anonymous'),
    userId: user?.id || '0',
    canEdit,
  });

  const locksRef = useRef(locks);
  const lockObjectRef = useRef(lockObject);
  const unlockObjectRef = useRef(unlockObject);
  useEffect(() => { locksRef.current = locks; }, [locks]);
  useEffect(() => { lockObjectRef.current = lockObject; }, [lockObject]);
  useEffect(() => { unlockObjectRef.current = unlockObject; }, [unlockObject]);

  useEffect(() => {
    if (!editorInstance || !user) return;
    const selectedByOther = editorInstance.getSelectedShapeIds().some(
      shapeId => {
        const owner = locks.get(shapeId);
        return owner !== undefined && owner !== String(user.id);
      },
    );
    if (selectedByOther) editorInstance.selectNone();
  }, [editorInstance, locks, user]);

  // ---- tldraw/Yjs collaboration + durable HTTP snapshots ----
  const { onMount: onTldrawMount } = useTldrawSync({ canvasId, doc, canEdit });

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
    catch (err) { setLoadError(err instanceof Error ? err.message : t('Publish failed')); }
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
      const activeObjectLocks = new Set<string>();
      const currentUserId = String(user?.id || '0');

      const syncSelectionLocks = () => {
        if (!canEdit) return;
        const selected = new Set<string>(editor.getSelectedShapeIds());
        const blocked = Array.from(selected).find(shapeId => {
          const owner = locksRef.current.get(shapeId);
          return owner !== undefined && owner !== currentUserId;
        });
        if (blocked) {
          setLockNotice(t('This shape is being edited by another collaborator'));
          editor.selectNone();
          selected.clear();
        }
        for (const objectId of activeObjectLocks) {
          if (!selected.has(objectId)) {
            unlockObjectRef.current(objectId);
            activeObjectLocks.delete(objectId);
          }
        }
        for (const objectId of selected) {
          if (!activeObjectLocks.has(objectId)) {
            lockObjectRef.current(objectId);
            activeObjectLocks.add(objectId);
          }
        }
      };
      const unlistenSelection = editor.store.listen(syncSelectionLocks, { scope: 'session' });
      const lockRenewTimer = window.setInterval(() => {
        for (const objectId of activeObjectLocks) lockObjectRef.current(objectId);
      }, 5000);
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

      // Read-only mode: lock editing and start in the hand tool so viewers
      // can only navigate the canvas instead of selecting drawing objects.
      if (!canEdit) {
        editor.updateInstanceState({ isReadonly: true });
        editor.selectNone();
        editor.setCurrentTool('hand');
      }

      // Pointer tracking for awareness (editors only).
      if (canEdit) {
        const container = editor.getContainer();
        const onPointerDown = (event: PointerEvent) => {
          const point = editor.screenToPage({ x: event.clientX, y: event.clientY });
          const shape = editor.getShapeAtPoint(point);
          if (!shape) return;
          const owner = locksRef.current.get(shape.id);
          if (owner === undefined || owner === currentUserId) return;
          event.preventDefault();
          event.stopImmediatePropagation();
          editor.selectNone();
          setLockNotice(t('This shape is being edited by another collaborator'));
        };
        const onPointerMove = (event: PointerEvent) => {
          pointerInside = true;
          const pagePoint = editor.screenToPage({ x: event.clientX, y: event.clientY });
          handleCursorMove({ x: pagePoint.x, y: pagePoint.y, pageId: editor.getCurrentPageId() });
        };
        const onPointerLeave = () => {
          pointerInside = false;
          handleCursorMove(null);
        };
        container.addEventListener('pointerdown', onPointerDown, true);
        container.addEventListener('pointermove', onPointerMove);
        container.addEventListener('pointerleave', onPointerLeave);
        const origDispose = editor.dispose.bind(editor);
        editor.dispose = () => {
          container.removeEventListener('pointerdown', onPointerDown, true);
          container.removeEventListener('pointermove', onPointerMove);
          container.removeEventListener('pointerleave', onPointerLeave);
          for (const objectId of activeObjectLocks) unlockObjectRef.current(objectId);
          window.clearInterval(lockRenewTimer);
          unlistenSelection();
          unlistenViewport();
          if (viewportFrame !== undefined) cancelAnimationFrame(viewportFrame);
          setEditorInstance(null);
          origDispose();
        };
      } else {
        const origDispose = editor.dispose.bind(editor);
        editor.dispose = () => {
          window.clearInterval(lockRenewTimer);
          unlistenSelection();
          unlistenViewport();
          if (viewportFrame !== undefined) cancelAnimationFrame(viewportFrame);
          setEditorInstance(null);
          origDispose();
        };
      }
    },
    [onTldrawMount, handleCursorMove, canEdit, user?.id, t],
  );

  if (!canvas) {
    return (
      <Layout>
        <div className="mx-auto max-w-5xl px-4 py-8">
          {loadError
            ? <ErrorState title={t('Canvas unavailable')} message={loadError} onRetry={() => window.location.reload()} />
            : <div className="rounded-2xl border border-slate-200 bg-white"><div className="skeleton h-[55vh] rounded-t-2xl" /><InlineLoader label={t('Loading collaborative canvas...')} /></div>}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="h-[calc(100vh-3.5rem)] flex flex-col">
        {/* Toolbar */}
        <div className="z-50 flex h-11 shrink-0 items-center justify-between gap-1.5 border-b border-gray-200 bg-white px-2 sm:gap-3 sm:px-4">
          <div className="flex min-w-0 flex-1 items-center gap-1.5 sm:gap-3">
            <Link to="/dashboard" className="shrink-0 text-gray-400 transition-colors hover:text-gray-600" title={t('Back to dashboard')}>
              <ArrowLeft size={20} />
            </Link>
            <h2 className="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900" title={canvas.title}>{canvas.title}</h2>

            {/* Read-only badge */}
            {!canEdit && (
              <span className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full bg-amber-50 px-2 py-0.5 text-xs text-amber-600">
                <EyeOff size={12} /> {t('Read-only')}
              </span>
            )}

            {isOwner && (
              <button onClick={() => setShowMembers(true)}
                      className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-200 sm:px-2.5">
                <UserPlus size={13} /> {t('Members')}
              </button>
            )}

            {/* Publish button (owner only) */}
            {isOwner && canvas.visibility !== 'published' && (
              <button onClick={handlePublish} disabled={publishing}
                      className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md bg-indigo-50 px-2 py-1 text-xs font-medium text-indigo-600 transition-colors hover:bg-indigo-100 disabled:opacity-50 sm:px-2.5">
                <Globe size={13} /> {t(publishing ? 'Publishing...' : 'Publish')}
              </button>
            )}
            {canvas.visibility === 'published' && (
              <span className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-600">
                <Globe size={12} /> {t('Published')}
              </span>
            )}
          </div>

          <OnlineUsers users={onlineUsers} connected={connected} currentUsername={user?.display_name || user?.username || t('You')} currentAvatarURL={user?.avatar_url} />
        </div>

        {showMembers && <MemberManager canvasId={canvasId} onClose={() => setShowMembers(false)} />}

        {/* tldraw canvas */}
        <div className="flex-1 relative" ref={containerRef}>
          {lockNotice && (
            <button
              type="button"
              onClick={() => setLockNotice('')}
              className="absolute top-3 left-1/2 -translate-x-1/2 z-[1100] rounded-full bg-amber-950/90 px-4 py-2 text-xs font-medium text-white shadow-lg"
            >
              {lockNotice}
            </button>
          )}
          <Tldraw key={canvasId} locale={language === 'zh' ? 'zh-cn' : 'en'} onMount={handleMount} components={tldrawComponents} />
          <RemoteCursors cursors={awareness} editor={editorInstance} viewportRevision={viewportRevision} />
        </div>
      </div>
    </Layout>
  );
}

import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Tldraw } from '@tldraw/tldraw';
import '@tldraw/tldraw/tldraw.css';
import { canvasService } from '../services/canvas';
import { getAccessToken } from '../services/api';
import { useYjsProvider } from '../hooks/useYjsProvider';
import { useTldrawSync } from '../hooks/useTldrawSync';
import { useAuth } from '../store/AuthContext';
import { Layout } from '../components/layout/Layout';
import type { CanvasInfo } from '../types';
import { ArrowLeft, Users, Wifi, WifiOff } from 'lucide-react';

function RemoteCursors({
  cursors,
}: {
  cursors: Map<number, { x: number; y: number; username: string; color: string }>;
}) {
  if (cursors.size === 0) return null;
  return (
    <>
      {Array.from(cursors.entries()).map(([uid, pos]) => (
        <div
          key={uid}
          className="absolute pointer-events-none"
          style={{
            left: pos.x, top: pos.y, zIndex: 1000,
            transition: 'left 0.08s linear, top 0.08s linear',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 18 18">
            <path d="M3 1l12 12l-5 1l-3 4l-2-1l3-5z" fill={pos.color} stroke="white" strokeWidth="0.5" />
          </svg>
          <span className="text-[10px] text-white px-1.5 py-0.5 rounded ml-0.5 whitespace-nowrap" style={{ backgroundColor: pos.color }}>
            {pos.username}
          </span>
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
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas).catch(err => setLoadError(err.message));
  }, [canvasId]);

  // ---- WebSocket (awareness + real-time sync) ----
  const token = getAccessToken() || '';

  // We need applyRemoteSnapshot before useYjsProvider, so use a ref.
  const applyRemoteRef = useRef<(snapshot: unknown) => void>(() => {});

  const handleSyncMessage = useCallback((payload: unknown) => {
    // Received a snapshot from another user via WebSocket.
    applyRemoteRef.current(payload);
  }, []);

  const {
    connected,
    awareness,
    onlineUsers,
    sendAwareness,
    sendTextMessage,
  } = useYjsProvider({
    canvasId, token,
    username: user?.username || 'Anonymous',
    userId: user?.id || '0',
    onSyncMessage: handleSyncMessage,
  });

  // ---- tldraw persistence (REST save + WebSocket broadcast) ----
  const handleSnapshotSaved = useCallback((snapshot: unknown) => {
    // After saving locally, broadcast to other users in the room.
    sendTextMessage('sync', snapshot);
  }, [sendTextMessage]);

  const { onMount: onTldrawMount, applyRemoteSnapshot } = useTldrawSync({
    canvasId,
    onSnapshotSaved: handleSnapshotSaved,
  });

  // Keep applyRemoteRef in sync so the WebSocket callback can find it.
  useEffect(() => {
    applyRemoteRef.current = applyRemoteSnapshot;
  }, [applyRemoteSnapshot]);

  // ---- Awareness (cursor tracking) ----
  const awarenessThrottle = useRef<number>(0);
  const sendAwarenessRef = useRef(sendAwareness);
  useEffect(() => { sendAwarenessRef.current = sendAwareness; }, [sendAwareness]);

  const handleCursorMove = useCallback(
    (cursor: { x: number; y: number } | null) => {
      const now = Date.now();
      if (now - awarenessThrottle.current < 40) return;
      awarenessThrottle.current = now;
      sendAwarenessRef.current(cursor);
    }, [],
  );

  const handleMount = useCallback(
    (editor: Parameters<typeof onTldrawMount>[0]) => {
      onTldrawMount(editor);

      const container = editor.getContainer();
      const onPointerMove = (e: PointerEvent) => handleCursorMove({ x: e.clientX, y: e.clientY });
      container.addEventListener('pointermove', onPointerMove);

      const origDispose = editor.dispose.bind(editor);
      editor.dispose = () => {
        container.removeEventListener('pointermove', onPointerMove);
        origDispose();
      };
    },
    [onTldrawMount, handleCursorMove],
  );

  if (!canvas) {
    return (
      <Layout>
        <div className="max-w-5xl mx-auto px-4 py-8 text-center text-gray-400">
          {loadError || 'Loading canvas...'}
        </div>
      </Layout>
    );
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
            <span className="flex items-center gap-1.5 text-xs">
              {connected ? (
                <>
                  <Wifi size={12} className="text-green-500" />
                  <span className="text-green-600">
                    {onlineUsers.length > 0 ? `${onlineUsers.length + 1} online` : 'Connected'}
                  </span>
                </>
              ) : (
                <>
                  <WifiOff size={12} className="text-red-500" />
                  <span className="text-red-500">Reconnecting...</span>
                </>
              )}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Users size={14} className="text-gray-400" />
            <div className="flex -space-x-1.5">
              <div className="w-6 h-6 rounded-full bg-indigo-500 border-2 border-white flex items-center justify-center text-[10px] font-medium text-white"
                   title={`${user?.username || 'You'} (you)`}>
                {(user?.username || 'Y')[0]?.toUpperCase()}
              </div>
              {onlineUsers.slice(0, 5).map(m => (
                <div key={m.user_id}
                     className="w-6 h-6 rounded-full bg-indigo-100 border-2 border-white flex items-center justify-center text-[10px] font-medium text-indigo-600"
                     title={m.username}>
                  {m.username[0]?.toUpperCase() || '?'}
                </div>
              ))}
              {onlineUsers.length > 5 && (
                <div className="w-6 h-6 rounded-full bg-gray-100 border-2 border-white flex items-center justify-center text-[10px] font-medium text-gray-500">
                  +{onlineUsers.length - 5}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* tldraw canvas */}
        <div className="flex-1 relative" ref={containerRef}>
          <Tldraw onMount={handleMount} />
          <RemoteCursors cursors={awareness} />
        </div>
      </div>
    </Layout>
  );
}

import { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { canvasService } from '../services/canvas';
import { useWebSocket } from '../hooks/useWebSocket';
import { Layout } from '../components/layout/Layout';
import type { CanvasInfo, WSMessage, PresenceMember, AwarenessPayload } from '../types';
import { ArrowLeft, Users } from 'lucide-react';

export function Editor() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const canvasId = id!;
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [onlineUsers, setOnlineUsers] = useState<PresenceMember[]>([]);
  const [cursors, setCursors] = useState<Map<string, { x: number; y: number; username: string }>>(new Map());
  const [error, setError] = useState('');

  // Load canvas metadata.
  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas).catch(err => setError(err.message));
  }, [canvasId]);

  // Handle incoming WS messages.
  const handleMessage = useCallback((msg: WSMessage) => {
    switch (msg.type) {
      case 'sync':
        // CRDT update — in production, apply to Yjs document.
        break;
      case 'awareness': {
        const payload = msg.payload as AwarenessPayload;
        if (payload?.cursor) {
          setCursors(prev => {
            const next = new Map(prev);
            next.set('0', {
              x: payload.cursor!.x,
              y: payload.cursor!.y,
              username: 'User',
            });
            return next;
          });
        }
        break;
      }
    }
  }, []);

  const handlePresence = useCallback((members: PresenceMember[]) => {
    setOnlineUsers(members);
  }, []);

  const handleError = useCallback((msg: string) => {
    setError(msg);
  }, []);

  const { connected, send: _send } = useWebSocket({
    canvasId,
    onMessage: handleMessage,
    onPresence: handlePresence,
    onError: handleError,
  });

  if (!canvas) {
    return (
      <Layout>
        <div className="max-w-5xl mx-auto px-4 py-8 text-center text-gray-400">
          {error || 'Loading canvas...'}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="h-[calc(100vh-3.5rem)] flex flex-col">
        {/* Toolbar */}
        <div className="h-12 bg-white border-b border-gray-200 flex items-center justify-between px-4">
          <div className="flex items-center gap-3">
            <Link to="/dashboard" className="text-gray-400 hover:text-gray-600 transition-colors">
              <ArrowLeft size={20} />
            </Link>
            <h2 className="font-semibold text-gray-900">{canvas.title}</h2>
            <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="text-xs text-gray-400">
              {connected ? 'Connected' : 'Reconnecting...'}
            </span>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5 text-sm text-gray-500">
              <Users size={16} />
              <span>{onlineUsers.length} online</span>
            </div>

            {/* Online avatars */}
            <div className="flex -space-x-2">
              {onlineUsers.slice(0, 5).map(m => (
                <div
                  key={m.user_id}
                  className="w-7 h-7 rounded-full bg-indigo-100 border-2 border-white flex items-center justify-center text-xs font-medium text-indigo-600"
                  title={m.username}
                >
                  {m.username[0].toUpperCase()}
                </div>
              ))}
              {onlineUsers.length > 5 && (
                <div className="w-7 h-7 rounded-full bg-gray-100 border-2 border-white flex items-center justify-center text-xs text-gray-500">
                  +{onlineUsers.length - 5}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Canvas area */}
        <div className="flex-1 bg-gray-100 relative overflow-hidden">
          {/* Placeholder canvas — replace with tldraw in production. */}
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="text-center">
              <div className="w-32 h-32 rounded-2xl bg-white shadow-lg border border-gray-200 flex items-center justify-center mx-auto mb-4">
                <span className="text-4xl">🎨</span>
              </div>
              <h3 className="text-lg font-semibold text-gray-700 mb-1">Canvas Ready</h3>
              <p className="text-sm text-gray-400 max-w-md">
                {connected
                  ? 'Real-time collaboration is active. Replace this placeholder with tldraw + Yjs for the full drawing experience.'
                  : 'Connecting to collaboration server...'}
              </p>

              {/* Online user list */}
              {onlineUsers.length > 0 && (
                <div className="mt-6 inline-block text-left">
                  <p className="text-xs font-medium text-gray-500 mb-2">Online Members</p>
                  <div className="space-y-1.5">
                    {onlineUsers.map(m => (
                      <div key={m.user_id} className="flex items-center gap-2 text-sm text-gray-600">
                        <div
                          className="w-6 h-6 rounded-full bg-indigo-100 flex items-center justify-center text-xs font-medium text-indigo-600"
                        >
                          {m.username[0].toUpperCase()}
                        </div>
                        <span>{m.username}</span>
                        <span className="text-xs text-gray-400">({m.role})</span>
                        {m.user_id === user?.id && <span className="text-xs text-indigo-500">(you)</span>}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Cursor overlays */}
          {Array.from(cursors.entries()).map(([uid, pos]) => (
            <div
              key={uid}
              className="absolute pointer-events-none z-10"
              style={{ left: pos.x, top: pos.y }}
            >
              <svg width="18" height="18" viewBox="0 0 18 18">
                <path d="M3 1l12 12l-5 1l-3 4l-2-1l3-5z" fill="#6366f1" />
              </svg>
              <span className="text-xs text-white bg-indigo-500 px-1.5 py-0.5 rounded ml-0.5">
                {pos.username}
              </span>
            </div>
          ))}
        </div>
      </div>
    </Layout>
  );
}

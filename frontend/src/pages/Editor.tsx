import { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Tldraw } from '@tldraw/tldraw';
import '@tldraw/tldraw/tldraw.css';
import { canvasService } from '../services/canvas';
import { useWebSocket } from '../hooks/useWebSocket';
import { Layout } from '../components/layout/Layout';
import type { CanvasInfo, WSMessage, PresenceMember, AwarenessPayload } from '../types';
import { ArrowLeft, Users } from 'lucide-react';

export function Editor() {
  const { id } = useParams<{ id: string }>();
  const canvasId = id!;
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [onlineUsers, setOnlineUsers] = useState<PresenceMember[]>([]);
  const [cursors, setCursors] = useState<Map<string, { x: number; y: number; username: string }>>(new Map());
  const [error, setError] = useState('');

  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas).catch(err => setError(err.message));
  }, [canvasId]);

  const handleMessage = useCallback((msg: WSMessage) => {
    switch (msg.type) {
      case 'awareness': {
        const p = msg.payload as AwarenessPayload;
        if (p?.cursor) {
          setCursors(prev => {
            const next = new Map(prev);
            next.set('user', { x: p.cursor!.x, y: p.cursor!.y, username: 'User' });
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

  const { connected } = useWebSocket({
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
        <div className="h-11 bg-white border-b border-gray-200 flex items-center justify-between px-4 z-50 shrink-0">
          <div className="flex items-center gap-3">
            <Link to="/dashboard" className="text-gray-400 hover:text-gray-600 transition-colors">
              <ArrowLeft size={20} />
            </Link>
            <h2 className="font-semibold text-gray-900 text-sm">{canvas.title}</h2>
            <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="text-xs text-gray-400">
              {connected ? `${onlineUsers.length + 1} online` : 'Reconnecting...'}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Users size={14} className="text-gray-400" />
            <div className="flex -space-x-1.5">
              {onlineUsers.slice(0, 4).map(m => (
                <div
                  key={m.user_id}
                  className="w-6 h-6 rounded-full bg-indigo-100 border-2 border-white flex items-center justify-center text-[10px] font-medium text-indigo-600"
                  title={m.username}
                >
                  {m.username[0]?.toUpperCase() || '?'}
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* tldraw drawing canvas */}
        <div className="flex-1 relative">
          <Tldraw />
        </div>

        {/* Remote cursor overlays */}
        {Array.from(cursors.entries()).map(([uid, pos]) => (
          <div key={uid} className="absolute pointer-events-none" style={{ left: pos.x, top: pos.y, zIndex: 1000 }}>
            <svg width="18" height="18" viewBox="0 0 18 18">
              <path d="M3 1l12 12l-5 1l-3 4l-2-1l3-5z" fill="#6366f1" />
            </svg>
            <span className="text-[10px] text-white bg-indigo-500 px-1.5 py-0.5 rounded ml-0.5">
              {pos.username}
            </span>
          </div>
        ))}
      </div>
    </Layout>
  );
}

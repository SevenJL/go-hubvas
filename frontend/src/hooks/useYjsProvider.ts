import { useEffect, useRef, useCallback, useState } from 'react';
import * as Y from 'yjs';
import type { WSMessage, PresenceMember } from '../types';

/**
 * Custom Yjs provider that syncs a Y.Doc over our application WebSocket
 * instead of the y-websocket protocol.
 *
 * Architecture:
 *   - Y.Doc 'update' events → binary frames sent over WebSocket
 *   - Binary frames from WebSocket → Y.applyUpdate(doc, ...)
 *   - Awareness state synced via JSON awareness messages
 */

interface UseYjsProviderOptions {
  canvasId: string;
  token: string;
  username: string;
  userId: string;
}

interface UseYjsProviderResult {
  /** The shared Yjs document */
  doc: Y.Doc;
  /** Whether the WebSocket is connected */
  connected: boolean;
  /** Remote awareness states (cursors, selections) */
  awareness: Map<number, { x: number; y: number; username: string; color: string }>;
  /** Remote user presence list */
  onlineUsers: PresenceMember[];
  /** Send a sync update (called automatically, but exposed for testing) */
  sendSync: (update: Uint8Array) => void;
  /** Send awareness update */
  sendAwareness: (cursor: { x: number; y: number } | null, selection?: unknown) => void;
}

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

export function useYjsProvider({
  canvasId,
  token,
  username,
  userId,
}: UseYjsProviderOptions): UseYjsProviderResult {
  const docRef = useRef<Y.Doc>(new Y.Doc());
  const wsRef = useRef<WebSocket | null>(null);
  const seqRef = useRef(0);
  const reconnectAttempt = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();

  const [connected, setConnected] = useState(false);
  const [onlineUsers, setOnlineUsers] = useState<PresenceMember[]>([]);
  const [awareness, setAwareness] = useState<
    Map<number, { x: number; y: number; username: string; color: string }>
  >(new Map());

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const url = `${protocol}//${host}/ws?canvas=${canvasId}&token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      setConnected(true);
      reconnectAttempt.current = 0;

      // Send initial presence.
      ws.send(JSON.stringify({
        type: 'presence',
        seq: ++seqRef.current,
        payload: { user_id: userId, username },
      }));
    };

    ws.onmessage = (event) => {
      // Handle binary frames (CRDT updates)
      if (event.data instanceof ArrayBuffer) {
        try {
          const update = new Uint8Array(event.data);
          Y.applyUpdate(docRef.current, update);
        } catch {
          // Skip malformed binary frames
        }
        return;
      }

      // Handle text messages (JSON)
      try {
        const msg: WSMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'sync': {
            // Server-relayed CRDT update — may be base64-encoded or raw.
            const payload = msg.payload as { update?: string } | undefined;
            if (payload?.update) {
              try {
                const binary = Uint8Array.from(atob(payload.update), c => c.charCodeAt(0));
                Y.applyUpdate(docRef.current, binary);
              } catch {
                // Skip malformed updates
              }
            }
            break;
          }

          case 'awareness': {
            const p = msg.payload as {
              user_id?: string;
              username?: string;
              cursor?: { x: number; y: number };
            } | undefined;
            if (p?.cursor) {
              setAwareness(prev => {
                const next = new Map(prev);
                next.set(Number(p.user_id || '0'), {
                  x: p.cursor!.x,
                  y: p.cursor!.y,
                  username: p.username || 'Unknown',
                  color: stringToColor(p.user_id || '0'),
                });
                return next;
              });
            }
            break;
          }

          case 'presence': {
            const p = msg.payload as { online?: PresenceMember[] } | undefined;
            if (p?.online) {
              setOnlineUsers(p.online);
            }
            break;
          }

          case 'error': {
            const p = msg.payload as { message?: string } | undefined;
            if (p?.message) {
              console.warn('[yjs] server error:', p.message);
            }
            break;
          }

          case 'ack':
            // Server acknowledged our message — nothing to do.
            break;
        }
      } catch {
        // Skip malformed JSON messages
      }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;

      const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt.current, RECONNECT_MAX_MS);
      reconnectAttempt.current++;
      reconnectTimer.current = setTimeout(connect, delay);
    };

    ws.onerror = () => {
      // onclose will fire next; reconnect logic is there.
    };
  }, [canvasId, token, userId, username]);

  // Connect on mount, disconnect on unmount.
  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  // Listen for local Y.Doc updates and send them over WebSocket.
  useEffect(() => {
    const doc = docRef.current;
    const handler = (update: Uint8Array, origin: unknown) => {
      // Skip updates originating from remote (to avoid echo loops).
      if (origin === 'remote') return;

      const ws = wsRef.current;
      if (ws?.readyState === WebSocket.OPEN) {
        // Send as binary frame for efficiency.
        ws.send(update.buffer);
      }
    };

    doc.on('update', handler);
    return () => {
      doc.off('update', handler);
    };
  }, []);

  const sendSync = useCallback((update: Uint8Array) => {
    const ws = wsRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(update.buffer);
    }
  }, []);

  const sendAwareness = useCallback(
    (cursor: { x: number; y: number } | null, selection?: unknown) => {
      const ws = wsRef.current;
      if (ws?.readyState !== WebSocket.OPEN) return;

      ws.send(
        JSON.stringify({
          type: 'awareness',
          seq: ++seqRef.current,
          payload: { cursor, selection, user_id: userId, username },
        }),
      );
    },
    [userId, username],
  );

  return {
    doc: docRef.current,
    connected,
    awareness,
    onlineUsers,
    sendSync,
    sendAwareness,
  };
}

/** Generate a consistent color from a string (for cursor colors). */
function stringToColor(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  const colors = [
    '#6366f1', '#ec4899', '#f59e0b', '#10b981',
    '#3b82f6', '#8b5cf6', '#ef4444', '#06b6d4',
  ];
  return colors[Math.abs(hash) % colors.length];
}

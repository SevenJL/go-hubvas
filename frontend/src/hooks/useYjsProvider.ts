import { useEffect, useRef, useCallback, useState } from 'react';
import * as Y from 'yjs';
import type { WSMessage, PresenceMember } from '../types';

interface UseYjsProviderOptions {
  canvasId: string;
  token: string;
  username: string;
  userId: string;
  /** Called when a remote sync message is received (JSON text frame with type="sync"). */
  onSyncMessage?: (payload: unknown) => void;
}

interface UseYjsProviderResult {
  doc: Y.Doc;
  connected: boolean;
  awareness: Map<number, { x: number; y: number; username: string; color: string }>;
  onlineUsers: PresenceMember[];
  sendSync: (update: Uint8Array) => void;
  sendAwareness: (cursor: { x: number; y: number } | null, selection?: unknown) => void;
  /** Send a JSON text message (type + payload) over the WebSocket. */
  sendTextMessage: (type: string, payload: unknown) => void;
}

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

export function useYjsProvider({
  canvasId,
  token,
  username,
  userId,
  onSyncMessage,
}: UseYjsProviderOptions): UseYjsProviderResult {
  const docRef = useRef<Y.Doc>(new Y.Doc());
  const wsRef = useRef<WebSocket | null>(null);
  const seqRef = useRef(0);
  const reconnectAttempt = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const onSyncMessageRef = useRef(onSyncMessage);
  onSyncMessageRef.current = onSyncMessage;

  const [connected, setConnected] = useState(false);
  const [onlineUsers, setOnlineUsers] = useState<PresenceMember[]>([]);
  const [awareness, setAwareness] = useState<
    Map<number, { x: number; y: number; username: string; color: string }>
  >(new Map());

  const sendTextMessage = useCallback((type: string, payload: unknown) => {
    const ws = wsRef.current;
    if (ws?.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type, seq: ++seqRef.current, payload }));
  }, []);

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
      sendTextMessage('presence', { user_id: userId, username });
    };

    ws.onmessage = (event) => {
      // Binary frames — Yjs CRDT updates.
      if (event.data instanceof ArrayBuffer) {
        try {
          Y.applyUpdate(docRef.current, new Uint8Array(event.data));
        } catch { /* skip */ }
        return;
      }

      // Text frames — JSON protocol.
      try {
        const msg: WSMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'sync': {
            // Text-based sync message (tldraw JSON snapshot).
            onSyncMessageRef.current?.(msg.payload);
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
            if (p?.online) setOnlineUsers(p.online);
            break;
          }

          case 'error': {
            const p = msg.payload as { message?: string } | undefined;
            if (p?.message) console.warn('[ws] server error:', p.message);
            break;
          }
        }
      } catch { /* skip */ }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt.current, RECONNECT_MAX_MS);
      reconnectAttempt.current++;
      reconnectTimer.current = setTimeout(connect, delay);
    };
  }, [canvasId, token, userId, username, sendTextMessage]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  // Yjs doc → binary WebSocket frames.
  useEffect(() => {
    const doc = docRef.current;
    const handler = (update: Uint8Array, origin: unknown) => {
      if (origin === 'remote') return;
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(update as Uint8Array<ArrayBuffer>);
      }
    };
    doc.on('update', handler);
    return () => { doc.off('update', handler); };
  }, []);

  const sendSync = useCallback((update: Uint8Array) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(update as Uint8Array<ArrayBuffer>);
    }
  }, []);

  const sendAwareness = useCallback(
    (cursor: { x: number; y: number } | null, selection?: unknown) => {
      sendTextMessage('awareness', { cursor, selection, user_id: userId, username });
    },
    [userId, username, sendTextMessage],
  );

  return {
    doc: docRef.current,
    connected,
    awareness,
    onlineUsers,
    sendSync,
    sendAwareness,
    sendTextMessage,
  };
}

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

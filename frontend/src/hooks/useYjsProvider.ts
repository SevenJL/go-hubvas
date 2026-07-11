import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import * as Y from 'yjs';
import type { WSMessage, PresenceMember } from '../types';

interface UseYjsProviderOptions {
  canvasId: string;
  token: string;
  username: string;
  userId: string;
  canEdit: boolean;
  /** Called when a remote sync message is received (JSON text frame with type="sync"). */
  onSyncMessage?: (payload: unknown) => void;
}

interface UseYjsProviderResult {
  doc: Y.Doc;
  connected: boolean;
  awareness: Map<string, { x: number; y: number; pageId?: string; username: string; color: string }>;
  onlineUsers: PresenceMember[];
  sendSync: (update: Uint8Array) => void;
  sendAwareness: (cursor: { x: number; y: number; pageId?: string } | null, selection?: unknown) => void;
  /** Send a JSON text message (type + payload) over the WebSocket. */
  sendTextMessage: (type: string, payload: unknown) => void;
}

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

function toArrayBuffer(update: Uint8Array): ArrayBuffer {
  return new Uint8Array(update).buffer;
}

export function useYjsProvider({
  canvasId,
  token,
  username,
  userId,
  canEdit,
  onSyncMessage,
}: UseYjsProviderOptions): UseYjsProviderResult {
  const doc = useMemo(() => {
    // A canvas change must get an isolated CRDT document.
    void canvasId;
    return new Y.Doc();
  }, [canvasId]);
  const wsRef = useRef<WebSocket | null>(null);
  const seqRef = useRef(0);
  const reconnectAttempt = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const connectRef = useRef<() => void>(() => undefined);
  const shouldReconnectRef = useRef(false);
  const onSyncMessageRef = useRef(onSyncMessage);

  const [connected, setConnected] = useState(false);
  const [onlineUsers, setOnlineUsers] = useState<PresenceMember[]>([]);
  const [awareness, setAwareness] = useState<
    Map<string, { x: number; y: number; pageId?: string; username: string; color: string }>
  >(new Map());

  useEffect(() => {
    onSyncMessageRef.current = onSyncMessage;
  }, [onSyncMessage]);

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
      // Re-send the complete local Yjs state after every connection. This is
      // idempotent and ensures edits made while disconnected are merged back
      // into the room as soon as an editor reconnects.
      if (canEdit) {
        ws.send(toArrayBuffer(Y.encodeStateAsUpdate(doc)));
      }
      sendTextMessage('presence', { user_id: userId, username });
    };

    ws.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        try {
          Y.applyUpdate(doc, new Uint8Array(event.data), 'remote');
        } catch {
          // Ignore malformed remote updates.
        }
        return;
      }

      try {
        const msg: WSMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'sync':
            onSyncMessageRef.current?.(msg.payload);
            break;
          case 'awareness': {
            const p = msg.payload as {
              user_id?: string;
              username?: string;
              cursor?: { x: number; y: number; pageId?: string } | null;
            } | undefined;
            if (p && 'cursor' in p) {
              setAwareness(prev => {
                const next = new Map(prev);
                const remoteUserId = p.user_id || '0';
                if (!p.cursor) {
                  next.delete(remoteUserId);
                } else {
                  next.set(remoteUserId, {
                    x: p.cursor.x,
                    y: p.cursor.y,
                    pageId: p.cursor.pageId,
                    username: p.username || 'Unknown',
                    color: stringToColor(p.user_id || '0'),
                  });
                }
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
      } catch {
        // Ignore malformed text messages.
      }
    };

    ws.onclose = () => {
      setConnected(false);
      if (wsRef.current === ws) wsRef.current = null;
      if (!shouldReconnectRef.current) return;

      const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt.current, RECONNECT_MAX_MS);
      reconnectAttempt.current++;
      reconnectTimer.current = setTimeout(() => connectRef.current(), delay);
    };
  }, [canvasId, token, userId, username, sendTextMessage, doc, canEdit]);

  useEffect(() => {
    connectRef.current = connect;
    shouldReconnectRef.current = true;
    connect();
    return () => {
      shouldReconnectRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      reconnectTimer.current = undefined;
      const ws = wsRef.current;
      if (ws) {
        ws.onclose = null;
        ws.close();
      }
      wsRef.current = null;
    };
  }, [connect]);

  useEffect(() => {
    const handler = (update: Uint8Array, origin: unknown) => {
      if (origin === 'remote') return;
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(toArrayBuffer(update));
      }
    };
    doc.on('update', handler);
    return () => doc.off('update', handler);
  }, [doc]);

  useEffect(() => () => doc.destroy(), [doc]);

  const sendSync = useCallback((update: Uint8Array) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(toArrayBuffer(update));
    }
  }, []);

  const sendAwareness = useCallback(
    (cursor: { x: number; y: number; pageId?: string } | null, selection?: unknown) => {
      sendTextMessage('awareness', { cursor, selection, user_id: userId, username });
    },
    [userId, username, sendTextMessage],
  );

  return { doc, connected, awareness, onlineUsers, sendSync, sendAwareness, sendTextMessage };
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

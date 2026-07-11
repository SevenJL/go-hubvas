import { useRef, useEffect, useCallback, useState } from 'react';
import { getAccessToken } from '../services/api';
import type { WSMessage } from '../types';

interface UseWebSocketOptions {
  canvasId: string;
  onMessage?: (msg: WSMessage) => void;
  onPresence?: (members: import('../types').PresenceMember[]) => void;
  onError?: (error: string) => void;
}

export function useWebSocket({ canvasId, onMessage, onPresence, onError }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const seqRef = useRef(0);
  const [connected, setConnected] = useState(false);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const reconnectAttempt = useRef(0);
  const connectRef = useRef<() => void>(() => undefined);
  const shouldReconnectRef = useRef(false);

  const connect = useCallback(() => {
    const token = getAccessToken();
    if (!token) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const url = `${protocol}//${host}/ws?canvas=${canvasId}&token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      reconnectAttempt.current = 0;
    };

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'presence':
            if (onPresence && msg.payload) {
              onPresence((msg.payload as { online: import('../types').PresenceMember[] }).online);
            }
            break;
          case 'error':
            if (onError && msg.payload) {
              onError((msg.payload as { message: string }).message);
            }
            break;
          default:
            onMessage?.(msg);
        }
      } catch {
        // Binary frame — forward as sync update.
        onMessage?.({ type: 'sync', payload: event.data });
      }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;

      // Exponential backoff reconnect.
      const delay = Math.min(1000 * 2 ** reconnectAttempt.current, 30000);
      reconnectAttempt.current++;
      if (shouldReconnectRef.current) {
        reconnectTimer.current = setTimeout(() => connectRef.current(), delay);
      }
    };

    ws.onerror = () => {
      onError?.('WebSocket connection error');
    };
  }, [canvasId, onMessage, onPresence, onError]);

  useEffect(() => {
    connectRef.current = connect;
    shouldReconnectRef.current = true;
    connect();
    return () => {
      shouldReconnectRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      const ws = wsRef.current;
      if (ws) {
        ws.onclose = null;
        ws.close();
      }
      wsRef.current = null;
    };
  }, [connect]);

  const send = useCallback((msg: WSMessage) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({ ...msg, seq: ++seqRef.current }));
  }, []);

  const sendSync = useCallback((update: Uint8Array) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(new Uint8Array(update).buffer); // Binary frame.
    }
  }, []);

  return { connected, send, sendSync };
}

import { useCallback, useEffect, useRef, useState } from 'react';
import { getAccessToken } from '../services/api';
import { socialService } from '../services/social';

export function useNotifications(enabled: boolean) {
  const [unread, setUnread] = useState(0);
  const retry = useRef(0);
  const sync = useCallback(async () => {
    if (!enabled) {
      setUnread(0);
      return;
    }
    try {
      setUnread((await socialService.unread()).count);
    } catch {
      // REST retry occurs on reconnect, focus, or the next notification event.
    }
  }, [enabled]);

  useEffect(() => {
    void sync();
    if (!enabled) return;

    let ws: WebSocket | undefined;
    let timer: number | undefined;
    let closed = false;
    const connect = () => {
      const token = getAccessToken();
      if (!token || closed) return;
      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
      ws = new WebSocket(`${protocol}//${location.host}/ws/notifications?token=${encodeURIComponent(token)}`);
      ws.onopen = () => {
        retry.current = 0;
        void sync();
      };
      ws.onmessage = event => {
        try {
          const message = JSON.parse(event.data);
          if (message.type === 'notification.created') {
            setUnread(count => count + 1);
            window.dispatchEvent(new CustomEvent('hubvas:notification', { detail: message.payload }));
          }
        } catch {
          // Ignore malformed frames; REST remains the source of truth.
        }
      };
      ws.onclose = () => {
        if (!closed) timer = window.setTimeout(connect, Math.min(30_000, 1_000 * 2 ** retry.current++));
      };
    };

    connect();
    const resync = () => void sync();
    window.addEventListener('focus', resync);
    window.addEventListener('hubvas:notification-read', resync);
    return () => {
      closed = true;
      if (timer !== undefined) window.clearTimeout(timer);
      ws?.close();
      window.removeEventListener('focus', resync);
      window.removeEventListener('hubvas:notification-read', resync);
    };
  }, [enabled, sync]);

  return { unread, sync, setUnread };
}

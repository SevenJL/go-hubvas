import { useState, useRef, useEffect } from 'react';
import { Users, Wifi, WifiOff } from 'lucide-react';
import type { PresenceMember } from '../../types';
import { useI18n } from '../../i18n';
import { Avatar } from '../ui';

interface OnlineUsersProps {
  users: PresenceMember[];
  connected: boolean;
  currentUsername: string;
  currentAvatarURL?: string;
}

/** Consistent color for each user ID. */
function userColor(userId: string): string {
  let hash = 0;
  for (let i = 0; i < userId.length; i++) {
    hash = userId.charCodeAt(i) + ((hash << 5) - hash);
  }
  const colors = [
    '#6366f1', '#ec4899', '#f59e0b', '#10b981',
    '#3b82f6', '#8b5cf6', '#ef4444', '#06b6d4',
  ];
  return colors[Math.abs(hash) % colors.length];
}

export function OnlineUsers({ users, connected, currentUsername, currentAvatarURL }: OnlineUsersProps) {
  const [open, setOpen] = useState(false);
  const { t } = useI18n();
  const ref = useRef<HTMLDivElement>(null);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const total = users.length + 1; // +1 for current user

  return (
    <div className="relative" ref={ref}>
      {/* Trigger */}
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 text-xs text-gray-600 hover:text-gray-900 transition-colors px-2 py-1 rounded-md hover:bg-gray-100"
        title={t('{count} online', { count: total })}
      >
        {connected ? (
          <Wifi size={12} className="text-green-500" />
        ) : (
          <WifiOff size={12} className="text-red-500" />
        )}
        <Users size={14} />
        <span className="font-medium tabular-nums">{total}</span>
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute right-0 top-full mt-2 w-56 bg-white rounded-xl border border-gray-200 shadow-lg z-50 overflow-hidden">
          {/* Header */}
          <div className="px-3 py-2 border-b border-gray-100">
            <span className="text-xs font-medium text-gray-500">
              {t('{count} users online', { count: total })}
            </span>
          </div>

          {/* List */}
          <div className="max-h-64 overflow-y-auto py-1">
            {/* Current user */}
            <div className="flex items-center gap-2.5 px-3 py-2">
<Avatar name={currentUsername || t('You')} src={currentAvatarURL} size="sm" />
              <span className="text-sm text-gray-900 truncate">{currentUsername || t('You')}</span>
              <span className="ml-auto text-[10px] text-gray-400 bg-gray-100 px-1.5 py-0.5 rounded-full">{t('you')}</span>
            </div>

            {/* Divider if there are others */}
            {users.length > 0 && (
              <div className="mx-3 my-1 border-t border-gray-50" />
            )}

            {/* Remote users */}
            {users.map(u => (
              <div key={u.user_id} className="flex items-center gap-2.5 px-3 py-2 hover:bg-gray-50">
                {u.avatar_url ? (
                  <Avatar name={u.username || t('Anonymous')} src={u.avatar_url} size="sm" />
                ) : (
                  <div
                    className="w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-semibold text-white shrink-0"
                    style={{ backgroundColor: userColor(u.user_id) }}
                  >
                    {(u.username || '?')[0]?.toUpperCase()}
                  </div>
                )}
                <span className="text-sm text-gray-700 truncate">{u.username || t('Anonymous')}</span>
                <span className="ml-auto text-[10px] text-gray-400 capitalize">{t(u.role || 'viewer')}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

import { useCallback, useEffect, useState } from 'react';
import { Bell, CheckCheck, GitFork, Heart, MessageCircle, UserPlus } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Layout } from '../components/layout/Layout';
import { Avatar, Button, PageLoader } from '../components/ui';
import { socialService } from '../services/social';
import type { NotificationInfo } from '../types';
import { useI18n } from '../i18n';

const icons = { follow: UserPlus, like: Heart, comment: MessageCircle, reply: MessageCircle, fork: GitFork };

export function Notifications() {
  const [items, setItems] = useState<NotificationInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const { t } = useI18n();
  const load = useCallback(async () => {
    try { setItems((await socialService.notifications()).items); } finally { setLoading(false); }
  }, []);
  useEffect(() => {
    void load();
    const refresh = () => void load();
    window.addEventListener('hubvas:notification', refresh);
    return () => window.removeEventListener('hubvas:notification', refresh);
  }, [load]);
  const copy = (notification: NotificationInfo) => t({
    follow: 'started following you', like: 'liked your canvas', comment: 'commented on your canvas',
    reply: 'replied to your comment', fork: 'forked your canvas',
  }[notification.event_type] || 'sent an update');
  const open = async (notification: NotificationInfo) => {
    if (!notification.read_at) {
      await socialService.read(notification.id);
      setItems(current => current.map(item => item.id === notification.id ? { ...item, read_at: new Date().toISOString() } : item));
      window.dispatchEvent(new Event('hubvas:notification-read'));
    }
    if (notification.target_type === 'canvas') navigate(`/canvas/${notification.target_id}`);
    else if (notification.target_type === 'comment' && notification.data.canvas_id) navigate(`/canvas/${String(notification.data.canvas_id)}`);
    else if (notification.actor) navigate(`/users/${notification.actor.username}`);
  };
  const readAll = async () => {
    await socialService.readAll();
    setItems(current => current.map(item => ({ ...item, read_at: item.read_at || new Date().toISOString() })));
    window.dispatchEvent(new Event('hubvas:notification-read'));
  };
  return <Layout><main className="mx-auto max-w-3xl px-4 py-10">
    <div className="mb-7 flex items-center justify-between"><div><p className="text-sm font-semibold text-indigo-600">{t('INBOX')}</p><h1 className="text-3xl font-bold text-slate-950">{t('Notifications')}</h1></div><Button variant="secondary" onClick={() => void readAll()}><CheckCheck size={16} />{t('Mark all read')}</Button></div>
    {loading ? <PageLoader label={t('Loading notifications...')} /> : items.length ? <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">{items.map(notification => { const Icon = icons[notification.event_type] || Bell; return <button key={notification.id} onClick={() => void open(notification)} className={`flex w-full items-center gap-4 border-b border-slate-100 p-5 text-left last:border-0 hover:bg-slate-50 ${!notification.read_at ? 'bg-indigo-50/50' : ''}`}><div className="relative"><Avatar name={notification.actor?.display_name || 'Hubvas'} src={notification.actor?.avatar_url} /><span className="absolute -bottom-1 -right-1 grid h-5 w-5 place-items-center rounded-full bg-white text-indigo-600 shadow"><Icon size={12} /></span></div><div className="flex-1"><p className="text-sm text-slate-700"><strong>{notification.actor?.display_name || notification.actor?.username || t('Someone')}</strong> {copy(notification)}</p><p className="mt-1 text-xs text-slate-400">{new Date(notification.created_at).toLocaleString()}</p></div>{!notification.read_at && <span className="h-2.5 w-2.5 rounded-full bg-indigo-500" />}</button>; })}</div> : <div className="rounded-2xl border border-dashed border-slate-300 p-14 text-center text-slate-500"><Bell className="mx-auto mb-3" />{t('You are all caught up.')}</div>}
  </main></Layout>;
}

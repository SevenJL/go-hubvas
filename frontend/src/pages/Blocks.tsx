import { useCallback, useEffect, useState } from 'react';
import { Layout } from '../components/layout/Layout';
import { Avatar, Button, PageLoader } from '../components/ui';
import { socialService } from '../services/social';
import type { Actor } from '../types';
import { useI18n } from '../i18n';

export function Blocks() {
  const [items, setItems] = useState<Actor[]>([]);
  const [loading, setLoading] = useState(true);
  const { t } = useI18n();
  const load = useCallback(async () => {
    setLoading(true);
    try { setItems((await socialService.blocks()).items); } finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  return <Layout><main className="mx-auto max-w-3xl px-4 py-10">
    <h1 className="text-3xl font-bold text-slate-950">{t('Blocked accounts')}</h1>
    <p className="mt-2 text-slate-500">{t('Blocked people cannot follow or interact with your work.')}</p>
    {loading ? <PageLoader label={t('Loading blocked accounts...')} /> : <div className="mt-7 rounded-2xl border border-slate-200 bg-white">{items.length ? items.map(actor => <div key={actor.id} className="flex items-center gap-3 border-b border-slate-100 p-4 last:border-0"><Avatar name={actor.display_name || actor.username} src={actor.avatar_url} /><div className="flex-1"><p className="font-medium text-slate-900">{actor.display_name || actor.username}</p><p className="text-xs text-slate-400">@{actor.username}</p></div><Button variant="secondary" onClick={async () => { await socialService.unblock(actor.id); void load(); }}>{t('Unblock')}</Button></div>) : <p className="p-12 text-center text-slate-500">{t('You have not blocked anyone.')}</p>}</div>}
  </main></Layout>;
}

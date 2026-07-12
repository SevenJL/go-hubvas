import { useCallback, useEffect, useState } from 'react';
import { Compass, Search, Users } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { FeedCard } from '../components/community/FeedCard';
import { Button, ErrorState, PageLoader } from '../components/ui';
import { communityService } from '../services/community';
import { socialService } from '../services/social';
import { useAuth } from '../store/AuthContext';
import type { FeedResponse } from '../types';
import { useI18n } from '../i18n';

export function Community() {
  const { user } = useAuth();
  const { t } = useI18n();
  const [tab, setTab] = useState<'discover' | 'following'>('discover');
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState('latest');
  const [page, setPage] = useState(1);
  const [feed, setFeed] = useState<FeedResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try { setFeed(tab === 'following' ? await socialService.followingFeed(page) : await communityService.browse({ q: query, sort_by: sort, page, page_size: 12 })); }
    catch (loadError) { setError(loadError instanceof Error ? loadError.message : t('Could not load feed')); }
    finally { setLoading(false); }
  }, [tab, page, query, sort, t]);
  useEffect(() => {
    if (tab === 'following' && !user) { setLoading(false); setFeed(null); return; }
    void load();
  }, [load, tab, user]);
  return <Layout><main className="mx-auto max-w-7xl px-4 py-10">
    <div className="flex flex-col justify-between gap-5 md:flex-row md:items-end"><div><p className="text-sm font-semibold text-indigo-600">{t('COMMUNITY')}</p><h1 className="mt-1 text-4xl font-bold tracking-tight text-slate-950">{t('Ideas made visible.')}</h1><p className="mt-2 text-slate-500">{t('Discover public canvases or catch up with creators you follow.')}</p></div><div className="flex rounded-xl bg-slate-100 p-1"><button onClick={() => { setTab('discover'); setPage(1); }} className={`flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium ${tab === 'discover' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500'}`}><Compass size={16} />{t('Discover')}</button><button onClick={() => { setTab('following'); setPage(1); }} className={`flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium ${tab === 'following' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500'}`}><Users size={16} />{t('Following')}</button></div></div>
    {tab === 'discover' && <div className="mt-8 flex gap-3"><div className="relative flex-1"><Search className="absolute left-4 top-3.5 text-slate-400" size={18} /><input value={query} onChange={event => setQuery(event.target.value)} placeholder={t('Search canvases')} className="w-full rounded-xl border border-slate-300 py-3 pl-11 pr-4 outline-none focus:border-indigo-500" /></div><select value={sort} onChange={event => setSort(event.target.value)} className="rounded-xl border border-slate-300 bg-white px-4"><option value="latest">{t('Latest')}</option><option value="popular">{t('Popular')}</option><option value="trending">{t('Trending')}</option></select></div>}
    <div className="mt-8">{tab === 'following' && !user ? <div className="rounded-3xl bg-slate-950 p-12 text-center text-white"><Users className="mx-auto mb-4" /><h2 className="text-2xl font-bold">{t('Your following feed starts after sign in')}</h2><p className="mt-2 text-slate-400">{t('Follow creators and see their newest public work here.')}</p><Button className="mt-6" onClick={() => location.assign('/login')}>{t('Sign in')}</Button></div> : loading ? <PageLoader label={t('Loading community...')} /> : error ? <ErrorState title={t('Could not load community')} message={error} onRetry={() => void load()} /> : feed?.items.length ? <><div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{feed.items.map(item => <FeedCard key={item.canvas_id} item={item} />)}</div><div className="mt-8 flex justify-center gap-3"><Button variant="secondary" disabled={page <= 1} onClick={() => setPage(current => current - 1)}>{t('Previous')}</Button><span className="px-3 py-2 text-sm text-slate-500">{t('Page {page}', { page })}</span><Button variant="secondary" disabled={page * 12 >= (feed.total_count || 0)} onClick={() => setPage(current => current + 1)}>{t('Next')}</Button></div></> : <div className="rounded-2xl border border-dashed border-slate-300 p-14 text-center text-slate-500">{t('Nothing here yet.')}</div>}</div>
  </main></Layout>;
}

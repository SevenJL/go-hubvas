import { useCallback, useEffect, useState } from 'react';
import { Compass, Search, Users } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { FeedCard } from '../components/community/FeedCard';
import { Button, ErrorState, PageLoader } from '../components/ui';
import { communityService } from '../services/community';
import { socialService } from '../services/social';
import { useAuth } from '../store/authStore';
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
  return <Layout><main className="community-page"><div className="community-inner">
    <div className="community-hero"><div><p className="community-kicker">{t('COMMUNITY')}</p><h1 className="community-title">{t('Ideas made visible.')}</h1><p className="community-subtitle">{t('Discover public canvases or catch up with creators you follow.')}</p></div><div className="community-tabs"><button onClick={() => { setTab('discover'); setPage(1); }} className={`community-tab ${tab === 'discover' ? 'active' : ''}`}><Compass size={16} />{t('Discover')}</button><button onClick={() => { setTab('following'); setPage(1); }} className={`community-tab ${tab === 'following' ? 'active' : ''}`}><Users size={16} />{t('Following')}</button></div></div>
    {tab === 'discover' && <div className="community-toolbar"><div className="community-search"><Search className="community-search-icon" size={18} /><input value={query} onChange={event => setQuery(event.target.value)} placeholder={t('Search canvases')} className="community-search-input" /></div><select value={sort} onChange={event => setSort(event.target.value)} className="community-sort"><option value="latest">{t('Latest')}</option><option value="popular">{t('Popular')}</option><option value="trending">{t('Trending')}</option></select></div>}
    <div>{tab === 'following' && !user ? <div className="community-empty community-empty-auth"><Users className="community-empty-icon" /><h2 className="community-empty-title">{t('Your following feed starts after sign in')}</h2><p className="community-empty-copy">{t('Follow creators and see their newest public work here.')}</p><Button className="community-empty-action" onClick={() => location.assign('/login')}>{t('Sign in')}</Button></div> : loading ? <PageLoader label={t('Loading community...')} /> : error ? <div className="community-error"><ErrorState title={t('Could not load community')} message={error} onRetry={() => void load()} /></div> : feed?.items.length ? <><div className="community-grid">{feed.items.map((item, index) => <FeedCard key={item.canvas_id} item={item} index={index} />)}</div><div className="community-pagination"><Button variant="secondary" disabled={page <= 1} onClick={() => setPage(current => current - 1)}>{t('Previous')}</Button><span className="px-3 py-2 text-sm text-slate-500">{t('Page {page}', { page })}</span><Button variant="secondary" disabled={page * 12 >= (feed.total_count || 0)} onClick={() => setPage(current => current + 1)}>{t('Next')}</Button></div></> : <div className="community-empty text-slate-500">{t('Nothing here yet.')}</div>}</div>
  </div></main></Layout>;
}

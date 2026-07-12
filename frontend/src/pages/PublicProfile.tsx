import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Ban, Flag, Globe2, UserMinus, UserPlus, X } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { Avatar, Button, ErrorState, PageLoader, useToast } from '../components/ui';
import { FeedCard } from '../components/community/FeedCard';
import { socialService } from '../services/social';
import { useAuth } from '../store/authStore';
import type { Actor, PublicProfile, PublishedCanvas } from '../types';
import { useI18n } from '../i18n';

type RelationKind = 'followers' | 'following';

export function PublicProfilePage() {
  const { username = '' } = useParams();
  const { user } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const { t } = useI18n();
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [items, setItems] = useState<PublishedCanvas[]>([]);
  const [loading, setLoading] = useState(true);
  const [relations, setRelations] = useState<{ kind: RelationKind; items: Actor[] } | null>(null);
  const [relationLoading, setRelationLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [nextProfile, canvases] = await Promise.all([socialService.profile(username), socialService.canvases(username)]);
      setProfile(nextProfile);
      setItems(canvases.items);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Unable to load profile'));
    } finally { setLoading(false); }
  }, [username, toast, t]);
  useEffect(() => { void load(); }, [load]);

  if (loading) return <Layout><PageLoader label={t('Loading profile...')} /></Layout>;
  if (!profile) return <Layout><ErrorState title={t('Profile unavailable')} message={t('This user does not exist or is unavailable.')} /></Layout>;

  const requireAuth = () => { if (user) return true; navigate('/login'); return false; };
  const follow = async () => {
    if (!requireAuth()) return;
    try {
      if (profile.is_following) await socialService.unfollow(profile.id); else await socialService.follow(profile.id);
      await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : t('Could not update follow')); }
  };
  const block = async () => {
    if (!requireAuth()) return;
    try {
      if (profile.is_blocked) await socialService.unblock(profile.id); else await socialService.block(profile.id);
      toast.success(profile.is_blocked ? t('User unblocked') : t('User blocked'));
      await load();
    } catch (error) { toast.error(error instanceof Error ? error.message : t('Could not update block')); }
  };
  const report = async () => {
    if (!requireAuth()) return;
    const details = window.prompt(t('Describe the issue (optional)')) || '';
    try { await socialService.report('user', profile.id, 'other', details); toast.success(t('Report submitted')); }
    catch (error) { toast.error(error instanceof Error ? error.message : t('Could not submit report')); }
  };
  const showRelations = async (kind: RelationKind) => {
    if (!requireAuth()) return;
    setRelations({ kind, items: [] });
    setRelationLoading(true);
    try {
      const page = kind === 'followers' ? await socialService.followers(profile.id) : await socialService.following(profile.id);
      setRelations({ kind, items: page.items });
    } catch (error) {
      setRelations(null);
      toast.error(error instanceof Error ? error.message : t('Could not load relationships'));
    } finally { setRelationLoading(false); }
  };

  return <Layout><main>
    <section className="border-b border-slate-200 bg-slate-950 text-white"><div className="mx-auto max-w-6xl px-4 py-12"><div className="flex flex-col gap-7 sm:flex-row sm:items-end">
      <Avatar size="xl" name={profile.display_name || profile.username} src={profile.avatar_url} className="ring-4 ring-white/15" />
      <div className="flex-1"><p className="text-sm text-indigo-300">@{profile.username}</p><h1 className="mt-1 text-4xl font-bold">{profile.display_name || profile.username}</h1><p className="mt-3 max-w-2xl text-slate-300">{profile.bio || t('This creator has not added a bio yet.')}</p>{profile.website && <a className="mt-3 inline-flex items-center gap-2 text-sm text-indigo-300 hover:text-white" href={profile.website} target="_blank" rel="noreferrer"><Globe2 size={15} />{profile.website}</a>}
        <div className="mt-5 flex gap-6 text-sm"><button onClick={() => void showRelations('followers')} className="text-left hover:text-indigo-200"><strong className="text-lg">{profile.followers_count}</strong> {t('followers')}</button><button onClick={() => void showRelations('following')} className="text-left hover:text-indigo-200"><strong className="text-lg">{profile.following_count}</strong> {t('following')}</button><span><strong className="text-lg">{profile.published_count}</strong> {t('works')}</span></div>
      </div>
      {user?.id !== profile.id && <div className="flex flex-wrap gap-2"><Button onClick={() => void follow()} disabled={profile.is_blocked || profile.is_blocked_by}>{profile.is_following ? <><UserMinus size={16} />{t('Unfollow')}</> : <><UserPlus size={16} />{t('Follow')}</>}</Button><Button variant="secondary" onClick={() => void block()}><Ban size={16} />{profile.is_blocked ? t('Unblock') : t('Block')}</Button><Button variant="ghost" className="text-slate-300" onClick={() => void report()} title={t('Report user')}><Flag size={16} /></Button></div>}
    </div></div></section>
    <section className="mx-auto max-w-6xl px-4 py-10"><h2 className="mb-6 text-xl font-bold text-slate-950">{t('Published canvases')}</h2>{items.length ? <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">{items.map(item => <FeedCard key={item.canvas_id} item={item} />)}</div> : <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500">{t('No public canvases yet.')}</div>}</section>

    {relations && <div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/60 p-4" role="dialog" aria-modal="true"><div className="w-full max-w-md rounded-3xl bg-white p-6 shadow-2xl"><div className="flex items-center justify-between"><h2 className="text-xl font-bold text-slate-950">{t(relations.kind === 'followers' ? 'Followers' : 'Following')}</h2><button onClick={() => setRelations(null)} className="rounded-full p-2 text-slate-400 hover:bg-slate-100"><X size={18} /></button></div>{relationLoading ? <PageLoader label={t('Loading relationships...')} /> : relations.items.length ? <div className="mt-5 max-h-96 divide-y divide-slate-100 overflow-y-auto">{relations.items.map(actor => <button key={actor.id} onClick={() => { setRelations(null); navigate(`/users/${actor.username}`); }} className="flex w-full items-center gap-3 py-3 text-left"><Avatar name={actor.display_name || actor.username} src={actor.avatar_url} /><div><p className="font-medium text-slate-900">{actor.display_name || actor.username}</p><p className="text-xs text-slate-400">@{actor.username}</p></div></button>)}</div> : <p className="py-10 text-center text-slate-500">{t('No users to show.')}</p>}</div></div>}
  </main></Layout>;
}

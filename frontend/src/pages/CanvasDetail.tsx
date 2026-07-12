import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Flag, GitFork, Heart, MessageCircle, PencilLine, Reply, Send, Trash2 } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { canvasService } from '../services/canvas';
import { communityService, CommunityRequestError } from '../services/community';
import { socialService } from '../services/social';
import { useAuth } from '../store/authStore';
import { CanvasThumbnail } from '../components/canvas/CanvasThumbnail';
import { Avatar, Button, ErrorState, InlineLoader, useToast } from '../components/ui';
import type { CanvasInfo, CommentInfo, PublishedCanvas } from '../types';
import { useI18n } from '../i18n';

export function CanvasDetail() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const { language, t } = useI18n();
  const canvasId = id!;
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [published, setPublished] = useState<PublishedCanvas | null>(null);
  const [comments, setComments] = useState<CommentInfo[]>([]);
  const [commentPage, setCommentPage] = useState(1);
  const [commentTotal, setCommentTotal] = useState(0);
  const [loadingMoreComments, setLoadingMoreComments] = useState(false);
  const [newComment, setNewComment] = useState('');
  const [replyTo, setReplyTo] = useState<string | undefined>();
  const [liked, setLiked] = useState(false);
  const [likeCount, setLikeCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [liking, setLiking] = useState(false);
  const [commenting, setCommenting] = useState(false);
  const [forking, setForking] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [canvasInfo, detail, commentPage] = await Promise.all([
        canvasService.get(canvasId),
        communityService.getPublished(canvasId),
        communityService.getComments(canvasId),
      ]);
      setCanvas(canvasInfo);
      setPublished(detail);
      setLiked(detail.is_liked);
      setLikeCount(detail.like_count);
      setComments(commentPage.items);
      setCommentPage(1);
      setCommentTotal(commentPage.total);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : t('This canvas could not be loaded.'));
    } finally {
      setLoading(false);
    }
  }, [canvasId, t]);

  useEffect(() => { void load(); }, [load, user?.id]);

  useEffect(() => {
    setReplyTo(undefined);
    setNewComment('');
  }, [canvasId]);

  useEffect(() => {
    if (!replyTo) return;
    const target = comments.find(comment => comment.id === replyTo);
    if (target && !target.deleted && target.moderation_status === 'visible') return;
    setReplyTo(undefined);
    setNewComment('');
  }, [comments, replyTo]);

  const requireLogin = () => {
    if (user) return true;
    toast.info({ title: t('Sign in required'), message: t('Sign in to interact with community canvases.') });
    navigate('/login');
    return false;
  };

  const handleLike = async () => {
    if (!requireLogin() || liking) return;
    setLiking(true);
    try {
      const status = liked ? await communityService.unlike(canvasId) : await communityService.like(canvasId);
      setLiked(status.liked);
      setLikeCount(status.like_count);
    } catch (err) {
      toast.error({ title: t('Could not update like'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setLiking(false);
    }
  };

  const handleComment = async (event: FormEvent) => {
    event.preventDefault();
    if (!newComment.trim() || !requireLogin() || commenting) return;
    setCommenting(true);
    try {
      const comment = await communityService.postComment(canvasId, newComment.trim(), replyTo);
      setComments(prev => [comment, ...prev]);
      setNewComment('');
      setReplyTo(undefined);
      toast.success(t('Comment posted'));
    } catch (err) {
      const unavailableReply = Boolean(replyTo && err instanceof CommunityRequestError && (err.status === 404 || err.status === 409));
      if (unavailableReply) {
        setReplyTo(undefined);
        setNewComment('');
      }
      toast.error({
        title: t('Could not post comment'),
        message: unavailableReply
          ? t('The comment you were replying to is no longer available. Please select another comment.')
          : err instanceof Error ? err.message : t('Please try again.'),
      });
    } finally {
      setCommenting(false);
    }
  };

  const loadMoreComments = async () => {
    if (loadingMoreComments) return;
    setLoadingMoreComments(true);
    try {
      const nextPage = commentPage + 1;
      const result = await communityService.getComments(canvasId, nextPage);
      setComments(current => {
        const known = new Set(current.map(comment => comment.id));
        return [...current, ...result.items.filter(comment => !known.has(comment.id))];
      });
      setCommentPage(nextPage);
      setCommentTotal(result.total);
    } catch (err) {
      toast.error({ title: t('Could not load more comments'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setLoadingMoreComments(false);
    }
  };

  const handleReportCanvas = async () => {
    if (!requireLogin()) return;
    const details = window.prompt(t('Describe the issue (optional)')) || '';
    try {
      await socialService.report('canvas', canvasId, 'other', details);
      toast.success(t('Report submitted'));
    } catch (err) {
      toast.error({ title: t('Could not submit report'), message: err instanceof Error ? err.message : t('Please try again.') });
    }
  };

  const ownsCanvas = Boolean(user && canvas && user.id === canvas.owner_id);
  const canEditCanvas = Boolean(ownsCanvas || canvas?.current_role === 'owner' || canvas?.current_role === 'editor');
  // Published canvases can be opened by any signed-in user; the editor enforces read-only mode for non-editors.
  const canOpenCanvas = Boolean(user);

  const handleFork = async () => {
    if (!requireLogin() || forking) return;
    setForking(true);
    try {
      const forked = await canvasService.fork(canvasId);
      toast.success({ title: t('Fork created'), message: t('Opening your editable copy.') });
      navigate(`/canvas/${forked.id}/edit`);
    } catch (err) {
      toast.error({ title: t('Fork failed'), message: err instanceof Error ? err.message : t('Please try again.') });
      setForking(false);
    }
  };

  return (
    <Layout>
      <div className="mx-auto max-w-4xl px-4 py-8">
        <button onClick={() => navigate(-1)} className="mb-4 flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800">
          <ArrowLeft size={16} /> {t('Back')}
        </button>

        {loading ? (
          <div className="rounded-2xl border border-slate-200 bg-white"><div className="skeleton aspect-video rounded-t-2xl" /><InlineLoader label={t('Loading canvas and discussion...')} /></div>
        ) : loadError || !canvas ? (
          <ErrorState title={t('Canvas unavailable')} message={loadError || t('The canvas may have been removed or made private.')} onRetry={() => void load()} />
        ) : (
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <CanvasThumbnail canvasId={canvas.id} />
            <div className="p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0">
                  <h1 className="break-words text-2xl font-bold text-slate-900">{canvas.title}</h1>
                  <p className="mt-1 text-sm text-slate-500">{t('by {author}', { author: published?.author_name || t('User #{id}', { id: canvas.owner_id }) })} · {t('{count} members', { count: canvas.member_count })}</p>
                </div>
                {canOpenCanvas && (
                  <Button
                    className="w-full shrink-0 sm:w-auto"
                    onClick={() => navigate(`/canvas/${canvas.id}/edit`)}
                  >
                    <PencilLine size={16} />
                    {t(canEditCanvas ? 'Edit canvas' : 'View canvas')}
                  </Button>
                )}
              </div>

              <div className="mt-4 flex flex-wrap items-center gap-3 border-b border-slate-100 pb-4">
                <button disabled={liking} onClick={() => void handleLike()} className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm transition-colors disabled:opacity-50 ${liked ? 'bg-red-50 text-red-600' : 'text-slate-500 hover:bg-slate-100'}`}>
                  <Heart size={16} fill={liked ? 'currentColor' : 'none'} /> {likeCount}
                </button>
                <span className="flex items-center gap-1.5 text-sm text-slate-500"><MessageCircle size={16} /> {commentTotal}</span>
                <Button variant="ghost" size="sm" loading={forking} onClick={() => void handleFork()}><GitFork size={16} /> {t('Fork')}</Button>{user?.id !== canvas.owner_id && <Button variant="ghost" size="sm" onClick={() => void handleReportCanvas()}><Flag size={16} /> {t('Report')}</Button>}
              </div>

              <div className="mt-5">
                <h3 className="mb-3 font-semibold text-slate-900">{t('Comments ({count})', { count: commentTotal })}</h3>
                {user ? (
                  <form onSubmit={handleComment} className="mb-5">
                    {replyTo && <div className="mb-2 flex items-center justify-between rounded-lg bg-indigo-50 px-3 py-2 text-xs text-indigo-700"><span>{t('Replying to a comment')}</span><button type="button" onClick={() => setReplyTo(undefined)}>{t('Cancel')}</button></div>}
                    <div className="flex gap-2">
                    <input type="text" className="input-field flex-1" placeholder={t('Write a comment...')} value={newComment} onChange={event => setNewComment(event.target.value)} maxLength={5000} />
                    <Button type="submit" loading={commenting} disabled={!newComment.trim()} aria-label={t('Post comment')}><Send size={16} /></Button>
                    </div>
                  </form>
                ) : (
                  <button onClick={() => navigate('/login')} className="mb-5 w-full rounded-xl border border-dashed border-slate-300 px-4 py-3 text-sm text-slate-500 hover:border-indigo-300 hover:bg-indigo-50/50 hover:text-indigo-600">{t('Sign in to join the discussion')}</button>
                )}

                {comments.length === 0 ? (
                  <p className="rounded-xl bg-slate-50 py-8 text-center text-sm text-slate-400">{t('No comments yet. Start the conversation.')}</p>
                ) : (
                  <div className="space-y-5">
                    {comments.filter(comment => !comment.parent_comment_id).map(comment => {
                      const replies = comments.filter(reply => reply.parent_comment_id === comment.id);
                      const renderComment = (entry: CommentInfo, nested = false) => (
                        <div key={entry.id} className={`flex gap-3 ${nested ? 'ml-10 border-l-2 border-slate-100 pl-4' : ''}`}>
                          <Avatar name={entry.author_name || entry.author_username} src={entry.author_avatar_url} size="sm" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2"><button onClick={() => navigate(`/users/${entry.author_username}`)} className="text-sm font-medium text-slate-900 hover:text-indigo-600">{entry.author_name || entry.author_username}</button><span className="text-xs text-slate-400">{new Date(entry.created_at * 1000).toLocaleDateString(language === 'zh' ? 'zh-CN' : 'en-US')}</span></div>
                            <p className={`mt-1 break-words text-sm ${entry.deleted || entry.moderation_status === 'hidden' ? 'italic text-slate-400' : 'text-slate-600'}`}>{entry.deleted || entry.moderation_status === 'hidden' ? t('Comment deleted') : entry.content}</p>
                            {user && !entry.deleted && entry.moderation_status === 'visible' && <div className="mt-2 flex gap-3 text-xs text-slate-400">{!nested && <button onClick={() => { setReplyTo(entry.id); setNewComment(`@${entry.author_username} `); }} className="flex items-center gap-1 hover:text-indigo-600"><Reply size={12}/>{t('Reply')}</button>}{user.id === entry.author_id ? <button onClick={async () => { await communityService.deleteComment(entry.id); await load(); }} className="flex items-center gap-1 hover:text-rose-600"><Trash2 size={12}/>{t('Delete')}</button> : <button onClick={async () => { await socialService.report('comment', entry.id, 'other'); toast.success(t('Report submitted')); }} className="flex items-center gap-1 hover:text-rose-600"><Flag size={12}/>{t('Report')}</button>}</div>}
                          </div>
                        </div>
                      );
                      return <div key={comment.id} className="space-y-4">{renderComment(comment)}{replies.map(reply => renderComment(reply, true))}</div>;
                    })}
                    {comments.filter(comment => !comment.parent_comment_id).length < commentTotal && <div className="flex justify-center pt-2"><Button variant="secondary" loading={loadingMoreComments} onClick={() => void loadMoreComments()}>{t('Load more comments')}</Button></div>}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}

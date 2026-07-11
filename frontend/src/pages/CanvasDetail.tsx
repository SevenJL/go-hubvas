import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, GitFork, Heart, MessageCircle, Send } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { canvasService } from '../services/canvas';
import { communityService } from '../services/community';
import { useAuth } from '../store/AuthContext';
import { CanvasThumbnail } from '../components/canvas/CanvasThumbnail';
import { Avatar, Button, ErrorState, InlineLoader, useToast } from '../components/ui';
import type { CanvasInfo, CommentInfo, PublishedCanvas } from '../types';

export function CanvasDetail() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const canvasId = id!;
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [published, setPublished] = useState<PublishedCanvas | null>(null);
  const [comments, setComments] = useState<CommentInfo[]>([]);
  const [newComment, setNewComment] = useState('');
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
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : 'This canvas could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [canvasId]);

  useEffect(() => { void load(); }, [load, user?.id]);

  const requireLogin = () => {
    if (user) return true;
    toast.info({ title: 'Sign in required', message: 'Sign in to interact with community canvases.' });
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
      toast.error({ title: 'Could not update like', message: err instanceof Error ? err.message : 'Please try again.' });
    } finally {
      setLiking(false);
    }
  };

  const handleComment = async (event: FormEvent) => {
    event.preventDefault();
    if (!newComment.trim() || !requireLogin() || commenting) return;
    setCommenting(true);
    try {
      const comment = await communityService.postComment(canvasId, newComment.trim());
      setComments(prev => [comment, ...prev]);
      setNewComment('');
      toast.success('Comment posted');
    } catch (err) {
      toast.error({ title: 'Could not post comment', message: err instanceof Error ? err.message : 'Please try again.' });
    } finally {
      setCommenting(false);
    }
  };

  const handleFork = async () => {
    if (!requireLogin() || forking) return;
    setForking(true);
    try {
      const forked = await canvasService.fork(canvasId);
      toast.success({ title: 'Fork created', message: 'Opening your editable copy.' });
      navigate(`/canvas/${forked.id}/edit`);
    } catch (err) {
      toast.error({ title: 'Fork failed', message: err instanceof Error ? err.message : 'Please try again.' });
      setForking(false);
    }
  };

  return (
    <Layout>
      <div className="mx-auto max-w-4xl px-4 py-8">
        <button onClick={() => navigate(-1)} className="mb-4 flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800">
          <ArrowLeft size={16} /> Back
        </button>

        {loading ? (
          <div className="rounded-2xl border border-slate-200 bg-white"><div className="skeleton aspect-video rounded-t-2xl" /><InlineLoader label="Loading canvas and discussion..." /></div>
        ) : loadError || !canvas ? (
          <ErrorState title="Canvas unavailable" message={loadError || 'The canvas may have been removed or made private.'} onRetry={() => void load()} />
        ) : (
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <CanvasThumbnail canvasId={canvas.id} />
            <div className="p-6">
              <h1 className="text-2xl font-bold text-slate-900">{canvas.title}</h1>
              <p className="mt-1 text-sm text-slate-500">by {published?.author_name || `User #${canvas.owner_id}`} · {canvas.member_count} members</p>

              <div className="mt-4 flex items-center gap-3 border-b border-slate-100 pb-4">
                <button disabled={liking} onClick={() => void handleLike()} className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm transition-colors disabled:opacity-50 ${liked ? 'bg-red-50 text-red-600' : 'text-slate-500 hover:bg-slate-100'}`}>
                  <Heart size={16} fill={liked ? 'currentColor' : 'none'} /> {likeCount}
                </button>
                <span className="flex items-center gap-1.5 text-sm text-slate-500"><MessageCircle size={16} /> {comments.length}</span>
                <Button variant="ghost" size="sm" loading={forking} onClick={() => void handleFork()}><GitFork size={16} /> Fork</Button>
              </div>

              <div className="mt-5">
                <h3 className="mb-3 font-semibold text-slate-900">Comments ({comments.length})</h3>
                {user ? (
                  <form onSubmit={handleComment} className="mb-5 flex gap-2">
                    <input type="text" className="input-field flex-1" placeholder="Write a comment..." value={newComment} onChange={event => setNewComment(event.target.value)} maxLength={5000} />
                    <Button type="submit" loading={commenting} disabled={!newComment.trim()} aria-label="Post comment"><Send size={16} /></Button>
                  </form>
                ) : (
                  <button onClick={() => navigate('/login')} className="mb-5 w-full rounded-xl border border-dashed border-slate-300 px-4 py-3 text-sm text-slate-500 hover:border-indigo-300 hover:bg-indigo-50/50 hover:text-indigo-600">Sign in to join the discussion</button>
                )}

                {comments.length === 0 ? (
                  <p className="rounded-xl bg-slate-50 py-8 text-center text-sm text-slate-400">No comments yet. Start the conversation.</p>
                ) : (
                  <div className="space-y-4">
                    {comments.map(comment => (
                      <div key={comment.id} className="flex gap-3">
                        <Avatar name={comment.author_name || `User #${comment.author_id}`} size="sm" />
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-slate-900">{comment.author_name || `User #${comment.author_id}`}</span>
                            <span className="text-xs text-slate-400">{new Date(comment.created_at * 1000).toLocaleDateString()}</span>
                          </div>
                          <p className="mt-0.5 break-words text-sm text-slate-600">{comment.content}</p>
                        </div>
                      </div>
                    ))}
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

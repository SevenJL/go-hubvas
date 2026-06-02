import { useEffect, useState, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Layout } from '../components/layout/Layout';
import { canvasService } from '../services/canvas';
import { communityService } from '../services/community';
import { useAuth } from '../store/AuthContext';
import type { CanvasInfo, CommentInfo } from '../types';
import { Heart, MessageCircle, GitFork, Send, ArrowLeft } from 'lucide-react';

export function CanvasDetail() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const navigate = useNavigate();
  const canvasId = Number(id);
  const [canvas, setCanvas] = useState<CanvasInfo | null>(null);
  const [comments, setComments] = useState<CommentInfo[]>([]);
  const [newComment, setNewComment] = useState('');
  const [liked, setLiked] = useState(false);
  const [likeCount, setLikeCount] = useState(0);

  useEffect(() => {
    canvasService.get(canvasId).then(setCanvas);
    communityService.getComments(canvasId).then(res => setComments(res.items));
  }, [canvasId]);

  const handleLike = async () => {
    if (!user) return;
    try {
      if (liked) {
        await communityService.unlike(canvasId);
        setLiked(false);
        setLikeCount(c => c - 1);
      } else {
        await communityService.like(canvasId);
        setLiked(true);
        setLikeCount(c => c + 1);
      }
    } catch { /* ignore */ }
  };

  const handleComment = async (e: FormEvent) => {
    e.preventDefault();
    if (!newComment.trim() || !user) return;
    try {
      const c = await communityService.postComment(canvasId, newComment.trim());
      setComments(prev => [c, ...prev]);
      setNewComment('');
    } catch { /* ignore */ }
  };

  const handleFork = async () => {
    if (!user) return navigate('/login');
    try {
      const c = await canvasService.fork(canvasId);
      navigate(`/canvas/${c.id}`);
    } catch { /* ignore */ }
  };

  if (!canvas) {
    return (
      <Layout>
        <div className="max-w-5xl mx-auto px-4 py-8 text-center text-gray-400">Loading...</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-4xl mx-auto px-4 py-8">
        <button onClick={() => navigate(-1)} className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 mb-4">
          <ArrowLeft size={16} /> Back
        </button>

        <div className="card overflow-hidden">
          {/* Canvas preview */}
          <div className="aspect-video bg-gradient-to-br from-indigo-100 via-purple-100 to-pink-100 flex items-center justify-center">
            <div className="text-center">
              <span className="text-6xl">🎨</span>
              <p className="text-gray-400 text-sm mt-2">Canvas Preview</p>
            </div>
          </div>

          <div className="p-6">
            <h1 className="text-2xl font-bold text-gray-900">{canvas.title}</h1>
            <p className="text-sm text-gray-500 mt-1">
              by User #{canvas.owner_id} · {canvas.member_count} members
            </p>

            {/* Actions */}
            <div className="flex items-center gap-4 mt-4 pb-4 border-b border-gray-100">
              <button
                onClick={handleLike}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${
                  liked ? 'bg-red-50 text-red-600' : 'text-gray-500 hover:bg-gray-100'
                }`}
              >
                <Heart size={16} fill={liked ? 'currentColor' : 'none'} />
                {likeCount || 0}
              </button>

              <span className="flex items-center gap-1.5 text-sm text-gray-500">
                <MessageCircle size={16} /> {comments.length}
              </span>

              <button
                onClick={handleFork}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm text-gray-500 hover:bg-gray-100 transition-colors"
              >
                <GitFork size={16} /> Fork
              </button>
            </div>

            {/* Comments */}
            <div className="mt-4">
              <h3 className="font-semibold text-gray-900 mb-3">Comments ({comments.length})</h3>

              {user && (
                <form onSubmit={handleComment} className="flex gap-2 mb-4">
                  <input
                    type="text"
                    className="input-field flex-1"
                    placeholder="Write a comment..."
                    value={newComment}
                    onChange={e => setNewComment(e.target.value)}
                  />
                  <button type="submit" className="btn-primary" disabled={!newComment.trim()}>
                    <Send size={16} />
                  </button>
                </form>
              )}

              {comments.length === 0 ? (
                <p className="text-sm text-gray-400">No comments yet.</p>
              ) : (
                <div className="space-y-3">
                  {comments.map(c => (
                    <div key={c.id} className="flex gap-3">
                      <div className="w-8 h-8 rounded-full bg-indigo-100 flex items-center justify-center text-xs font-medium text-indigo-600 shrink-0">
                        {c.author_name?.[0] || 'U'}
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-gray-900">{c.author_name || `User #${c.author_id}`}</span>
                          <span className="text-xs text-gray-400">{new Date(c.created_at * 1000).toLocaleDateString()}</span>
                        </div>
                        <p className="text-sm text-gray-600 mt-0.5">{c.content}</p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}

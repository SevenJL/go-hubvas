import { useState, type FormEvent } from 'react';
import { Send } from 'lucide-react';
import { Avatar } from '../ui/Avatar';
import { Button } from '../ui/Button';
import type { CommentInfo } from '../../types';

interface CommentSectionProps {
  comments: CommentInfo[];
  isLoggedIn: boolean;
  onSubmit: (content: string) => Promise<void>;
}

export function CommentSection({ comments, isLoggedIn, onSubmit }: CommentSectionProps) {
  const [newComment, setNewComment] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!newComment.trim() || submitting) return;

    setSubmitting(true);
    try {
      await onSubmit(newComment.trim());
      setNewComment('');
    } catch {
      // Error handling is done by the parent.
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="mt-4">
      <h3 className="font-semibold text-gray-900 mb-3">Comments ({comments.length})</h3>

      {isLoggedIn && (
        <form onSubmit={handleSubmit} className="flex gap-2 mb-4">
          <input
            type="text"
            className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm
                       placeholder:text-gray-400 focus:outline-none focus:ring-2
                       focus:ring-indigo-500 focus:border-transparent"
            placeholder="Write a comment..."
            value={newComment}
            onChange={e => setNewComment(e.target.value)}
            maxLength={5000}
          />
          <Button type="submit" size="sm" loading={submitting} disabled={!newComment.trim()}>
            <Send size={14} />
          </Button>
        </form>
      )}

      {comments.length === 0 ? (
        <p className="text-sm text-gray-400">No comments yet.</p>
      ) : (
        <div className="space-y-3 max-h-96 overflow-y-auto">
          {comments.map(c => (
            <div key={c.id} className="flex gap-3">
              <Avatar name={c.author_name || `User #${c.author_id}`} size="sm" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-900">
                    {c.author_name || `User #${c.author_id}`}
                  </span>
                  <span className="text-xs text-gray-400">
                    {new Date(c.created_at * 1000).toLocaleDateString()}
                  </span>
                </div>
                <p className="text-sm text-gray-600 mt-0.5 break-words">{c.content}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

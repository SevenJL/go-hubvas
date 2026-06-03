import { useNavigate } from 'react-router-dom';
import { Heart, MessageCircle, GitFork } from 'lucide-react';
import { Card } from '../ui/Card';
import type { PublishedCanvas } from '../../types';

interface FeedCardProps {
  item: PublishedCanvas;
}

export function FeedCard({ item }: FeedCardProps) {
  const navigate = useNavigate();

  return (
    <Card hover className="overflow-hidden" onClick={() => navigate(`/canvas/${item.canvas_id}`)}>
      <div className="aspect-video bg-gradient-to-br from-indigo-100 to-purple-100 flex items-center justify-center">
        <span className="text-3xl">🎨</span>
      </div>
      <div className="p-4">
        <h3 className="font-semibold text-gray-900 truncate">{item.title}</h3>
        <p className="text-xs text-gray-500 mt-0.5">
          by {item.author_name || `User #${item.author_id}`}
        </p>

        {/* Tags */}
        {item.tags && item.tags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-2">
            {item.tags.map(tag => (
              <span
                key={tag}
                className="text-[10px] px-1.5 py-0.5 bg-gray-100 text-gray-600 rounded-full"
              >
                {tag}
              </span>
            ))}
          </div>
        )}

        <div className="flex items-center gap-4 mt-3 text-sm text-gray-500">
          <span className="flex items-center gap-1">
            <Heart size={14} className="text-red-400" /> {item.like_count}
          </span>
          <span className="flex items-center gap-1">
            <MessageCircle size={14} /> {item.comment_count}
          </span>
          <span className="flex items-center gap-1">
            <GitFork size={14} /> {item.fork_count}
          </span>
        </div>
      </div>
    </Card>
  );
}

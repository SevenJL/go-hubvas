import { useNavigate } from 'react-router-dom';
import { Heart, MessageCircle, GitFork } from 'lucide-react';
import { CanvasThumbnail } from '../canvas/CanvasThumbnail';
import { Card } from '../ui/Card';
import type { PublishedCanvas } from '../../types';
import { useI18n } from '../../i18n';

interface FeedCardProps {
  item: PublishedCanvas;
}

export function FeedCard({ item }: FeedCardProps) {
  const navigate = useNavigate();
  const { t } = useI18n();

  return (
    <Card hover className="overflow-hidden" onClick={() => navigate(`/canvas/${item.canvas_id}`)}>
      <CanvasThumbnail canvasId={item.canvas_id} />
      <div className="p-4">
        <h3 className="font-semibold text-gray-900 truncate">{item.title}</h3>
        <p className="text-xs text-gray-500 mt-0.5">
          {t('by {author}', { author: item.author_name || t('User #{id}', { id: item.author_id }) })}
        </p>

        {item.tags && item.tags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-2">
            {item.tags.map(tag => (
              <span key={tag} className="text-[10px] px-1.5 py-0.5 bg-gray-100 text-gray-600 rounded-full">
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

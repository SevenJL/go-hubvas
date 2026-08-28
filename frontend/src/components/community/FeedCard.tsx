import { useNavigate } from 'react-router-dom';
import { GitFork, Heart, MessageCircle } from 'lucide-react';
import { Avatar } from '../ui/Avatar';
import { CanvasThumbnail } from '../canvas/CanvasThumbnail';
import type { PublishedCanvas } from '../../types';

export function FeedCard({ item, index = 0 }: { item: PublishedCanvas; index?: number }) {
  const navigate = useNavigate();
  const authorName = item.author_name || item.author_username;
  const score = item.like_count > 999 ? '999+' : item.like_count;

  return (
    <article className="community-card" style={{ animationDelay: `${index * 60}ms` }}>
      <div className="community-card-preview">
        <span className="community-card-score" aria-label={`${item.like_count} likes`}>{score}</span>
        <button type="button" onClick={() => navigate(`/canvas/${item.canvas_id}`)} aria-label={`Open ${item.title}`}>
          <CanvasThumbnail canvasId={item.canvas_id} />
        </button>
      </div>
      <div className="community-card-body">
        <button type="button" onClick={() => item.author_username && navigate(`/users/${item.author_username}`)} className="community-card-author">
          <Avatar name={authorName} src={item.author_avatar_url} />
          <span><strong>{authorName}</strong><small>@{item.author_username}</small></span>
        </button>
        <button type="button" onClick={() => navigate(`/canvas/${item.canvas_id}`)} className="community-card-title">
          <h2>{item.title}</h2>
          {item.tags?.length > 0 && <div className="community-tags">{item.tags.map(tag => <span key={tag} className="community-tag">#{tag}</span>)}</div>}
          <div className="community-stats">
            <span><Heart size={14} fill={item.is_liked ? 'currentColor' : 'none'} />{item.like_count}</span>
            <span><MessageCircle size={14} />{item.comment_count}</span>
            <span><GitFork size={14} />{item.fork_count}</span>
          </div>
        </button>
      </div>
    </article>
  );
}

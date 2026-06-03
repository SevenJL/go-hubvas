import { Link } from 'react-router-dom';
import { Clock, Globe, Lock, Users } from 'lucide-react';
import { Card } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { CanvasThumbnail } from './CanvasThumbnail';
import type { CanvasInfo } from '../../types';

interface CanvasCardProps {
  canvas: CanvasInfo;
  showActions?: boolean;
  onPublish?: (id: string) => void;
  onFork?: (id: string) => void;
  onDelete?: (id: string) => void;
}

export function CanvasCard({ canvas, showActions, onPublish, onFork, onDelete }: CanvasCardProps) {
  return (
    <Card hover className="group">
      <Link to={`/canvas/${canvas.id}/edit`} className="block">
        <div className="rounded-lg mb-3 overflow-hidden">
          <CanvasThumbnail canvasId={canvas.id} />
        </div>
        <h3 className="font-semibold text-gray-900 truncate mb-2">{canvas.title}</h3>
        <div className="flex items-center gap-3 text-xs text-gray-500">
          <span className="flex items-center gap-1">
            {canvas.visibility === 'published' ? (
              <Globe size={13} className="text-green-500" />
            ) : (
              <Lock size={13} />
            )}
            {canvas.visibility}
          </span>
          <span className="flex items-center gap-1">
            <Users size={13} /> {canvas.member_count}
          </span>
          {canvas.online_count > 0 && (
            <Badge variant="success">{canvas.online_count} online</Badge>
          )}
        </div>
        <div className="flex items-center gap-1.5 mt-2 text-xs text-gray-400">
          <Clock size={12} />
          <span>{new Date(canvas.updated_at).toLocaleDateString()}</span>
        </div>
      </Link>

      {showActions && (
        <div className="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 opacity-0 group-hover:opacity-100 transition-opacity">
          {canvas.visibility !== 'published' && onPublish && (
            <button
              onClick={(e) => { e.preventDefault(); onPublish(canvas.id); }}
              className="text-xs text-indigo-600 hover:underline"
            >
              Publish
            </button>
          )}
          {onFork && (
            <button
              onClick={(e) => { e.preventDefault(); onFork(canvas.id); }}
              className="text-xs text-gray-500 hover:underline"
            >
              Fork
            </button>
          )}
          {onDelete && (
            <button
              onClick={(e) => { e.preventDefault(); onDelete(canvas.id); }}
              className="text-xs text-red-500 hover:underline ml-auto"
            >
              Delete
            </button>
          )}
        </div>
      )}
    </Card>
  );
}

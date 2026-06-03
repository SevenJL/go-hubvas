import { useEffect, useState } from 'react';
import { getAccessToken } from '../../services/api';

interface CanvasThumbnailProps {
  canvasId: string;
}

export function CanvasThumbnail({ canvasId }: CanvasThumbnailProps) {
  const [thumbnail, setThumbnail] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const token = getAccessToken();

    fetch(`/api/canvases/${canvasId}/snapshot`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
      .then(r => r.json())
      .then(body => {
        if (cancelled) return;
        if (body.code === 0 && body.data?.thumbnail) {
          setThumbnail(body.data.thumbnail);
        }
      })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });

    return () => { cancelled = true; };
  }, [canvasId]);

  if (loading) {
    return (
      <div className="aspect-video bg-gradient-to-br from-indigo-50 to-purple-50 flex items-center justify-center">
        <div className="w-6 h-6 border-2 border-indigo-200 border-t-indigo-500 rounded-full animate-spin" />
      </div>
    );
  }

  if (thumbnail) {
    return (
      <img
        src={thumbnail}
        alt="Canvas preview"
        className="aspect-video w-full object-cover bg-white"
      />
    );
  }

  // Fallback: no thumbnail yet
  return (
    <div className="aspect-video bg-gradient-to-br from-indigo-100 to-purple-100 flex items-center justify-center">
      <span className="text-3xl">🎨</span>
    </div>
  );
}

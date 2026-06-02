import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Layout } from '../components/layout/Layout';
import { communityService } from '../services/community';
import type { PublishedCanvas } from '../types';
import { Heart, MessageCircle, GitFork, Search, TrendingUp, Clock } from 'lucide-react';

export function Community() {
  const [items, setItems] = useState<PublishedCanvas[]>([]);
  const [loading, setLoading] = useState(true);
  const [sortBy, setSortBy] = useState('latest');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const navigate = useNavigate();

  const load = async (p = 1) => {
    setLoading(true);
    try {
      const res = await communityService.browse({ sort_by: sortBy, q: keyword || undefined, page: p });
      setItems(res.items);
      setTotal(res.total_count);
      setPage(p);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(1); }, [sortBy]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    load(1);
  };

  const totalPages = Math.ceil(total / 20);

  return (
    <Layout>
      <div className="max-w-5xl mx-auto px-4 py-8">
        <h1 className="text-2xl font-bold text-gray-900 mb-6">Community</h1>

        {/* Search & sort */}
        <div className="flex flex-col sm:flex-row gap-3 mb-6">
          <form onSubmit={handleSearch} className="flex-1 flex gap-2">
            <div className="relative flex-1">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                type="text"
                className="input-field pl-9"
                placeholder="Search canvases..."
                value={keyword}
                onChange={e => setKeyword(e.target.value)}
              />
            </div>
            <button type="submit" className="btn-primary text-sm">Search</button>
          </form>

          <div className="flex gap-1">
            <button
              onClick={() => setSortBy('latest')}
              className={`px-3 py-2 rounded-lg text-sm flex items-center gap-1.5 transition-colors ${
                sortBy === 'latest' ? 'bg-indigo-100 text-indigo-700' : 'text-gray-500 hover:bg-gray-100'
              }`}
            >
              <Clock size={14} /> Latest
            </button>
            <button
              onClick={() => setSortBy('popular')}
              className={`px-3 py-2 rounded-lg text-sm flex items-center gap-1.5 transition-colors ${
                sortBy === 'popular' ? 'bg-indigo-100 text-indigo-700' : 'text-gray-500 hover:bg-gray-100'
              }`}
            >
              <Heart size={14} /> Popular
            </button>
            <button
              onClick={() => setSortBy('trending')}
              className={`px-3 py-2 rounded-lg text-sm flex items-center gap-1.5 transition-colors ${
                sortBy === 'trending' ? 'bg-indigo-100 text-indigo-700' : 'text-gray-500 hover:bg-gray-100'
              }`}
            >
              <TrendingUp size={14} /> Trending
            </button>
          </div>
        </div>

        {loading ? (
          <div className="text-center py-12 text-gray-400">Loading...</div>
        ) : items.length === 0 ? (
          <div className="text-center py-12 card text-gray-400">No published canvases yet</div>
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {items.map(c => (
                <div key={c.canvas_id} className="card overflow-hidden group cursor-pointer"
                     onClick={() => navigate(`/canvas/${c.canvas_id}`)}>
                  <div className="aspect-video bg-gradient-to-br from-indigo-100 to-purple-100 flex items-center justify-center">
                    <span className="text-3xl">🎨</span>
                  </div>
                  <div className="p-4">
                    <h3 className="font-semibold text-gray-900 truncate">{c.title}</h3>
                    <p className="text-xs text-gray-500 mt-0.5">by {c.author_name || `User #${c.author_id}`}</p>
                    <div className="flex items-center gap-4 mt-3 text-sm text-gray-500">
                      <span className="flex items-center gap-1">
                        <Heart size={14} className="text-red-400" /> {c.like_count}
                      </span>
                      <span className="flex items-center gap-1">
                        <MessageCircle size={14} /> {c.comment_count}
                      </span>
                      <span className="flex items-center gap-1">
                        <GitFork size={14} /> {c.fork_count}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex justify-center items-center gap-2 mt-8">
                <button
                  onClick={() => load(page - 1)}
                  disabled={page <= 1}
                  className="btn-secondary text-sm disabled:opacity-30"
                >
                  Previous
                </button>
                <span className="text-sm text-gray-500">Page {page} of {totalPages}</span>
                <button
                  onClick={() => load(page + 1)}
                  disabled={page >= totalPages}
                  className="btn-secondary text-sm disabled:opacity-30"
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </Layout>
  );
}

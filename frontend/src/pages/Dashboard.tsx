import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { canvasService } from '../services/canvas';
import { Layout } from '../components/layout/Layout';
import type { CanvasInfo } from '../types';
import { Plus, Users, Globe, Lock, Trash2, GitFork } from 'lucide-react';

export function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [canvases, setCanvases] = useState<CanvasInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [error, setError] = useState('');

  const load = async () => {
    try {
      setCanvases(await canvasService.listMine());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load canvases');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    try {
      const c = await canvasService.create(newTitle.trim());
      setCanvases(prev => [c, ...prev]);
      setShowCreate(false);
      setNewTitle('');
      navigate(`/canvas/${c.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create canvas');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this canvas?')) return;
    try {
      await canvasService.delete(id);
      setCanvases(prev => prev.filter(c => c.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete');
    }
  };

  const handleFork = async (id: number) => {
    try {
      const c = await canvasService.fork(id);
      setCanvases(prev => [c, ...prev]);
      navigate(`/canvas/${c.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fork');
    }
  };

  return (
    <Layout>
      <div className="max-w-5xl mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">My Canvases</h1>
            <p className="text-sm text-gray-500 mt-1">
              {user ? `Welcome, ${user.username}` : 'Loading...'}
            </p>
          </div>
          <button onClick={() => setShowCreate(true)} className="btn-primary flex items-center gap-2">
            <Plus size={18} /> New Canvas
          </button>
        </div>

        {error && (
          <div className="mb-4 text-red-600 text-sm bg-red-50 px-3 py-2 rounded-lg">{error}</div>
        )}

        {showCreate && (
          <div className="card p-4 mb-6">
            <input
              type="text"
              className="input-field mb-3"
              placeholder="Canvas title..."
              value={newTitle}
              onChange={e => setNewTitle(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreate()}
              autoFocus
            />
            <div className="flex gap-2">
              <button onClick={handleCreate} className="btn-primary text-sm" disabled={!newTitle.trim()}>
                Create
              </button>
              <button onClick={() => setShowCreate(false)} className="btn-secondary text-sm">Cancel</button>
            </div>
          </div>
        )}

        {loading ? (
          <div className="text-center py-12 text-gray-400">Loading...</div>
        ) : canvases.length === 0 ? (
          <div className="text-center py-12 card">
            <p className="text-gray-400 mb-2">No canvases yet</p>
            <button onClick={() => setShowCreate(true)} className="btn-primary text-sm">Create your first canvas</button>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {canvases.map(c => (
              <div key={c.id} className="card p-4 group">
                <Link to={`/canvas/${c.id}`} className="block">
                  <h3 className="font-semibold text-gray-900 truncate mb-2">{c.title}</h3>
                  <div className="flex items-center gap-3 text-xs text-gray-500">
                    <span className="flex items-center gap-1">
                      {c.visibility === 'published' ? <Globe size={14} /> : <Lock size={14} />}
                      {c.visibility}
                    </span>
                    <span className="flex items-center gap-1">
                      <Users size={14} /> {c.member_count}
                    </span>
                    {c.online_count > 0 && (
                      <span className="text-green-600">{c.online_count} online</span>
                    )}
                  </div>
                </Link>
                <div className="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 opacity-0 group-hover:opacity-100 transition-opacity">
                  {c.visibility !== 'published' && (
                    <button
                      onClick={() => canvasService.publish(c.id).then(load)}
                      className="text-xs text-indigo-600 hover:underline"
                    >
                      Publish
                    </button>
                  )}
                  <button
                    onClick={() => handleFork(c.id)}
                    className="text-xs text-gray-500 hover:underline flex items-center gap-1"
                  >
                    <GitFork size={12} /> Fork
                  </button>
                  <button
                    onClick={() => handleDelete(c.id)}
                    className="text-xs text-red-500 hover:underline flex items-center gap-1 ml-auto"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </Layout>
  );
}

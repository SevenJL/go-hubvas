import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { canvasService } from '../services/canvas';
import { Layout } from '../components/layout/Layout';
import type { CanvasInfo } from '../types';
import { Plus, Users, Globe, Lock, Trash2, GitFork, Share2 } from 'lucide-react';

function CanvasGrid({ canvases, owned, onDelete, onFork, onPublish }: {
  canvases: CanvasInfo[];
  owned: boolean;
  onDelete: (id: string) => void;
  onFork: (id: string) => void;
  onPublish: (id: string) => void;
}) {
  if (canvases.length === 0) {
    return <div className="card py-10 text-center text-sm text-gray-400">{owned ? 'No canvases yet' : 'No canvases have been shared with you'}</div>;
  }
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {canvases.map(c => (
        <div key={c.id} className="card p-4 group">
          <Link to={`/canvas/${c.id}/edit`} className="block">
            <div className="flex items-start justify-between gap-2">
              <h3 className="font-semibold text-gray-900 truncate mb-2">{c.title}</h3>
              {!owned && <span className="text-[10px] uppercase tracking-wide text-indigo-600 bg-indigo-50 px-1.5 py-0.5 rounded">{c.current_role}</span>}
            </div>
            <div className="flex items-center gap-3 text-xs text-gray-500">
              <span className="flex items-center gap-1">
                {c.visibility === 'published' ? <Globe size={14} /> : <Lock size={14} />}
                {c.visibility}
              </span>
              <span className="flex items-center gap-1"><Users size={14} /> {c.member_count}</span>
              {c.online_count > 0 && <span className="text-green-600">{c.online_count} online</span>}
            </div>
          </Link>
          <div className="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 opacity-0 group-hover:opacity-100 transition-opacity">
            {owned && c.visibility !== 'published' && (
              <button onClick={() => onPublish(c.id)} className="text-xs text-indigo-600 hover:underline">Publish</button>
            )}
            <button onClick={() => onFork(c.id)} className="text-xs text-gray-500 hover:underline flex items-center gap-1">
              <GitFork size={12} /> Fork
            </button>
            {owned && (
              <button onClick={() => onDelete(c.id)} className="text-xs text-red-500 hover:underline flex items-center gap-1 ml-auto">
                <Trash2 size={12} />
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

export function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [canvases, setCanvases] = useState<CanvasInfo[]>([]);
  const [shared, setShared] = useState<CanvasInfo[]>([]);
  const [activeTab, setActiveTab] = useState<'owned' | 'shared'>('owned');
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [error, setError] = useState('');

  const load = async () => {
    setError('');
    try {
      const [mine, sharedWithMe] = await Promise.all([canvasService.listMine(), canvasService.listShared()]);
      setCanvases(mine);
      setShared(sharedWithMe);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load canvases');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    try {
      const c = await canvasService.create(newTitle.trim());
      setCanvases(prev => [c, ...prev]);
      setShowCreate(false);
      setNewTitle('');
      navigate(`/canvas/${c.id}/edit`);
    } catch (err) { setError(err instanceof Error ? err.message : 'Failed to create canvas'); }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this canvas?')) return;
    try { await canvasService.delete(id); setCanvases(prev => prev.filter(c => c.id !== id)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Failed to delete'); }
  };

  const handleFork = async (id: string) => {
    try {
      const c = await canvasService.fork(id);
      setCanvases(prev => [c, ...prev]);
      navigate(`/canvas/${c.id}/edit`);
    } catch (err) { setError(err instanceof Error ? err.message : 'Failed to fork'); }
  };

  const handlePublish = async (id: string) => {
    try { await canvasService.publish(id); await load(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Failed to publish'); }
  };

  return (
    <Layout>
      <div className="max-w-5xl mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Canvases</h1>
            <p className="text-sm text-gray-500 mt-1">{user ? `Welcome, ${user.username}` : 'Loading...'}</p>
          </div>
          <button onClick={() => setShowCreate(true)} className="btn-primary flex items-center gap-2"><Plus size={18} /> New Canvas</button>
        </div>

        {error && <div className="mb-4 text-red-600 text-sm bg-red-50 px-3 py-2 rounded-lg">{error}</div>}
        {showCreate && (
          <div className="card p-4 mb-6">
            <input type="text" className="input-field mb-3" placeholder="Canvas title..." value={newTitle}
              onChange={e => setNewTitle(e.target.value)} onKeyDown={e => e.key === 'Enter' && void handleCreate()} autoFocus />
            <div className="flex gap-2">
              <button onClick={() => void handleCreate()} className="btn-primary text-sm" disabled={!newTitle.trim()}>Create</button>
              <button onClick={() => setShowCreate(false)} className="btn-secondary text-sm">Cancel</button>
            </div>
          </div>
        )}

        <div className="flex gap-1 border-b border-gray-200 mb-5">
          <button onClick={() => setActiveTab('owned')} className={`px-4 py-2 text-sm font-medium border-b-2 ${activeTab === 'owned' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500'}`}>
            My canvases <span className="ml-1 text-xs">{canvases.length}</span>
          </button>
          <button onClick={() => setActiveTab('shared')} className={`px-4 py-2 text-sm font-medium border-b-2 flex items-center gap-1.5 ${activeTab === 'shared' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500'}`}>
            <Share2 size={14} /> Shared with me <span className="text-xs">{shared.length}</span>
          </button>
        </div>

        {loading ? <div className="text-center py-12 text-gray-400">Loading...</div> : (
          <CanvasGrid canvases={activeTab === 'owned' ? canvases : shared} owned={activeTab === 'owned'}
            onDelete={id => void handleDelete(id)} onFork={id => void handleFork(id)} onPublish={id => void handlePublish(id)} />
        )}
      </div>
    </Layout>
  );
}

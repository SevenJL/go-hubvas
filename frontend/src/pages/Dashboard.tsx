import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { GitFork, Globe, Lock, Plus, Share2, Trash2, Users } from 'lucide-react';
import { useAuth } from '../store/authStore';
import { canvasService } from '../services/canvas';
import { Layout } from '../components/layout/Layout';
import { Button, CanvasGridSkeleton, ConfirmDialog, ErrorState, Input, Modal, useToast } from '../components/ui';
import type { CanvasInfo } from '../types';
import { useI18n } from '../i18n';

interface CanvasGridProps {
  canvases: CanvasInfo[];
  owned: boolean;
  busyId: string | null;
  onDelete: (canvas: CanvasInfo) => void;
  onFork: (canvas: CanvasInfo) => void;
  onPublish: (canvas: CanvasInfo) => void;
}

function CanvasGrid({ canvases, owned, busyId, onDelete, onFork, onPublish }: CanvasGridProps) {
  const { t } = useI18n();
  if (canvases.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-6 py-14 text-center">
        <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-indigo-50 text-indigo-600">
          {owned ? <Plus size={20} /> : <Share2 size={20} />}
        </div>
        <p className="mt-3 text-sm font-medium text-slate-700">{t(owned ? 'No canvases yet' : 'Nothing shared with you yet')}</p>
        <p className="mt-1 text-xs text-slate-400">{t(owned ? 'Create a canvas to start drawing with your team.' : 'Shared canvases will appear here automatically.')}</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {canvases.map(canvas => {
        const busy = busyId === canvas.id;
        return (
          <article key={canvas.id} className="group rounded-xl border border-slate-200 bg-white p-4 shadow-sm transition-all hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md">
            <Link to={`/canvas/${canvas.id}/edit`} className="block">
              <div className="flex items-start justify-between gap-2">
                <h3 className="mb-2 truncate font-semibold text-slate-900">{canvas.title}</h3>
                {!owned && <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-indigo-600">{t(canvas.current_role || 'viewer')}</span>}
              </div>
              <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500">
                <span className="flex items-center gap-1">
                  {canvas.visibility === 'published' ? <Globe size={14} className="text-emerald-500" /> : <Lock size={14} />}
                  {t(canvas.visibility)}
                </span>
                <span className="flex items-center gap-1"><Users size={14} /> {canvas.member_count}</span>
                {canvas.online_count > 0 && <span className="rounded-full bg-emerald-50 px-2 py-0.5 font-medium text-emerald-600">{t('{count} online', { count: canvas.online_count })}</span>}
              </div>
            </Link>
            <div className="mt-3 flex min-h-8 items-center gap-2 border-t border-slate-100 pt-3 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
              {owned && canvas.visibility !== 'published' && (
                <button disabled={busy} onClick={() => onPublish(canvas)} className="text-xs font-medium text-indigo-600 hover:text-indigo-800 disabled:opacity-40">{t('Publish')}</button>
              )}
              <button disabled={busy} onClick={() => onFork(canvas)} className="flex items-center gap-1 text-xs font-medium text-slate-500 hover:text-slate-800 disabled:opacity-40">
                <GitFork size={12} /> {t('Fork')}
              </button>
              {owned && (
                <button disabled={busy} onClick={() => onDelete(canvas)} className="ml-auto rounded-md p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-40" aria-label={t('Delete {title}', { title: canvas.title })}>
                  <Trash2 size={14} />
                </button>
              )}
            </div>
          </article>
        );
      })}
    </div>
  );
}

export function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const { t } = useI18n();
  const [canvases, setCanvases] = useState<CanvasInfo[]>([]);
  const [shared, setShared] = useState<CanvasInfo[]>([]);
  const [activeTab, setActiveTab] = useState<'owned' | 'shared'>('owned');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [creating, setCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CanvasInfo | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoadError('');
    setLoading(true);
    try {
      const [mine, sharedWithMe] = await Promise.all([canvasService.listMine(), canvasService.listShared()]);
      setCanvases(mine);
      setShared(sharedWithMe);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : t('Failed to load canvases'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  const handleCreate = async () => {
    const title = newTitle.trim();
    if (!title || creating) return;
    setCreating(true);
    try {
      const canvas = await canvasService.create(title);
      setCanvases(prev => [canvas, ...prev]);
      setShowCreate(false);
      setNewTitle('');
      toast.success({ title: t('Canvas created'), message: t('“{title}” is ready to edit.', { title: canvas.title }) });
      navigate(`/canvas/${canvas.id}/edit`);
    } catch (err) {
      toast.error({ title: t('Could not create canvas'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setBusyId(deleteTarget.id);
    try {
      await canvasService.delete(deleteTarget.id);
      setCanvases(prev => prev.filter(canvas => canvas.id !== deleteTarget.id));
      toast.success({ title: t('Canvas deleted'), message: t('“{title}” was removed.', { title: deleteTarget.title }) });
      setDeleteTarget(null);
    } catch (err) {
      toast.error({ title: t('Delete failed'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setBusyId(null);
    }
  };

  const handleFork = async (canvas: CanvasInfo) => {
    setBusyId(canvas.id);
    try {
      const forked = await canvasService.fork(canvas.id);
      setCanvases(prev => [forked, ...prev]);
      toast.success({ title: t('Fork created'), message: t('Opening your editable copy.') });
      navigate(`/canvas/${forked.id}/edit`);
    } catch (err) {
      toast.error({ title: t('Fork failed'), message: err instanceof Error ? err.message : t('Please try again.') });
      setBusyId(null);
    }
  };

  const handlePublish = async (canvas: CanvasInfo) => {
    setBusyId(canvas.id);
    try {
      await canvasService.publish(canvas.id);
      setCanvases(prev => prev.map(item => item.id === canvas.id ? { ...item, visibility: 'published' } : item));
      toast.success({ title: t('Canvas published'), message: t('“{title}” is now visible in the community.', { title: canvas.title }) });
    } catch (err) {
      toast.error({ title: t('Publish failed'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setBusyId(null);
    }
  };

  return (
    <Layout>
      <div className="mx-auto max-w-5xl px-4 py-8">
        <div className="mb-6 flex items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">{t('Canvases')}</h1>
            <p className="mt-1 text-sm text-slate-500">{user ? t('Welcome back, {name}', { name: user.username }) : t('Your collaborative workspace')}</p>
          </div>
          <Button onClick={() => setShowCreate(true)}><Plus size={18} /> {t('New Canvas')}</Button>
        </div>

        <div className="mb-5 flex gap-1 border-b border-slate-200">
          <button onClick={() => setActiveTab('owned')} className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${activeTab === 'owned' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-800'}`}>
            {t('My canvases')} <span className="ml-1 text-xs">{canvases.length}</span>
          </button>
          <button onClick={() => setActiveTab('shared')} className={`flex items-center gap-1.5 border-b-2 px-4 py-2 text-sm font-medium transition-colors ${activeTab === 'shared' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-800'}`}>
            <Share2 size={14} /> {t('Shared with me')} <span className="text-xs">{shared.length}</span>
          </button>
        </div>

        {loading ? (
          <CanvasGridSkeleton />
        ) : loadError ? (
          <ErrorState title={t("We couldn't load your canvases")} message={loadError} onRetry={() => void load()} />
        ) : (
          <CanvasGrid
            canvases={activeTab === 'owned' ? canvases : shared}
            owned={activeTab === 'owned'}
            busyId={busyId}
            onDelete={setDeleteTarget}
            onFork={canvas => void handleFork(canvas)}
            onPublish={canvas => void handlePublish(canvas)}
          />
        )}
      </div>

      <Modal
        open={showCreate}
        title={t('Create a new canvas')}
        description={t('Give your workspace a clear name. You can change it later.')}
        onClose={() => { if (!creating) { setShowCreate(false); setNewTitle(''); } }}
        size="sm"
        closeOnBackdrop={!creating}
        footer={
          <>
            <Button variant="secondary" onClick={() => { setShowCreate(false); setNewTitle(''); }} disabled={creating}>{t('Cancel')}</Button>
            <Button onClick={() => void handleCreate()} loading={creating} disabled={!newTitle.trim()}>{t('Create canvas')}</Button>
          </>
        }
      >
        <Input
          label={t('Canvas title')}
          placeholder={t('e.g. Product planning')}
          value={newTitle}
          onChange={event => setNewTitle(event.target.value)}
          onKeyDown={event => event.key === 'Enter' && void handleCreate()}
          maxLength={120}
          autoFocus
        />
        <p className="mt-2 text-right text-xs text-slate-400">{newTitle.length}/120</p>
      </Modal>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title={t('Delete this canvas?')}
        description={deleteTarget ? t('“{title}” and its collaborative data will be permanently removed. This action cannot be undone.', { title: deleteTarget.title }) : ''}
        confirmLabel={t('Delete canvas')}
        danger
        loading={Boolean(deleteTarget && busyId === deleteTarget.id)}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void handleDelete()}
      />
    </Layout>
  );
}

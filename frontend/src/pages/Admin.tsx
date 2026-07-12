import { useCallback, useEffect, useState } from 'react';
import { ClipboardList, Eye, EyeOff, PauseCircle, PlayCircle, ScrollText } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { Button, PageLoader, useToast } from '../components/ui';
import { socialService } from '../services/social';
import type { AdminAuditLog, ReportInfo } from '../types';
import { useI18n } from '../i18n';

type QueueStatus = ReportInfo['status'];
const statuses: QueueStatus[] = ['pending', 'reviewing', 'resolved', 'dismissed'];

export function Admin() {
  const [items, setItems] = useState<ReportInfo[]>([]);
  const [auditItems, setAuditItems] = useState<AdminAuditLog[]>([]);
  const [view, setView] = useState<'reports' | 'audit'>('reports');
  const [status, setStatus] = useState<QueueStatus>('pending');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const toast = useToast();
  const { t } = useI18n();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      if (view === 'reports') setItems((await socialService.reports(status)).items);
      else setAuditItems((await socialService.auditLogs()).items);
    }
    catch (error) { toast.error(error instanceof Error ? error.message : t('Admin access required')); }
    finally { setLoading(false); }
  }, [status, toast, t, view]);

  useEffect(() => { void load(); }, [load]);

  const review = async (report: ReportInfo, nextStatus: 'reviewing' | 'resolved' | 'dismissed') => {
    const note = nextStatus === 'reviewing' ? '' : (window.prompt(t('Review note')) || '');
    setBusy(`review:${report.id}`);
    try {
      await socialService.reviewReport(report.id, nextStatus, note);
      toast.success(t('Report updated'));
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Could not update report'));
    } finally { setBusy(null); }
  };

  const moderate = async (report: ReportInfo, action: 'restrict' | 'restore') => {
    const key = `moderate:${report.id}:${action}`;
    setBusy(key);
    try {
      if (report.target_type === 'user') await socialService.setUserStatus(report.target_id, action === 'restrict' ? 'suspended' : 'active');
      if (report.target_type === 'comment') await socialService.moderateComment(report.target_id, action === 'restrict' ? 'hidden' : 'visible');
      if (report.target_type === 'canvas') await socialService.moderateCanvas(report.target_id, action === 'restrict' ? 'hidden' : 'visible');
      toast.success(t(action === 'restrict' ? 'Moderation restriction applied' : 'Content restored'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Could not apply moderation action'));
    } finally { setBusy(null); }
  };

  const restrictCopy = (report: ReportInfo) => report.target_type === 'user' ? 'Suspend user' : report.target_type === 'comment' ? 'Hide comment' : 'Unpublish canvas';
  const restoreCopy = (report: ReportInfo) => report.target_type === 'user' ? 'Restore user' : report.target_type === 'comment' ? 'Show comment' : 'Republish canvas';

  return <Layout><main className="mx-auto max-w-6xl px-4 py-10">
    <p className="text-sm font-semibold text-rose-600">{t('TRUST & SAFETY')}</p>
    <h1 className="text-3xl font-bold text-slate-950">{t('Moderation queue')}</h1>
    <p className="mt-2 text-slate-500">{t('Review pending user, canvas and comment reports.')}</p>

    <div className="mt-6 flex gap-2 border-b border-slate-200 pb-4">
      <button onClick={() => setView('reports')} className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold ${view === 'reports' ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'}`}><ClipboardList size={16} />{t('Reports')}</button>
      <button onClick={() => setView('audit')} className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold ${view === 'audit' ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'}`}><ScrollText size={16} />{t('Audit log')}</button>
    </div>

    {view === 'reports' && <div className="mt-5 flex flex-wrap gap-2" aria-label={t('Report status filter')}>
      {statuses.map(option => <button key={option} onClick={() => setStatus(option)} className={`rounded-full px-4 py-2 text-sm font-semibold transition ${status === option ? 'bg-slate-950 text-white' : 'bg-white text-slate-600 ring-1 ring-slate-200 hover:bg-slate-50'}`}>{t(option)}</button>)}
    </div>}

    {loading ? <PageLoader label={t(view === 'reports' ? 'Loading reports...' : 'Loading audit log...')} /> : view === 'reports' ? <div className="mt-8 overflow-hidden rounded-2xl border border-slate-200 bg-white">
      {items.length ? items.map(report => <article key={report.id} className="border-b border-slate-100 p-5 last:border-0">
        <div className="grid gap-5 lg:grid-cols-[1fr_auto]">
          <div>
            <div className="flex flex-wrap gap-2">
              <span className="rounded-full bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700">{t(report.reason)}</span>
              <span className="rounded-full bg-slate-100 px-2 py-1 text-xs text-slate-600">{t(report.target_type)} #{report.target_id}</span>
              <span className="rounded-full bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700">{t(report.status)}</span>
            </div>
            <p className="mt-3 text-sm text-slate-700">{report.details || t('No additional details.')}</p>
            <p className="mt-2 text-xs text-slate-400">{t('Reporter')} #{report.reporter_id} · {new Date(report.created_at).toLocaleString()}</p>
            {report.review_note && <p className="mt-2 rounded-xl bg-slate-50 p-3 text-xs text-slate-600"><strong>{t('Review note')}:</strong> {report.review_note}</p>}
          </div>
          <div className="flex max-w-lg flex-wrap items-center justify-start gap-2 lg:justify-end">
            <Button variant="danger" loading={busy === `moderate:${report.id}:restrict`} disabled={busy !== null} onClick={() => void moderate(report, 'restrict')}>
              {report.target_type === 'user' ? <PauseCircle size={16} /> : <EyeOff size={16} />}{t(restrictCopy(report))}
            </Button>
            <Button variant="secondary" loading={busy === `moderate:${report.id}:restore`} disabled={busy !== null} onClick={() => void moderate(report, 'restore')}>
              {report.target_type === 'user' ? <PlayCircle size={16} /> : <Eye size={16} />}{t(restoreCopy(report))}
            </Button>
            {report.status === 'pending' && <Button variant="secondary" disabled={busy !== null} onClick={() => void review(report, 'reviewing')}>{t('Start review')}</Button>}
            {(report.status === 'pending' || report.status === 'reviewing') && <>
              <Button variant="secondary" disabled={busy !== null} onClick={() => void review(report, 'dismissed')}>{t('Dismiss')}</Button>
              <Button disabled={busy !== null} onClick={() => void review(report, 'resolved')}>{t('Resolve')}</Button>
            </>}
          </div>
        </div>
      </article>) : <div className="p-14 text-center text-slate-500">{t('No reports in this queue.')}</div>}
    </div> : <div className="mt-8 overflow-hidden rounded-2xl border border-slate-200 bg-white">
      {auditItems.length ? auditItems.map(item => <article key={item.id} className="grid gap-2 border-b border-slate-100 p-5 last:border-0 sm:grid-cols-[1fr_auto]">
        <div><p className="font-medium text-slate-900">{item.action}</p><p className="mt-1 text-sm text-slate-500">{item.target_type} #{item.target_id} · {item.admin_display_name || item.admin_username || t('Deleted administrator')}</p></div>
        <time className="text-xs text-slate-400">{new Date(item.created_at).toLocaleString()}</time>
        {Object.keys(item.metadata).length > 0 && <pre className="overflow-x-auto rounded-xl bg-slate-50 p-3 text-xs text-slate-600 sm:col-span-2">{JSON.stringify(item.metadata, null, 2)}</pre>}
      </article>) : <div className="p-14 text-center text-slate-500">{t('No audit events yet.')}</div>}
    </div>}
  </main></Layout>;
}

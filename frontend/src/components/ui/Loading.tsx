import { LoaderCircle, RefreshCw } from 'lucide-react';
import { Button } from './Button';
import { useI18n } from '../../i18n';

export function Spinner({ size = 'md', className = '' }: { size?: 'sm' | 'md' | 'lg'; className?: string }) {
  const sizes = { sm: 'h-4 w-4', md: 'h-6 w-6', lg: 'h-9 w-9' };
  return <LoaderCircle className={`${sizes[size]} animate-spin ${className}`} aria-hidden="true" />;
}

export function PageLoader({ label }: { label?: string }) {
  const { t } = useI18n();
  return (
    <div className="min-h-screen bg-slate-50 flex items-center justify-center p-6">
      <div className="flex flex-col items-center">
        <div className="relative flex h-16 w-16 items-center justify-center rounded-2xl bg-white shadow-lg shadow-indigo-100 ring-1 ring-slate-200">
          <div className="absolute inset-2 rounded-xl border border-indigo-100" />
          <Spinner size="lg" className="text-indigo-600" />
        </div>
        <p className="mt-4 text-sm font-medium text-slate-600">{label || t('Loading workspace...')}</p>
        <p className="mt-1 text-xs text-slate-400">{t('Preparing your collaborative canvas')}</p>
      </div>
    </div>
  );
}

export function CanvasCardSkeleton() {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm" aria-hidden="true">
      <div className="skeleton h-28 rounded-lg" />
      <div className="skeleton mt-4 h-4 w-3/5 rounded" />
      <div className="mt-3 flex gap-2">
        <div className="skeleton h-3 w-16 rounded" />
        <div className="skeleton h-3 w-12 rounded" />
      </div>
    </div>
  );
}

export function CanvasGridSkeleton({ count = 6 }: { count?: number }) {
  const { t } = useI18n();
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-label={t('Loading canvases')}>
      {Array.from({ length: count }, (_, index) => <CanvasCardSkeleton key={index} />)}
    </div>
  );
}

export function InlineLoader({ label }: { label: string }) {
  return <div className="flex items-center justify-center gap-2 py-8 text-sm text-slate-500"><Spinner size="sm" className="text-indigo-500" />{label}</div>;
}

export function ErrorState({ title, message, onRetry }: { title: string; message: string; onRetry?: () => void }) {
  const { t } = useI18n();
  return (
    <div className="rounded-2xl border border-red-100 bg-red-50/70 px-6 py-10 text-center">
      <p className="font-semibold text-slate-900">{title}</p>
      <p className="mx-auto mt-1 max-w-md text-sm text-slate-500">{message}</p>
      {onRetry && <Button variant="secondary" className="mt-4" onClick={onRetry}><RefreshCw size={15} />{t('Try again')}</Button>}
    </div>
  );
}

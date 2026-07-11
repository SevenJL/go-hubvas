import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react';
import { ToastContext, type ToastOptions } from './toast-context';
import { useI18n } from '../../i18n';

type ToastType = 'success' | 'error' | 'info';

interface ToastItem {
  id: number;
  type: ToastType;
  title: string;
  message?: string;
}

let toastId = 0;

const styles = {
  success: { icon: CheckCircle2, iconClass: 'text-emerald-600', bar: 'bg-emerald-500' },
  error: { icon: AlertCircle, iconClass: 'text-red-600', bar: 'bg-red-500' },
  info: { icon: Info, iconClass: 'text-indigo-600', bar: 'bg-indigo-500' },
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const [items, setItems] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: number) => {
    setItems(current => current.filter(item => item.id !== id));
  }, []);

  const show = useCallback((type: ToastType, options: ToastOptions | string) => {
    const normalized = typeof options === 'string' ? { title: options } : options;
    const id = ++toastId;
    setItems(current => [...current.slice(-3), { id, type, title: normalized.title, message: normalized.message }]);
    window.setTimeout(() => dismiss(id), normalized.duration ?? 3600);
  }, [dismiss]);

  const value = useMemo(() => ({
    success: (options: ToastOptions | string) => show('success', options),
    error: (options: ToastOptions | string) => show('error', options),
    info: (options: ToastOptions | string) => show('info', options),
  }), [show]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed right-4 top-4 z-[4000] flex w-[min(92vw,380px)] flex-col gap-2" aria-live="polite">
        {items.map(item => {
          const config = styles[item.type];
          const Icon = config.icon;
          return (
            <div key={item.id} className="pointer-events-auto relative overflow-hidden rounded-xl border border-slate-200/80 bg-white p-4 shadow-xl shadow-slate-900/10 animate-[toast-in_220ms_ease-out]">
              <div className={`absolute inset-y-0 left-0 w-1 ${config.bar}`} />
              <div className="flex items-start gap-3 pl-1">
                <Icon size={19} className={`mt-0.5 shrink-0 ${config.iconClass}`} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold text-slate-900">{item.title}</p>
                  {item.message && <p className="mt-0.5 text-sm leading-5 text-slate-500">{item.message}</p>}
                </div>
                <button type="button" onClick={() => dismiss(item.id)} className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700" aria-label={t('Dismiss notification')}>
                  <X size={15} />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}

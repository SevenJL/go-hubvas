import { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle, House, RefreshCw } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { useI18n } from '../../i18n';
import { Button } from '../ui';

interface BoundaryProps {
  children: ReactNode;
  title: string;
  message: string;
  retryLabel: string;
  homeLabel: string;
}

interface BoundaryState {
  failed: boolean;
}

class RouteErrorBoundaryInner extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { failed: false };

  static getDerivedStateFromError(): BoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Route chunk failed to load', error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-950 px-5 py-12 text-slate-100">
        <section className="w-full max-w-lg rounded-3xl border border-white/10 bg-slate-900/90 p-7 text-center shadow-2xl shadow-black/40 sm:p-10">
          <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-400/10 text-amber-300">
            <AlertTriangle size={28} aria-hidden="true" />
          </span>
          <h1 className="mt-6 text-2xl font-semibold tracking-tight">{this.props.title}</h1>
          <p className="mx-auto mt-3 max-w-md text-sm leading-6 text-slate-400">{this.props.message}</p>
          <div className="mt-7 flex flex-col justify-center gap-3 sm:flex-row">
            <Button onClick={() => window.location.reload()} className="justify-center">
              <RefreshCw size={16} aria-hidden="true" />
              {this.props.retryLabel}
            </Button>
            <Button variant="secondary" onClick={() => window.location.assign('/community')} className="justify-center">
              <House size={16} aria-hidden="true" />
              {this.props.homeLabel}
            </Button>
          </div>
        </section>
      </main>
    );
  }
}

export function RouteErrorBoundary({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { t } = useI18n();
  return (
    <RouteErrorBoundaryInner
      key={pathname}
      title={t('This page could not be loaded')}
      message={t('The application may have been updated or your network was interrupted. Retry to download the latest page files.')}
      retryLabel={t('Retry loading')}
      homeLabel={t('Back to community')}
    >
      {children}
    </RouteErrorBoundaryInner>
  );
}

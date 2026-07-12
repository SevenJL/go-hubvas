import { useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  Bell,
  ChevronDown,
  Globe,
  LayoutDashboard,
  LogOut,
  Palette,
  Shield,
  UserRound,
} from 'lucide-react';
import { useAuth } from '../../store/authStore';
import { LanguageToggle } from '../ui';
import { Avatar } from '../ui/Avatar';
import { useNotifications } from '../../hooks/useNotifications';
import { useI18n } from '../../i18n';

export function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { unread } = useNotifications(Boolean(user));
  const { t } = useI18n();
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setAccountMenuOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!accountMenuOpen) return;

    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!accountMenuRef.current?.contains(event.target as Node)) {
        setAccountMenuOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setAccountMenuOpen(false);
    };

    document.addEventListener('pointerdown', closeOnOutsidePointer);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsidePointer);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [accountMenuOpen]);

  const signOut = () => {
    setAccountMenuOpen(false);
    logout();
    navigate('/login');
  };

  const accountName = user?.display_name || user?.username || '';

  return (
    <div className="min-h-screen bg-slate-50">
      <header className={`sticky top-0 border-b border-slate-200/80 bg-white/90 backdrop-blur ${accountMenuOpen ? 'z-[10000]' : 'z-50'}`}>
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between gap-2 px-2.5 sm:px-4">
          <Link to="/" className="flex min-w-0 shrink-0 items-center gap-2 text-lg font-bold text-slate-950">
            <span className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-indigo-600 text-white">
              <Palette size={20} />
            </span>
            <span className="max-[389px]:hidden">Hubvas</span>
          </Link>

          <nav className="flex min-w-0 items-center gap-0.5 sm:gap-1" aria-label="Primary navigation">
            <Link to="/dashboard" className="nav-link" aria-label={t('Dashboard')} title={t('Dashboard')}>
              <LayoutDashboard size={17} />
              <span className="hidden lg:inline">{t('Dashboard')}</span>
            </Link>
            <Link to="/community" className="nav-link" aria-label={t('Community')} title={t('Community')}>
              <Globe size={17} />
              <span className="hidden lg:inline">{t('Community')}</span>
            </Link>

            {user && (
              <>
                <Link
                  to="/notifications"
                  className="relative nav-link"
                  aria-label={t('Notifications')}
                  title={t('Notifications')}
                >
                  <Bell size={18} />
                  {unread > 0 && (
                    <span className="absolute right-0 top-0 min-w-4 rounded-full bg-rose-500 px-1 text-center text-[10px] font-bold leading-4 text-white">
                      {unread > 99 ? '99+' : unread}
                    </span>
                  )}
                </Link>
                {user.account_role === 'admin' && (
                  <Link to="/admin" className="nav-link hidden sm:flex" title={t('Moderation')} aria-label={t('Moderation')}>
                    <Shield size={17} />
                  </Link>
                )}
              </>
            )}

            <LanguageToggle compact className="sm:hidden" />
            <LanguageToggle className="hidden sm:flex" />

            {user ? (
              <div ref={accountMenuRef} className="relative ml-0.5 sm:ml-1">
                <button
                  type="button"
                  onClick={() => setAccountMenuOpen(open => !open)}
                  className="flex max-w-[12rem] items-center gap-2 rounded-xl p-1.5 text-left transition-colors hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2 sm:px-2"
                  aria-label={t('Profile settings')}
                  aria-haspopup="menu"
                  aria-expanded={accountMenuOpen}
                >
                  <Avatar size="sm" name={accountName} src={user.avatar_url} />
                  <span className="hidden min-w-0 sm:block">
                    <span className="block truncate text-sm font-semibold text-slate-800">{accountName}</span>
                    <span className="block truncate text-xs text-slate-500">@{user.username}</span>
                  </span>
                  <ChevronDown
                    size={15}
                    className={`hidden shrink-0 text-slate-400 transition-transform sm:block ${accountMenuOpen ? 'rotate-180' : ''}`}
                  />
                </button>

                {accountMenuOpen && (
                  <div
                    role="menu"
                    aria-label={t('Profile')}
                    className="absolute right-0 top-[calc(100%+0.4rem)] z-[10010] w-[min(15.5rem,calc(100vw-0.75rem))] overflow-hidden rounded-xl border border-slate-200 bg-white p-1 shadow-2xl shadow-slate-900/15 sm:top-[calc(100%+0.65rem)] sm:w-72 sm:rounded-2xl sm:p-1.5"
                  >
                    <div className="flex items-center gap-2 border-b border-slate-100 px-2.5 py-2 sm:gap-3 sm:px-3 sm:py-3">
                      <span className="sm:hidden"><Avatar size="sm" name={accountName} src={user.avatar_url} /></span>
                      <span className="hidden sm:block"><Avatar size="md" name={accountName} src={user.avatar_url} /></span>
                      <div className="min-w-0">
                        <p className="truncate text-[13px] font-semibold text-slate-900 sm:text-sm">{accountName}</p>
                        <p className="truncate text-[11px] text-slate-500 sm:text-xs">{user.email}</p>
                      </div>
                    </div>

                    <div className="py-1">
                      <Link
                        to="/profile"
                        role="menuitem"
                        className="flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-700 transition-colors hover:bg-slate-100 hover:text-slate-950 sm:gap-3 sm:rounded-xl sm:px-3 sm:py-2.5 sm:text-sm"
                      >
                        <UserRound size={17} className="text-slate-400" />
                        {t('Profile settings')}
                      </Link>
                      {user.account_role === 'admin' && (
                        <Link
                          to="/admin"
                          role="menuitem"
                          className="flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-700 transition-colors hover:bg-slate-100 hover:text-slate-950 sm:hidden"
                        >
                          <Shield size={17} className="text-slate-400" />
                          {t('Moderation')}
                        </Link>
                      )}
                    </div>

                    <div className="border-t border-slate-100 pt-1">
                      <button
                        type="button"
                        role="menuitem"
                        onClick={signOut}
                        className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-[13px] font-semibold text-rose-600 transition-colors hover:bg-rose-50 sm:gap-3 sm:rounded-xl sm:px-3 sm:py-2.5 sm:text-sm"
                      >
                        <LogOut size={17} />
                        {t('Sign out')}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <Link to="/login" className="ml-0.5 shrink-0 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white sm:ml-1 sm:px-4">
                {t('Sign in')}
              </Link>
            )}
          </nav>
        </div>
      </header>
      <main>{children}</main>
    </div>
  );
}

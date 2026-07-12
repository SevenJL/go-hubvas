import { Link, useNavigate } from 'react-router-dom';
import { Bell, Globe, LayoutDashboard, LogOut, Palette, Shield } from 'lucide-react';
import { useAuth } from '../../store/AuthContext';
import { LanguageToggle } from '../ui';
import { Avatar } from '../ui/Avatar';
import { useNotifications } from '../../hooks/useNotifications';
import { useI18n } from '../../i18n';

export function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { unread } = useNotifications(Boolean(user));
  const { t } = useI18n();
  return (
    <div className="min-h-screen bg-slate-50">
      <header className="sticky top-0 z-50 border-b border-slate-200/80 bg-white/90 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4">
          <Link to="/" className="flex items-center gap-2 text-lg font-bold text-slate-950">
            <span className="grid h-9 w-9 place-items-center rounded-xl bg-indigo-600 text-white"><Palette size={20} /></span>Hubvas
          </Link>
          <nav className="flex items-center gap-1">
            <Link to="/dashboard" className="nav-link"><LayoutDashboard size={17} /><span className="hidden md:inline">{t('Dashboard')}</span></Link>
            <Link to="/community" className="nav-link"><Globe size={17} /><span className="hidden md:inline">{t('Community')}</span></Link>
            {user && <>
              <Link to="/notifications" className="relative nav-link" aria-label={t('Notifications')} title={t('Notifications')}>
                <Bell size={18} />
                {unread > 0 && <span className="absolute right-0 top-0 min-w-4 rounded-full bg-rose-500 px-1 text-center text-[10px] font-bold text-white">{unread > 99 ? '99+' : unread}</span>}
              </Link>
              {user.account_role === 'admin' && <Link to="/admin" className="nav-link" title={t('Moderation')}><Shield size={17} /></Link>}
            </>}
            <LanguageToggle />
            <span className="mx-1 h-6 w-px bg-slate-200" />
            {user ? <>
              <Link to="/profile" className="flex items-center gap-2 rounded-xl px-2 py-1.5 hover:bg-slate-100">
                <Avatar size="sm" name={user.display_name || user.username} src={user.avatar_url} />
                <span className="hidden text-sm font-medium text-slate-700 sm:block">{user.display_name || user.username}</span>
              </Link>
              <button onClick={() => { logout(); navigate('/login'); }} className="nav-link" aria-label={t('Sign out')} title={t('Sign out')}><LogOut size={17} /></button>
            </> : <Link to="/login" className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white">{t('Sign in')}</Link>}
          </nav>
        </div>
      </header>
      <main>{children}</main>
    </div>
  );
}

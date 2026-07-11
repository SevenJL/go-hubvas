import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../../store/AuthContext';
import { useI18n } from '../../i18n';
import { LanguageToggle } from '../ui';
import { LayoutDashboard, Globe, LogOut, Palette, User } from 'lucide-react';

export function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const { t } = useI18n();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen flex flex-col">
      <header className="bg-white border-b border-gray-200 sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 text-indigo-600 font-bold text-lg">
            <Palette size={24} />
            Hubvas
          </Link>

          <nav className="flex items-center gap-1">
            <Link to="/dashboard" className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-gray-600 hover:bg-gray-100 transition-colors">
              <LayoutDashboard size={16} />
              <span className="hidden sm:inline">{t('Dashboard')}</span>
            </Link>
            <Link to="/community" className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-gray-600 hover:bg-gray-100 transition-colors">
              <Globe size={16} />
              <span className="hidden sm:inline">{t('Community')}</span>
            </Link>

            <LanguageToggle />

            <div className="w-px h-6 bg-gray-200 mx-1 sm:mx-2" />

            {user ? (
              <>
                <Link to="/profile" className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-gray-600 hover:bg-gray-100 transition-colors">
                  <User size={16} />
                  <span className="hidden sm:inline">{user.username}</span>
                </Link>
                <button
                  onClick={handleLogout}
                  className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-gray-500 hover:text-red-600 hover:bg-red-50 transition-colors"
                  title={t('Sign out')}
                  aria-label={t('Sign out')}
                >
                  <LogOut size={16} />
                </button>
              </>
            ) : (
              <Link to="/login" className="btn-primary text-sm">{t('Sign In')}</Link>
            )}
          </nav>
        </div>
      </header>

      <main className="flex-1">{children}</main>
    </div>
  );
}

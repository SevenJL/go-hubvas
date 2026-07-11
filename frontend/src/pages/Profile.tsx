import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { Layout } from '../components/layout/Layout';
import { authService } from '../services/auth';
import { ArrowLeft, User, Link2, Save } from 'lucide-react';
import { useToast } from '../components/ui';
import { useI18n } from '../i18n';

export function Profile() {
  const { user, setUser } = useAuth();
  const [avatarUrl, setAvatarUrl] = useState(user?.avatar_url || '');
  const toast = useToast();
  const { language, t } = useI18n();
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (avatarUrl && !avatarUrl.startsWith('http')) {
      toast.error({ title: t('Invalid avatar URL'), message: t('The address must start with http:// or https://.') });
      return;
    }

    setLoading(true);
    try {
      const updated = await authService.updateProfile({ avatar_url: avatarUrl });
      setUser(updated);
      toast.success({ title: t('Profile updated'), message: t('Your changes have been saved.') });
    } catch (err) {
      toast.error({ title: t('Update failed'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setLoading(false);
    }
  };

  if (!user) {
    return (
      <Layout>
        <div className="max-w-5xl mx-auto px-4 py-8 text-center text-gray-400">{t('Please log in first.')}</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-lg mx-auto px-4 py-8">
        <Link to="/dashboard" className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 mb-6">
          <ArrowLeft size={16} /> {t('Back to dashboard')}
        </Link>

        <h1 className="text-2xl font-bold text-gray-900 mb-8">{t('Profile')}</h1>

        {/* User info card */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 mb-6">
          <div className="flex items-center gap-4 mb-6">
            <div className="w-16 h-16 rounded-full bg-indigo-500 flex items-center justify-center text-white text-xl font-bold shrink-0">
              {user.username[0]?.toUpperCase()}
            </div>
            <div>
              <h2 className="text-lg font-semibold text-gray-900">{user.username}</h2>
              <p className="text-sm text-gray-500">{user.email}</p>
              <p className="text-xs text-gray-400 mt-0.5">
                {t('Joined {date}', { date: new Date(user.created_at).toLocaleDateString(language === 'zh' ? 'zh-CN' : 'en-US') })}
              </p>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Username (read-only) */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">{t('Username')}</label>
              <div className="relative">
                <User size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  type="text"
                  className="w-full pl-9 pr-3 py-2.5 border border-gray-200 rounded-lg text-sm bg-gray-50 text-gray-500 cursor-not-allowed"
                  value={user.username}
                  disabled
                />
              </div>
              <p className="text-xs text-gray-400 mt-1">{t('Username cannot be changed')}</p>
            </div>

            {/* Email (read-only) */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">{t('Email')}</label>
              <input
                type="email"
                className="w-full px-3 py-2.5 border border-gray-200 rounded-lg text-sm bg-gray-50 text-gray-500 cursor-not-allowed"
                value={user.email}
                disabled
              />
              <p className="text-xs text-gray-400 mt-1">{t('Email cannot be changed')}</p>
            </div>

            {/* Avatar URL */}
            <div>
              <label htmlFor="avatar-url" className="block text-sm font-medium text-gray-700 mb-1.5">
                {t('Avatar URL')}
              </label>
              <div className="relative">
                <Link2 size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  id="avatar-url"
                  type="url"
                  className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                             placeholder:text-gray-400
                             focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                  value={avatarUrl}
                  onChange={e => setAvatarUrl(e.target.value)}
                  placeholder={t('https://example.com/avatar.png')}
                />
              </div>
              <p className="text-xs text-gray-400 mt-1">{t('Enter a URL for your avatar image (optional)')}</p>
            </div>

            {/* Submit */}
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-2 bg-indigo-500 text-white px-4 py-2.5 rounded-lg font-medium text-sm
                         hover:bg-indigo-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500
                         disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? (
                <>
                  <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  {t('Saving...')}
                </>
              ) : (
                <>
                  <Save size={16} />
                  {t('Save changes')}
                </>
              )}
            </button>
          </form>
        </div>
      </div>
    </Layout>
  );
}

import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { Palette, Mail, Key, AlertCircle } from 'lucide-react';
import { useI18n } from '../i18n';
import { LanguageToggle } from '../components/ui';

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const { t } = useI18n();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    // Client-side validation.
    if (!email.trim()) {
      setError(t('Email is required'));
      return;
    }
    if (!email.includes('@')) {
      setError(t('Please enter a valid email address'));
      return;
    }
    if (!password) {
      setError(t('Password is required'));
      return;
    }

    setLoading(true);
    try {
      await login(email.trim(), password);
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Login failed. Please try again.'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
      <LanguageToggle className="absolute right-4 top-4 bg-white shadow-sm ring-1 ring-gray-200" />
      <div className="w-full max-w-sm">
        {/* Brand */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-xl bg-indigo-500 shadow-lg shadow-indigo-200 mb-4">
            <Palette size={28} className="text-white" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">{t('Welcome back')}</h1>
          <p className="text-sm text-gray-500 mt-1">{t('Sign in to continue to Hubvas')}</p>
        </div>

        {/* Form */}
        <form
          onSubmit={handleSubmit}
          className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4"
          noValidate
        >
          {error && (
            <div className="flex items-start gap-2 text-red-600 text-sm bg-red-50 px-3 py-2.5 rounded-lg border border-red-100">
              <AlertCircle size={16} className="shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          {/* Email */}
          <div>
            <label htmlFor="login-email" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Email address')}
            </label>
            <div className="relative">
              <Mail size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="login-email"
                type="email"
                className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder={t('you@example.com')}
                autoComplete="email"
                autoFocus
                disabled={loading}
              />
            </div>
          </div>

          {/* Password */}
          <div>
            <label htmlFor="login-password" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Password')}
            </label>
            <div className="relative">
              <Key size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="login-password"
                type="password"
                className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder={t('Enter your password')}
                autoComplete="current-password"
                disabled={loading}
              />
            </div>
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-indigo-500 text-white py-2.5 rounded-lg font-medium text-sm
                       hover:bg-indigo-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500
                       disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {loading ? (
              <span className="inline-flex items-center gap-2">
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                {t('Signing in...')}
              </span>
            ) : (
              t('Sign in')
            )}
          </button>

          {/* Register link */}
          <p className="text-center text-sm text-gray-500">
            {t("Don't have an account?")}{' '}
            <Link to="/register" className="text-indigo-600 hover:text-indigo-700 font-medium hover:underline">
              {t('Create one')}
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}

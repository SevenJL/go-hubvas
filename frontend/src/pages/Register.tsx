import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../store/AuthContext';
import { Palette, User, Mail, Key, AlertCircle } from 'lucide-react';
import { useI18n } from '../i18n';
import { LanguageToggle } from '../components/ui';

export function Register() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const { t } = useI18n();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const validate = (): string | null => {
    if (!username.trim()) return t('Username is required');
    if (username.trim().length < 3) return t('Username must be at least 3 characters');
    if (username.trim().length > 50) return t('Username must be at most 50 characters');

    if (!email.trim()) return t('Email is required');
    if (!email.includes('@')) return t('Please enter a valid email address');

    if (!password) return t('Password is required');
    if (password.length < 8) return t('Password must be at least 8 characters');
    if (password.length > 128) return t('Password must be at most 128 characters');

    if (password !== confirmPassword) return t('Passwords do not match');

    return null;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }

    setLoading(true);
    try {
      await register(username.trim(), email.trim(), password);
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Registration failed. Please try again.'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4 py-8">
      <LanguageToggle className="absolute right-4 top-4 bg-white shadow-sm ring-1 ring-gray-200" />
      <div className="w-full max-w-sm">
        {/* Brand */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-xl bg-indigo-500 shadow-lg shadow-indigo-200 mb-4">
            <Palette size={28} className="text-white" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">{t('Create an account')}</h1>
          <p className="text-sm text-gray-500 mt-1">{t('Start drawing together')}</p>
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

          {/* Username */}
          <div>
            <label htmlFor="reg-username" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Username')}
            </label>
            <div className="relative">
              <User size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="reg-username"
                type="text"
                className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50"
                value={username}
                onChange={e => setUsername(e.target.value)}
                placeholder={t('yourname')}
                autoComplete="username"
                minLength={3}
                maxLength={50}
                autoFocus
                disabled={loading}
              />
            </div>
          </div>

          {/* Email */}
          <div>
            <label htmlFor="reg-email" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Email address')}
            </label>
            <div className="relative">
              <Mail size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="reg-email"
                type="email"
                className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder={t('you@example.com')}
                autoComplete="email"
                disabled={loading}
              />
            </div>
          </div>

          {/* Password */}
          <div>
            <label htmlFor="reg-password" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Password')}
            </label>
            <div className="relative">
              <Key size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="reg-password"
                type="password"
                className="w-full pl-9 pr-3 py-2.5 border border-gray-300 rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder={t('At least 8 characters')}
                autoComplete="new-password"
                minLength={8}
                maxLength={128}
                disabled={loading}
              />
            </div>
          </div>

          {/* Confirm Password */}
          <div>
            <label htmlFor="reg-confirm" className="block text-sm font-medium text-gray-700 mb-1.5">
              {t('Confirm password')}
            </label>
            <div className="relative">
              <Key size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="reg-confirm"
                type="password"
                className={`w-full pl-9 pr-3 py-2.5 border rounded-lg text-sm
                           placeholder:text-gray-400
                           focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
                           disabled:bg-gray-50
                           ${confirmPassword && password !== confirmPassword
                             ? 'border-red-300 bg-red-50'
                             : 'border-gray-300'
                           }`}
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                placeholder={t('Re-enter your password')}
                autoComplete="new-password"
                disabled={loading}
              />
            </div>
            {confirmPassword && password !== confirmPassword && (
              <p className="mt-1 text-xs text-red-600">{t('Passwords do not match')}</p>
            )}
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
                {t('Creating account...')}
              </span>
            ) : (
              t('Create account')
            )}
          </button>

          {/* Login link */}
          <p className="text-center text-sm text-gray-500">
            {t('Already have an account?')}{' '}
            <Link to="/login" className="text-indigo-600 hover:text-indigo-700 font-medium hover:underline">
              {t('Sign in')}
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}

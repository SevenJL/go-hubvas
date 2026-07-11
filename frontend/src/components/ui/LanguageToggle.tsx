import { Languages } from 'lucide-react';
import { useI18n } from '../../i18n';

interface LanguageToggleProps {
  compact?: boolean;
  className?: string;
}

export function LanguageToggle({ compact = false, className = '' }: LanguageToggleProps) {
  const { language, toggleLanguage, t } = useI18n();
  const label = language === 'en' ? t('Switch to Chinese') : t('Switch to English');

  return (
    <button
      type="button"
      onClick={toggleLanguage}
      className={`flex items-center gap-1.5 rounded-lg px-2.5 py-2 text-sm font-medium text-gray-500 transition-colors hover:bg-indigo-50 hover:text-indigo-600 ${className}`}
      title={label}
      aria-label={label}
    >
      <Languages size={16} />
      {!compact && <span className="text-xs">{language === 'en' ? '中文' : 'EN'}</span>}
    </button>
  );
}

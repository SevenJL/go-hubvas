import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { I18nContext } from './context';
import { translate, type Language, type TranslationParams } from './translations';

const STORAGE_KEY = 'hubvas-language';

function getInitialLanguage(): Language {
  const saved = window.localStorage.getItem(STORAGE_KEY);
  if (saved === 'en' || saved === 'zh') return saved;
  return window.navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(getInitialLanguage);

  const setLanguage = useCallback((nextLanguage: Language) => {
    setLanguageState(nextLanguage);
    window.localStorage.setItem(STORAGE_KEY, nextLanguage);
  }, []);

  const toggleLanguage = useCallback(() => {
    setLanguage(language === 'en' ? 'zh' : 'en');
  }, [language, setLanguage]);

  const t = useCallback((source: string, params?: TranslationParams) => (
    translate(language, source, params)
  ), [language]);

  useEffect(() => {
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
  }, [language]);

  const value = useMemo(() => ({ language, setLanguage, toggleLanguage, t }), [language, setLanguage, toggleLanguage, t]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

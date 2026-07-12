import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import { gsap } from 'gsap';
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
  const pageRef = useRef<HTMLDivElement>(null);
  const timelineRef = useRef<gsap.core.Timeline | null>(null);
  const animatingRef = useRef(false);

  const setLanguage = useCallback((nextLanguage: Language) => {
    setLanguageState(nextLanguage);
    window.localStorage.setItem(STORAGE_KEY, nextLanguage);
  }, []);

  const toggleLanguage = useCallback(() => {
    if (animatingRef.current) return;

    const nextLanguage: Language = language === 'en' ? 'zh' : 'en';
    const page = pageRef.current;
    const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;

    if (!page || prefersReducedMotion) {
      setLanguage(nextLanguage);
      return;
    }

    animatingRef.current = true;
    timelineRef.current?.kill();

    const timeline = gsap.timeline({
      defaults: { overwrite: 'auto' },
      onComplete: () => {
        gsap.set(page, { clearProps: 'opacity,transform,filter' });
        animatingRef.current = false;
        timelineRef.current = null;
      },
    });

    timelineRef.current = timeline;
    timeline
      .to(page, {
        autoAlpha: 0.18,
        y: -10,
        scale: 0.995,
        filter: 'blur(3px)',
        duration: 0.18,
        ease: 'power2.in',
      })
      .add(() => {
        flushSync(() => setLanguage(nextLanguage));
      })
      .set(page, { y: 10 })
      .to(page, {
        autoAlpha: 1,
        y: 0,
        scale: 1,
        filter: 'blur(0px)',
        duration: 0.34,
        ease: 'power3.out',
      });
  }, [language, setLanguage]);

  const t = useCallback((source: string, params?: TranslationParams) => (
    translate(language, source, params)
  ), [language]);

  useEffect(() => {
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
  }, [language]);

  useEffect(() => () => {
    timelineRef.current?.kill();
    if (pageRef.current) gsap.killTweensOf(pageRef.current);
  }, []);

  const value = useMemo(() => ({ language, setLanguage, toggleLanguage, t }), [language, setLanguage, toggleLanguage, t]);

  return (
    <I18nContext.Provider value={value}>
      <div ref={pageRef} className="min-h-screen">
        {children}
      </div>
    </I18nContext.Provider>
  );
}

import { lazy, type ComponentType, type LazyExoticComponent } from 'react';

const RETRY_PREFIX = 'hubvas:lazy-retry:';

type ComponentModule<T extends ComponentType<object>> = { default: T };

export function lazyWithRetry<T extends ComponentType<object>>(
  key: string,
  importer: () => Promise<ComponentModule<T>>,
): LazyExoticComponent<T> {
  return lazy(async () => {
    const storageKey = `${RETRY_PREFIX}${key}`;
    try {
      const loaded = await importer();
      window.sessionStorage.removeItem(storageKey);
      return loaded;
    } catch (error) {
      if (window.sessionStorage.getItem(storageKey) !== 'reloaded') {
        window.sessionStorage.setItem(storageKey, 'reloaded');
        window.location.reload();
        // Keep Suspense active while the browser reloads instead of briefly
        // rendering an error state from the old document.
        return await new Promise<ComponentModule<T>>(() => undefined);
      }
      throw error;
    }
  });
}

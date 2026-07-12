import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, test, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { RouteErrorBoundary } from './RouteErrorBoundary';

function BrokenRoute(): never {
  throw new Error('chunk unavailable');
}

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => undefined);
  window.localStorage.setItem('hubvas-language', 'en');
});

test('shows a recoverable global fallback when a route chunk throws', () => {
  render(
    <MemoryRouter initialEntries={['/broken']}>
      <I18nProvider>
        <RouteErrorBoundary><BrokenRoute /></RouteErrorBoundary>
      </I18nProvider>
    </MemoryRouter>,
  );
  expect(screen.getByRole('heading', { name: 'This page could not be loaded' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retry loading' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Back to community' })).toBeInTheDocument();
});

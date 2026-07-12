import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Avatar } from './Avatar';

describe('Avatar', () => {
  it('falls back to stable initials when the image cannot load', () => {
    render(<Avatar name="Ada Lovelace" src="https://cdn.example/avatar.webp" />);

    fireEvent.error(screen.getByRole('img', { name: 'Ada Lovelace' }));

    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByText('AL')).toBeInTheDocument();
  });

  it('retries image rendering when the source changes', () => {
    const { rerender } = render(<Avatar name="Grace Hopper" src="/broken.webp" />);
    fireEvent.error(screen.getByRole('img', { name: 'Grace Hopper' }));
    expect(screen.getByText('GH')).toBeInTheDocument();

    rerender(<Avatar name="Grace Hopper" src="/new.webp" />);

    expect(screen.getByRole('img', { name: 'Grace Hopper' })).toHaveAttribute('src', '/new.webp');
  });
});

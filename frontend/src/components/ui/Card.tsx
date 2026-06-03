import { type HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hover?: boolean;
  padding?: boolean;
}

export function Card({
  children,
  hover = false,
  padding = true,
  className = '',
  ...props
}: CardProps) {
  return (
    <div
      className={`
        bg-white rounded-xl border border-gray-200 shadow-sm
        ${hover ? 'hover:shadow-md hover:border-gray-300 transition-shadow cursor-pointer' : ''}
        ${padding ? 'p-4' : ''}
        ${className}
      `.trim()}
      {...props}
    >
      {children}
    </div>
  );
}

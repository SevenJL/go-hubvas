import { createContext, useContext } from 'react';

export interface ToastOptions {
  title: string;
  message?: string;
  duration?: number;
}

export interface ToastContextValue {
  success: (options: ToastOptions | string) => void;
  error: (options: ToastOptions | string) => void;
  info: (options: ToastOptions | string) => void;
}

export const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) throw new Error('useToast must be used inside ToastProvider');
  return context;
}

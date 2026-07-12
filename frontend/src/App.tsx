import { Suspense, type ComponentType, type ReactNode } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './store/AuthContext';
import { PageLoader, ToastProvider } from './components/ui';
import { I18nProvider, useI18n } from './i18n';
import { RouteErrorBoundary } from './components/routing/RouteErrorBoundary';
import { lazyWithRetry } from './utils/lazyWithRetry';

const route = <T extends ComponentType<object>>(key: string, importer: () => Promise<{ default: T }>) => lazyWithRetry(key, importer);

const Login = route('login', () => import('./pages/Login').then(module => ({ default: module.Login })));
const Register = route('register', () => import('./pages/Register').then(module => ({ default: module.Register })));
const Community = route('community', () => import('./pages/Community').then(module => ({ default: module.Community })));
const CanvasDetail = route('canvas-detail', () => import('./pages/CanvasDetail').then(module => ({ default: module.CanvasDetail })));
const PublicProfilePage = route('public-profile', () => import('./pages/PublicProfile').then(module => ({ default: module.PublicProfilePage })));
const Dashboard = route('dashboard', () => import('./pages/Dashboard').then(module => ({ default: module.Dashboard })));
const Profile = route('profile', () => import('./pages/Profile').then(module => ({ default: module.Profile })));
const Notifications = route('notifications', () => import('./pages/Notifications').then(module => ({ default: module.Notifications })));
const Blocks = route('blocks', () => import('./pages/Blocks').then(module => ({ default: module.Blocks })));

// Keep privileged moderation code out of the public application bundle.
const Admin = route('admin', () => import('./pages/Admin').then(module => ({ default: module.Admin })));

// The editor owns the large tldraw/Yjs dependency graph and is downloaded only
// after an authenticated user opens an edit route.
const Editor = route('editor', () => import('./pages/Editor').then(module => ({ default: module.Editor })));

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const { t } = useI18n();
  if (loading) return <PageLoader label={t('Restoring your session...')} />;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RouteFallback() {
  const { t } = useI18n();
  return <PageLoader label={t('Loading workspace...')} />;
}

function AppRoutes() {
  return (
    <RouteErrorBoundary>
      <Suspense fallback={<RouteFallback />}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/community" element={<Community />} />
        <Route path="/canvas/:id" element={<CanvasDetail />} />
        <Route path="/users/:username" element={<PublicProfilePage />} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile"
          element={
            <ProtectedRoute>
              <Profile />
            </ProtectedRoute>
          }
        />
        <Route path="/notifications" element={<ProtectedRoute><Notifications /></ProtectedRoute>} />
        <Route path="/admin" element={<ProtectedRoute><Admin /></ProtectedRoute>} />
        <Route path="/blocks" element={<ProtectedRoute><Blocks /></ProtectedRoute>} />
        <Route
          path="/canvas/:id/edit"
          element={
            <ProtectedRoute>
              <Editor />
            </ProtectedRoute>
          }
        />
        <Route path="/" element={<Navigate to="/community" replace />} />
        <Route path="*" element={<Navigate to="/community" replace />} />
      </Routes>
      </Suspense>
    </RouteErrorBoundary>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <I18nProvider>
        <ToastProvider>
          <AuthProvider>
            <AppRoutes />
          </AuthProvider>
        </ToastProvider>
      </I18nProvider>
    </BrowserRouter>
  );
}

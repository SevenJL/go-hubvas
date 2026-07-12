import { lazy, Suspense, type ReactNode } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './store/AuthContext';
import { PageLoader, ToastProvider } from './components/ui';
import { I18nProvider, useI18n } from './i18n';

const Login = lazy(() => import('./pages/Login').then(module => ({ default: module.Login })));
const Register = lazy(() => import('./pages/Register').then(module => ({ default: module.Register })));
const Community = lazy(() => import('./pages/Community').then(module => ({ default: module.Community })));
const CanvasDetail = lazy(() => import('./pages/CanvasDetail').then(module => ({ default: module.CanvasDetail })));
const PublicProfilePage = lazy(() => import('./pages/PublicProfile').then(module => ({ default: module.PublicProfilePage })));
const Dashboard = lazy(() => import('./pages/Dashboard').then(module => ({ default: module.Dashboard })));
const Profile = lazy(() => import('./pages/Profile').then(module => ({ default: module.Profile })));
const Notifications = lazy(() => import('./pages/Notifications').then(module => ({ default: module.Notifications })));
const Blocks = lazy(() => import('./pages/Blocks').then(module => ({ default: module.Blocks })));

// Keep privileged moderation code out of the public application bundle.
const Admin = lazy(() => import('./pages/Admin').then(module => ({ default: module.Admin })));

// The editor owns the large tldraw/Yjs dependency graph and is downloaded only
// after an authenticated user opens an edit route.
const Editor = lazy(() => import('./pages/Editor').then(module => ({ default: module.Editor })));

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

import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './store/AuthContext';
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Dashboard } from './pages/Dashboard';
import { Editor } from './pages/Editor';
import { Community } from './pages/Community';
import { CanvasDetail } from './pages/CanvasDetail';
import { Profile } from './pages/Profile';
import { PublicProfilePage } from './pages/PublicProfile';
import { Notifications } from './pages/Notifications';
import { Admin } from './pages/Admin';
import { Blocks } from './pages/Blocks';
import { PageLoader, ToastProvider } from './components/ui';
import { I18nProvider, useI18n } from './i18n';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const { t } = useI18n();
  if (loading) return <PageLoader label={t('Restoring your session...')} />;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function AppRoutes() {
  return (
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

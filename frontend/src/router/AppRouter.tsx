import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import Login from '../pages/Login';
import Register from '../pages/Register';
import Dashboard from '../pages/Dashboard';
import PendingConsents from '../pages/PendingConsents';
import ConsentHistory from '../pages/ConsentHistory';
import AuditLog from '../pages/AuditLog';
import Profile from '../pages/Profile';
import { useAuth } from '../context/AuthContext';

/** Redirect /login to /dashboard when a session already exists. */
function GuestRoute({ children }: { children: React.ReactNode }) {
  const { status } = useAuth();
  const location = useLocation();
  if (status === 'authenticated') {
    return <Navigate to="/dashboard" replace state={{ from: location.pathname }} />;
  }
  return <>{children}</>;
}

export default function AppRouter() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <GuestRoute>
            <Login />
          </GuestRoute>
        }
      />
      <Route
        path="/register"
        element={
          <GuestRoute>
            <Register />
          </GuestRoute>
        }
      />
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/consent/pending"
        element={
          <ProtectedRoute>
            <PendingConsents />
          </ProtectedRoute>
        }
      />
      <Route
        path="/consent/history"
        element={
          <ProtectedRoute>
            <ConsentHistory />
          </ProtectedRoute>
        }
      />
      <Route
        path="/audit"
        element={
          <ProtectedRoute>
            <AuditLog />
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
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}

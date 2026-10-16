import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/useAuth';
import AuthError from './AuthError';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { authEnabled, isAuthenticated, loading, error } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background text-muted-foreground">
        加载中...
      </div>
    );
  }

  if (error) return <AuthError />;

  if (authEnabled && !isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

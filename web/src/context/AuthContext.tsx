import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import client from '../api/client';

interface AuthMeData {
  auth_enabled: boolean;
  username?: string;
}

interface AuthContextValue {
  authEnabled: boolean;
  isAuthenticated: boolean;
  username: string | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [authEnabled, setAuthEnabled] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [username, setUsername] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const applyAuthState = useCallback((data: AuthMeData) => {
    if (!data.auth_enabled) {
      setAuthEnabled(false);
      setIsAuthenticated(true);
      setUsername(null);
      return;
    }

    setAuthEnabled(true);
    setIsAuthenticated(true);
    setUsername(data.username ?? null);
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await client.get<{ data: AuthMeData }>('/auth/me');
      applyAuthState(res.data.data);
    } catch (error) {
      if (axiosIsUnauthorized(error)) {
        setAuthEnabled(true);
        setIsAuthenticated(false);
        setUsername(null);
      } else {
        setAuthEnabled(false);
        setIsAuthenticated(true);
        setUsername(null);
      }
    } finally {
      setLoading(false);
    }
  }, [applyAuthState]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const login = useCallback(async (inputUsername: string, password: string) => {
    const res = await client.post<{ data: AuthMeData }>('/auth/login', {
      username: inputUsername,
      password,
    });
    applyAuthState(res.data.data);
  }, [applyAuthState]);

  const logout = useCallback(async () => {
    await client.post('/auth/logout');
    setAuthEnabled(true);
    setIsAuthenticated(false);
    setUsername(null);
  }, []);

  const value = useMemo(
    () => ({
      authEnabled,
      isAuthenticated,
      username,
      loading,
      login,
      logout,
      refresh,
    }),
    [authEnabled, isAuthenticated, username, loading, login, logout, refresh]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}

function axiosIsUnauthorized(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'response' in error &&
    typeof (error as { response?: { status?: number } }).response?.status === 'number' &&
    (error as { response?: { status?: number } }).response?.status === 401
  );
}

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import client from '../api/client';
import { AuthContext } from './auth';
import type { AuthConfigData, AuthMeData } from './auth';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authEnabled, setAuthEnabled] = useState(true);
  const [passwordEnabled, setPasswordEnabled] = useState(false);
  const [oauthEnabled, setOauthEnabled] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [username, setUsername] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const applyAuthState = useCallback((data: AuthMeData) => {
    if (typeof data?.auth_enabled !== 'boolean' || (data.auth_enabled && (typeof data.username !== 'string' || !data.username))) {
      throw new Error('无效的登录状态');
    }
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
    setError(null);
    let configLoaded = false;
    try {
      const config = await client.get<{ data: AuthConfigData }>('/auth/config');
      const data = config.data.data;
      if (typeof data?.auth_enabled !== 'boolean' || typeof data.password_enabled !== 'boolean' || typeof data.oauth_enabled !== 'boolean') {
        throw new Error('无效的登录配置');
      }
      configLoaded = true;
      setAuthEnabled(data.auth_enabled);
      setPasswordEnabled(data.password_enabled);
      setOauthEnabled(data.oauth_enabled);
      if (!data.auth_enabled) {
        applyAuthState({ auth_enabled: false });
        return;
      }
      const res = await client.get<{ data: AuthMeData }>('/auth/me');
      applyAuthState(res.data.data);
    } catch (error) {
      if (configLoaded && axiosIsUnauthorized(error)) {
        setAuthEnabled(true);
        setIsAuthenticated(false);
        setUsername(null);
      } else {
        setAuthEnabled(true);
        setIsAuthenticated(false);
        setUsername(null);
        setError('无法获取登录状态，请检查网络后重试');
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
      passwordEnabled,
      oauthEnabled,
      error,
      isAuthenticated,
      username,
      loading,
      login,
      logout,
      refresh,
    }),
    [authEnabled, passwordEnabled, oauthEnabled, error, isAuthenticated, username, loading, login, logout, refresh]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
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

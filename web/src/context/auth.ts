import { createContext } from 'react';

export interface AuthMeData {
  auth_enabled: boolean;
  username?: string;
  auth_method?: 'password' | 'oauth';
}

export interface AuthConfigData {
  auth_enabled: boolean;
  password_enabled: boolean;
  oauth_enabled: boolean;
}

export interface AuthContextValue {
  authEnabled: boolean;
  passwordEnabled: boolean;
  oauthEnabled: boolean;
  error: string | null;
  isAuthenticated: boolean;
  username: string | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

import { createContext } from 'react';

export interface AuthMeData {
  auth_enabled: boolean;
  username?: string;
}

export interface AuthContextValue {
  authEnabled: boolean;
  isAuthenticated: boolean;
  username: string | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

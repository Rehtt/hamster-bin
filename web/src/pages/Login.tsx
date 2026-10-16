import { useEffect, useState, type FormEvent } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import toast from 'react-hot-toast';
import { useAuth } from '../context/useAuth';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card';
import AuthError from '../components/AuthError';

export default function Login() {
  const { authEnabled, passwordEnabled, oauthEnabled, isAuthenticated, loading, error, login } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [oauthError] = useState(() => searchParams.get('oauth_error'));
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (searchParams.has('oauth_error')) {
      const next = new URLSearchParams(searchParams);
      next.delete('oauth_error');
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background text-muted-foreground">
        加载中...
      </div>
    );
  }

  if (error) return <AuthError />;

  if (!authEnabled || isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setSubmitting(true);
    try {
      await login(username, password);
      toast.success('登录成功');
    } catch (error) {
      const message =
        typeof error === 'object' &&
        error !== null &&
        'response' in error &&
        typeof (error as { response?: { data?: { error?: string } } }).response?.data?.error === 'string'
          ? (error as { response?: { data?: { error?: string } } }).response?.data?.error
          : '登录失败';
      toast.error(message || '登录失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="flex justify-center mb-4">
            <img src="/logo.svg" alt="Logo" className="h-12 w-12" />
          </div>
          <CardTitle>登录库存管理系统</CardTitle>
        </CardHeader>
        <CardContent>
          {oauthError && (
            <p role="alert" className="text-sm text-destructive mb-4">
              {oauthError === 'access_denied'
                ? '授权未完成或没有应用访问权，请重试或联系管理员'
                : oauthError === 'unavailable'
                  ? '统一登录暂不可用，请稍后重试'
                  : '登录请求已失效或验证失败，请重新登录'}
            </p>
          )}
          {oauthEnabled && (
            <Button className="w-full" onClick={() => { window.location.assign('/api/v1/auth/oauth/login'); }}>
              使用统一账号登录
            </Button>
          )}
          {oauthEnabled && passwordEnabled && <p className="text-center text-sm text-muted-foreground my-4">或使用管理员账号</p>}
          {passwordEnabled && <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <label htmlFor="username" className="text-sm font-medium">
                用户名
              </label>
              <Input
                id="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="password" className="text-sm font-medium">
                密码
              </label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? '登录中...' : '登录'}
            </Button>
          </form>}
        </CardContent>
      </Card>
    </div>
  );
}

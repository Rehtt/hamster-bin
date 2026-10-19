import { useEffect, useState, type FormEvent } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import toast from 'react-hot-toast';
import { useAuth } from '../context/useAuth';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card';
import { ThemeSelect } from '../components/ThemeSelect';
import { Package, ScanLine, ArrowRight } from 'lucide-react';
import AuthError from '../components/AuthError';

export default function Login() {
  const { authEnabled, passwordEnabled, oauthEnabled, isAuthenticated, loading, error, login } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [oauthError] = useState(() => searchParams.get('oauth_error'));
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { document.title = '登录 · Hamster Bin'; }, []);

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
    if (submitting) return;
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
    <div className="relative grid min-h-dvh bg-background lg:grid-cols-2">
      <div className="absolute right-5 top-5 z-10"><ThemeSelect /></div>
      <section className="relative hidden flex-col justify-between overflow-hidden border-r bg-card p-12 lg:flex"><LinkBrand /><div className="max-w-lg"><div className="mb-8 flex size-20 items-center justify-center rounded-3xl bg-primary/10 text-primary"><Package className="size-10" strokeWidth={1.25} /></div><p className="text-xs font-medium tracking-[0.25em] text-primary">A PLACE FOR EVERY COMPONENT</p><h1 className="mt-5 text-4xl font-semibold leading-snug tracking-tight">每一颗元件，<br />都有自己的位置。</h1><p className="mt-6 max-w-sm text-base leading-8 text-muted-foreground">整理你的元件库，从容开始下一次创作。查询、采购与库存记录，在一个工作空间完成。</p><div className="mt-8 inline-flex items-center gap-3 rounded-lg border bg-background px-4 py-3 text-sm text-muted-foreground"><ScanLine className="size-5 text-primary" />扫码或拍照，让入库更轻松<ArrowRight className="size-4" /></div></div><p className="text-xs text-muted-foreground">Hamster Bin · 电子元件库存管理</p></section>
      <div className="flex items-center justify-center px-6 py-20"><Card className="w-full max-w-[400px] border-0 bg-transparent shadow-none">
        <CardHeader className="text-center">
          <div className="flex justify-center mb-4">
            <img src="/logo.svg" alt="Hamster Bin" className="h-12 w-12" />
          </div>
          <CardTitle className="text-2xl">欢迎回到 Hamster Bin</CardTitle><p className="pt-2 text-sm text-muted-foreground">登录库存管理系统，继续管理你的元件</p>
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
      </Card></div>
    </div>
  );
}

function LinkBrand() { return <div className="flex items-center gap-3"><img src="/logo.svg" alt="" className="size-10" /><span className="text-xl font-bold tracking-tight">Hamster Bin</span></div>; }

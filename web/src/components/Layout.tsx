import { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import * as Dialog from '@radix-ui/react-dialog';
import { LayoutDashboard, Package, ClipboardList, FolderTree, History, Menu, X, PanelLeftClose, PanelLeftOpen, LogOut, Sparkles, ArrowUpRight } from 'lucide-react';
import { cn } from '../utils/cn';
import { useAuth } from '../context/useAuth';
import { ThemeSelect } from './ThemeSelect';
import { Button } from './ui/Button';
import { useTasks, taskPending } from '../context/tasks';
const navItems = [
  { name: '仪表盘', href: '/', icon: LayoutDashboard },
  { name: '元件管理', href: '/components', icon: Package },
  { name: '预入库', href: '/pre-stocks', icon: ClipboardList },
  { name: '分类管理', href: '/categories', icon: FolderTree },
  { name: '任务中心', href: '/tasks', icon: Sparkles },
  { name: '库存记录', href: '/logs', icon: History },
];
export default function Layout({ children }: { children: React.ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(() => { try { return localStorage.getItem('hamster-sidebar-collapsed') === 'true'; } catch { return false; } });
  const location = useLocation();
  const navigate = useNavigate();
  const { authEnabled, isAuthenticated, username, logout } = useAuth();
  const { tasks } = useTasks();
  const running = Object.values(tasks).filter(taskPending).length;
  const current = navItems.find(item => item.href === location.pathname)?.name || '库存管理';
  useEffect(() => { document.title = `${current} · Hamster Bin`; }, [current]);
  const toggle = () => { setCollapsed(value => !value); try { localStorage.setItem('hamster-sidebar-collapsed', String(!collapsed)); } catch { /* 保留会话选择 */ } };
  const nav = (compact: boolean) => <>
    <Link to="/" onClick={() => setMobileOpen(false)} className={cn('flex h-20 items-center gap-3 border-b px-5', compact && 'justify-center px-2')}><img src="/logo.svg" alt="" className="size-9 shrink-0" />{!compact && <div><span className="text-lg font-bold tracking-tight">Hamster Bin</span><p className="mt-0.5 text-[11px] text-muted-foreground">每一颗元件，都井井有条</p></div>}</Link>
    <nav aria-label="主导航" className={cn('flex-1 space-y-1 px-3 py-6', compact && 'px-2')}>{!compact && <p className="mb-3 px-3 text-[10px] font-semibold tracking-widest text-muted-foreground">工作空间</p>}{navItems.map(item => <Link key={item.href} to={item.href} aria-current={location.pathname === item.href ? 'page' : undefined} title={compact ? item.name : undefined} onClick={() => setMobileOpen(false)} className={cn('flex min-h-11 items-center gap-3 rounded-md px-3 transition-colors', compact && 'justify-center px-0', location.pathname === item.href ? 'bg-primary/10 font-medium text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground')}><item.icon className="size-[18px] shrink-0" />{!compact && <><span>{item.name}</span>{item.href === '/tasks' && running > 0 && <span className="ml-auto rounded-full bg-primary px-1.5 text-xs text-primary-foreground">{running}</span>}</>}</Link>)}</nav>
    {!compact && <div className="mx-4 mb-5 rounded-lg border border-primary/10 bg-primary/5 p-4"><p className="text-xs font-medium">从识别到入库</p><p className="mt-2 text-xs leading-5 text-muted-foreground">扫码或拍照，快速整理你的元件。</p><Link to="/components?new=1" onClick={() => setMobileOpen(false)} className="mt-3 flex items-center gap-1 text-xs font-medium text-primary">快速录入<ArrowUpRight className="size-3" /></Link></div>}
    {authEnabled && isAuthenticated && <div className="border-t p-3">{!compact && <p className="truncate px-3 pb-2 text-xs text-muted-foreground">{username || '管理员'}</p>}<Button variant="ghost" className={cn('w-full justify-start text-muted-foreground', compact && 'justify-center px-0')} aria-label="退出登录" onClick={async () => { await logout(); navigate('/login'); }}><LogOut className="size-4" />{!compact && <span className="ml-3">退出登录</span>}</Button></div>}
  </>;
  return <div className="min-h-dvh">
    <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[90] focus:rounded-md focus:bg-card focus:p-3">跳到主要内容</a>
    <aside className={cn('fixed inset-y-0 left-0 z-30 hidden flex-col border-r bg-card md:flex', collapsed ? 'w-[72px]' : 'w-60')}>{nav(collapsed)}<Button variant="ghost" className="m-3 border" onClick={toggle} aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}>{collapsed ? <PanelLeftOpen className="size-4" /> : <><PanelLeftClose className="mr-2 size-4" />收起侧边栏</>}</Button></aside>
    <div className={cn('flex min-h-dvh min-w-0 flex-col', collapsed ? 'md:ml-[72px]' : 'md:ml-60')}>
      <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center gap-3 border-b bg-card/95 px-4 backdrop-blur-sm sm:px-8">
        <Dialog.Root open={mobileOpen} onOpenChange={setMobileOpen}><Dialog.Trigger asChild><Button variant="ghost" size="icon" aria-label="打开导航" className="md:hidden"><Menu className="size-5" /></Button></Dialog.Trigger><Dialog.Portal><Dialog.Overlay className="fixed inset-0 z-50 bg-black/40 md:hidden" /><Dialog.Content aria-describedby={undefined} className="fixed inset-y-0 left-0 z-50 flex w-72 max-w-[85vw] flex-col bg-card shadow-xl md:hidden"><Dialog.Title className="sr-only">导航</Dialog.Title><Dialog.Close asChild><Button variant="ghost" size="icon" aria-label="关闭导航" className="absolute right-2 top-20"><X className="size-4" /></Button></Dialog.Close>{nav(false)}</Dialog.Content></Dialog.Portal></Dialog.Root>
        <span className="hidden text-xs text-muted-foreground sm:inline">工作空间</span><span className="hidden text-muted-foreground/40 sm:inline">/</span><span className="text-sm font-medium">{current}</span>
        <div className="ml-auto flex items-center gap-2"><Link to="/tasks" className="mr-1 flex items-center gap-2 rounded-md px-2 py-2 text-xs text-muted-foreground hover:bg-muted"><span className={cn('size-1.5 rounded-full', running ? 'animate-pulse bg-primary' : 'bg-emerald-500')} />{running ? `${running} 项解析中` : '任务中心'}</Link><ThemeSelect /><div className="flex size-8 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary" title={username || '本地工作空间'}>{username?.slice(0, 1).toUpperCase() || 'HB'}</div></div>
      </header>
      <main id="main-content" className="mx-auto w-full min-w-0 max-w-[1800px] flex-1 p-4 sm:p-6 lg:p-8">{children}</main>
      <footer className="flex justify-between px-4 py-5 text-[11px] text-muted-foreground sm:px-8"><span>Hamster Bin · 电子元件库存管理</span><span>让创作井然有序</span></footer>
    </div>
  </div>;
}

import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LayoutDashboard, Package, FolderTree, History, Menu, X, ChevronLeft, ChevronRight } from 'lucide-react';
import { cn } from '../utils/cn';

const SIDEBAR_COLLAPSED_KEY = 'hamster-sidebar-collapsed';

const navItems = [
  { name: '仪表盘', href: '/', icon: LayoutDashboard },
  { name: '元件管理', href: '/components', icon: Package },
  { name: '分类管理', href: '/categories', icon: FolderTree },
  { name: '库存记录', href: '/logs', icon: History },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(
    () => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true'
  );
  const location = useLocation();

  const toggleCollapsed = () => {
    setIsCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next));
      return next;
    });
  };

  return (
    <div className="min-h-screen bg-background text-foreground flex">
      {/* Sidebar */}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex flex-col w-64 transform bg-card border-r border-border transition-all duration-200 ease-in-out md:translate-x-0 md:static md:inset-auto",
          isCollapsed ? "md:w-16" : "md:w-64",
          isSidebarOpen ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div
          className={cn(
            "flex items-center justify-between p-4 border-b border-border",
            isCollapsed && "md:justify-center md:px-2"
          )}
        >
          <div className={cn("flex items-center gap-2", isCollapsed && "md:gap-0")}>
            <img src="/logo.svg" alt="Logo" className="h-8 w-8 shrink-0" />
            <h1
              className={cn(
                "text-xl font-bold bg-gradient-to-r from-primary to-secondary bg-clip-text text-transparent whitespace-nowrap",
                isCollapsed && "md:hidden"
              )}
            >
              库存管理系统
            </h1>
          </div>
          <button onClick={() => setIsSidebarOpen(false)} className="md:hidden">
            <X className="h-6 w-6" />
          </button>
        </div>
        <nav className="flex-1 p-4 space-y-2">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = location.pathname === item.href;
            return (
              <Link
                key={item.name}
                to={item.href}
                title={isCollapsed ? item.name : undefined}
                className={cn(
                  "flex items-center gap-3 px-3 py-2 rounded-md transition-colors",
                  isCollapsed && "md:justify-center md:px-2 md:gap-0",
                  isActive
                    ? "bg-primary/10 text-primary"
                    : "hover:bg-accent hover:text-accent-foreground text-muted-foreground"
                )}
                onClick={() => setIsSidebarOpen(false)}
              >
                <Icon className="h-5 w-5 shrink-0" />
                <span className={cn(isCollapsed && "md:hidden")}>{item.name}</span>
              </Link>
            );
          })}
        </nav>
        <button
          type="button"
          onClick={toggleCollapsed}
          aria-label={isCollapsed ? '展开侧边栏' : '收起侧边栏'}
          className="hidden md:flex items-center justify-center p-4 border-t border-border hover:bg-accent hover:text-accent-foreground text-muted-foreground transition-colors"
        >
          {isCollapsed ? <ChevronRight className="h-5 w-5" /> : <ChevronLeft className="h-5 w-5" />}
        </button>
      </aside>

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="md:hidden p-4 border-b border-border bg-card flex items-center">
          <button onClick={() => setIsSidebarOpen(true)}>
            <Menu className="h-6 w-6" />
          </button>
          <span className="ml-4 font-semibold">
            {navItems.find((item) => item.href === location.pathname)?.name || '库存管理'}
          </span>
        </header>
        <main className="flex-1 p-6 overflow-auto">
          {children}
        </main>
      </div>

      {/* Overlay for mobile sidebar */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-30 md:hidden"
          onClick={() => setIsSidebarOpen(false)}
        />
      )}
    </div>
  );
}

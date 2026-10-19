import { useEffect, useState, type ReactNode } from 'react';
import { ThemeContext, type Theme } from './theme';
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    try { const value = localStorage.getItem('hamster-theme'); return value === 'dark' || value === 'system' ? value : 'light'; } catch { return 'light'; }
  });
  useEffect(() => {
    const media = matchMedia('(prefers-color-scheme: dark)');
    const apply = () => {
      const dark = theme === 'dark' || (theme === 'system' && media.matches);
      document.documentElement.classList.toggle('dark', dark);
      document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#10131e' : '#f6f8fb');
    };
    apply();
    try { localStorage.setItem('hamster-theme', theme); } catch { /* 隐私模式下仍支持切换 */ }
    media.addEventListener('change', apply);
    return () => media.removeEventListener('change', apply);
  }, [theme]);
  return <ThemeContext.Provider value={{ theme, setTheme }}>{children}</ThemeContext.Provider>;
}

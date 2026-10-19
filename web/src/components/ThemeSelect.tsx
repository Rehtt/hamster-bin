import { Monitor, Moon, Sun } from 'lucide-react';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { useTheme, type Theme } from '../context/theme';
import { Button } from './ui/Button';
const options = [{ value: 'light', label: '浅色模式', icon: Sun }, { value: 'dark', label: '深色模式', icon: Moon }, { value: 'system', label: '跟随系统', icon: Monitor }] as const;
export function ThemeSelect() {
  const { theme, setTheme } = useTheme();
  const Icon = options.find(option => option.value === theme)!.icon;
  return <DropdownMenu.Root><DropdownMenu.Trigger asChild><Button variant="ghost" size="icon" aria-label="切换主题"><Icon className="size-4" /></Button></DropdownMenu.Trigger><DropdownMenu.Portal><DropdownMenu.Content align="end" sideOffset={8} className="z-[65] min-w-40 rounded-lg border bg-card p-1.5 shadow-lg"><DropdownMenu.RadioGroup value={theme} onValueChange={value => setTheme(value as Theme)}>{options.map(option => <DropdownMenu.RadioItem key={option.value} value={option.value} className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2.5 text-sm outline-none data-[highlighted]:bg-accent data-[state=checked]:text-primary"><option.icon className="size-4" />{option.label}<DropdownMenu.ItemIndicator className="ml-auto">✓</DropdownMenu.ItemIndicator></DropdownMenu.RadioItem>)}</DropdownMenu.RadioGroup></DropdownMenu.Content></DropdownMenu.Portal></DropdownMenu.Root>;
}

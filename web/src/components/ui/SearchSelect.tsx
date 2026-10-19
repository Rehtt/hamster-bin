import { useEffect, useState } from 'react';
import * as Popover from '@radix-ui/react-popover';
import { Command } from 'cmdk';
import { Check, ChevronsUpDown } from 'lucide-react';
import client from '../../api/client';
import type { Category, Supplier } from '../../types';
import { Button } from './Button';

type Option = { value: string; label: string };
type Source = 'categories' | 'active-categories' | 'suppliers' | 'manufacturers' | 'packages' | 'locations';

// options 仅用于展示已有值；候选项始终由数据库查询，不使用 cmdk 本地过滤。
export function SearchSelect({ id, label, value, options, source, keepCategoryID, onChange, creatable = false, placeholder = '请选择' }: {
  id?: string; label: string; value: string; options: Option[]; source: Source; keepCategoryID?: string;
  onChange: (value: string) => void; creatable?: boolean; placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [revision, setRevision] = useState(0);
  const [selected, setSelected] = useState<Option>();
  const [result, setResult] = useState<{ key: string; options: Option[]; error?: string }>();
  const params = new URLSearchParams({ keyword: search.trim() });
  const categories = source === 'categories' || source === 'active-categories';
  if (source === 'active-categories') {
    params.set('status', 'active');
    if (keepCategoryID) params.set('keep_id', keepCategoryID);
  }
  if (!categories && source !== 'suppliers') params.set('field', source);
  const url = `${categories ? '/categories' : source === 'suppliers' ? '/suppliers' : '/components/options'}?${params}`;
  const key = `${url}:${revision}`;
  const loading = !result || result.key !== key;
  const remoteOptions = !loading ? result.options : [];
  const error = !loading ? result.error : undefined;

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const res = await client.get(url, { signal: controller.signal });
        let items: Option[];
        if (categories) {
          items = (res.data.data as Category[]).map(c => ({ value: String(c.id), label: c.name + (source === 'active-categories' && c.status !== 'active' ? '（未启用，仅保留原关联）' : '') }));
        } else if (source === 'suppliers') {
          items = (res.data.data as Supplier[]).map(s => ({ value: s.name, label: s.name }));
        } else {
          items = ((res.data.data[source] || []) as string[]).map(value => ({ value, label: value }));
        }
        if (!controller.signal.aborted) setResult({ key, options: items });
      } catch {
        if (!controller.signal.aborted) setResult({ key, options: [], error: '加载失败，请重试' });
      }
    }, search.trim() ? 300 : 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [open, url, key, categories, source, search]);

  const select = (next: Option) => { setSelected(next); onChange(next.value); setOpen(false); };
  const display = remoteOptions.find(option => option.value === value) || (selected?.value === value ? selected : undefined) || options.find(option => option.value === value);
  return <Popover.Root open={open} onOpenChange={next => { setOpen(next); setSearch(''); if (next) setRevision(value => value + 1); }}>
    <Popover.Trigger asChild><Button id={id} type="button" variant="outline" role="combobox" aria-label={label} aria-expanded={open} className="w-full justify-between gap-2 font-normal"><span className="truncate">{display?.label || value || placeholder}</span><ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" /></Button></Popover.Trigger>
    <Popover.Portal><Popover.Content align="start" sideOffset={5} className="z-[65] w-[var(--radix-popover-trigger-width)] min-w-48 rounded-lg border bg-popover p-1 shadow-lg">
      <Command label={label} shouldFilter={false}><Command.Input value={search} onValueChange={setSearch} placeholder={`搜索${label}`} className="h-10 w-full border-b bg-transparent px-3 text-sm outline-none" /><Command.List aria-busy={loading} className="max-h-64 overflow-y-auto p-1">
        {loading && <p role="status" className="p-3 text-sm text-muted-foreground">搜索中…</p>}
        {error && <div role="alert" className="p-3 text-sm">{error}<Button size="sm" variant="ghost" onClick={() => setRevision(value => value + 1)}>重试</Button></div>}
        {!loading && !error && !remoteOptions.length && <p className="p-3 text-sm text-muted-foreground">无匹配选项</p>}
        {!creatable && !search.trim() && <Command.Item value="全部" onSelect={() => select({ value: '', label: '' })} className="cursor-pointer rounded-md p-2 text-sm data-[selected=true]:bg-accent">全部</Command.Item>}
        {remoteOptions.map(option => <Command.Item key={option.value} value={option.label} onSelect={() => select(option)} className="flex cursor-pointer items-center justify-between rounded-md p-2 text-sm data-[selected=true]:bg-accent">{option.label}{option.value === value && <Check className="size-4 text-primary" />}</Command.Item>)}
        {creatable && !loading && !error && search.trim() && !remoteOptions.some(option => option.label.toLowerCase() === search.trim().toLowerCase()) && <Command.Item value={search} onSelect={() => select({ value: search.trim(), label: search.trim() })} className="cursor-pointer rounded-md p-2 text-sm text-primary data-[selected=true]:bg-accent">使用「{search.trim()}」</Command.Item>}
        {creatable && value && !search.trim() && <Command.Item value="清空" onSelect={() => select({ value: '', label: '' })} className="cursor-pointer rounded-md p-2 text-sm data-[selected=true]:bg-accent">清空</Command.Item>}
      </Command.List></Command>
    </Popover.Content></Popover.Portal>
  </Popover.Root>;
}

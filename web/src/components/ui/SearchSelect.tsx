import { useState } from 'react';
import * as Popover from '@radix-ui/react-popover';
import { Command } from 'cmdk';
import { Check, ChevronsUpDown } from 'lucide-react';
import { Button } from './Button';
// 选项值与展示文字分离，支持选择已有记录和表单内创建新名称。
export function SearchSelect({ id, label, value, options, onChange, creatable = false, placeholder = '请选择' }: {
  id?: string; label: string; value: string; options: { value: string; label: string }[];
  onChange: (value: string) => void; creatable?: boolean; placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const select = (next: string) => { onChange(next); setOpen(false); };
  return <Popover.Root open={open} onOpenChange={next => { setOpen(next); setSearch(''); }}>
    <Popover.Trigger asChild><Button id={id} type="button" variant="outline" role="combobox" aria-label={label} aria-expanded={open} className="w-full justify-between gap-2 font-normal"><span className="truncate">{options.find(option => option.value === value)?.label || value || placeholder}</span><ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" /></Button></Popover.Trigger>
    <Popover.Portal><Popover.Content align="start" sideOffset={5} className="z-[65] w-[var(--radix-popover-trigger-width)] min-w-48 rounded-lg border bg-popover p-1 shadow-lg">
      <Command label={label}><Command.Input value={search} onValueChange={setSearch} placeholder={`搜索${label}`} className="h-10 w-full border-b bg-transparent px-3 text-sm outline-none" /><Command.List className="max-h-64 overflow-y-auto p-1"><Command.Empty className="p-3 text-sm text-muted-foreground">无匹配选项</Command.Empty>
        {!creatable && <Command.Item value="全部" onSelect={() => select('')} className="cursor-pointer rounded-md p-2 text-sm data-[selected=true]:bg-accent">全部</Command.Item>}
        {options.map(option => <Command.Item key={option.value} value={option.label} onSelect={() => select(option.value)} className="flex cursor-pointer items-center justify-between rounded-md p-2 text-sm data-[selected=true]:bg-accent">{option.label}{option.value === value && <Check className="size-4 text-primary" />}</Command.Item>)}
        {creatable && search.trim() && !options.some(option => option.label.toLowerCase() === search.trim().toLowerCase()) && <Command.Item value={search} onSelect={() => select(search.trim())} className="cursor-pointer rounded-md p-2 text-sm text-primary data-[selected=true]:bg-accent">使用「{search.trim()}」</Command.Item>}
        {creatable && value && <Command.Item value="清空" onSelect={() => select('')} className="cursor-pointer rounded-md p-2 text-sm data-[selected=true]:bg-accent">清空</Command.Item>}
      </Command.List></Command>
    </Popover.Content></Popover.Portal>
  </Popover.Root>;
}

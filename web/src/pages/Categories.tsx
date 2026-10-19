import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Pencil, Trash2, FolderTree, Search } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { ApiResponse, Category } from '../types';
import { useResource } from '../hooks/useResource';
import { useListQuery } from '../hooks/useListQuery';
import { useFormGuard, useSubmitLock } from '../hooks/useFormGuard';
import { useConfirm } from '../context/confirm';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { Label } from '../components/ui/Label';
import { PageHeader } from '../components/ui/PageHeader';
import { PageState } from '../components/ui/PageState';
import { Pagination } from '../components/ui/Pagination';
import { RowActionsMenu } from '../components/ui/RowActionsMenu';
export default function Categories() {
  const resource = useResource<ApiResponse<Category[]>>('/categories');
  const { params, page, pageSize, updateQuery } = useListQuery();
  const keyword = params.get('keyword') || '';
  const [search, setSearch] = useState(keyword);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Category>();
  const [name, setName] = useState('');
  const [activeRow, setActiveRow] = useState<number | null>(null);
  const mutation = useSubmitLock();
  const guard = useFormGuard(name);
  const confirm = useConfirm();
  const categories = (resource.data?.data || []).filter(category => category.name.toLowerCase().includes(keyword.toLowerCase()));
  useEffect(() => { setSearch(keyword); }, [keyword]);
  useEffect(() => { const last = Math.max(1, Math.ceil(categories.length / pageSize)); if (resource.data && page > last) updateQuery({ page: last }, true); }, [categories.length, page, pageSize, updateQuery, resource.data]);
  const edit = (category?: Category) => { setEditing(category); setName(category?.name || ''); guard.reset(category?.name || ''); setOpen(true); };
  const save = () => mutation.run(async () => {
    if (!name.trim()) { toast.error('请输入分类名称'); document.getElementById('category-name')?.focus(); return; }
    try { if (editing) await client.put(`/categories/${editing.id}`, { name: name.trim() }); else await client.post('/categories', { name: name.trim() }); toast.success(editing ? '分类已更新' : '分类已添加'); setOpen(false); resource.refresh(); }
    catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '保存失败'); }
  });
  const remove = async (category: Category) => {
    if (!await confirm({ title: '删除分类？', message: `确定删除「${category.name}」？关联元件（包括零库存）、任意状态的预入库记录或子分类时无法删除。`, confirmLabel: '删除分类' })) return;
    await mutation.run(async () => { try { await client.delete(`/categories/${category.id}`); toast.success('分类已删除'); resource.refresh(); } catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '删除失败'); } });
  };
  return <div className="space-y-6"><PageHeader title="分类管理" actions={<Button onClick={() => edit()}><Plus className="mr-2 size-4" />添加分类</Button>} /><form className="surface flex gap-2 p-4" onSubmit={e => { e.preventDefault(); updateQuery({ keyword: search.trim(), page: 1 }); }}><div className="relative flex-1"><Search className="absolute left-3 top-3 size-4 text-muted-foreground" /><Input aria-label="搜索分类" className="pl-9" placeholder="搜索分类名称" value={search} onChange={e => setSearch(e.target.value)} /></div><Button type="submit">搜索</Button>{keyword && <Button variant="outline" onClick={() => updateQuery({ keyword: '', page: 1 })}>清空</Button>}</form><div className="surface"><PageState loading={resource.loading} error={resource.error} empty={!categories.length} filtered={!!keyword} onRetry={resource.refresh} onClear={() => updateQuery({ keyword: '', page: 1 })} />{!resource.loading && !resource.error && <div className="divide-y">{categories.slice((page - 1) * pageSize, page * pageSize).map(category => <article key={category.id} className="flex items-center gap-4 p-4 sm:px-6"><div className="rounded-lg bg-primary/5 p-3 text-primary"><FolderTree className="size-5" /></div><div className="min-w-0 flex-1"><p className="break-all font-medium">{category.name}</p><Link to={`/components?category_id=${category.id}`} className="mt-1 inline-block text-xs text-muted-foreground hover:text-primary">查看此分类元件 →</Link></div><RowActionsMenu rowId={category.id} activeRowId={activeRow} onActiveRowChange={setActiveRow} actions={[{ key: 'edit', icon: Pencil, label: '编辑', onClick: () => edit(category), disabled: mutation.busy }, { key: 'delete', icon: Trash2, label: '删除', destructive: true, onClick: () => void remove(category), disabled: mutation.busy }]} /></article>)}</div>}</div><Pagination pagination={{ page, page_size: pageSize, total: categories.length }} onPageChange={page => updateQuery({ page })} onPageSizeChange={page_size => updateQuery({ page_size, page: 1 })} disabled={resource.loading} /><Modal isOpen={open} onClose={() => setOpen(false)} title={editing ? '编辑分类' : '添加分类'} dirty={guard.dirty} busy={mutation.busy} footer={<><Button variant="outline" onClick={async () => { if (await guard.allowClose()) setOpen(false); }} disabled={mutation.busy}>取消</Button><Button type="submit" form="category-form" disabled={mutation.busy}>{mutation.busy ? '保存中…' : '保存'}</Button></>}><form id="category-form" onSubmit={e => { e.preventDefault(); void save(); }} className="space-y-2"><Label htmlFor="category-name">分类名称</Label><Input id="category-name" value={name} onChange={e => setName(e.target.value)} placeholder="例如：电阻、电容" required autoFocus /></form></Modal></div>;
}

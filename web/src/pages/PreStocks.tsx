import { inventoryApi } from '../api/inventory';
import { useConfirm } from '../context/confirm';
import { useListQuery } from '../hooks/useListQuery';
import { useResource } from '../hooks/useResource';
import { useFormGuard } from '../hooks/useFormGuard';
import { InventoryFields } from '../components/inventory/InventoryFields';
import { SmartEntry } from '../components/inventory/SmartEntry';
import { PageState } from '../components/ui/PageState';
import { SafeImage } from '../components/ui/SafeImage';
import { Pagination as PaginationControl } from '../components/ui/Pagination';
import type { ParseMode } from '../types';
import { useParseTask } from '../hooks/useParseTask';
import { useTaskApplication } from '../hooks/useTaskApplication';
import { mergeParsed, parsedComponentFields, matchingSupplierReference } from '../utils/parseResult';
import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { CheckCircle2, Edit, Loader2, Plus, QrCode, Search, Trash2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Category, type ComponentOptions, type Pagination, type PreStock, type PreStockStatus, type Supplier, type ParsedComponentInfo, type ReviewedImageItem, type ParseTask } from '../types';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { PageHeader } from '../components/ui/PageHeader';
import { RowActionsMenu } from '../components/ui/RowActionsMenu';
import { formatCents, yuanToCents } from '../utils/price';
import { copyToClipboard } from '../utils/clipboard';
import { cn } from '../utils/cn';
import { usePhotoDrafts } from '../hooks/usePhotoDrafts';
import { PhotoDraftNavigation } from '../components/PhotoDraftNavigation';
import { photoComponentInfo } from '../utils/photoResult';
import { PhotoParseModal } from '../components/PhotoParseModal';

const QRScanner = lazy(() => import('../components/QRScanner'));

const STATUS_FILTER_LABELS: Record<PreStockStatus | 'all', string> = {
  pending: '待入库',
  confirmed: '已入库',
  all: '全部',
};

type PreStockForm = {
  review_confirmations?: Record<string,string>;
  component_number: string;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  package: string;
  supplier_part_number: string;
  description: string;
  expected_quantity: number | null;
  total_price_yuan: string;
  location: string;
  datasheet_url: string;
  image_url: string;
};




const EMPTY_FORM: PreStockForm = {
  component_number: '',
  name: '',
  model: '',
  manufacturer: '',
  value: '',
  package: '',
  supplier_part_number: '',
  description: '',
  expected_quantity: 1,
  total_price_yuan: '',
  location: '',
  datasheet_url: '',
  image_url: '',
};

const getSupplierNameFromPlatform = (platformName?: string) => {
  if (!platformName) return '';
  if (platformName.includes('立创') || platformName.includes('LCSC')) return '嘉立创';
  return platformName;
};

const statusLabel = (status: PreStockStatus) => status === 'confirmed' ? '已入库' : '待入库';

export default function PreStocks() {
  const confirm = useConfirm();
  const { params, updateQuery, page, pageSize } = useListQuery();
  const status = params.get('status') === 'confirmed' || params.get('status') === 'all' ? params.get('status') as PreStockStatus | 'all' : 'pending';
  const resource = useResource<{ data: PreStock[]; pagination: Pagination }>(`/pre-stocks?page=${page}&page_size=${pageSize}&status=${status}`);
  const loading = resource.loading;

  const [items, setItems] = useState<PreStock[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const statusFilter = status;
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isScannerOpen, setIsScannerOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<PreStock | null>(null);
  const [previewImage, setPreviewImage] = useState<string | null>(null);
  const [form, setForm] = useState<PreStockForm>(EMPTY_FORM);
  const [categoryInput, setCategoryInput] = useState('');
  const [supplierInput, setSupplierInput] = useState('');
  const [manufacturerOptions, setManufacturerOptions] = useState<string[]>([]);
  const [packageOptions, setPackageOptions] = useState<string[]>([]);
  const [locationOptions, setLocationOptions] = useState<string[]>([]);
  const [platformCode, setPlatformCode] = useState('');
  const [supplierReference, setSupplierReference] = useState<ParsedComponentInfo>();
  const [parseMode, setParseMode] = useState<ParseMode>('standard');
  const [isPhotoParseOpen, setIsPhotoParseOpen] = useState(false);
  const photoDrafts = usePhotoDrafts<PhotoDraft>();
  const [recoveredPhotoTask, setRecoveredPhotoTask] = useState<ParseTask>();
  const savingRef = useRef(false);

  const [saving, setSaving] = useState(false);
  const [confirmingId, setConfirmingId] = useState<number | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [activeRowId, setActiveRowId] = useState<number | null>(null);

  const formGuard = useFormGuard({ form, category: categoryInput, supplier: supplierInput });
  const normalizationSession = useRef(0);
  const closeForm = () => { normalizationSession.current++; parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false); };
  const cancelForm = async () => { if (!saving && await formGuard.allowClose()) closeForm(); };
  const fetchCategories = async () => {
    const res = await client.get('/categories');
    setCategories(res.data.data || []);
  };

  const fetchSuppliers = async () => {
    const res = await client.get('/suppliers');
    setSuppliers(res.data.data || []);
  };

  const fetchComponentOptions = async () => {
    const res = await client.get('/components/options');
    const data = (res.data.data || {}) as ComponentOptions;
    setManufacturerOptions(data.manufacturers || []);
    setPackageOptions(data.packages || []);
    setLocationOptions(data.locations || []);
  };

  const fetchItems = async (nextPage = page, nextSize = pageSize, nextStatus = statusFilter) => {
    updateQuery({ page: nextPage, page_size: nextSize, status: nextStatus }); resource.refresh();
  };
  useEffect(() => {
    if (!resource.data) return;
    setItems(resource.data.data || []); setPagination(resource.data.pagination);
    const last = Math.max(1, Math.ceil(resource.data.pagination.total / pageSize));
    if (page > last) updateQuery({ page: last }, true);
  }, [resource.data, page, pageSize, updateQuery]);
  useEffect(() => {
    void Promise.all([fetchCategories(), fetchSuppliers(), fetchComponentOptions()]).catch(() => toast.error('加载表单选项失败，请刷新重试'));
  }, []);

  const openForm = (item?: PreStock) => {
    normalizationSession.current++;
    parseTask.reset(); setSupplierReference(undefined);
    photoDrafts.reset();
    setIsPhotoParseOpen(false); setRecoveredPhotoTask(undefined);
    if (item) {
      setEditingItem(item);
      const initialForm = {
        component_number: item.component_number || '',
        name: item.name || '',
        model: item.model || '',
        manufacturer: item.manufacturer || '',
        value: item.value || '',
        package: item.package || '',
        supplier_part_number: item.supplier_part_number || '',
        description: item.description || '',
        expected_quantity: item.expected_quantity ?? 1,
        total_price_yuan: item.total_price_cents ? String((item.total_price_cents / 100).toFixed(2)) : '',
        location: item.location || '',
        datasheet_url: item.datasheet_url || '',
        image_url: item.image_url || '',
      };
      setForm(initialForm);
      formGuard.reset({ form: initialForm, category: String(item.category_id), supplier: item.supplier?.name || '' });
      setCategoryInput(String(item.category_id));
      setSupplierInput(item.supplier?.name || '');
      setPlatformCode(item.supplier_part_number || '');
    } else {
      setEditingItem(null);
      setForm(EMPTY_FORM);
      formGuard.reset({ form: EMPTY_FORM, category: '', supplier: '' });
      setCategoryInput('');
      setSupplierInput('');
      setPlatformCode('');
    }
    setIsFormOpen(true);
  };

  const ensureCategoryID = async () => {
    const id = Number(categoryInput);
    if (categories.some(c => c.id === id && (c.status === 'active' || c.id === editingItem?.category_id))) return id;
    toast.error('请选择启用分类，需要新分类请进入分类管理'); return null;
  };

  const ensureSupplierID = async () => {
    const name = supplierInput.trim();
    if (!name) return null;
    const existing = suppliers.find(s => s.name.toLowerCase() === name.toLowerCase());
    if (existing) return existing.id;

    try {
      const res = await client.post('/suppliers', { name });
      const id = res.data.data?.id as number | undefined;
      if (!id) throw new Error('missing id');
      toast.success(`自动创建供应商: ${name}`);
      await fetchSuppliers();
      return id;
    } catch {
      toast.error('自动创建供应商失败');
      return null;
    }
  };

  const currentPhotoDraft = (): PhotoDraft => ({ ...parseSnapshot(), platformCode, parsed: supplierReference });
  const loadPhotoDraft = (draft: PhotoDraft) => {
    parseTask.reset(); setSupplierReference(draft.parsed); setForm(draft.form); setCategoryInput(draft.category);
    setSupplierInput(draft.supplier); setPlatformCode(draft.platformCode);
  };
  const switchPhotoDraft = (index: number) => loadPhotoDraft(photoDrafts.select(index, currentPhotoDraft()));
  const handleSubmit = async () => {
    if (savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    try {
      if (!editingItem && photoDrafts.drafts.length) {
        const drafts = photoDrafts.merge(currentPhotoDraft());
        const invalid = drafts.findIndex(draft => !draft.form.name.trim() || !draft.category.trim() || !Number.isInteger(draft.form.expected_quantity) || Number(draft.form.expected_quantity) < 0);
        if (invalid >= 0) {
          switchPhotoDraft(invalid);
          toast.error(`请补全第 ${invalid + 1} 条的名称、分类和非负整数数量`);
          return;
        }
        await client.post('/pre-stocks/batch', { items: drafts.map(draft => ({
          ...draft.form, total_price_yuan: undefined,
          component_number: draft.form.component_number.trim() || undefined,
          category_id: Number(draft.category), supplier_name: draft.supplier.trim(),
          total_price_cents: yuanToCents(draft.form.total_price_yuan),
        })) });
        toast.success(`已批量创建 ${drafts.length} 条预入库记录`);
        photoDrafts.reset(); parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false);
        await Promise.all([fetchItems(pagination.page, pagination.page_size), fetchComponentOptions(), fetchCategories(), fetchSuppliers()]);
        return;
      }
      if (!form.name.trim() || !Number.isInteger(form.expected_quantity) || Number(form.expected_quantity) < 0) {
        toast.error('请填写名称和非负整数预计数量'); return;
      }
      const categoryID = await ensureCategoryID();
      if (!categoryID) return;
      const supplierID = await ensureSupplierID();
      const totalPriceCents = yuanToCents(form.total_price_yuan);
      const payload = {
        review_confirmations: form.review_confirmations,
        category_id: categoryID,
        supplier_id: supplierID,
        component_number: form.component_number.trim() || undefined,
        name: form.name,
        model: form.model.trim(),
        manufacturer: form.manufacturer.trim(),
        value: form.value.trim(),
        package: form.package.trim(),
        supplier_part_number: form.supplier_part_number.trim(),
        description: form.description,
        expected_quantity: Number(form.expected_quantity),
        total_price_cents: totalPriceCents > 0 ? totalPriceCents : 0,
        location: form.location.trim(),
        datasheet_url: form.datasheet_url.trim(),
        image_url: form.image_url.trim(),
      };

      if (editingItem) {
        await inventoryApi.savePreStock(editingItem.id, payload);
        toast.success('预入库记录已更新');
      } else {
        const res = await inventoryApi.savePreStock(undefined, payload);
        const number = res.data.data?.component_number;
        toast.success(number ? `已生成编号 ${number}` : '预入库记录已创建');
      }

      parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false);
      await fetchItems(pagination.page, pagination.page_size);
      await fetchComponentOptions();
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      const message = err.response?.data?.error || '保存失败';
      const failedItem = /第 (\d+) 条/.exec(message);
      if (failedItem && photoDrafts.drafts[Number(failedItem[1]) - 1]) switchPhotoDraft(Number(failedItem[1]) - 1);
      toast.error(message);
    } finally {
      savingRef.current = false; setSaving(false);
    }
  };

  type ParseSnapshot = { platformCode: string; form: PreStockForm; category: string; supplier: string };
  type PhotoDraft = ParseSnapshot & { platformCode: string; parsed?: ParsedComponentInfo };
  const applyParsedComponent = (component: ParsedComponentInfo, quantity?: number, snapshot?: ParseSnapshot) => {
    if (!component.normalization) { const session = normalizationSession.current; void client.post('/normalization/preview', component).then(res => { if (session === normalizationSession.current) applyParsedComponent({ ...component, ...res.data.data.candidate, normalization: res.data.data.normalization }, quantity, snapshot); }).catch(() => toast.error('规范预览失败，请重试')); return; }

    setSupplierReference({...component, quantity});
    setPlatformCode(prev => !snapshot || prev === snapshot.platformCode ? component.platform_code || prev : prev);
    const parsed: Partial<PreStockForm> = parsedComponentFields(component);
    if (quantity !== undefined && quantity >= 0) parsed.expected_quantity = quantity;
    setForm(prev => mergeParsed(prev, parsed, snapshot?.form));
    if (component.category_name) setCategoryInput(prev => !prev && (!snapshot || prev === snapshot.category) && component.category_id ? String(component.category_id) : prev);
    const supplier = component.supplier_name || getSupplierNameFromPlatform(component.platform_name);
    if (supplier) setSupplierInput(prev => !prev && (!snapshot || prev === snapshot.supplier) ? supplier : prev);
  };
  const applyPhotoResults = (items: ReviewedImageItem[]) => {
    parseTask.reset(); setSupplierReference(undefined);
    const first = items[0];
    if (editingItem) {
      applyParsedComponent(photoComponentInfo(first), first.quantity);
      setSupplierInput(prev => prev || first.supplier_name);
      return;
    }
    const drafts: PhotoDraft[] = items.map((item, index) => ({
      parsed: {...photoComponentInfo(item),quantity:item.quantity},
      form: { ...mergeParsed(index === 0 ? form : EMPTY_FORM, parsedComponentFields(photoComponentInfo(item))), expected_quantity: index === 0 && form.expected_quantity ? form.expected_quantity : item.quantity, review_confirmations:item.review_confirmations },
      category: index === 0 && categoryInput ? categoryInput : String(item.category_id || ''), supplier: index === 0 && supplierInput ? supplierInput : item.supplier_name, platformCode: item.supplier_part_number,
    }));
    photoDrafts.start(drafts); loadPhotoDraft(drafts[0]);
    toast.success(`已载入 ${items.length} 条照片识别结果，请逐条确认后保存`);
  };
  const parseTask = useParseTask<ParseSnapshot>((result, snapshot) => { if (result.component) applyParsedComponent(result.component, result.quantity, snapshot); });
  const isParsing = parseTask.busy;
  const parseSnapshot = (): ParseSnapshot => ({ platformCode, form: { ...form }, category: categoryInput, supplier: supplierInput });
  const parsePlatform = async () => {
    const code = platformCode.trim();
    if (!code) { toast.error('请输入供应商料号'); return; }
    setSupplierReference(undefined);
    await parseTask.start({ type: 'component_parse', input: { code, parse_mode: parseMode }, source: 'pre-stocks', record_id: editingItem?.id }, parseSnapshot());
  };
  const handleScan = async (data: string) => {
    if (isParsing) return;
    setIsScannerOpen(false);
    let snapshot = parseSnapshot();
    if (!isFormOpen) { openForm(); snapshot = { platformCode: '', form: { ...EMPTY_FORM }, category: '', supplier: '' }; }
    setSupplierReference(undefined);
    await parseTask.start({ type: 'qrcode_parse', input: { qrcode_data: data, parse_mode: parseMode }, source: 'pre-stocks', record_id: isFormOpen ? editingItem?.id : undefined }, snapshot);
  };
  useTaskApplication<PreStock>('pre-stocks', (task, record) => {
    openForm(record);
    if (task.type === 'image_parse') {
      setRecoveredPhotoTask(task); setIsPhotoParseOpen(true);
    } else if (task.result?.component) {
      applyParsedComponent(task.result.component, task.result.quantity);
      setPlatformCode(task.result.component.platform_code || '');
    }
  });

  const handleConfirm = async (item: PreStock) => {
    if (!await confirm(`确认将 ${item.component_number || item.name} 转为正式入库？`)) return;
    if (confirmingId !== null || deletingId !== null) return;
    setConfirmingId(item.id);
    try {
      await client.post(`/pre-stocks/${item.id}/confirm`);
      toast.success('预入库确认成功');
      await fetchItems(pagination.page, pagination.page_size);
      await fetchComponentOptions();
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '确认失败');
    } finally {
      setConfirmingId(null);
    }
  };

  const handleDelete = async (item: PreStock) => {
    if (!await confirm(`确定删除预入库记录 ${item.component_number || item.name}？`)) return;
    if (confirmingId !== null || deletingId !== null) return;
    setDeletingId(item.id);
    try {
      await client.delete(`/pre-stocks/${item.id}`);
      toast.success('删除成功');
      await fetchItems(pagination.page, pagination.page_size);
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '删除失败');
    } finally {
      setDeletingId(null);
    }
  };

  const handleStatusChange = (status: PreStockStatus | 'all') => {
    updateQuery({ status, page: 1 });
    void fetchItems(1, pagination.page_size, status);
  };

  const handlePageSizeChange = (pageSize: number) => {
    void fetchItems(1, pageSize);
  };

  const handlePageChange = (page: number) => {
    void fetchItems(page, pagination.page_size);
  };

  const renderNumber = (item: PreStock) => {
    const number = item.component_number?.trim();
    if (!number) return '-';
    return (
      <button
        type="button"
        className="font-mono text-xs hover:text-primary hover:underline"
        onClick={async () => {
          const ok = await copyToClipboard(number);
          if (ok) toast.success('已复制');
          else toast.error('复制失败');
        }}
      >
        {number}
      </button>
    );
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="预入库"
        actions={
          <>
            <Button variant="outline" onClick={() => fetchItems(1, pagination.page_size)} disabled={loading}>
              <Search className="mr-2 h-4 w-4" /> 刷新
            </Button>
            <Button variant="outline" onClick={() => setIsScannerOpen(true)}><QrCode className="mr-2 size-4" />扫码录入</Button>
            <Button onClick={() => openForm()}>
              <Plus className="mr-2 h-4 w-4" /> 新建预入库
            </Button>
          </>
        }
      />

      <div className="flex flex-wrap items-center justify-between gap-3"><div aria-label="预入库状态" className="inline-flex rounded-lg border bg-card p-1">{(['pending', 'confirmed', 'all'] as const).map(value => <Button key={value} size="sm" variant={statusFilter === value ? 'default' : 'ghost'} aria-pressed={statusFilter === value} onClick={() => handleStatusChange(value)}>{STATUS_FILTER_LABELS[value]}</Button>)}</div><p className="text-xs text-muted-foreground">共 {pagination.total} 条记录</p></div>
      {(loading || resource.error || !items.length) && <div className="surface"><PageState loading={loading} error={resource.error} empty={!items.length} filtered={statusFilter !== 'all'} onRetry={resource.refresh} onClear={() => handleStatusChange('all')} /></div>}
      <div className={cn('surface hidden md:block', (loading || !!resource.error || !items.length) && 'md:hidden')}>
        <div className="w-full overflow-auto">
          <table className="w-full caption-bottom text-sm text-left">
            <thead>
              <tr className="border-b">
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">编号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">图片</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">名称</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">厂家型号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">分类</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">供应商</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">料号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">预计数量</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">采购总价</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">状态</th>
                <th className="sticky right-0 z-[6] h-12 bg-background px-4 font-medium text-muted-foreground whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading && items.length === 0 ? (
                <tr><td colSpan={11} className="p-4 text-center">加载中...</td></tr>
              ) : items.length === 0 ? (
                <tr><td colSpan={11} className="p-4 text-center text-muted-foreground">暂无预入库记录</td></tr>
              ) : items.map(item => (
                <tr
                  key={item.id}
                  className={cn(
                    'group border-b transition-colors hover:bg-muted/50',

                  )}
                >
                  <td className="p-4 align-middle whitespace-nowrap">{renderNumber(item)}</td>
                  <td className="p-4 align-middle">
                    <button type="button" disabled={!item.image_url} aria-label={`预览 ${item.name} 图片`} onClick={() => setPreviewImage(item.image_url)}><SafeImage src={item.image_url} alt={item.name} className="size-10" /></button>
                  </td>
                  <td className="p-4 align-middle min-w-40"><button className="text-left font-medium text-primary hover:underline" disabled={item.status !== 'pending'} onClick={() => openForm(item)}>{item.name}</button>{!!item.review?.length && <p className="text-xs text-muted-foreground">{item.review.map(r=>`${r.field === 'name' ? '名称' : '描述'}待整理${r.confirmed ? '（已确认保留原文）' : '（待确认）'}`).join('；')}</p>}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.model || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.category?.name || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.supplier?.name || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.supplier_part_number || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.expected_quantity}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{formatCents(item.total_price_cents || 0)}</td>
                  <td className="p-4 align-middle whitespace-nowrap">
                    <span className={item.status === 'confirmed' ? 'status-badge' : 'status-badge bg-amber-500/10 text-amber-600 dark:text-amber-400'}>
                      {statusLabel(item.status)}
                    </span>
                  </td>
                  <td className="sticky right-0 z-[5] bg-background/95 p-2 align-middle shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.08)] backdrop-blur-sm">
                    {item.status === 'pending' ? (
                      <RowActionsMenu
                        rowId={item.id}
                        activeRowId={activeRowId}
                        onActiveRowChange={setActiveRowId}
                        actions={[
                          { key: 'edit', icon: Edit, label: '编辑', onClick: () => openForm(item) },
                          {
                            key: 'confirm',
                            icon: CheckCircle2,
                            label: '确认入库',
                            onClick: () => handleConfirm(item),
                            disabled: confirmingId === item.id,
                            iconClassName: 'text-green-600',
                          },
                          {
                            key: 'delete',
                            icon: Trash2,
                            label: '删除',
                            onClick: () => handleDelete(item),
                            disabled: deletingId === item.id,
                            destructive: true,
                          },
                        ]}
                      />
                    ) : item.component_id ? (
                      <span className="text-xs text-muted-foreground whitespace-nowrap">元件 #{item.component_id}</span>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {!loading && !resource.error && <div className="space-y-3 md:hidden">{items.map(item => <article key={item.id} className="surface p-4"><div className="flex items-start gap-3"><SafeImage src={item.image_url} alt={item.name} className="size-12 shrink-0" /><div className="min-w-0 flex-1"><p className="break-all font-semibold">{item.name}</p><p className="mt-1 break-all text-xs text-muted-foreground">{item.component_number} · {item.model || item.supplier_part_number || '暂无型号'}</p></div><span className="status-badge shrink-0">{statusLabel(item.status)}</span></div><div className="mt-3 flex justify-between border-t pt-3 text-xs text-muted-foreground"><span>预计 {item.expected_quantity} 件 · {item.location || '未设置位置'}</span><span>{formatCents(item.total_price_cents || 0)}</span></div>{item.status === 'pending' ? <div className="mt-4 flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={() => openForm(item)}>编辑</Button><Button size="sm" onClick={() => void handleConfirm(item)} disabled={confirmingId !== null || deletingId !== null}>确认入库</Button><Button size="sm" variant="ghost" className="ml-auto text-destructive" onClick={() => void handleDelete(item)} disabled={confirmingId !== null || deletingId !== null}>删除</Button></div> : <p className="mt-3 text-xs text-muted-foreground">关联元件 #{item.component_id}</p>}</article>)}</div>}
      <PaginationControl pagination={pagination} onPageChange={handlePageChange} onPageSizeChange={handlePageSizeChange} disabled={loading} />

      <PhotoParseModal isOpen={isPhotoParseOpen} onClose={() => { setIsPhotoParseOpen(false); setRecoveredPhotoTask(undefined); }} source="pre-stocks" recordID={editingItem?.id} initialTask={recoveredPhotoTask} onConfirm={applyPhotoResults} />

      <Modal isOpen={isFormOpen && !isPhotoParseOpen} onClose={closeForm} dirty={formGuard.dirty} busy={saving} title={editingItem ? '编辑预入库' : '新建预入库'} variant="drawer"
        footer={<><span className="mr-auto text-xs text-muted-foreground">收货后确认进入正式库存</span><Button variant="outline" onClick={() => void cancelForm()} disabled={saving}>取消</Button><Button type="submit" form="inventory-form" disabled={saving}>{saving ? <><Loader2 className="mr-2 size-4 animate-spin" />保存中…</> : '保存'}</Button></>}>
        <div className="space-y-4"><PhotoDraftNavigation index={photoDrafts.index} count={photoDrafts.drafts.length} disabled={saving} onSelect={switchPhotoDraft} />
          <SmartEntry code={platformCode} onCodeChange={setPlatformCode} onParse={() => void parsePlatform()} onScan={() => setIsScannerOpen(true)} onPhoto={() => setIsPhotoParseOpen(true)} busy={isParsing} mode={parseMode} onModeChange={setParseMode} task={parseTask.task} reference={matchingSupplierReference(supplierReference, form.supplier_part_number, platformCode)} />
          <InventoryFields value={{ ...form, quantity: form.expected_quantity, category: categoryInput, supplier: supplierInput, totalPrice: form.total_price_yuan }} onChange={patch => {
            const { quantity, category, supplier, totalPrice, ...fields } = patch;
            if ('quantity' in patch) setForm(prev => ({ ...prev, expected_quantity: quantity ?? null }));
            if (category !== undefined) setCategoryInput(category); if (supplier !== undefined) setSupplierInput(supplier);
            setForm(prev => ({ ...prev, ...fields, component_number: fields.component_number ?? prev.component_number, total_price_yuan: totalPrice ?? prev.total_price_yuan }));
          }} onSubmit={() => void handleSubmit()} categories={categories} parsed={supplierReference} suppliers={suppliers.map(s => s.name)} manufacturers={manufacturerOptions} packages={packageOptions} locations={locationOptions} preStock editing={!!editingItem} busy={saving} review={editingItem?.review} />
        </div>
      </Modal>

      <Modal isOpen={isScannerOpen} onClose={() => setIsScannerOpen(false)} title="扫码录入" className="max-w-xl">
        <Suspense fallback={<div className="p-4 text-center text-muted-foreground">加载扫码组件...</div>}>
          <QRScanner onScan={handleScan} onClose={() => setIsScannerOpen(false)} />
        </Suspense>
      </Modal>

      <Modal
        isOpen={!!previewImage}
        onClose={() => setPreviewImage(null)}
        title="图片预览"
        className="max-w-4xl"
      >
        <div className="flex justify-center items-center bg-black/5 rounded-md overflow-hidden min-h-[200px]">
          {previewImage && (
            <SafeImage src={previewImage} alt="图片预览" className="max-h-[70vh] w-full" />
          )}
        </div>
      </Modal>
    </div>
  );
}

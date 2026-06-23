import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { CheckCircle2, Edit, Hash, Loader2, Plus, QrCode, Search, Trash2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Category, type ComponentOptions, type Pagination, type PreStock, type PreStockStatus, type Supplier } from '../types';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { Modal } from '../components/ui/Modal';
import { calcUnitPriceMicro, formatCents, formatMicro, yuanToCents } from '../utils/price';
import { copyToClipboard } from '../utils/clipboard';

const QRScanner = lazy(() => import('../components/QRScanner'));

type PreStockForm = {
  component_number: string;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  package: string;
  supplier_part_number: string;
  description: string;
  expected_quantity: number;
  total_price_yuan: string;
  location: string;
  datasheet_url: string;
  image_url: string;
};

type ParsedComponentInfo = {
  name?: string;
  category_name?: string;
  model?: string;
  value?: string;
  package?: string;
  platform_code?: string;
  platform_name?: string;
  description?: string;
  manufacturer?: string;
  datasheet_url?: string;
  image_url?: string;
};

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

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
  const [items, setItems] = useState<PreStock[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [statusFilter, setStatusFilter] = useState<PreStockStatus | 'all'>('pending');
  const [loading, setLoading] = useState(false);
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isScannerOpen, setIsScannerOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<PreStock | null>(null);
  const [form, setForm] = useState<PreStockForm>(EMPTY_FORM);
  const [categoryInput, setCategoryInput] = useState('');
  const [supplierInput, setSupplierInput] = useState('');
  const [manufacturerOptions, setManufacturerOptions] = useState<string[]>([]);
  const [packageOptions, setPackageOptions] = useState<string[]>([]);
  const [locationOptions, setLocationOptions] = useState<string[]>([]);
  const [showCategoryDropdown, setShowCategoryDropdown] = useState(false);
  const [showSupplierDropdown, setShowSupplierDropdown] = useState(false);
  const [showManufacturerDropdown, setShowManufacturerDropdown] = useState(false);
  const [showPackageDropdown, setShowPackageDropdown] = useState(false);
  const [showLocationDropdown, setShowLocationDropdown] = useState(false);
  const [platformCode, setPlatformCode] = useState('');
  const [useAIParse, setUseAIParse] = useState(false);
  const [isParsing, setIsParsing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [confirmingId, setConfirmingId] = useState<number | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const totalPage = pagination.total_page || Math.ceil(pagination.total / pagination.page_size);
  const filteredCategories = useMemo(
    () => categories.filter(c => c.name.toLowerCase().includes(categoryInput.toLowerCase())),
    [categories, categoryInput],
  );
  const filteredSuppliers = useMemo(
    () => suppliers.filter(s => s.name.toLowerCase().includes(supplierInput.toLowerCase())),
    [suppliers, supplierInput],
  );
  const filteredManufacturerOptions = useMemo(
    () => manufacturerOptions.filter(option => option.toLowerCase().includes(form.manufacturer.toLowerCase())),
    [manufacturerOptions, form.manufacturer],
  );
  const filteredPackageOptions = useMemo(
    () => packageOptions.filter(option => option.toLowerCase().includes(form.package.toLowerCase())),
    [packageOptions, form.package],
  );
  const filteredLocationOptions = useMemo(
    () => locationOptions.filter(option => option.toLowerCase().includes(form.location.toLowerCase())),
    [locationOptions, form.location],
  );

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

  const fetchItems = async (page = pagination.page, pageSize = pagination.page_size, status = statusFilter) => {
    setLoading(true);
    try {
      const res = await client.get('/pre-stocks', {
        params: { page, page_size: pageSize, status },
      });
      setItems(res.data.data || []);
      setPagination(res.data.pagination || { page, page_size: pageSize, total: 0, total_page: 0 });
    } catch {
      toast.error('加载预入库记录失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void Promise.all([
      fetchItems(1, pagination.page_size, statusFilter),
      fetchCategories(),
      fetchSuppliers(),
      fetchComponentOptions(),
    ]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openForm = (item?: PreStock) => {
    if (item) {
      setEditingItem(item);
      setForm({
        component_number: item.component_number || '',
        name: item.name || '',
        model: item.model || '',
        manufacturer: item.manufacturer || '',
        value: item.value || '',
        package: item.package || '',
        supplier_part_number: item.supplier_part_number || '',
        description: item.description || '',
        expected_quantity: item.expected_quantity || 1,
        total_price_yuan: item.total_price_cents ? String((item.total_price_cents / 100).toFixed(2)) : '',
        location: item.location || '',
        datasheet_url: item.datasheet_url || '',
        image_url: item.image_url || '',
      });
      setCategoryInput(item.category?.name || '');
      setSupplierInput(item.supplier?.name || '');
      setPlatformCode(item.supplier_part_number || '');
    } else {
      setEditingItem(null);
      setForm(EMPTY_FORM);
      setCategoryInput('');
      setSupplierInput('');
      setPlatformCode('');
    }
    setIsFormOpen(true);
  };

  const ensureCategoryID = async () => {
    const name = categoryInput.trim();
    if (!name) {
      toast.error('请输入分类');
      return null;
    }
    const existing = categories.find(c => c.name.toLowerCase() === name.toLowerCase());
    if (existing) return existing.id;

    try {
      const res = await client.post('/categories', { name });
      const id = res.data.data?.id as number | undefined;
      if (!id) throw new Error('missing id');
      toast.success(`自动创建分类: ${name}`);
      await fetchCategories();
      return id;
    } catch {
      toast.error('自动创建分类失败');
      return null;
    }
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

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      toast.error('请输入名称');
      return;
    }
    if (form.expected_quantity < 0) {
      toast.error('预计数量不能小于 0');
      return;
    }

    setSaving(true);
    try {
      const categoryID = await ensureCategoryID();
      if (!categoryID) return;
      const supplierID = await ensureSupplierID();
      const totalPriceCents = yuanToCents(form.total_price_yuan);
      const payload = {
        category_id: categoryID,
        supplier_id: supplierID,
        component_number: form.component_number.trim() || undefined,
        name: form.name.trim(),
        model: form.model.trim(),
        manufacturer: form.manufacturer.trim(),
        value: form.value.trim(),
        package: form.package.trim(),
        supplier_part_number: form.supplier_part_number.trim(),
        description: form.description.trim(),
        expected_quantity: Number(form.expected_quantity),
        total_price_cents: totalPriceCents > 0 ? totalPriceCents : 0,
        location: form.location.trim(),
        datasheet_url: form.datasheet_url.trim(),
        image_url: form.image_url.trim(),
      };

      if (editingItem) {
        await client.put(`/pre-stocks/${editingItem.id}`, payload);
        toast.success('预入库记录已更新');
      } else {
        const res = await client.post('/pre-stocks', payload);
        const number = res.data.data?.component_number;
        toast.success(number ? `已生成编号 ${number}` : '预入库记录已创建');
      }

      setIsFormOpen(false);
      await fetchItems(pagination.page, pagination.page_size);
      await fetchComponentOptions();
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const applyParsedComponent = (component: ParsedComponentInfo, quantity?: number) => {
    setForm(prev => ({
      ...prev,
      name: component.name || component.model || prev.name,
      model: component.model || prev.model,
      manufacturer: component.manufacturer || prev.manufacturer,
      value: component.value || prev.value,
      package: component.package || prev.package,
      supplier_part_number: component.platform_code || prev.supplier_part_number,
      description: component.description || prev.description,
      datasheet_url: component.datasheet_url || prev.datasheet_url,
      image_url: component.image_url || prev.image_url,
      expected_quantity: quantity && quantity > 0 ? quantity : prev.expected_quantity,
    }));
    if (component.category_name) setCategoryInput(component.category_name);
    const supplierName = getSupplierNameFromPlatform(component.platform_name);
    if (supplierName) setSupplierInput(supplierName);
  };

  const parsePlatform = async () => {
    const code = platformCode.trim();
    if (!code) {
      toast.error('请输入供应商料号');
      return;
    }
    setIsParsing(true);
    try {
      const res = await client.post('/components/parse', { code, use_llm: useAIParse });
      applyParsedComponent(res.data.data as ParsedComponentInfo);
      toast.success('解析成功');
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '解析失败');
    } finally {
      setIsParsing(false);
    }
  };

  const handleScan = async (data: string) => {
    if (isParsing) return;
    setIsScannerOpen(false);
    setIsParsing(true);
    try {
      const res = await client.post('/components/parse-qrcode', { qrcode_data: data, use_llm: useAIParse });
      const parsed = res.data.data as { component: ParsedComponentInfo; quantity: number };
      applyParsedComponent(parsed.component, parsed.quantity);
      setPlatformCode(parsed.component.platform_code || '');
      toast.success('已识别预入库信息');
      if (!isFormOpen) setIsFormOpen(true);
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '二维码解析失败');
    } finally {
      setIsParsing(false);
    }
  };

  const handleConfirm = async (item: PreStock) => {
    if (!confirm(`确认将 ${item.component_number || item.name} 转为正式入库？`)) return;
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
    if (!confirm(`确定删除预入库记录 ${item.component_number || item.name}？`)) return;
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
    setStatusFilter(status);
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

  const totalPriceCents = yuanToCents(form.total_price_yuan);
  const showUnitPrice = form.expected_quantity > 0 && totalPriceCents > 0;

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <h2 className="text-3xl font-bold tracking-tight">预入库</h2>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => fetchItems(1, pagination.page_size)} disabled={loading}>
            <Search className="mr-2 h-4 w-4" /> 刷新
          </Button>
          <Button onClick={() => openForm()}>
            <Plus className="mr-2 h-4 w-4" /> 新建预入库
          </Button>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row gap-3 sm:items-end rounded-md border p-4">
        <div className="space-y-1">
          <Label htmlFor="status-filter" className="text-xs text-muted-foreground">状态</Label>
          <select
            id="status-filter"
            value={statusFilter}
            onChange={e => handleStatusChange(e.target.value as PreStockStatus | 'all')}
            className="h-9 w-full sm:w-36 rounded-md border border-input bg-background px-2 text-sm"
          >
            <option value="pending">待入库</option>
            <option value="confirmed">已入库</option>
            <option value="all">全部</option>
          </select>
        </div>
        <div className="text-sm text-muted-foreground sm:ml-auto">共 {pagination.total} 条</div>
      </div>

      <div className="rounded-md border">
        <div className="w-full overflow-auto">
          <table className="w-full caption-bottom text-sm text-left">
            <thead>
              <tr className="border-b">
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">编号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">名称</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">厂家型号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">分类</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">供应商</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">料号</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">预计数量</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">采购总价</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">状态</th>
                <th className="h-12 px-4 font-medium text-muted-foreground whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading && items.length === 0 ? (
                <tr><td colSpan={10} className="p-4 text-center">加载中...</td></tr>
              ) : items.length === 0 ? (
                <tr><td colSpan={10} className="p-4 text-center text-muted-foreground">暂无预入库记录</td></tr>
              ) : items.map(item => (
                <tr key={item.id} className="border-b transition-colors hover:bg-muted/50">
                  <td className="p-4 align-middle whitespace-nowrap">{renderNumber(item)}</td>
                  <td className="p-4 align-middle min-w-40">{item.name}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.model || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.category?.name || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.supplier?.name || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.supplier_part_number || '-'}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{item.expected_quantity}</td>
                  <td className="p-4 align-middle whitespace-nowrap">{formatCents(item.total_price_cents || 0)}</td>
                  <td className="p-4 align-middle whitespace-nowrap">
                    <span className={item.status === 'confirmed' ? 'text-green-600' : 'text-amber-600'}>
                      {statusLabel(item.status)}
                    </span>
                  </td>
                  <td className="p-4 align-middle">
                    <div className="flex gap-2">
                      {item.status === 'pending' && (
                        <>
                          <Button variant="ghost" size="icon" onClick={() => openForm(item)} title="编辑">
                            <Edit className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => handleConfirm(item)} title="确认入库" disabled={confirmingId === item.id}>
                            {confirmingId === item.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4 text-green-600" />}
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => handleDelete(item)} title="删除" className="text-destructive" disabled={deletingId === item.id}>
                            {deletingId === item.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                          </Button>
                        </>
                      )}
                      {item.status === 'confirmed' && item.component_id && (
                        <span className="text-xs text-muted-foreground whitespace-nowrap">元件 #{item.component_id}</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row justify-between gap-3 items-start sm:items-center">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <span>每页</span>
          <select
            className="h-9 rounded-md border border-input bg-background px-2 text-sm"
            value={pagination.page_size}
            onChange={e => handlePageSizeChange(Number(e.target.value))}
          >
            {PAGE_SIZE_OPTIONS.map(size => (
              <option key={size} value={size}>{size}</option>
            ))}
          </select>
          <span>条</span>
        </div>
        <div className="flex gap-2 items-center">
          <Button variant="outline" disabled={pagination.page <= 1} onClick={() => handlePageChange(pagination.page - 1)}>上一页</Button>
          <span>第 {pagination.page} / {Math.max(totalPage, 1)} 页</span>
          <Button variant="outline" disabled={totalPage === 0 || pagination.page >= totalPage} onClick={() => handlePageChange(pagination.page + 1)}>下一页</Button>
        </div>
      </div>

      <Modal
        isOpen={isFormOpen}
        onClose={() => setIsFormOpen(false)}
        title={editingItem ? '编辑预入库' : '新建预入库'}
        className="max-w-2xl"
        footer={
          <>
            <Button variant="outline" onClick={() => setIsFormOpen(false)} disabled={saving}>取消</Button>
            <Button onClick={handleSubmit} disabled={saving}>
              {saving ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />保存中...</> : '保存'}
            </Button>
          </>
        }
      >
        <div className="grid gap-4 py-4 max-h-[70vh] overflow-y-auto px-1">
          <div className="bg-secondary/20 p-4 rounded-md space-y-4">
            <Label>快速导入</Label>
            <div className="flex gap-2">
              <Input
                placeholder="输入平台编码 (如 C2040)"
                value={platformCode}
                onChange={e => setPlatformCode(e.target.value)}
                disabled={isParsing}
              />
              <Button onClick={parsePlatform} disabled={isParsing || !platformCode.trim()}>
                {isParsing ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />解析中...</> : '解析'}
              </Button>
              <Button variant="outline" onClick={() => setIsScannerOpen(true)} disabled={isParsing}>
                <QrCode className="mr-2 h-4 w-4" /> 扫码
              </Button>
            </div>
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <input
                type="checkbox"
                checked={useAIParse}
                onChange={e => setUseAIParse(e.target.checked)}
                disabled={isParsing}
                className="h-4 w-4 rounded border-input disabled:opacity-50"
              />
              AI 解析
            </label>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>分类</Label>
              <div className="relative">
                <Input
                  value={categoryInput}
                  onChange={e => {
                    setCategoryInput(e.target.value);
                    setShowCategoryDropdown(true);
                  }}
                  onFocus={() => setShowCategoryDropdown(true)}
                  onBlur={() => setTimeout(() => setShowCategoryDropdown(false), 200)}
                  placeholder="输入或选择分类"
                />
                {showCategoryDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                    {filteredCategories.map(c => (
                      <div
                        key={c.id}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setCategoryInput(c.name);
                          setShowCategoryDropdown(false);
                        }}
                      >
                        {c.name}
                      </div>
                    ))}
                    {filteredCategories.length === 0 && (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        {categoryInput ? '保存时创建新分类' : '无匹配分类'}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label>元件编号</Label>
              <div className="relative">
                <Hash className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-9"
                  value={form.component_number}
                  onChange={e => setForm({ ...form, component_number: e.target.value })}
                  placeholder="留空则保存时自动生成"
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label>名称</Label>
              <Input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label>厂家型号</Label>
              <Input value={form.model} onChange={e => setForm({ ...form, model: e.target.value })} placeholder="例如 RC0603FR-0710KL" />
            </div>
            <div className="space-y-2">
              <Label>制造商</Label>
              <div className="relative">
                <Input
                  value={form.manufacturer}
                  onChange={e => {
                    setForm({ ...form, manufacturer: e.target.value });
                    setShowManufacturerDropdown(true);
                  }}
                  onFocus={() => setShowManufacturerDropdown(true)}
                  onBlur={() => setTimeout(() => setShowManufacturerDropdown(false), 200)}
                  placeholder="例如 YAGEO"
                />
                {showManufacturerDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                    {filteredManufacturerOptions.map(option => (
                      <div
                        key={option}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setForm({ ...form, manufacturer: option });
                          setShowManufacturerDropdown(false);
                        }}
                      >
                        {option}
                      </div>
                    ))}
                    {filteredManufacturerOptions.length === 0 && (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        {form.manufacturer ? '无匹配制造商' : '无历史制造商'}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label>参数值</Label>
              <Input value={form.value} onChange={e => setForm({ ...form, value: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label>封装</Label>
              <div className="relative">
                <Input
                  value={form.package}
                  onChange={e => {
                    setForm({ ...form, package: e.target.value });
                    setShowPackageDropdown(true);
                  }}
                  onFocus={() => setShowPackageDropdown(true)}
                  onBlur={() => setTimeout(() => setShowPackageDropdown(false), 200)}
                  placeholder="例如 0603"
                />
                {showPackageDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                    {filteredPackageOptions.map(option => (
                      <div
                        key={option}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setForm({ ...form, package: option });
                          setShowPackageDropdown(false);
                        }}
                      >
                        {option}
                      </div>
                    ))}
                    {filteredPackageOptions.length === 0 && (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        {form.package ? '无匹配封装' : '无历史封装'}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label>供应商</Label>
              <div className="relative">
                <Input
                  value={supplierInput}
                  onChange={e => {
                    setSupplierInput(e.target.value);
                    setShowSupplierDropdown(true);
                  }}
                  onFocus={() => setShowSupplierDropdown(true)}
                  onBlur={() => setTimeout(() => setShowSupplierDropdown(false), 200)}
                  placeholder="例如 嘉立创"
                />
                {showSupplierDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                    {filteredSuppliers.map(s => (
                      <div
                        key={s.id}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setSupplierInput(s.name);
                          setShowSupplierDropdown(false);
                        }}
                      >
                        {s.name}
                      </div>
                    ))}
                    {filteredSuppliers.length === 0 && (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        {supplierInput ? '保存时创建新供应商' : '无匹配供应商'}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label>供应商料号</Label>
              <Input value={form.supplier_part_number} onChange={e => setForm({ ...form, supplier_part_number: e.target.value })} placeholder="例如 C2040" />
            </div>
            <div className="space-y-2">
              <Label>预计数量</Label>
              <Input
                type="number"
                min="0"
                value={form.expected_quantity}
                onChange={e => setForm({ ...form, expected_quantity: Number(e.target.value) })}
              />
            </div>
            <div className="space-y-2">
              <Label>采购总价（元）</Label>
              <Input
                type="number"
                min="0"
                step="0.01"
                value={form.total_price_yuan}
                onChange={e => setForm({ ...form, total_price_yuan: e.target.value })}
                placeholder="可选，确认入库时分摊单价"
              />
              {showUnitPrice && (
                <p className="text-xs text-muted-foreground">
                  分摊单价：{formatMicro(calcUnitPriceMicro(totalPriceCents, form.expected_quantity))}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label>位置</Label>
              <div className="relative">
                <Input
                  value={form.location}
                  onChange={e => {
                    setForm({ ...form, location: e.target.value });
                    setShowLocationDropdown(true);
                  }}
                  onFocus={() => setShowLocationDropdown(true)}
                  onBlur={() => setTimeout(() => setShowLocationDropdown(false), 200)}
                  placeholder="例如 A1-03"
                />
                {showLocationDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                    {filteredLocationOptions.map(option => (
                      <div
                        key={option}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setForm({ ...form, location: option });
                          setShowLocationDropdown(false);
                        }}
                      >
                        {option}
                      </div>
                    ))}
                    {filteredLocationOptions.length === 0 && (
                      <div className="px-3 py-2 text-sm text-muted-foreground">
                        {form.location ? '无匹配位置' : '无历史位置'}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label>数据手册 URL</Label>
              <Input value={form.datasheet_url} onChange={e => setForm({ ...form, datasheet_url: e.target.value })} />
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label>图片 URL</Label>
              <Input value={form.image_url} onChange={e => setForm({ ...form, image_url: e.target.value })} />
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label>描述</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                value={form.description}
                onChange={e => setForm({ ...form, description: e.target.value })}
              />
            </div>
          </div>
        </div>
      </Modal>

      <Modal isOpen={isScannerOpen} onClose={() => setIsScannerOpen(false)} title="扫码录入" className="max-w-xl">
        <Suspense fallback={<div className="p-4 text-center text-muted-foreground">加载扫码组件...</div>}>
          <QRScanner onScan={handleScan} onClose={() => setIsScannerOpen(false)} />
        </Suspense>
      </Modal>
    </div>
  );
}

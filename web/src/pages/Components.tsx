import { useEffect, useState } from 'react';
import { useRef } from 'react';
import { Plus, Minus, Search, Edit, Trash2, Database, History, QrCode, Camera, Upload, Link, X, Loader2, Hash } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Component, type Category, type Supplier, type StockLog, type Pagination, type ComponentOptions } from '../types';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { Label } from '../components/ui/Label';
import QRScanner from '../components/QRScanner';
import CameraCapture from '../components/CameraCapture';

type ComponentSearchParams = {
  page: number;
  page_size: number;
  keyword?: string;
  category_id?: string;
};

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

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
  category?: Category;
};

export default function Components() {
  const [components, setComponents] = useState<Component[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [isBatchLocationOpen, setIsBatchLocationOpen] = useState(false);
  const [batchLocation, setBatchLocation] = useState('');
  const [isBatchUpdating, setIsBatchUpdating] = useState(false);
  const [isGeneratingNumbers, setIsGeneratingNumbers] = useState(false);

  // Modals
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isStockOpen, setIsStockOpen] = useState(false);
  const [isLogsOpen, setIsLogsOpen] = useState(false);
  const [isScannerOpen, setIsScannerOpen] = useState(false);
  const [isCameraOpen, setIsCameraOpen] = useState(false);
  const [previewImage, setPreviewImage] = useState<string | null>(null);

  // Form State
  const [editingComponent, setEditingComponent] = useState<Component | null>(null);
  const [formData, setFormData] = useState<Partial<Component>>({
    name: '', category_id: undefined, stock_quantity: 0
  });
  const [categoryInput, setCategoryInput] = useState('');
  const [showCategoryDropdown, setShowCategoryDropdown] = useState(false);
  const [supplierInput, setSupplierInput] = useState('');
  const [showSupplierDropdown, setShowSupplierDropdown] = useState(false);
  const [packageOptions, setPackageOptions] = useState<string[]>([]);
  const [locationOptions, setLocationOptions] = useState<string[]>([]);
  const [showPackageDropdown, setShowPackageDropdown] = useState(false);
  const [showLocationDropdown, setShowLocationDropdown] = useState(false);
  const [showBatchLocationDropdown, setShowBatchLocationDropdown] = useState(false);
  
  // Image Upload State
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string>('');
  const [showImageMenu, setShowImageMenu] = useState(false);
  const [showUrlInput, setShowUrlInput] = useState(false);


  // Stock State
  const [stockForm, setStockForm] = useState({ type: 'in', amount: 1, reason: '' });
  
  // Logs State
  const [componentLogs, setComponentLogs] = useState<StockLog[]>([]);

  // Platform Import
  const [platformCode, setPlatformCode] = useState('');
  const [isImportParsing, setIsImportParsing] = useState(false);
  const [useAIParse, setUseAIParse] = useState(false);

  const [isMobile, setIsMobile] = useState(false);

  const getSupplierNameFromPlatform = (platformName?: string) => {
    if (!platformName) return '';
    if (platformName.includes('立创') || platformName.includes('LCSC')) return '嘉立创';
    return platformName;
  };

  const parsedInfoToFormData = (info: ParsedComponentInfo): Partial<Component> => ({
    name: info.name || info.model || '',
    model: info.model || '',
    manufacturer: info.manufacturer || '',
    value: info.value || '',
    package: info.package || '',
    supplier_part_number: info.platform_code || '',
    description: info.description || '',
    datasheet_url: info.datasheet_url || '',
    image_url: info.image_url || '',
  });

  const applyParsedComponent = (component: ParsedComponentInfo, quantity?: number) => {
    const parsedForm = parsedInfoToFormData(component);
    setSupplierInput(getSupplierNameFromPlatform(component.platform_name));
    if (component.category_name || component.category?.name) {
      setCategoryInput(component.category_name || component.category?.name || '');
    }
    setFormData(prev => ({
      ...prev,
      ...Object.fromEntries(
        Object.entries(parsedForm).filter(([, v]) => v !== '' && v !== undefined)
      ),
      stock_quantity: quantity && quantity > 0 ? quantity : prev.stock_quantity,
    }));
  };

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  const fetchComponents = async (page = 1, pageSize = pagination.page_size) => {
    setLoading(true);
    try {
      const params: ComponentSearchParams = { page, page_size: pageSize };
      if (search) params.keyword = search;
      if (selectedCategory) params.category_id = selectedCategory;

      const res = await client.get('/components', { params });
      setComponents(res.data.data || []);
      setPagination(res.data.pagination || { page: 1, page_size: 20, total: 0, total_page: 0 });
      setSelectedIds([]);
    } catch {
      toast.error('加载元件失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchCategories = async () => {
    try {
      const res = await client.get('/categories');
      setCategories(res.data.data || []);
    } catch (error) {
      console.error(error);
    }
  };

  const fetchSuppliers = async () => {
    try {
      const res = await client.get('/suppliers');
      setSuppliers(res.data.data || []);
    } catch {
      toast.error('加载供应商失败');
    }
  };

  const fetchComponentOptions = async () => {
    try {
      const res = await client.get('/components/options');
      const options = res.data.data as ComponentOptions;
      setPackageOptions(options.packages || []);
      setLocationOptions(options.locations || []);
    } catch {
      toast.error('加载历史选项失败');
    }
  };

  useEffect(() => {
    fetchCategories();
    fetchSuppliers();
    fetchComponentOptions();
    fetchComponents();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSearch = () => {
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1, pagination.page_size);
  };

  const handlePageSizeChange = (pageSize: number) => {
    setPagination(prev => ({ ...prev, page: 1, page_size: pageSize }));
    fetchComponents(1, pageSize);
  };

  const handlePageChange = (page: number) => {
    setPagination(prev => ({ ...prev, page }));
    fetchComponents(page, pagination.page_size);
  };

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? components.map(c => c.id) : []);
  };

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds(prev => checked ? [...prev, id] : prev.filter(item => item !== id));
  };

  const handleBatchLocationSubmit = async () => {
    if (selectedIds.length === 0) return;
    setIsBatchUpdating(true);
    try {
      await client.patch('/components/batch-location', {
        ids: selectedIds,
        location: batchLocation,
      });
      toast.success(`已更新 ${selectedIds.length} 个元件的位置`);
      setIsBatchLocationOpen(false);
      setBatchLocation('');
      fetchComponents(pagination.page, pagination.page_size);
      fetchComponentOptions();
    } catch {
      toast.error('批量更新位置失败');
    } finally {
      setIsBatchUpdating(false);
    }
  };

  const handleGenerateNumbers = async () => {
    if (!confirm('将为所有未编号的元件自动生成编号，是否继续？')) return;
    setIsGeneratingNumbers(true);
    try {
      const res = await client.patch('/components/generate-numbers');
      const updated = res.data.updated ?? 0;
      toast.success(updated > 0 ? `已为 ${updated} 个元件生成编号` : '没有需要编号的元件');
      fetchComponents(pagination.page, pagination.page_size);
    } catch {
      toast.error('自动编号失败');
    } finally {
      setIsGeneratingNumbers(false);
    }
  };

  const isAllSelected = components.length > 0 && selectedIds.length === components.length;
  const totalPage = pagination.total_page ?? (pagination.total > 0 ? Math.ceil(pagination.total / pagination.page_size) : 0);

  const handleDelete = async (id: number) => {
    if (!confirm('确定删除此元件吗？')) return;
    try {
      await client.delete(`/components/${id}`);
      toast.success('删除成功');
      fetchComponents(pagination.page, pagination.page_size);
    } catch {
      toast.error('删除失败');
    }
  };

  // Form Handlers
  const openForm = (component?: Component) => {
    setSelectedFile(null);
    setShowImageMenu(false);
    setShowUrlInput(false);
    if (component) {
      if (component.id) {
          setEditingComponent(component);
          // 使用时间戳防止缓存
          setPreviewUrl(`/api/v1/components/${component.id}/image?t=${Date.now()}`);
      } else {
          setEditingComponent(null);
          setPreviewUrl('');
      }
      setFormData(component);
      setCategoryInput(component.category?.name || '');
      setSupplierInput(component.supplier?.name || '');
    } else {
      setEditingComponent(null);
      setFormData({ stock_quantity: 0 });
      setCategoryInput('');
      setSupplierInput('');
      setPreviewUrl('');
    }
    setIsFormOpen(true);
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      const url = URL.createObjectURL(file);
      setPreviewUrl(url);
      setFormData(prev => ({ ...prev, image_url: '' })); // 清除 URL，优先显示本地预览
      setShowImageMenu(false);
    }
  };

  const handleCapture = (file: File) => {
    setSelectedFile(file);
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    setFormData(prev => ({ ...prev, image_url: '' }));
    setIsCameraOpen(false);
    setShowImageMenu(false);
  };

  const handleFormSubmit = async () => {
    try {
      if (!categoryInput.trim()) {
        toast.error('请输入分类');
        return;
      }

      let categoryId = categories.find(c => c.name.toLowerCase() === categoryInput.trim().toLowerCase())?.id;
      
      if (!categoryId) {
        // Auto create category
        try {
           const res = await client.post('/categories', { name: categoryInput.trim() });
           categoryId = res.data.data?.id;
           if (!categoryId) throw new Error('创建分类失败');
           toast.success(`自动创建分类: ${categoryInput}`);
           await fetchCategories(); // Refresh categories
        } catch {
           toast.error('自动创建分类失败');
           return;
        }
      }

      let supplierId: number | null = null;
      const supplierName = supplierInput.trim();
      if (supplierName) {
        const existingSupplier = suppliers.find(s => s.name.toLowerCase() === supplierName.toLowerCase());
        if (existingSupplier) {
          supplierId = existingSupplier.id;
        } else {
          try {
            const res = await client.post('/suppliers', { name: supplierName });
            supplierId = res.data.data?.id;
            if (!supplierId) throw new Error('创建供应商失败');
            toast.success(`自动创建供应商: ${supplierName}`);
            await fetchSuppliers();
          } catch {
            toast.error('自动创建供应商失败');
            return;
          }
        }
      }

      const data = {
        ...formData,
        category_id: Number(categoryId),
        supplier_id: supplierId,
        supplier: undefined,
        category: undefined,
        component_number: formData.component_number?.trim() || undefined,
        stock_quantity: Number(formData.stock_quantity)
      };

      let savedId = editingComponent?.id;

      if (editingComponent) {
        await client.put(`/components/${editingComponent.id}`, data);
        toast.success('更新成功');
      } else {
        const res = await client.post('/components', data);
        savedId = res.data.data.id;
        toast.success('添加成功');
      }

      // Upload Image if selected
      if (selectedFile && savedId) {
          const imageFormData = new FormData();
          imageFormData.append('image', selectedFile);
          try {
             await client.post(`/components/${savedId}/image`, imageFormData, {
                 headers: { 'Content-Type': 'multipart/form-data' }
             });
             // toast.success('图片上传成功');
          } catch (error) {
             console.error(error);
             toast.error('图片上传失败');
          }
      }

      setIsFormOpen(false);
      fetchComponents(pagination.page, pagination.page_size);
      fetchComponentOptions();
    } catch (error) {
      console.error(error);
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '保存失败');
    }
  };

  const parsePlatform = async (code?: string) => {
    const parseCode = (code ?? platformCode).trim();
    if (!parseCode || isImportParsing) {
      if (!parseCode) toast.error('请输入供应商料号');
      return;
    }
    setIsImportParsing(true);
    try {
      const res = await client.post('/components/parse', { code: parseCode, use_llm: useAIParse });
      const data = res.data.data as ParsedComponentInfo;
      applyParsedComponent(data);
      toast.success('解析成功');
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '解析失败');
    } finally {
      setIsImportParsing(false);
    }
  };

  // Stock Handlers
  const openStock = (component: Component) => {
    setEditingComponent(component);
    setStockForm({ type: 'in', amount: 1, reason: '' });
    setIsStockOpen(true);
  };

  const handleStockSubmit = async () => {
    if (!editingComponent) return;
    try {
      const amount = stockForm.type === 'in' ? stockForm.amount : -stockForm.amount;
      await client.post(`/components/${editingComponent.id}/stock`, {
        amount,
        reason: stockForm.reason
      });
      toast.success('库存更新成功');
      setIsStockOpen(false);
      fetchComponents(pagination.page, pagination.page_size);
    } catch {
      toast.error('更新失败');
    }
  };

  // Logs Handlers
  const openLogs = async (component: Component) => {
    setEditingComponent(component);
    setIsLogsOpen(true);
    try {
      const res = await client.get(`/components/${component.id}/logs`);
      setComponentLogs(res.data.data || []);
    } catch {
      toast.error('加载记录失败');
    }
  };

  // QR Handlers
  const handleScan = async (data: string) => {
    if (isImportParsing) return;
    setIsScannerOpen(false);
    setIsImportParsing(true);
    try {
      const res = await client.post('/components/parse-qrcode', { qrcode_data: data, use_llm: useAIParse });
      const { component, quantity } = res.data.data as { component: ParsedComponentInfo; quantity: number };

      if (isFormOpen) {
        applyParsedComponent(component, quantity);
        toast.success('已识别元件信息');
      } else {
        toast.success(`识别成功: ${component.name}`);
        openForm(parsedInfoToFormData(component) as Component);
        applyParsedComponent(component, quantity);
      }
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '二维码解析失败');
    } finally {
      setIsImportParsing(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <h2 className="text-3xl font-bold tracking-tight">元件管理</h2>
        <div className="flex gap-2">
            {isMobile && (
                <Button onClick={() => setIsScannerOpen(true)} variant="outline" disabled={isImportParsing}>
                    <QrCode className="mr-2 h-4 w-4" /> 扫码录入
                </Button>
            )}
            <Button variant="outline" onClick={handleGenerateNumbers} disabled={isGeneratingNumbers}>
              {isGeneratingNumbers ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  编号中...
                </>
              ) : (
                <>
                  <Hash className="mr-2 h-4 w-4" /> 自动编号
                </>
              )}
            </Button>
            <Button onClick={() => openForm()}>
                <Plus className="mr-2 h-4 w-4" /> 添加元件
            </Button>
        </div>
      </div>

      <div className="flex flex-col md:flex-row gap-4">
        <div className="flex-1 flex gap-2">
          <Input 
            placeholder="搜索编号、元件、厂家型号、制造商、参数、供应商或料号..." 
            value={search} 
            onChange={e => setSearch(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
          />
          <Button onClick={handleSearch} variant="secondary"><Search className="h-4 w-4" /></Button>
        </div>
        <select 
          className="h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          value={selectedCategory}
          onChange={e => { setSelectedCategory(e.target.value); setTimeout(() => handleSearch(), 0); }}
        >
          <option value="">所有分类</option>
          {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>

      {selectedIds.length > 0 && (
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 rounded-md border bg-muted/30 px-4 py-3">
          <span className="text-sm text-muted-foreground">已选择 {selectedIds.length} 项</span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setSelectedIds([])}>取消选择</Button>
            <Button onClick={() => setIsBatchLocationOpen(true)}>批量修改位置</Button>
          </div>
        </div>
      )}

      <div className="rounded-md border">
        <div className="w-full overflow-auto">
          <table className="w-full caption-bottom text-sm text-left">
            <thead className="[&_tr]:border-b">
              <tr className="border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted">
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap w-10">
                  <input
                    type="checkbox"
                    checked={isAllSelected}
                    onChange={e => toggleSelectAll(e.target.checked)}
                    className="h-4 w-4 rounded border-input"
                    aria-label="全选"
                  />
                </th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">图片</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">编号</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">名称</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">厂家型号</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">制造商</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">参数</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">封装</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">供应商</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">供应商料号</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">分类</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">库存</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">位置</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">描述</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">数据手册</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody className="[&_tr:last-child]:border-0">
              {loading ? (
                <tr><td colSpan={16} className="p-4 text-center">加载中...</td></tr>
              ) : components.map(component => (
                <tr key={component.id} className="border-b transition-colors hover:bg-muted/50">
                  <td className="p-4 align-middle">
                    <input
                      type="checkbox"
                      checked={selectedIds.includes(component.id)}
                      onChange={e => toggleSelectOne(component.id, e.target.checked)}
                      className="h-4 w-4 rounded border-input"
                      aria-label={`选择 ${component.name}`}
                    />
                  </td>
                  <td className="p-4 align-middle">
                    <div 
                        className="w-10 h-10 rounded overflow-hidden bg-secondary/20 flex items-center justify-center cursor-pointer hover:opacity-80 transition-opacity"
                        onClick={() => setPreviewImage(`/api/v1/components/${component.id}/image`)}
                    >
                        <img 
                            src={`/api/v1/components/${component.id}/image`} 
                            alt="" 
                            className="w-full h-full object-cover"
                            onError={(e) => {
                                (e.target as HTMLImageElement).style.display = 'none';
                                (e.target as HTMLImageElement).parentElement!.innerText = '📦';
                                // 如果加载失败，移除点击事件，或者点击时不显示大图（可以在点击处理中判断）
                                // 这里简单处理：如果显示了 emoji，点击依然会尝试加载图片但会失败
                            }}
                        />
                    </div>
                  </td>
                  <td className="p-4 align-middle font-mono text-xs">{component.component_number || '-'}</td>
                  <td className="p-4 align-middle font-medium">{component.name}</td>
                  <td className="p-4 align-middle">{component.model || '-'}</td>
                  <td className="p-4 align-middle">{component.manufacturer || '-'}</td>
                  <td className="p-4 align-middle">{component.value}</td>
                  <td className="p-4 align-middle">{component.package}</td>
                  <td className="p-4 align-middle">{component.supplier?.name || '-'}</td>
                  <td className="p-4 align-middle">{component.supplier_part_number || '-'}</td>
                  <td className="p-4 align-middle">{component.category?.name}</td>
                  <td className="p-4 align-middle">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                        component.stock_quantity < 10 ? 'bg-red-100 text-red-800' : 
                        component.stock_quantity < 50 ? 'bg-yellow-100 text-yellow-800' : 'bg-green-100 text-green-800'
                    }`}>
                        {component.stock_quantity}
                    </span>
                  </td>
                  <td className="p-4 align-middle">{component.location}</td>
                  <td className="p-4 align-middle max-w-[200px] truncate" title={component.description}>{component.description}</td>
                  <td className="p-4 align-middle">
                    {component.datasheet_url ? (
                        <a href={component.datasheet_url} target="_blank" rel="noreferrer" className="text-primary hover:underline">查看</a>
                    ) : '-'}
                  </td>
                  <td className="p-4 align-middle">
                    <div className="flex gap-2">
                        <Button variant="ghost" size="icon" onClick={() => openForm(component)} title="编辑"><Edit className="h-4 w-4" /></Button>
                        <Button variant="ghost" size="icon" onClick={() => openStock(component)} title="库存"><Database className="h-4 w-4 text-blue-500" /></Button>
                        <Button variant="ghost" size="icon" onClick={() => openLogs(component)} title="记录"><History className="h-4 w-4 text-gray-500" /></Button>
                        <Button variant="ghost" size="icon" onClick={() => handleDelete(component.id)} title="删除" className="text-destructive"><Trash2 className="h-4 w-4" /></Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      
      {/* Pagination */}
      <div className="flex flex-col sm:flex-row justify-between gap-3 items-start sm:items-center">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <span>共 {pagination.total} 条</span>
          <div className="flex items-center gap-2">
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
        </div>
        <div className="flex gap-2 items-center">
          <Button 
            variant="outline" 
            disabled={pagination.page <= 1} 
            onClick={() => handlePageChange(pagination.page - 1)}
          >上一页</Button>
          <span>第 {pagination.page} / {Math.max(totalPage, 1)} 页</span>
          <Button 
            variant="outline" 
            disabled={totalPage === 0 || pagination.page >= totalPage} 
            onClick={() => handlePageChange(pagination.page + 1)}
          >下一页</Button>
        </div>
      </div>

      {/* Batch Location Modal */}
      <Modal
        isOpen={isBatchLocationOpen}
        onClose={() => setIsBatchLocationOpen(false)}
        title="批量修改位置"
        footer={
          <>
            <Button variant="outline" onClick={() => setIsBatchLocationOpen(false)} disabled={isBatchUpdating}>取消</Button>
            <Button onClick={handleBatchLocationSubmit} disabled={isBatchUpdating}>
              {isBatchUpdating ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  更新中...
                </>
              ) : '确认'}
            </Button>
          </>
        }
      >
        <div className="space-y-4 py-4">
          <p className="text-sm text-muted-foreground">将为选中的 {selectedIds.length} 个元件设置相同位置。</p>
          <div className="space-y-2">
            <Label>位置</Label>
            <div className="relative">
              <Input
                value={batchLocation}
                onChange={e => {
                  setBatchLocation(e.target.value);
                  setShowBatchLocationDropdown(true);
                }}
                onFocus={() => setShowBatchLocationDropdown(true)}
                onBlur={() => setTimeout(() => setShowBatchLocationDropdown(false), 200)}
                placeholder="例如 A1-03"
                autoFocus
              />
              {showBatchLocationDropdown && (
                <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                  {locationOptions
                    .filter(loc => loc.toLowerCase().includes(batchLocation.toLowerCase()))
                    .map(loc => (
                      <div
                        key={loc}
                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                        onClick={() => {
                          setBatchLocation(loc);
                          setShowBatchLocationDropdown(false);
                        }}
                      >
                        {loc}
                      </div>
                    ))}
                  {locationOptions.filter(loc => loc.toLowerCase().includes(batchLocation.toLowerCase())).length === 0 && (
                    <div className="px-3 py-2 text-sm text-muted-foreground">
                      {batchLocation ? '无匹配位置' : '无历史位置'}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </Modal>

      {/* Edit/Add Modal */}
      <Modal 
        isOpen={isFormOpen} 
        onClose={() => setIsFormOpen(false)} 
        title={editingComponent ? '编辑元件' : '添加元件'}
        footer={
          <>
            <Button variant="outline" onClick={() => setIsFormOpen(false)} disabled={isImportParsing && !editingComponent}>取消</Button>
            <Button onClick={handleFormSubmit} disabled={isImportParsing && !editingComponent}>保存</Button>
          </>
        }
        className="max-w-2xl"
      >
        <div className="grid gap-4 py-4 max-h-[70vh] overflow-y-auto px-1">
             {/* Import Tools */}
             <div className="bg-secondary/20 p-4 rounded-md space-y-4">
                <Label>{editingComponent ? '根据供应商料号更新' : '快速导入'}</Label>
                <div className="flex gap-2">
                    <Input
                      placeholder="输入平台编码 (如 C2040)"
                      value={editingComponent ? (formData.supplier_part_number || '') : platformCode}
                      onChange={e => {
                        if (editingComponent) {
                          setFormData({ ...formData, supplier_part_number: e.target.value });
                        } else {
                          setPlatformCode(e.target.value);
                        }
                      }}
                      disabled={isImportParsing}
                    />
                    <Button
                      onClick={() => parsePlatform(editingComponent ? formData.supplier_part_number : undefined)}
                      disabled={isImportParsing || !(editingComponent ? formData.supplier_part_number?.trim() : platformCode.trim())}
                    >
                      {isImportParsing ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          解析中...
                        </>
                      ) : editingComponent ? '更新信息' : '解析'}
                    </Button>
                    {!editingComponent && (
                      <Button variant="outline" onClick={() => setIsScannerOpen(true)} disabled={isImportParsing}>
                        <QrCode className="mr-2 h-4 w-4" /> 扫码
                      </Button>
                    )}
                </div>
                <label className="flex items-center gap-2 text-sm text-muted-foreground">
                    <input
                        type="checkbox"
                        checked={useAIParse}
                        onChange={e => setUseAIParse(e.target.checked)}
                        disabled={isImportParsing}
                        className="h-4 w-4 rounded border-input disabled:opacity-50"
                    />
                    AI 解析（平台编码 / 扫码）
                </label>
             </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label>分类</Label>
                    <div className="relative">
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
                                    {categories
                                        .filter(c => c.name.toLowerCase().includes(categoryInput.toLowerCase()))
                                        .map(c => (
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
                                    {categories.filter(c => c.name.toLowerCase().includes(categoryInput.toLowerCase())).length === 0 && (
                                        <div className="px-3 py-2 text-sm text-muted-foreground">
                                            {categoryInput ? '按回车创建新分类' : '无匹配分类'}
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
                <div className="space-y-2">
                    <Label>元件编号</Label>
                    <Input
                      value={formData.component_number || ''}
                      onChange={e => setFormData({ ...formData, component_number: e.target.value })}
                      placeholder="留空则保存时自动生成，例如 HB-000001"
                    />
                </div>
                <div className="space-y-2">
                    <Label>名称</Label>
                    <Input value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
                </div>
                <div className="space-y-2">
                    <Label>厂家型号</Label>
                    <Input value={formData.model || ''} onChange={e => setFormData({...formData, model: e.target.value})} placeholder="例如 RC0603FR-0710KL" />
                </div>
                <div className="space-y-2">
                    <Label>制造商</Label>
                    <Input value={formData.manufacturer || ''} onChange={e => setFormData({...formData, manufacturer: e.target.value})} placeholder="例如 YAGEO" />
                </div>
                <div className="space-y-2">
                    <Label>参数值</Label>
                    <Input value={formData.value || ''} onChange={e => setFormData({...formData, value: e.target.value})} />
                </div>
                <div className="space-y-2">
                    <Label>封装</Label>
                    <div className="relative">
                        <Input
                            value={formData.package || ''}
                            onChange={e => {
                                setFormData({ ...formData, package: e.target.value });
                                setShowPackageDropdown(true);
                            }}
                            onFocus={() => setShowPackageDropdown(true)}
                            onBlur={() => setTimeout(() => setShowPackageDropdown(false), 200)}
                            placeholder="例如 0603"
                        />
                        {showPackageDropdown && (
                            <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                                {packageOptions
                                    .filter(pkg => pkg.toLowerCase().includes((formData.package || '').toLowerCase()))
                                    .map(pkg => (
                                    <div
                                        key={pkg}
                                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                                        onClick={() => {
                                            setFormData({ ...formData, package: pkg });
                                            setShowPackageDropdown(false);
                                        }}
                                    >
                                        {pkg}
                                    </div>
                                ))}
                                {packageOptions.filter(pkg => pkg.toLowerCase().includes((formData.package || '').toLowerCase())).length === 0 && (
                                    <div className="px-3 py-2 text-sm text-muted-foreground">
                                        {(formData.package || '') ? '无匹配封装' : '无历史封装'}
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
                                {suppliers
                                    .filter(s => s.name.toLowerCase().includes(supplierInput.toLowerCase()))
                                    .map(s => (
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
                                {suppliers.filter(s => s.name.toLowerCase().includes(supplierInput.toLowerCase())).length === 0 && (
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
                    <Input value={formData.supplier_part_number || ''} onChange={e => setFormData({...formData, supplier_part_number: e.target.value})} placeholder="例如 C2040" />
                </div>
                <div className="space-y-2">
                    <Label>库存数量</Label>
                    <div className="flex items-center space-x-2">
                        <Button 
                            variant="outline" 
                            size="icon" 
                            className="h-10 w-10 shrink-0"
                            onClick={() => setFormData(prev => ({...prev, stock_quantity: Math.max(0, (prev.stock_quantity || 0) - 1)}))}
                        >
                            <Minus className="h-4 w-4" />
                        </Button>
                        <Input 
                            type="number" 
                            value={formData.stock_quantity} 
                            onChange={e => setFormData({...formData, stock_quantity: Number(e.target.value)})} 
                            className="text-center"
                        />
                         <Button 
                            variant="outline" 
                            size="icon" 
                            className="h-10 w-10 shrink-0"
                            onClick={() => setFormData(prev => ({...prev, stock_quantity: (prev.stock_quantity || 0) + 1}))}
                        >
                            <Plus className="h-4 w-4" />
                        </Button>
                    </div>
                </div>
                <div className="space-y-2">
                    <Label>位置</Label>
                    <div className="relative">
                        <Input
                            value={formData.location || ''}
                            onChange={e => {
                                setFormData({ ...formData, location: e.target.value });
                                setShowLocationDropdown(true);
                            }}
                            onFocus={() => setShowLocationDropdown(true)}
                            onBlur={() => setTimeout(() => setShowLocationDropdown(false), 200)}
                            placeholder="例如 A1-03"
                        />
                        {showLocationDropdown && (
                            <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                                {locationOptions
                                    .filter(loc => loc.toLowerCase().includes((formData.location || '').toLowerCase()))
                                    .map(loc => (
                                    <div
                                        key={loc}
                                        className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                                        onClick={() => {
                                            setFormData({ ...formData, location: loc });
                                            setShowLocationDropdown(false);
                                        }}
                                    >
                                        {loc}
                                    </div>
                                ))}
                                {locationOptions.filter(loc => loc.toLowerCase().includes((formData.location || '').toLowerCase())).length === 0 && (
                                    <div className="px-3 py-2 text-sm text-muted-foreground">
                                        {(formData.location || '') ? '无匹配位置' : '无历史位置'}
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </div>
            <div className="space-y-2">
                <Label>描述</Label>
                <textarea 
                    className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={formData.description || ''}
                    onChange={e => setFormData({...formData, description: e.target.value})}
                />
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label>图片</Label>
                    <div className="relative">
                        <div 
                            className="w-full h-48 bg-secondary/20 rounded-md border flex items-center justify-center overflow-hidden relative cursor-pointer group hover:bg-secondary/30 transition-colors"
                            onClick={() => setShowImageMenu(true)}
                        >
                            {previewUrl || formData.image_url ? (
                                <img 
                                    src={previewUrl || formData.image_url} 
                                    alt="Preview" 
                                    className="w-full h-full object-contain" 
                                    onError={(e) => {
                                         // 错误处理：如果是本地预览URL失效或后端404，显示占位
                                         // 为避免死循环，仅隐藏并显示父容器的备选内容（如果有的话，但这里父容器被img占满了）
                                         // 简单的做法是隐藏 img，显示一个 fallback div
                                         const target = e.target as HTMLImageElement;
                                         target.style.display = 'none';
                                         // 这里我们依赖 react 重新渲染或者简单的 DOM 操作显示 fallback
                                         // 我们可以通过 state 控制，但为了简单，直接操作 DOM 添加一个兄弟元素显示
                                         const parent = target.parentElement;
                                         if (parent) {
                                             const fallback = document.createElement('div');
                                             fallback.className = "flex flex-col items-center text-muted-foreground absolute inset-0 justify-center";
                                             fallback.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mb-2"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg><span>点击上传/拍摄图片</span>';
                                             parent.appendChild(fallback);
                                         }
                                    }}
                                />
                            ) : (
                                <div className="flex flex-col items-center text-muted-foreground">
                                    <Camera className="h-8 w-8 mb-2" />
                                    <span>点击上传/拍摄图片</span>
                                </div>
                            )}
                            
                            {/* 隐藏的文件输入 */}
                            <input 
                                type="file" 
                                ref={fileInputRef} 
                                className="hidden" 
                                accept="image/*" 
                                onChange={handleFileSelect} 
                            />

                            {/* 菜单遮罩 */}
                            {showImageMenu && (
                                <div 
                                    className="absolute inset-0 bg-background/95 z-10 flex flex-col items-center justify-center gap-2 p-8 animate-in fade-in duration-200" 
                                    onClick={(e) => e.stopPropagation()}
                                >
                                    <Button onClick={() => { fileInputRef.current?.click(); setShowImageMenu(false); }} className="w-full">
                                        <Upload className="mr-2 h-4 w-4" /> 上传图片
                                    </Button>
                                    <Button onClick={() => setIsCameraOpen(true)} className="w-full">
                                        <Camera className="mr-2 h-4 w-4" /> 拍摄图片
                                    </Button>
                                    <Button variant="outline" onClick={() => { setShowUrlInput(true); setShowImageMenu(false); }} className="w-full">
                                        <Link className="mr-2 h-4 w-4" /> 输入 URL
                                    </Button>
                                    <Button variant="ghost" onClick={() => setShowImageMenu(false)} className="w-full text-destructive">
                                        取消
                                    </Button>
                                </div>
                            )}
                        </div>
                        
                        {/* URL 输入框 */}
                        {showUrlInput && (
                            <div className="mt-2 flex gap-2 animate-in slide-in-from-top-2">
                                <Input 
                                    value={formData.image_url || ''} 
                                    onChange={e => {
                                        setFormData({...formData, image_url: e.target.value});
                                        if (e.target.value) {
                                            setPreviewUrl(''); // 如果输入 URL，清除本地预览
                                            setSelectedFile(null);
                                        }
                                    }} 
                                    placeholder="输入图片 URL" 
                                    autoFocus
                                />
                                <Button variant="ghost" size="icon" onClick={() => setShowUrlInput(false)}><X className="h-4 w-4" /></Button>
                            </div>
                        )}
                    </div>
                </div>
                <div className="space-y-2">
                    <Label>数据手册 URL</Label>
                    <Input value={formData.datasheet_url || ''} onChange={e => setFormData({...formData, datasheet_url: e.target.value})} placeholder="http://..." />
                </div>
            </div>
        </div>
      </Modal>

      {/* Stock Modal */}
      <Modal
        isOpen={isStockOpen}
        onClose={() => setIsStockOpen(false)}
        title="库存变更"
        footer={<><Button variant="outline" onClick={() => setIsStockOpen(false)}>取消</Button><Button onClick={handleStockSubmit}>确认</Button></>}
      >
        <div className="space-y-4 py-4">
            <div className="flex gap-4">
                <Button 
                    variant={stockForm.type === 'in' ? 'default' : 'outline'} 
                    onClick={() => setStockForm({...stockForm, type: 'in'})}
                    className="flex-1"
                >入库</Button>
                <Button 
                    variant={stockForm.type === 'out' ? 'destructive' : 'outline'} 
                    onClick={() => setStockForm({...stockForm, type: 'out'})}
                    className="flex-1"
                >出库</Button>
            </div>
            <div className="space-y-2">
                <Label>数量</Label>
                <div className="flex items-center space-x-2">
                    <Button 
                        variant="outline" 
                        size="icon" 
                        className="h-10 w-10 shrink-0"
                        onClick={() => setStockForm(prev => ({...prev, amount: Math.max(1, prev.amount - 1)}))}
                    >
                        <Minus className="h-4 w-4" />
                    </Button>
                    <Input 
                        type="number" 
                        min="1" 
                        value={stockForm.amount} 
                        onChange={e => setStockForm({...stockForm, amount: Number(e.target.value)})} 
                        className="text-center"
                    />
                     <Button 
                        variant="outline" 
                        size="icon" 
                        className="h-10 w-10 shrink-0"
                        onClick={() => setStockForm(prev => ({...prev, amount: prev.amount + 1}))}
                    >
                        <Plus className="h-4 w-4" />
                    </Button>
                </div>
            </div>
             <div className="space-y-2">
                <Label>备注</Label>
                <Input value={stockForm.reason} onChange={e => setStockForm({...stockForm, reason: e.target.value})} />
            </div>
        </div>
      </Modal>

      {/* Logs Modal */}
      <Modal isOpen={isLogsOpen} onClose={() => setIsLogsOpen(false)} title="库存记录">
         <div className="max-h-[60vh] overflow-auto space-y-4">
            {componentLogs.length === 0 ? <div className="text-center text-muted-foreground py-8">暂无记录</div> : 
             componentLogs.map(log => (
                 <div key={log.id} className="flex justify-between items-center p-3 rounded-lg bg-secondary/10">
                     <div className="flex items-center gap-3">
                         <div className={`w-8 h-8 rounded-full flex items-center justify-center ${log.change_amount > 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                             {log.change_amount > 0 ? '+' : '-'}
                         </div>
                         <div>
                             <div className="font-medium">{Math.abs(log.change_amount)}</div>
                             <div className="text-xs text-muted-foreground">{new Date(log.created_at).toLocaleString()}</div>
                         </div>
                     </div>
                     <div className="text-sm text-right text-muted-foreground">{log.reason || '无备注'}</div>
                 </div>
             ))
            }
         </div>
      </Modal>

      {/* Scanner Modal */}
      <Modal isOpen={isScannerOpen} onClose={() => setIsScannerOpen(false)} title="扫描二维码">
        <QRScanner onScan={handleScan} onClose={() => setIsScannerOpen(false)} autoStart={isMobile} />
      </Modal>

      {/* Camera Modal */}
      <Modal isOpen={isCameraOpen} onClose={() => setIsCameraOpen(false)} title="拍摄图片" className="max-w-md">
        <div className="h-[60vh] md:h-[500px]">
            {isCameraOpen && <CameraCapture onCapture={handleCapture} onClose={() => setIsCameraOpen(false)} />}
        </div>
      </Modal>

      {/* Image Preview Modal */}
      <Modal 
        isOpen={!!previewImage} 
        onClose={() => setPreviewImage(null)} 
        title="图片预览" 
        className="max-w-4xl"
      >
        <div className="flex justify-center items-center bg-black/5 rounded-md overflow-hidden min-h-[200px]">
            {previewImage && (
                <img 
                    src={previewImage} 
                    alt="Preview" 
                    className="max-w-full max-h-[80vh] object-contain" 
                />
            )}
        </div>
      </Modal>

      {isImportParsing && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="flex flex-col items-center gap-4 rounded-lg border bg-background px-8 py-6 shadow-lg">
            <Loader2 className="h-10 w-10 animate-spin text-primary" />
            <p className="text-sm font-medium">
              {useAIParse ? 'AI 解析中，请稍候...' : '正在解析元件信息...'}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

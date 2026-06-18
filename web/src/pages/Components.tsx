import { useEffect, useState } from 'react';
import { useRef } from 'react';
import { Plus, Minus, Search, Edit, Trash2, Database, History, QrCode, Camera, Upload, Link, X } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Component, type Category, type Supplier, type StockLog, type Pagination } from '../types';
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

export default function Components() {
  const [components, setComponents] = useState<Component[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0 });
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');

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
  const [platformParsing, setPlatformParsing] = useState(false);
  const [useAIParse, setUseAIParse] = useState(false);

  const [isMobile, setIsMobile] = useState(false);

  const getSupplierNameFromPlatform = (platformName?: string) => {
    if (!platformName) return '';
    if (platformName.includes('立创') || platformName.includes('LCSC')) return '嘉立创';
    return platformName;
  };

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  const fetchComponents = async (page = 1) => {
    setLoading(true);
    try {
      const params: ComponentSearchParams = { page, page_size: pagination.page_size };
      if (search) params.keyword = search;
      if (selectedCategory) params.category_id = selectedCategory;

      const res = await client.get('/components', { params });
      setComponents(res.data.data || []);
      setPagination(res.data.pagination || { page: 1, page_size: 20, total: 0 });
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

  useEffect(() => {
    fetchCategories();
    fetchSuppliers();
    fetchComponents();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSearch = () => {
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('确定删除此元件吗？')) return;
    try {
      await client.delete(`/components/${id}`);
      toast.success('删除成功');
      fetchComponents(pagination.page);
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
      fetchComponents(pagination.page);
    } catch (error) {
      console.error(error);
      toast.error('保存失败');
    }
  };

  const parsePlatform = async () => {
    if (!platformCode) return;
    setPlatformParsing(true);
    try {
      const res = await client.post('/components/parse', { code: platformCode, use_llm: useAIParse });
      const data = res.data.data;
      setSupplierInput(getSupplierNameFromPlatform(data.platform_name));
      setFormData(prev => ({
        ...prev,
        name: data.name || data.model,
        value: data.value,
        package: data.package,
        supplier_part_number: data.platform_code,
        description: data.description + (data.manufacturer ? `\n制造商: ${data.manufacturer}` : ''),
        datasheet_url: data.datasheet_url,
        image_url: data.image_url,
      }));
      toast.success('解析成功');
    } catch {
      toast.error('解析失败');
    } finally {
      setPlatformParsing(false);
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
      fetchComponents(pagination.page);
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
    setIsScannerOpen(false);
    try {
      const res = await client.post('/components/parse-qrcode', { qrcode_data: data });
      const { component, quantity } = res.data.data;
      
      // If we are in form mode (adding/editing), fill the form
      if (isFormOpen) {
         setFormData(prev => ({
            ...prev,
            name: component.name,
            value: component.value,
            package: component.package,
            supplier_part_number: component.platform_code,
            description: component.description,
            datasheet_url: component.datasheet_url,
            image_url: component.image_url,
            stock_quantity: quantity > 0 ? quantity : prev.stock_quantity
         }));
         setSupplierInput(getSupplierNameFromPlatform(component.platform_name));
         if (component.category?.name) {
             setCategoryInput(component.category.name);
         }
         toast.success('已识别元件信息');
      } else {
        // Just show info
        toast.success(`识别成功: ${component.name}`);
        // Automatically open form for new entry or edit
        openForm(component);
        setSupplierInput(getSupplierNameFromPlatform(component.platform_name));
        if (quantity) {
             setFormData(prev => ({ ...prev, stock_quantity: quantity }));
        }
      }
    } catch {
      toast.error('二维码解析失败');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <h2 className="text-3xl font-bold tracking-tight">元件管理</h2>
        <div className="flex gap-2">
            {isMobile && (
                <Button onClick={() => setIsScannerOpen(true)} variant="outline">
                    <QrCode className="mr-2 h-4 w-4" /> 扫码录入
                </Button>
            )}
            <Button onClick={() => openForm()}>
                <Plus className="mr-2 h-4 w-4" /> 添加元件
            </Button>
        </div>
      </div>

      <div className="flex flex-col md:flex-row gap-4">
        <div className="flex-1 flex gap-2">
          <Input 
            placeholder="搜索元件、参数、供应商或料号..." 
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

      <div className="rounded-md border">
        <div className="w-full overflow-auto">
          <table className="w-full caption-bottom text-sm text-left">
            <thead className="[&_tr]:border-b">
              <tr className="border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted">
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">图片</th>
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">名称</th>
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
                <tr><td colSpan={12} className="p-4 text-center">加载中...</td></tr>
              ) : components.map(component => (
                <tr key={component.id} className="border-b transition-colors hover:bg-muted/50">
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
                  <td className="p-4 align-middle font-medium">{component.name}</td>
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
      <div className="flex justify-end gap-2 items-center">
          <Button 
            variant="outline" 
            disabled={pagination.page <= 1} 
            onClick={() => { setPagination(p => ({...p, page: p.page - 1})); fetchComponents(pagination.page - 1); }}
          >上一页</Button>
          <span>第 {pagination.page} 页</span>
          <Button 
            variant="outline" 
            disabled={components.length < pagination.page_size} 
            onClick={() => { setPagination(p => ({...p, page: p.page + 1})); fetchComponents(pagination.page + 1); }}
          >下一页</Button>
      </div>

      {/* Edit/Add Modal */}
      <Modal 
        isOpen={isFormOpen} 
        onClose={() => setIsFormOpen(false)} 
        title={editingComponent ? '编辑元件' : '添加元件'}
        footer={<><Button variant="outline" onClick={() => setIsFormOpen(false)}>取消</Button><Button onClick={handleFormSubmit}>保存</Button></>}
        className="max-w-2xl"
      >
        <div className="grid gap-4 py-4 max-h-[70vh] overflow-y-auto px-1">
             {/* Import Tools */}
             {!editingComponent && (
                <div className="bg-secondary/20 p-4 rounded-md space-y-4">
                    <Label>快速导入</Label>
                    <div className="flex gap-2">
                        <Input placeholder="输入平台编码 (如 C2040)" value={platformCode} onChange={e => setPlatformCode(e.target.value)} />
                        <Button onClick={parsePlatform} disabled={platformParsing}>{platformParsing ? '解析中...' : '解析'}</Button>
                        <Button variant="outline" onClick={() => setIsScannerOpen(true)}><QrCode className="mr-2 h-4 w-4" /> 扫码</Button>
                    </div>
                    <label className="flex items-center gap-2 text-sm text-muted-foreground">
                        <input
                            type="checkbox"
                            checked={useAIParse}
                            onChange={e => setUseAIParse(e.target.checked)}
                            className="h-4 w-4 rounded border-input"
                        />
                        AI 解析
                    </label>
                </div>
             )}

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
                    <Label>名称</Label>
                    <Input value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
                </div>
                <div className="space-y-2">
                    <Label>参数值</Label>
                    <Input value={formData.value || ''} onChange={e => setFormData({...formData, value: e.target.value})} />
                </div>
                <div className="space-y-2">
                    <Label>封装</Label>
                    <Input value={formData.package || ''} onChange={e => setFormData({...formData, package: e.target.value})} />
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
                    <Input value={formData.location || ''} onChange={e => setFormData({...formData, location: e.target.value})} />
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
    </div>
  );
}

import { ComponentLogsDialog } from '../components/inventory/ComponentLogsDialog';
import { inventoryApi, componentSaveInput, type ComponentDraft } from '../api/inventory';
import { useConfirm } from '../context/confirm';
import { useListQuery } from '../hooks/useListQuery';
import { useResource } from '../hooks/useResource';
import { useFormGuard, useSubmitLock } from '../hooks/useFormGuard';
import { InventoryFields } from '../components/inventory/InventoryFields';
import { SmartEntry } from '../components/inventory/SmartEntry';
import { SearchSelect } from '../components/ui/SearchSelect';
import { PageState } from '../components/ui/PageState';
import { Pagination as PaginationControl } from '../components/ui/Pagination';
import { SafeImage } from '../components/ui/SafeImage';
import { ComponentDetail } from '../components/inventory/ComponentDetail';
import { ColumnSettingsPanel } from '../components/inventory/ColumnSettings';
import { EXPORT_COLUMNS, VALID_SORT_BY_KEYS, getColumnHeader, createDefaultExportColumns, createDefaultTableColumns, readStoredExportColumns, persistExportColumns, readStoredTableColumns, persistTableColumns, readStoredComponentSort, persistComponentSort, type ExportColumnKey, type ComponentSortOrder, type ColumnState } from '../components/inventory/columns';
import type { ParseMode } from '../types';
import { useParseTask } from '../hooks/useParseTask';
import { useTaskApplication } from '../hooks/useTaskApplication';
import { mergeParsed, parsedComponentFields, matchingSupplierReference } from '../utils/parseResult';
import { lazy, Suspense, useEffect, useState, useRef, Fragment } from 'react';
import { Plus, Minus, Search, Edit, Copy, Trash2, Database, History, QrCode, Camera, Upload, SlidersHorizontal, X, Loader2, Hash, Download, Coins, Columns3, PackageMinus } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Component, type Category, type Supplier, type Pagination, type ComponentOptions, type ParsedComponentInfo, type ReviewedImageItem, type ParseTask } from '../types';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { Label } from '../components/ui/Label';
import { PageHeader } from '../components/ui/PageHeader';
import { QuantityShortcuts } from '../components/ui/QuantityShortcuts';
import { RowActionsMenu } from '../components/ui/RowActionsMenu';
import { BatchStockOutModal } from '../components/BatchStockOutModal';
import { usePhotoDrafts } from '../hooks/usePhotoDrafts';
import { PhotoDraftNavigation } from '../components/PhotoDraftNavigation';
import { photoComponentInfo } from '../utils/photoResult';
import { PhotoParseModal } from '../components/PhotoParseModal';
const QRScanner = lazy(() => import('../components/QRScanner'));
const CameraCapture = lazy(() => import('../components/CameraCapture'));
import { yuanToCents, formatCents, formatMicro, calcUnitPriceMicro, calcOutboundCostCents } from '../utils/price';
import { copyToClipboard } from '../utils/clipboard';
import { cn } from '../utils/cn';


type ComponentSearchFilters = {
  keyword: string;
  component_number: string;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  supplier: string;
  supplier_part_number: string;
};

const EMPTY_SEARCH_FILTERS: ComponentSearchFilters = {
  keyword: '',
  component_number: '',
  name: '',
  model: '',
  manufacturer: '',
  value: '',
  supplier: '',
  supplier_part_number: '',
};

const SEARCH_FILTER_FIELDS: { key: keyof ComponentSearchFilters; label: string; placeholder: string }[] = [
  { key: 'component_number', label: '编号', placeholder: 'HB-000001' },
  { key: 'name', label: '名称', placeholder: '元件名称' },
  { key: 'model', label: '厂家型号', placeholder: 'RC0603FR-0710KL' },
  { key: 'value', label: '参数', placeholder: '10k（空格拆词）' },
  { key: 'supplier_part_number', label: '料号', placeholder: 'C2040' },
];

const SEARCH_PARAM_KEYS: (keyof ComponentSearchFilters)[] = [
  'keyword',
  'component_number',
  'name',
  'model',
  'manufacturer',
  'value',
  'supplier',
  'supplier_part_number',
];


const renderCopyableCell = (value: string | null | undefined, className?: string) => {
  const text = value?.trim();
  if (!text) {
    return <td className={cn('p-4 align-middle', className)}>-</td>;
  }
  return (
    <td className={cn('p-4 align-middle', className)}>
      <button
        type="button"
        title="点击复制"
        className="cursor-pointer text-left hover:text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
        onClick={async () => {
          const ok = await copyToClipboard(text);
          if (ok) toast.success('已复制');
          else toast.error('复制失败');
        }}
      >
        {text}
      </button>
    </td>
  );
};

export default function Components() {
  const confirm = useConfirm();
  const { params: urlParams, updateQuery, page: urlPage, pageSize: urlPageSize } = useListQuery();
  const storedSort = readStoredComponentSort();
  const query = new URLSearchParams();
  for (const key of SEARCH_PARAM_KEYS) { const value = urlParams.get(key)?.trim(); if (value) query.set(key, value); }
  if (urlParams.get('category_id')) query.set('category_id', urlParams.get('category_id')!);
  query.set('page', String(urlPage)); query.set('page_size', String(urlPageSize));
  query.set('sort_by', VALID_SORT_BY_KEYS.has(urlParams.get('sort_by') as ExportColumnKey) ? urlParams.get('sort_by')! : storedSort.sortBy);
  query.set('sort_order', urlParams.get('sort_order') === 'asc' || urlParams.get('sort_order') === 'desc' ? urlParams.get('sort_order')! : storedSort.sortOrder);
  const queryKey = query.toString();
  const resource = useResource<{ data: Component[]; pagination: Pagination }>(`/components?${queryKey}`);
  const loading = resource.loading;
  const [advanced, setAdvanced] = useState(false);
  const [detailID, setDetailID] = useState<number | null>(null);
  const stockMutation = useSubmitLock();
  const backfillMutation = useSubmitLock();

  const [components, setComponents] = useState<Component[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [searchFilters, setSearchFilters] = useState<ComponentSearchFilters>(() => Object.fromEntries(SEARCH_PARAM_KEYS.map(key => [key, urlParams.get(key) || ''])) as ComponentSearchFilters);
  const [selectedCategory, setSelectedCategory] = useState<string>(urlParams.get('category_id') || '');
  const [categorySearchInput, setCategorySearchInput] = useState('');
  const [manufacturerOptions, setManufacturerOptions] = useState<string[]>([]);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [activeRowId, setActiveRowId] = useState<number | null>(null);
  const [mobileActiveRowId, setMobileActiveRowId] = useState<number | null>(null);
  const [isBatchLocationOpen, setIsBatchLocationOpen] = useState(false);
  const [batchLocation, setBatchLocation] = useState('');
  const [isBatchUpdating, setIsBatchUpdating] = useState(false);
  const [isBatchStockOutOpen, setIsBatchStockOutOpen] = useState(false);
  const [batchStockOutSeed, setBatchStockOutSeed] = useState<Component[]>([]);
  const [isGeneratingNumbers, setIsGeneratingNumbers] = useState(false);
  const [isExportOpen, setIsExportOpen] = useState(false);
  const [exportColumns, setExportColumns] = useState<ColumnState[]>(() => readStoredExportColumns());
  const [exportColumnsDraft, setExportColumnsDraft] = useState<ColumnState[]>(() => readStoredExportColumns());
  const [isExporting, setIsExporting] = useState(false);
  const [tableColumns, setTableColumns] = useState<ColumnState[]>(() => readStoredTableColumns());
  const [tableColumnsDraft, setTableColumnsDraft] = useState<ColumnState[]>(() => readStoredTableColumns());
  const [isColumnSettingsOpen, setIsColumnSettingsOpen] = useState(false);
  const [sortBy, setSortBy] = useState<ExportColumnKey>(() => readStoredComponentSort().sortBy);
  const [sortOrder, setSortOrder] = useState<ComponentSortOrder>(() => readStoredComponentSort().sortOrder);

  // Modals
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isStockOpen, setIsStockOpen] = useState(false);
  const [isBackfillOpen, setIsBackfillOpen] = useState(false);
  const [isLogsOpen, setIsLogsOpen] = useState(false);
  const [isScannerOpen, setIsScannerOpen] = useState(false);
  const [isCameraOpen, setIsCameraOpen] = useState(false);
  const [previewImage, setPreviewImage] = useState<string | null>(null);

  // Form State
  const [editingComponent, setEditingComponent] = useState<Component | null>(null);
  const [isCopyingComponent, setIsCopyingComponent] = useState(false);
  const [formData, setFormData] = useState<ComponentDraft>({
    name: '', category_id: undefined, stock_quantity: 0
  });
  const [categoryInput, setCategoryInput] = useState('');
  const [supplierInput, setSupplierInput] = useState('');
  const [packageOptions, setPackageOptions] = useState<string[]>([]);
  const [locationOptions, setLocationOptions] = useState<string[]>([]);
  
  // Image Upload State
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string>('');
  const [formTotalPriceYuan, setFormTotalPriceYuan] = useState('');


  // Stock State
  const [stockForm, setStockForm] = useState({ type: 'in', amount: 1, reason: '', totalPriceYuan: '' });
  const [backfillTarget, setBackfillTarget] = useState<Component | null>(null);
  const [backfillForm, setBackfillForm] = useState({ totalPriceYuan: '', quantity: 1 });
  
  // Platform Import
  const [platformCode, setPlatformCode] = useState('');
  const [supplierReference, setSupplierReference] = useState<ParsedComponentInfo>();

  const [parseMode, setParseMode] = useState<ParseMode>('standard');
  const [isPhotoParseOpen, setIsPhotoParseOpen] = useState(false);
  const photoDrafts = usePhotoDrafts<PhotoDraft>();
  const [recoveredPhotoTask, setRecoveredPhotoTask] = useState<ParseTask>();
  const savingRef = useRef(false);
  const [saving, setSaving] = useState(false);

  const [isMobile, setIsMobile] = useState(false);
  const formGuard = useFormGuard({ form: formData, category: categoryInput, supplier: supplierInput, price: formTotalPriceYuan, file: selectedFile?.name || '' });
  const closeForm = () => { parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false); };
  const cancelForm = async () => { if (!saving && await formGuard.allowClose()) closeForm(); };


  const getSupplierNameFromPlatform = (platformName?: string) => {
    if (!platformName) return '';
    if (platformName.includes('立创') || platformName.includes('LCSC')) return '嘉立创';
    return platformName;
  };

  const parsedInfoToFormData = parsedComponentFields;

  type ParseSnapshot = { platformCode: string; form: ComponentDraft; category: string; supplier: string; price: string; preview: string; file: File | null };
  type PhotoDraft = ParseSnapshot & { platformCode: string };
  const applyParsedComponent = (component: ParsedComponentInfo, quantity?: number, snapshot?: ParseSnapshot) => {
    setSupplierReference(component);
    setPlatformCode(prev => !snapshot || prev === snapshot.platformCode ? component.platform_code || prev : prev);
    const parsedForm: Partial<Component> = parsedInfoToFormData(component);
    if (snapshot && selectedFile !== snapshot.file) delete parsedForm.image_url;
    if (quantity !== undefined && quantity >= 0) parsedForm.stock_quantity = quantity;
    const supplier = getSupplierNameFromPlatform(component.platform_name);
    if (supplier) setSupplierInput(prev => !snapshot || prev === snapshot.supplier ? supplier : prev);
    const category = component.category_name || component.category?.name;
    if (category) setCategoryInput(prev => !snapshot || prev === snapshot.category ? category : prev);
    setFormData(prev => mergeParsed(prev, parsedForm, snapshot?.form));
    if (component.image_url && (!snapshot || selectedFile === snapshot.file)) {
      setPreviewUrl(prev => !snapshot || prev === snapshot.preview ? '' : prev);
    }
    if (component.price && component.price > 0) {
      const qty = quantity && quantity > 0 ? quantity : 1;
      setFormTotalPriceYuan(prev => !snapshot || prev === snapshot.price ? (component.price! * qty).toFixed(2) : prev);
    }
  };
  const parseTask = useParseTask<ParseSnapshot>((result, snapshot) => { if (result.component) applyParsedComponent(result.component, result.quantity, snapshot); });
  const isImportParsing = parseTask.busy;
  const parseSnapshot = (): ParseSnapshot => ({ platformCode, form: { ...formData }, category: categoryInput, supplier: supplierInput, price: formTotalPriceYuan, preview: previewUrl, file: selectedFile });

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  const resolveCategoryId = (categoryInput: string, categoryId: string): string => {
    if (categoryId) return categoryId;
    const name = categoryInput.trim();
    if (!name) return '';
    const found = categories.find(c => c.name.toLowerCase() === name.toLowerCase());
    return found ? String(found.id) : '';
  };

  const fetchComponents = async (
    page = urlPage, pageSize = urlPageSize, filters: ComponentSearchFilters = searchFilters,
    categoryInput = categorySearchInput, categoryId = selectedCategory,
    nextSortBy: ExportColumnKey = sortBy, nextSortOrder: ComponentSortOrder = sortOrder,
  ) => {
    const patch: Record<string, string | number> = { page, page_size: pageSize, category_id: resolveCategoryId(categoryInput, categoryId), sort_by: nextSortBy, sort_order: nextSortOrder };
    for (const key of SEARCH_PARAM_KEYS) patch[key] = filters[key].trim();
    updateQuery(patch);
    resource.refresh();
  };
  useEffect(() => {
    const query = new URLSearchParams(queryKey);
    setSearchFilters(Object.fromEntries(SEARCH_PARAM_KEYS.map(key => [key, query.get(key) || ''])) as ComponentSearchFilters);
    setSelectedCategory(query.get('category_id') || '');
    setCategorySearchInput('');
    setSortBy(query.get('sort_by') as ExportColumnKey); setSortOrder(query.get('sort_order') as ComponentSortOrder);
    setSelectedIds([]);
  }, [queryKey]);
  useEffect(() => {
    if (!resource.data) return;
    setComponents(resource.data.data || []);
    const next = resource.data.pagination;
    setPagination(next);
    const lastPage = Math.max(1, Math.ceil(next.total / next.page_size));
    if (next.page > lastPage) updateQuery({ page: lastPage }, true);
  }, [resource.data, updateQuery]);

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
      setManufacturerOptions(options.manufacturers || []);
    } catch {
      toast.error('加载历史选项失败');
    }
  };

  useEffect(() => {
    fetchCategories();
    fetchSuppliers();
    fetchComponentOptions();
  }, []);

  const handleSearch = () => {
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1, pagination.page_size);

  };

  const handleClearFilters = () => {
    setSearchFilters(EMPTY_SEARCH_FILTERS);
    setSelectedCategory('');
    setCategorySearchInput('');
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1, pagination.page_size, EMPTY_SEARCH_FILTERS, '', '', sortBy, sortOrder);
  };

  const handleSortByChange = (value: ExportColumnKey) => {
    setSortBy(value);
    persistComponentSort(value, sortOrder);
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1, pagination.page_size, searchFilters, categorySearchInput, selectedCategory, value, sortOrder);
  };

  const handleSortOrderChange = (value: ComponentSortOrder) => {
    setSortOrder(value);
    persistComponentSort(sortBy, value);
    setPagination(prev => ({ ...prev, page: 1 }));
    fetchComponents(1, pagination.page_size, searchFilters, categorySearchInput, selectedCategory, sortBy, value);
  };

  const currentSortLabel = EXPORT_COLUMNS.find(column => column.key === sortBy)?.defaultHeader || sortBy;
  const currentSortOrderLabel = sortOrder === 'asc' ? '升序' : '降序';

  const hasActiveFilters =
    selectedCategory !== '' ||
    categorySearchInput.trim() !== '' ||
    SEARCH_PARAM_KEYS.some(key => searchFilters[key].trim() !== '');

  const activeFilterCount =
    SEARCH_PARAM_KEYS.filter(key => searchFilters[key].trim() !== '').length +
    (selectedCategory !== '' || categorySearchInput.trim() !== '' ? 1 : 0);

  const handleSearchFilterChange = (key: keyof ComponentSearchFilters, value: string) => {
    setSearchFilters(prev => ({ ...prev, [key]: value }));
  };

  const renderSearchField = (key: keyof ComponentSearchFilters) => {
    const field = SEARCH_FILTER_FIELDS.find(item => item.key === key);
    if (!field) return null;
    return (
      <div key={field.key} className="space-y-1">
        <Label htmlFor={`search-${field.key}`} className="text-xs text-muted-foreground">
          {field.label}
        </Label>
        <Input
          id={`search-${field.key}`}
          placeholder={field.placeholder}
          value={searchFilters[field.key]}
          onChange={e => handleSearchFilterChange(field.key, e.target.value)}
          onKeyDown={handleSearchKeyDown}
        />
      </div>
    );
  };

  const handleSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') handleSearch();
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

  const openBatchStockOut = (seed: Component[] = []) => {
    setBatchStockOutSeed(seed);
    setIsBatchStockOutOpen(true);
  };

  const handleBatchStockOutSuccess = () => {
    setSelectedIds([]);
    fetchComponents(pagination.page, pagination.page_size);
  };

  const handleGenerateNumbers = async () => {
    if (!await confirm('将为所有未编号的元件自动生成编号，是否继续？')) return;
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

  const buildExportSearchParams = () => {
    const params = new URLSearchParams();
    const resolvedCategoryId = resolveCategoryId(categorySearchInput, selectedCategory);
    if (resolvedCategoryId) params.set('category_id', resolvedCategoryId);
    for (const key of SEARCH_PARAM_KEYS) {
      const value = searchFilters[key].trim();
      if (value) params.set(key, value);
    }
    params.set('sort_by', sortBy);
    params.set('sort_order', sortOrder);
    return params;
  };

  const toggleExportColumn = (key: ExportColumnKey, selected: boolean) => {
    setExportColumnsDraft(prev => prev.map(column => (column.key === key ? { ...column, selected } : column)));
  };

  const updateExportHeader = (key: ExportColumnKey, header: string) => {
    setExportColumnsDraft(prev => prev.map(column => (column.key === key ? { ...column, header } : column)));
  };

  const toggleExportSelectAll = (selected: boolean) => {
    setExportColumnsDraft(prev => prev.map(column => ({ ...column, selected })));
  };

  const reorderExportColumns = (next: ColumnState[]) => {
    setExportColumnsDraft(next);
  };

  const resetExportColumns = () => {
    setExportColumnsDraft(createDefaultExportColumns());
  };

  const openExportModal = () => {
    setExportColumnsDraft(exportColumns);
    setIsExportOpen(true);
  };

  const toggleTableColumn = (key: ExportColumnKey, selected: boolean) => {
    setTableColumnsDraft(prev => prev.map(column => (column.key === key ? { ...column, selected } : column)));
  };

  const updateTableHeader = (key: ExportColumnKey, header: string) => {
    setTableColumnsDraft(prev => prev.map(column => (column.key === key ? { ...column, header } : column)));
  };

  const toggleTableSelectAll = (selected: boolean) => {
    setTableColumnsDraft(prev => prev.map(column => ({ ...column, selected })));
  };

  const reorderTableColumns = (next: ColumnState[]) => {
    setTableColumnsDraft(next);
  };

  const resetTableColumns = () => {
    setTableColumnsDraft(createDefaultTableColumns());
  };

  const openColumnSettings = () => {
    setTableColumnsDraft(tableColumns);
    setIsColumnSettingsOpen(true);
  };

  const confirmTableColumns = () => {
    const selectedCount = tableColumnsDraft.filter(column => column.selected).length;
    if (selectedCount === 0) {
      toast.error('请至少选择一列');
      return;
    }
    setTableColumns(tableColumnsDraft);
    persistTableColumns(tableColumnsDraft);
    setIsColumnSettingsOpen(false);
  };

  const visibleTableColumns = tableColumns.filter(column => column.selected);

  const renderTableCell = (key: ExportColumnKey, component: Component) => {
    switch (key) {
      case 'component_number':
        return renderCopyableCell(component.component_number, 'font-mono text-xs');
      case 'name':
        return <td className="p-4 align-middle font-medium"><button className="text-left text-primary hover:underline" onClick={() => setDetailID(component.id)}>{component.name}</button></td>;
      case 'model':
        return renderCopyableCell(component.model);
      case 'manufacturer':
        return <td className="p-4 align-middle">{component.manufacturer || '-'}</td>;
      case 'value':
        return <td className="p-4 align-middle">{component.value}</td>;
      case 'package':
        return <td className="p-4 align-middle">{component.package}</td>;
      case 'description':
        return (
          <td className="p-4 align-middle max-w-[200px] truncate" title={component.description}>
            {component.description}
          </td>
        );
      case 'category':
        return <td className="p-4 align-middle">{component.category?.name}</td>;
      case 'stock_quantity':
        return (
          <td className="p-4 align-middle">
            <span className={`px-2 py-1 rounded-full text-xs font-medium ${
              component.stock_quantity < 10 ? 'bg-red-100 text-red-800' :
              component.stock_quantity < 50 ? 'bg-yellow-100 text-yellow-800' : 'bg-green-100 text-green-800'
            }`}>
              {component.stock_quantity}
            </span>
          </td>
        );
      case 'unit_price':
        return (
          <td className="p-4 align-middle">
            {component.unit_price_micro != null ? formatMicro(component.unit_price_micro) : '-'}
          </td>
        );
      case 'location':
        return <td className="p-4 align-middle">{component.location}</td>;
      case 'supplier':
        return <td className="p-4 align-middle">{component.supplier?.name || '-'}</td>;
      case 'supplier_part_number':
        return renderCopyableCell(component.supplier_part_number);
      case 'datasheet_url':
        return (
          <td className="p-4 align-middle">
            {component.datasheet_url ? (
              <a href={component.datasheet_url} target="_blank" rel="noreferrer" className="text-primary hover:underline">查看</a>
            ) : '-'}
          </td>
        );
      case 'created_at':
        return (
          <td className="p-4 align-middle whitespace-nowrap">
            {component.created_at ? new Date(component.created_at).toLocaleString() : '-'}
          </td>
        );
      case 'updated_at':
        return (
          <td className="p-4 align-middle whitespace-nowrap">
            {component.updated_at ? new Date(component.updated_at).toLocaleString() : '-'}
          </td>
        );
      default:
        return <td className="p-4 align-middle">-</td>;
    }
  };

  const handleExportCSV = async () => {
    const selectedColumns = exportColumnsDraft.filter(column => column.selected);
    if (selectedColumns.length === 0) {
      toast.error('请至少选择一列');
      return;
    }

    setIsExporting(true);
    try {
      const params = buildExportSearchParams();
      params.set('columns', selectedColumns.map(column => column.key).join(','));
      params.set(
        'headers',
        selectedColumns.map(column => getColumnHeader(column)).join(','),
      );

      const response = await inventoryApi.exportCSV(params);
      const blob = response.data;
      const disposition = response.headers['content-disposition'] as string | undefined;
      let filename = 'components.csv';
      if (disposition) {
        const match = disposition.match(/filename="?([^"]+)"?/);
        if (match?.[1]) filename = match[1];
      }

      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      toast.success('导出成功');
      setExportColumns(exportColumnsDraft);
      persistExportColumns(exportColumnsDraft);
      setIsExportOpen(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : '导出失败';
      toast.error(message);
    } finally {
      setIsExporting(false);
    }
  };

  const isAllExportSelected = exportColumnsDraft.length > 0 && exportColumnsDraft.every(column => column.selected);
  const isAllTableSelected = tableColumnsDraft.length > 0 && tableColumnsDraft.every(column => column.selected);

  const isAllSelected = components.length > 0 && selectedIds.length === components.length;

  const handleDelete = async (id: number) => {
    if (!await confirm('确定删除此元件吗？')) return;
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
    formGuard.reset({ form: component || { stock_quantity: 0 }, category: component?.category?.name || '', supplier: component?.supplier?.name || '', price: '', file: '' });
    parseTask.reset(); setSupplierReference(undefined);
    photoDrafts.reset();
    setIsPhotoParseOpen(false); setRecoveredPhotoTask(undefined);
    setSelectedFile(null);
    setIsCopyingComponent(false);
    setPlatformCode('');
    if (component) {
      if (component.id) {
          setEditingComponent(component);
          // 使用时间戳防止缓存
          setPreviewUrl(`/api/v1/components/${component.id}/image?t=${component.updated_at || ''}`);
      } else {
          setEditingComponent(null);
          setPreviewUrl('');
      }
      setFormData(component);
      setCategoryInput(component.category?.name || '');
      setSupplierInput(component.supplier?.name || '');
      setFormTotalPriceYuan('');
    } else {
      setEditingComponent(null);
      setFormData({ stock_quantity: 0 });
      setFormTotalPriceYuan('');
      setCategoryInput('');
      setSupplierInput('');
      setPreviewUrl('');
    }
    setIsFormOpen(true);
  };

  const openCopyForm = (component: Component) => {
    parseTask.reset(); setSupplierReference(undefined);
    photoDrafts.reset(); setIsPhotoParseOpen(false); setRecoveredPhotoTask(undefined);
    setSelectedFile(null);
    setEditingComponent(null);
    setIsCopyingComponent(true);
    setPlatformCode('');
    const copy = {
      category_id: component.category_id,
      supplier_id: component.supplier_id,
      component_number: '',
      name: component.name,
      model: component.model,
      manufacturer: component.manufacturer,
      value: component.value,
      package: component.package,
      supplier_part_number: component.supplier_part_number,
      description: component.description,
      stock_quantity: 0,
      location: component.location,
      datasheet_url: component.datasheet_url,
      image_url: component.image_url,
      category: component.category,
      supplier: component.supplier,
    };
    setFormData(copy);
    formGuard.reset({ form: copy, category: component.category?.name || '', supplier: component.supplier?.name || '', price: '', file: '' });
    setCategoryInput(component.category?.name || '');
    setSupplierInput(component.supplier?.name || '');
    setFormTotalPriceYuan('');
    setPreviewUrl(component.image_url || '');
    setIsFormOpen(true);
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      const url = URL.createObjectURL(file);
      setPreviewUrl(url);
      setFormData(prev => ({ ...prev, image_url: '' })); // 清除 URL，优先显示本地预览
      }
  };

  const handleCapture = (file: File) => {
    setSelectedFile(file);
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    setFormData(prev => ({ ...prev, image_url: '' }));
    setIsCameraOpen(false);
  };

  const currentPhotoDraft = (): PhotoDraft => ({ ...parseSnapshot(), platformCode });
  const loadPhotoDraft = (draft: PhotoDraft) => {
    parseTask.reset(); setSupplierReference(undefined);
    setFormData(draft.form); setCategoryInput(draft.category); setSupplierInput(draft.supplier);
    setFormTotalPriceYuan(draft.price); setSelectedFile(draft.file); setPreviewUrl(draft.preview);
    setPlatformCode(draft.platformCode);
  };
  const switchPhotoDraft = (index: number) => loadPhotoDraft(photoDrafts.select(index, currentPhotoDraft()));
  const handleFormSubmit = async () => {
    if (savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    try {
      if (!editingComponent && photoDrafts.drafts.length) {
        const drafts = photoDrafts.merge(currentPhotoDraft());
        const invalid = drafts.findIndex(draft => !draft.form.name?.trim() || !draft.category.trim() || !Number.isInteger(draft.form.stock_quantity) || Number(draft.form.stock_quantity) < 0);
        if (invalid >= 0) {
          switchPhotoDraft(invalid);
          toast.error(`请补全第 ${invalid + 1} 条的名称、分类和非负整数数量`);
          return;
        }
        const res = await client.post('/components/batch', { items: drafts.map(draft => ({
          ...draft.form, id: undefined, category: undefined, supplier: undefined,
          category_id: undefined, supplier_id: undefined,
          component_number: draft.form.component_number?.trim() || undefined,
          category_name: draft.category.trim(), supplier_name: draft.supplier.trim(),
          total_price_cents: yuanToCents(draft.price),
        })) });
        const created = res.data.data as Component[];
        for (const [index, draft] of drafts.entries()) {
          if (!draft.file) continue;
          const image = new FormData(); image.append('image', draft.file);
          try { await client.post(`/components/${created[index].id}/image`, image); }
          catch { toast.error(`第 ${index + 1} 条 ${draft.form.name} 图片上传失败，元件已保存`); }
        }
        toast.success(`已批量添加 ${drafts.length} 个元件`);
        photoDrafts.reset(); parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false);
        await Promise.all([fetchComponents(pagination.page, pagination.page_size), fetchComponentOptions(), fetchCategories(), fetchSuppliers()]);
        return;
      }
      if (!formData.name?.trim() || !Number.isInteger(formData.stock_quantity) || Number(formData.stock_quantity) < 0) {
        toast.error('请填写名称和非负整数数量'); return;
      }
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

      const data = componentSaveInput(formData, Number(categoryId), supplierId, yuanToCents(formTotalPriceYuan), !editingComponent, editingComponent?.created_at);
      const response = await inventoryApi.saveComponent(editingComponent?.id, data);
      const savedId = response.data.data.id;
      toast.success(editingComponent ? '更新成功' : '添加成功');

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

      parseTask.reset(); setSupplierReference(undefined); setIsFormOpen(false);
      fetchComponents(pagination.page, pagination.page_size);
      fetchComponentOptions();
    } catch (error) {
      console.error(error);
      const err = error as { response?: { data?: { error?: string } } };
      const message = err.response?.data?.error || '保存失败';
      const failedItem = /第 (\d+) 条/.exec(message);
      if (failedItem && photoDrafts.drafts[Number(failedItem[1]) - 1]) switchPhotoDraft(Number(failedItem[1]) - 1);
      toast.error(message);
    } finally {
      savingRef.current = false; setSaving(false);
    }
  };

  const parsePlatform = async (code?: string) => {
    const parseCode = (code ?? platformCode).trim();
    if (!parseCode) { toast.error('请输入供应商料号'); return; }
    setSupplierReference(undefined);
    await parseTask.start({ type: 'component_parse', input: { code: parseCode, parse_mode: parseMode }, source: 'components', record_id: editingComponent?.id }, parseSnapshot());
  };

  const applyPhotoResults = (items: ReviewedImageItem[]) => {
    parseTask.reset(); setSupplierReference(undefined);
    const first = items[0];
    if (editingComponent) {
      applyParsedComponent(photoComponentInfo(first), first.quantity);
      setCategoryInput(first.category_name); setSupplierInput(first.supplier_name);
      return;
    }
    const drafts: PhotoDraft[] = items.map((item, index) => ({
      form: { ...mergeParsed(index === 0 ? formData : {}, parsedInfoToFormData(photoComponentInfo(item))), stock_quantity: item.quantity },
      category: item.category_name, supplier: item.supplier_name,
      price: index === 0 ? formTotalPriceYuan : '', preview: index === 0 ? previewUrl : '',
      file: index === 0 ? selectedFile : null, platformCode: item.supplier_part_number,
    }));
    photoDrafts.start(drafts); loadPhotoDraft(drafts[0]);
    toast.success(`已载入 ${items.length} 条照片识别结果，请逐条确认后保存`);
  };

  useTaskApplication<Component>('components', (task, record) => {
    openForm(record);
    if (task.type === 'image_parse') {
      setRecoveredPhotoTask(task); setIsPhotoParseOpen(true);
    } else if (task.result?.component) applyParsedComponent(task.result.component, task.result.quantity);
  });

  useEffect(() => {
    if (urlParams.get('new') !== '1') return;
    openForm(); updateQuery({ new: '' }, true);
  }, [urlParams, updateQuery]); // eslint-disable-line react-hooks/exhaustive-deps

  // Stock Handlers
  const openStock = (component: Component) => {
    setEditingComponent(component);
    setStockForm({ type: 'in', amount: 1, reason: '', totalPriceYuan: '' });
    setIsStockOpen(true);
  };

  const openBackfill = (component: Component) => {
    setBackfillTarget(component);
    setBackfillForm({
      totalPriceYuan: '',
      quantity: component.stock_quantity > 0 ? component.stock_quantity : 1,
    });
    setIsBackfillOpen(true);
  };

  const handleBackfillSubmit = () => backfillMutation.run(async () => {
    if (!backfillTarget) return;
    const totalPriceCents = yuanToCents(backfillForm.totalPriceYuan);
    if (totalPriceCents <= 0) {
      toast.error('请输入采购总价');
      return;
    }
    if (!Number.isSafeInteger(backfillForm.quantity) || backfillForm.quantity <= 0) {
      toast.error('采购数量必须大于 0');
      return;
    }
    try {
      await inventoryApi.backfillPrice(backfillTarget.id, {
        total_price_cents: totalPriceCents,
        quantity: backfillForm.quantity,
      });
      toast.success('补录价格成功');
      setIsBackfillOpen(false);
      fetchComponents(pagination.page, pagination.page_size);
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '补录失败');
    }
  });

  const handleStockSubmit = () => stockMutation.run(async () => {
    if (!Number.isSafeInteger(stockForm.amount) || stockForm.amount <= 0) { toast.error('请输入正整数数量'); return; }
    if (stockForm.type === 'out' && stockForm.amount > (editingComponent?.stock_quantity || 0)) { toast.error('出库数量不能超过当前库存'); return; }
    if (!editingComponent) return;
    try {
      const amount = stockForm.type === 'in' ? stockForm.amount : -stockForm.amount;
      const payload: { amount: number; reason: string; total_price_cents?: number } = {
        amount,
        reason: stockForm.reason
      };
      if (stockForm.type === 'in') {
        const totalPriceCents = yuanToCents(stockForm.totalPriceYuan);
        if (totalPriceCents > 0) {
          payload.total_price_cents = totalPriceCents;
        }
      }
      await inventoryApi.changeStock(editingComponent.id, payload);
      toast.success('库存更新成功');
      setIsStockOpen(false);
      fetchComponents(pagination.page, pagination.page_size);
    } catch {
      toast.error('更新失败');
    }
  });

  const openLogs = (component: Component) => { setEditingComponent(component); setIsLogsOpen(true); };

  // QR Handlers
  const handleScan = async (data: string) => {
    if (isImportParsing) return;
    setIsScannerOpen(false);
    let snapshot = parseSnapshot();
    if (!isFormOpen) {
      openForm();
      snapshot = { platformCode: '', form: { stock_quantity: 0 }, category: '', supplier: '', price: '', preview: '', file: null };
    }
    setSupplierReference(undefined);
    await parseTask.start({ type: 'qrcode_parse', input: { qrcode_data: data, parse_mode: parseMode }, source: 'components', record_id: isFormOpen ? editingComponent?.id : undefined }, snapshot);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="元件管理"
        actions={
          <>
            {isMobile && (
              <Button onClick={() => setIsScannerOpen(true)} variant="outline" disabled={isImportParsing}>
                <QrCode className="mr-2 h-4 w-4" /> 扫码录入
              </Button>
            )}
            <Button variant="outline" onClick={openColumnSettings}>
              <Columns3 className="mr-2 h-4 w-4" /> 列设置
            </Button>
            <Button variant="outline" onClick={openExportModal}>
              <Download className="mr-2 h-4 w-4" /> 导出 CSV
            </Button>
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
            <Button variant="outline" onClick={() => openBatchStockOut()}>
              <PackageMinus className="mr-2 h-4 w-4" /> 批量出库
            </Button>
            <Button onClick={() => openForm()}>
              <Plus className="mr-2 h-4 w-4" /> 添加元件
            </Button>
          </>
        }
      />

      <section className="surface space-y-4 p-4" aria-label="搜索与筛选">
        <form className="flex flex-wrap gap-2" onSubmit={event => { event.preventDefault(); handleSearch(); }}>
          <div className="relative min-w-48 flex-1"><Search className="absolute left-3 top-3 size-4 text-muted-foreground" /><Input aria-label="搜索元件" placeholder="搜索名称、编号、型号、料号…" value={searchFilters.keyword} onChange={e => handleSearchFilterChange('keyword', e.target.value)} className="pl-9" /></div>
          <div className="w-40"><SearchSelect label="分类筛选" placeholder="全部分类" value={selectedCategory} options={categories.map(c => ({ value: String(c.id), label: c.name }))} onChange={value => { setSelectedCategory(value); setCategorySearchInput(''); void fetchComponents(1, pagination.page_size, searchFilters, '', value); }} /></div>
          <Button type="submit"><Search className="mr-2 size-4" />搜索</Button><Button variant="outline" aria-expanded={advanced} onClick={() => setAdvanced(value => !value)}><SlidersHorizontal className="mr-2 size-4" />高级筛选{activeFilterCount > 0 && ` (${activeFilterCount})`}</Button>
        </form>
        {advanced && <div className="space-y-4 border-t pt-4"><div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {SEARCH_FILTER_FIELDS.map(field => renderSearchField(field.key))}
          <div className="space-y-1"><Label className="text-xs text-muted-foreground">制造商</Label><SearchSelect creatable label="制造商筛选" value={searchFilters.manufacturer} options={manufacturerOptions.map(value => ({ value, label: value }))} onChange={value => handleSearchFilterChange('manufacturer', value)} /></div>
          <div className="space-y-1"><Label className="text-xs text-muted-foreground">供应商</Label><SearchSelect creatable label="供应商筛选" value={searchFilters.supplier} options={suppliers.map(s => ({ value: s.name, label: s.name }))} onChange={value => handleSearchFilterChange('supplier', value)} /></div>
        </div><Button size="sm" onClick={handleSearch}>应用条件</Button></div>}
        <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-3"><div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"><span>已应用条件</span>
          {SEARCH_PARAM_KEYS.filter(key => query.get(key)).map(key => <button key={key} className="flex items-center gap-1 rounded-md bg-muted px-2 py-1.5 text-foreground" onClick={() => updateQuery({ [key]: '', page: 1 })}>{key === 'keyword' ? '关键词' : SEARCH_FILTER_FIELDS.find(field => field.key === key)?.label || (key === 'manufacturer' ? '制造商' : '供应商')}：{query.get(key)}<X className="size-3" /></button>)}
          {query.get('category_id') && <button className="flex items-center gap-1 rounded-md bg-muted px-2 py-1.5 text-foreground" onClick={() => updateQuery({ category_id: '', page: 1 })}>分类：{categories.find(c => String(c.id) === query.get('category_id'))?.name || query.get('category_id')}<X className="size-3" /></button>}
          {activeFilterCount > 0 ? <button className="text-primary" onClick={handleClearFilters}>清空筛选</button> : <span>全部元件</span>}
        </div><div className="flex items-center gap-2 text-xs text-muted-foreground"><label htmlFor="sort-by">排序</label><select id="sort-by" className="h-8 max-w-32 py-0 text-xs" value={sortBy} onChange={e => handleSortByChange(e.target.value as ExportColumnKey)}>{EXPORT_COLUMNS.map(column => <option key={column.key} value={column.key}>{column.defaultHeader}</option>)}</select><select aria-label="排序方向" className="h-8 py-0 text-xs" value={sortOrder} onChange={e => handleSortOrderChange(e.target.value as ComponentSortOrder)}><option value="desc">降序</option><option value="asc">升序</option></select></div></div>
      </section>
      {(loading || resource.error || !components.length) && <div className="surface"><PageState loading={loading} error={resource.error} empty={!components.length} filtered={hasActiveFilters} onRetry={resource.refresh} onClear={handleClearFilters} /></div>}

      {selectedIds.length > 0 && (
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 rounded-md border bg-muted/30 px-4 py-3">
          <span className="text-sm text-muted-foreground">已选择 {selectedIds.length} 项</span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setSelectedIds([])}>取消选择</Button>
            <Button variant="outline" onClick={() => openBatchStockOut(components.filter(c => selectedIds.includes(c.id)))}>
              批量出库
            </Button>
            <Button onClick={() => setIsBatchLocationOpen(true)}>批量修改位置</Button>
          </div>
        </div>
      )}

      <div className={cn('surface hidden md:block', (loading || !!resource.error || !components.length) && 'md:hidden')}>
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
                {visibleTableColumns.map(column => (
                  <th key={column.key} className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">
                    {getColumnHeader(column)}
                  </th>
                ))}
                <th className="sticky right-0 z-[6] h-12 bg-background px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody className="[&_tr:last-child]:border-0">
              {loading && components.length === 0 ? (
                <tr><td colSpan={2 + visibleTableColumns.length + 1} className="p-4 text-center">加载中...</td></tr>
              ) : components.map(component => (
                <tr
                  key={component.id}
                  className={cn(
                    'group border-b transition-colors hover:bg-muted/50',

                  )}
                >
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
                    <button aria-label={`预览 ${component.name} 图片`} onClick={() => setPreviewImage(`/api/v1/components/${component.id}/image`)}><SafeImage src={`/api/v1/components/${component.id}/image`} alt={component.name} className="size-10" /></button>
                  </td>
                  {visibleTableColumns.map(column => <Fragment key={column.key}>{renderTableCell(column.key, component)}</Fragment>)}
                  <td className="sticky right-0 z-[5] bg-background/95 p-2 align-middle shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.08)] backdrop-blur-sm">
                    <RowActionsMenu
                      rowId={component.id}
                      activeRowId={activeRowId}
                      onActiveRowChange={setActiveRowId}
                      actions={[
                        { key: 'edit', icon: Edit, label: '编辑', onClick: () => openForm(component) },
                        { key: 'stock', icon: Database, label: '库存', onClick: () => openStock(component), iconClassName: 'text-blue-500' },
                        { key: 'backfill', icon: Coins, label: '补录价格', onClick: () => openBackfill(component), iconClassName: 'text-amber-600' },
                        { key: 'logs', icon: History, label: '记录', onClick: () => openLogs(component), iconClassName: 'text-gray-500' },
                        { key: 'copy', icon: Copy, label: '复制', onClick: () => openCopyForm(component), iconClassName: 'text-emerald-600' },
                        { key: 'delete', icon: Trash2, label: '删除', onClick: () => handleDelete(component.id), destructive: true },
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      
      {!loading && !resource.error && <div className="space-y-3 md:hidden">{components.map(component => <article key={component.id} className="surface p-4"><div className="flex items-start gap-3"><input type="checkbox" aria-label={`选择 ${component.name}`} className="mt-2 size-4" checked={selectedIds.includes(component.id)} onChange={e => toggleSelectOne(component.id, e.target.checked)} /><SafeImage src={`/api/v1/components/${component.id}/image`} alt={component.name} className="size-12 shrink-0" /><div className="min-w-0 flex-1"><button className="text-left font-semibold text-primary" onClick={() => setDetailID(component.id)}>{component.name}</button><p className="mt-1 break-all text-xs text-muted-foreground">{component.component_number} · {component.model || component.value || '未填写型号'}</p></div><RowActionsMenu rowId={component.id} activeRowId={mobileActiveRowId} onActiveRowChange={setMobileActiveRowId}
                      actions={[
                        { key: 'edit', icon: Edit, label: '编辑', onClick: () => openForm(component) },
                        { key: 'stock', icon: Database, label: '库存', onClick: () => openStock(component), iconClassName: 'text-blue-500' },
                        { key: 'backfill', icon: Coins, label: '补录价格', onClick: () => openBackfill(component), iconClassName: 'text-amber-600' },
                        { key: 'logs', icon: History, label: '记录', onClick: () => openLogs(component), iconClassName: 'text-gray-500' },
                        { key: 'copy', icon: Copy, label: '复制', onClick: () => openCopyForm(component), iconClassName: 'text-emerald-600' },
                        { key: 'delete', icon: Trash2, label: '删除', onClick: () => handleDelete(component.id), destructive: true },
                      ]} /></div><div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t pt-3 text-xs text-muted-foreground"><span>{component.category?.name || '未分类'} · {component.location || '未设置位置'}</span><span className="status-badge">库存 {component.stock_quantity}</span></div></article>)}</div>}
      <PaginationControl pagination={pagination} onPageChange={handlePageChange} onPageSizeChange={handlePageSizeChange} disabled={loading} />
      <ComponentDetail id={detailID} onClose={() => setDetailID(null)} onEdit={component => { setDetailID(null); openForm(component); }} onStock={component => { setDetailID(null); openStock(component); }} onBackfill={component => { setDetailID(null); openBackfill(component); }} onLogs={component => { setDetailID(null); void openLogs(component); }} />

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
            <SearchSelect label="批量存放位置" value={batchLocation} onChange={setBatchLocation} creatable options={locationOptions.map(value => ({ value, label: value }))} placeholder="输入或选择位置" />
          </div>
        </div>
      </Modal>

      <BatchStockOutModal
        isOpen={isBatchStockOutOpen}
        onClose={() => setIsBatchStockOutOpen(false)}
        initialComponents={batchStockOutSeed}
        onSuccess={handleBatchStockOutSuccess}
      />

      <PhotoParseModal isOpen={isPhotoParseOpen} onClose={() => { setIsPhotoParseOpen(false); setRecoveredPhotoTask(undefined); }} source="components" recordID={editingComponent?.id} initialTask={recoveredPhotoTask} onConfirm={applyPhotoResults} />

      <Modal isOpen={isFormOpen && !isPhotoParseOpen} onClose={closeForm} dirty={formGuard.dirty} busy={saving} variant="drawer"
        title={editingComponent ? '编辑元件' : isCopyingComponent ? '复制元件' : '添加元件'}
        footer={<><span className="mr-auto text-xs text-muted-foreground">{saving ? '正在保存，请稍候…' : '保存后写入本地库存'}</span><Button variant="outline" onClick={() => void cancelForm()} disabled={saving}>取消</Button><Button type="submit" form="inventory-form" disabled={saving}>{saving ? '保存中…' : '保存'}</Button></>}>
        <div className="space-y-4">
          <PhotoDraftNavigation index={photoDrafts.index} count={photoDrafts.drafts.length} disabled={saving} onSelect={switchPhotoDraft} />
          <SmartEntry code={editingComponent ? formData.supplier_part_number || '' : platformCode} onCodeChange={value => editingComponent ? setFormData(prev => ({ ...prev, supplier_part_number: value })) : setPlatformCode(value)} onParse={() => void parsePlatform(editingComponent ? formData.supplier_part_number : undefined)} onScan={() => setIsScannerOpen(true)} onPhoto={() => setIsPhotoParseOpen(true)} busy={isImportParsing} mode={parseMode} onModeChange={setParseMode} task={parseTask.task} reference={matchingSupplierReference(supplierReference, formData.supplier_part_number || '', editingComponent ? formData.supplier_part_number || '' : platformCode)} />
          <InventoryFields value={{ ...formData, quantity: formData.stock_quantity, category: categoryInput, supplier: supplierInput, totalPrice: formTotalPriceYuan }} onChange={patch => {
            const { quantity, category, supplier, totalPrice, ...fields } = patch;
            if ('quantity' in patch) setFormData(prev => ({ ...prev, stock_quantity: quantity ?? undefined }));
            if (category !== undefined) setCategoryInput(category); if (supplier !== undefined) setSupplierInput(supplier); if (totalPrice !== undefined) setFormTotalPriceYuan(totalPrice);
            setFormData(prev => ({ ...prev, ...fields }));
            if ('image_url' in fields) { setPreviewUrl(''); setSelectedFile(null); }
          }} onSubmit={() => void handleFormSubmit()} categories={categories.map(c => c.name)} suppliers={suppliers.map(s => s.name)} manufacturers={manufacturerOptions} packages={packageOptions} locations={locationOptions} editing={!!editingComponent} busy={saving} imagePreview={previewUrl}
            imageControls={<div className="flex flex-wrap gap-2"><input type="file" ref={fileInputRef} className="hidden" accept="image/*" onChange={handleFileSelect} /><Button variant="outline" onClick={() => fileInputRef.current?.click()}><Upload className="mr-2 size-4" />上传图片</Button><Button variant="outline" onClick={() => setIsCameraOpen(true)}><Camera className="mr-2 size-4" />拍摄图片</Button></div>} />
        </div>
      </Modal>

      {/* Backfill Price Modal */}
      <Modal
        isOpen={isBackfillOpen}
        onClose={() => setIsBackfillOpen(false)} busy={backfillMutation.busy}
        title={backfillTarget ? `补录价格 · ${backfillTarget.name}` : '补录价格'}
        footer={
          <>
            <Button variant="outline" onClick={() => setIsBackfillOpen(false)}>取消</Button>
            <Button onClick={handleBackfillSubmit} disabled={backfillMutation.busy}>确认</Button>
          </>
        }
      >
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label>采购总价（元）</Label>
            <Input
              type="number"
              min="0"
              step="0.01"
              value={backfillForm.totalPriceYuan}
              onChange={e => setBackfillForm({ ...backfillForm, totalPriceYuan: e.target.value })}
              placeholder="本次采购总价"
            />
          </div>
          <div className="space-y-2">
            <Label>采购数量</Label>
            <div className="flex items-center space-x-2">
              <Button
                variant="outline"
                size="icon"
                className="h-10 w-10 shrink-0"
                onClick={() => setBackfillForm(prev => ({ ...prev, quantity: Math.max(1, prev.quantity - 1) }))}
              >
                <Minus className="h-4 w-4" />
              </Button>
              <Input
                type="number"
                min="1"
                value={backfillForm.quantity}
                onChange={e => setBackfillForm({ ...backfillForm, quantity: Number(e.target.value) })}
                className="text-center"
              />
              <Button
                variant="outline"
                size="icon"
                className="h-10 w-10 shrink-0"
                onClick={() => setBackfillForm(prev => ({ ...prev, quantity: prev.quantity + 1 }))}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
            <QuantityShortcuts
              value={backfillForm.quantity}
              onSelect={quantity => setBackfillForm(prev => ({ ...prev, quantity }))}
            />
          </div>
          {backfillForm.quantity > 0 && backfillForm.totalPriceYuan && yuanToCents(backfillForm.totalPriceYuan) > 0 && (
            <p className="text-xs text-muted-foreground">
              分摊单价：{formatMicro(calcUnitPriceMicro(yuanToCents(backfillForm.totalPriceYuan), backfillForm.quantity))}
            </p>
          )}
          {(backfillTarget?.unit_price_micro ?? 0) > 0 && (
            <p className="text-xs text-muted-foreground">
              当前参考单价：{formatMicro(backfillTarget!.unit_price_micro)}，补录后将按加权平均更新
            </p>
          )}
        </div>
      </Modal>

      {/* Stock Modal */}
      <Modal
        isOpen={isStockOpen}
        onClose={() => setIsStockOpen(false)} busy={stockMutation.busy}
        title="库存变更"
        footer={<><Button variant="outline" onClick={() => setIsStockOpen(false)}>取消</Button><Button onClick={handleStockSubmit} disabled={stockMutation.busy}>{stockMutation.busy ? '提交中…' : '确认'}</Button></>}
      >
        <div className="space-y-4 py-4">
          <div className="rounded-lg bg-muted p-4"><p className="font-medium">{editingComponent?.name}</p><div className="mt-3 flex items-center justify-between text-sm"><span>当前库存 <strong>{editingComponent?.stock_quantity ?? 0}</strong></span><span>→</span><span>变更后 <strong className={(editingComponent?.stock_quantity || 0) + (stockForm.type === 'in' ? stockForm.amount : -stockForm.amount) < 0 ? 'text-destructive' : 'text-primary'}>{(editingComponent?.stock_quantity || 0) + (stockForm.type === 'in' ? stockForm.amount : -stockForm.amount)}</strong></span></div></div>
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
                <QuantityShortcuts
                    value={stockForm.amount}
                    onSelect={quantity => setStockForm(prev => ({ ...prev, amount: quantity }))}
                />
            </div>
            {stockForm.type === 'in' && (
              <div className="space-y-2">
                <Label>采购总价（元）</Label>
                <Input
                  type="number"
                  min="0"
                  step="0.01"
                  value={stockForm.totalPriceYuan}
                  onChange={e => setStockForm({ ...stockForm, totalPriceYuan: e.target.value })}
                  placeholder="可选，按入库数量分摊单价"
                />
                {stockForm.amount > 0 && stockForm.totalPriceYuan && yuanToCents(stockForm.totalPriceYuan) > 0 && (
                  <p className="text-xs text-muted-foreground">
                    分摊单价：{formatMicro(calcUnitPriceMicro(yuanToCents(stockForm.totalPriceYuan), stockForm.amount))}
                  </p>
                )}
              </div>
            )}
            {stockForm.type === 'out' && (editingComponent?.unit_price_micro ?? 0) > 0 && stockForm.amount > 0 && (
              <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                <div>参考单价：{formatMicro(editingComponent?.unit_price_micro)}</div>
                <div>预估成本：{formatCents(calcOutboundCostCents(editingComponent!.unit_price_micro!, stockForm.amount))}</div>
              </div>
            )}
             <div className="space-y-2">
                <Label>备注</Label>
                <Input value={stockForm.reason} onChange={e => setStockForm({...stockForm, reason: e.target.value})} />
            </div>
        </div>
      </Modal>

      {isLogsOpen && editingComponent && <ComponentLogsDialog key={editingComponent.id} id={editingComponent.id} onClose={() => setIsLogsOpen(false)} onUpdated={() => void fetchComponents(pagination.page, pagination.page_size)} />}

      {/* Column Settings Modal */}
      <Modal
        isOpen={isColumnSettingsOpen}
        onClose={() => setIsColumnSettingsOpen(false)}
        title="列设置"
        className="max-w-2xl"
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setIsColumnSettingsOpen(false)}>
              取消
            </Button>
            <Button variant="outline" onClick={resetTableColumns}>
              恢复默认
            </Button>
            <Button onClick={confirmTableColumns}>
              确认
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            自定义列表显示列、表头名称与列顺序，设置会保存在本浏览器。可拖动手柄调整列顺序；勾选框、图片和操作列固定显示。
          </p>
          <ColumnSettingsPanel
            columns={tableColumnsDraft}
            isAllSelected={isAllTableSelected}
            onToggle={toggleTableColumn}
            onUpdateHeader={updateTableHeader}
            onToggleAll={toggleTableSelectAll}
            onReorder={reorderTableColumns}
          />
        </div>
      </Modal>

      {/* Export Modal */}
      <Modal
        isOpen={isExportOpen}
        onClose={() => !isExporting && setIsExportOpen(false)}
        title="导出 CSV"
        className="max-w-2xl"
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setIsExportOpen(false)} disabled={isExporting}>
              取消
            </Button>
            <Button variant="outline" onClick={resetExportColumns} disabled={isExporting}>
              恢复默认
            </Button>
            <Button onClick={handleExportCSV} disabled={isExporting}>
              {isExporting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  导出中...
                </>
              ) : (
                <>
                  <Download className="mr-2 h-4 w-4" />
                  确认导出
                </>
              )}
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            将按当前筛选条件导出全部匹配元件，可自定义导出列、表头名称与列顺序。可拖动手柄调整列顺序。
          </p>
          <p className="text-sm text-muted-foreground">
            导出顺序与当前列表排序一致（{currentSortLabel}，{currentSortOrderLabel}）。
          </p>
          <ColumnSettingsPanel
            columns={exportColumnsDraft}
            isAllSelected={isAllExportSelected}
            onToggle={toggleExportColumn}
            onUpdateHeader={updateExportHeader}
            onToggleAll={toggleExportSelectAll}
            onReorder={reorderExportColumns}
          />
        </div>
      </Modal>

      {/* Scanner Modal */}
      <Modal isOpen={isScannerOpen} onClose={() => setIsScannerOpen(false)} title="扫描二维码">
        <Suspense
          fallback={
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-10 w-10 animate-spin text-primary" />
            </div>
          }
        >
          <QRScanner onScan={handleScan} onClose={() => setIsScannerOpen(false)} autoStart={isMobile} />
        </Suspense>
      </Modal>

      {/* Camera Modal */}
      <Modal isOpen={isCameraOpen} onClose={() => setIsCameraOpen(false)} title="拍摄图片" className="max-w-md">
        <div className="h-[60vh] md:h-[500px]">
          {isCameraOpen && (
            <Suspense
              fallback={
                <div className="flex h-full items-center justify-center">
                  <Loader2 className="h-10 w-10 animate-spin text-primary" />
                </div>
              }
            >
              <CameraCapture onCapture={handleCapture} onClose={() => setIsCameraOpen(false)} />
            </Suspense>
          )}
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
                <SafeImage src={previewImage} alt="图片预览" className="max-h-[70vh] w-full object-contain" />
            )}
        </div>
      </Modal>


    </div>
  );
}

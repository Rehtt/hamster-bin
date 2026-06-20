import { lazy, Suspense, useEffect, useState, useRef } from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Plus, Minus, Search, Edit, Trash2, Database, History, QrCode, Camera, Upload, Link, X, Loader2, Hash, Download, Coins, Columns3, GripVertical } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Component, type Category, type Supplier, type StockLog, type Pagination, type ComponentOptions } from '../types';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { Label } from '../components/ui/Label';
const QRScanner = lazy(() => import('../components/QRScanner'));
const CameraCapture = lazy(() => import('../components/CameraCapture'));
import { yuanToCents, formatCents, calcUnitPriceCents, calcTotalPriceCents } from '../utils/price';
import {
  canRevoke,
  isReversal,
  isRevoked,
  stockLogAmountClass,
  stockLogCardClass,
  stockLogIconClass,
  formatStockLogChangeAmount,
  stockLogIconLabel,
  isBackfillLog,
} from '../utils/stockLog';

type ComponentSearchFilters = {
  component_number: string;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  supplier: string;
  supplier_part_number: string;
};

type ComponentSearchParams = {
  page: number;
  page_size: number;
  category_id?: string;
  component_number?: string;
  name?: string;
  model?: string;
  manufacturer?: string;
  value?: string;
  supplier?: string;
  supplier_part_number?: string;
  sort_by?: string;
  sort_order?: string;
};

const EMPTY_SEARCH_FILTERS: ComponentSearchFilters = {
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
  'component_number',
  'name',
  'model',
  'manufacturer',
  'value',
  'supplier',
  'supplier_part_number',
];

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

type ExportColumnKey =
  | 'component_number'
  | 'name'
  | 'model'
  | 'manufacturer'
  | 'value'
  | 'package'
  | 'description'
  | 'category'
  | 'stock_quantity'
  | 'unit_price'
  | 'location'
  | 'supplier'
  | 'supplier_part_number'
  | 'datasheet_url'
  | 'created_at'
  | 'updated_at';

type ComponentSortOrder = 'asc' | 'desc';

const DEFAULT_SORT_BY: ExportColumnKey = 'updated_at';
const DEFAULT_SORT_ORDER: ComponentSortOrder = 'desc';
const COMPONENT_SORT_STORAGE_KEY = 'hamster-components-sort';

type StoredComponentSort = {
  sortBy: ExportColumnKey;
  sortOrder: ComponentSortOrder;
};

type ExportColumnConfig = {
  key: ExportColumnKey;
  defaultHeader: string;
  defaultSelected: boolean;
  defaultTableSelected: boolean;
};

type ColumnState = {
  key: ExportColumnKey;
  selected: boolean;
  header: string;
};

const EXPORT_COLUMNS: ExportColumnConfig[] = [
  { key: 'component_number', defaultHeader: '系统编号', defaultSelected: true, defaultTableSelected: true },
  { key: 'name', defaultHeader: '名称', defaultSelected: true, defaultTableSelected: true },
  { key: 'model', defaultHeader: '厂家型号', defaultSelected: true, defaultTableSelected: true },
  { key: 'manufacturer', defaultHeader: '制造商', defaultSelected: true, defaultTableSelected: true },
  { key: 'value', defaultHeader: '参数', defaultSelected: true, defaultTableSelected: true },
  { key: 'package', defaultHeader: '封装', defaultSelected: true, defaultTableSelected: true },
  { key: 'description', defaultHeader: '描述', defaultSelected: false, defaultTableSelected: true },
  { key: 'category', defaultHeader: '分类', defaultSelected: true, defaultTableSelected: true },
  { key: 'stock_quantity', defaultHeader: '库存数量', defaultSelected: true, defaultTableSelected: true },
  { key: 'unit_price', defaultHeader: '参考单价', defaultSelected: true, defaultTableSelected: false },
  { key: 'location', defaultHeader: '存放位置', defaultSelected: true, defaultTableSelected: true },
  { key: 'supplier', defaultHeader: '供应商', defaultSelected: true, defaultTableSelected: true },
  { key: 'supplier_part_number', defaultHeader: '供应商料号', defaultSelected: true, defaultTableSelected: true },
  { key: 'datasheet_url', defaultHeader: '数据手册', defaultSelected: false, defaultTableSelected: true },
  { key: 'created_at', defaultHeader: '创建时间', defaultSelected: false, defaultTableSelected: false },
  { key: 'updated_at', defaultHeader: '更新时间', defaultSelected: false, defaultTableSelected: false },
];

const VALID_COLUMN_KEYS = new Set<ExportColumnKey>(EXPORT_COLUMNS.map(column => column.key));
const EXPORT_COLUMNS_STORAGE_KEY = 'hamster-components-export-columns';
const TABLE_COLUMNS_STORAGE_KEY = 'hamster-components-table-columns';

const getColumnConfig = (key: ExportColumnKey) =>
  EXPORT_COLUMNS.find(column => column.key === key);

const getColumnHeader = (column: ColumnState) => {
  const trimmed = column.header.trim();
  if (trimmed) return trimmed;
  return getColumnConfig(column.key)?.defaultHeader || column.key;
};

const createDefaultExportColumns = (): ColumnState[] =>
  EXPORT_COLUMNS.map(column => ({
    key: column.key,
    selected: column.defaultSelected,
    header: column.defaultHeader,
  }));

const createDefaultTableColumns = (): ColumnState[] =>
  EXPORT_COLUMNS.map(column => ({
    key: column.key,
    selected: column.defaultTableSelected,
    header: column.defaultHeader,
  }));

const normalizeStoredColumns = (
  stored: unknown,
  createDefault: () => ColumnState[],
  getDefaultSelected: (key: ExportColumnKey) => boolean,
): ColumnState[] => {
  const defaults = createDefault();
  if (!Array.isArray(stored)) return defaults;

  const result: ColumnState[] = [];
  const seen = new Set<ExportColumnKey>();

  for (const item of stored) {
    if (!item || typeof item !== 'object') continue;
    const key = (item as ColumnState).key;
    if (!VALID_COLUMN_KEYS.has(key) || seen.has(key)) continue;
    seen.add(key);
    const config = getColumnConfig(key)!;
    result.push({
      key,
      selected: typeof (item as ColumnState).selected === 'boolean'
        ? (item as ColumnState).selected
        : getDefaultSelected(key),
      header: typeof (item as ColumnState).header === 'string'
        ? (item as ColumnState).header
        : config.defaultHeader,
    });
  }

  for (const column of EXPORT_COLUMNS) {
    if (!seen.has(column.key)) {
      result.push({
        key: column.key,
        selected: getDefaultSelected(column.key),
        header: column.defaultHeader,
      });
    }
  }

  if (!result.some(column => column.selected)) return defaults;
  return result;
};

const readStoredExportColumns = (): ColumnState[] => {
  try {
    const raw = localStorage.getItem(EXPORT_COLUMNS_STORAGE_KEY);
    if (!raw) return createDefaultExportColumns();
    return normalizeStoredColumns(
      JSON.parse(raw),
      createDefaultExportColumns,
      key => getColumnConfig(key)!.defaultSelected,
    );
  } catch {
    return createDefaultExportColumns();
  }
};

const persistExportColumns = (columns: ColumnState[]) => {
  try {
    localStorage.setItem(EXPORT_COLUMNS_STORAGE_KEY, JSON.stringify(columns));
  } catch {
    // ignore quota or privacy mode errors
  }
};

const readStoredTableColumns = (): ColumnState[] => {
  try {
    const raw = localStorage.getItem(TABLE_COLUMNS_STORAGE_KEY);
    if (!raw) return createDefaultTableColumns();
    return normalizeStoredColumns(
      JSON.parse(raw),
      createDefaultTableColumns,
      key => getColumnConfig(key)!.defaultTableSelected,
    );
  } catch {
    return createDefaultTableColumns();
  }
};

const persistTableColumns = (columns: ColumnState[]) => {
  try {
    localStorage.setItem(TABLE_COLUMNS_STORAGE_KEY, JSON.stringify(columns));
  } catch {
    // ignore quota or privacy mode errors
  }
};

const VALID_SORT_BY_KEYS = new Set<ExportColumnKey>(EXPORT_COLUMNS.map(column => column.key));

const readStoredComponentSort = (): StoredComponentSort => {
  const fallback: StoredComponentSort = { sortBy: DEFAULT_SORT_BY, sortOrder: DEFAULT_SORT_ORDER };
  try {
    const raw = localStorage.getItem(COMPONENT_SORT_STORAGE_KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as { sortBy?: string; sortOrder?: string };
    const sortBy = VALID_SORT_BY_KEYS.has(parsed.sortBy as ExportColumnKey)
      ? (parsed.sortBy as ExportColumnKey)
      : DEFAULT_SORT_BY;
    const sortOrder = parsed.sortOrder === 'asc' || parsed.sortOrder === 'desc'
      ? parsed.sortOrder
      : DEFAULT_SORT_ORDER;
    return { sortBy, sortOrder };
  } catch {
    return fallback;
  }
};

const persistComponentSort = (sortBy: ExportColumnKey, sortOrder: ComponentSortOrder) => {
  try {
    localStorage.setItem(COMPONENT_SORT_STORAGE_KEY, JSON.stringify({ sortBy, sortOrder }));
  } catch {
    // ignore quota or privacy mode errors
  }
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
  price?: number;
  datasheet_url?: string;
  image_url?: string;
  category?: Category;
};

type SortableColumnRowProps = {
  column: ColumnState;
  onToggle: (key: ExportColumnKey, selected: boolean) => void;
  onUpdateHeader: (key: ExportColumnKey, header: string) => void;
};

function SortableColumnRow({ column, onToggle, onUpdateHeader }: SortableColumnRowProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: column.key });

  const defaultHeader = getColumnConfig(column.key)?.defaultHeader || column.key;
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="grid grid-cols-1 sm:grid-cols-[auto_auto_1fr] gap-2 sm:gap-3 items-center px-3 py-3 border-b last:border-b-0"
    >
      <button
        type="button"
        className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-grab active:cursor-grabbing touch-none"
        title="拖动调整顺序"
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-4 w-4" />
      </button>
      <label className="flex items-center gap-2 text-sm whitespace-nowrap">
        <input
          type="checkbox"
          checked={column.selected}
          onChange={e => onToggle(column.key, e.target.checked)}
          className="h-4 w-4 rounded border-input"
        />
        {defaultHeader}
      </label>
      <Input
        value={column.header}
        onChange={e => onUpdateHeader(column.key, e.target.value)}
        placeholder={defaultHeader}
        disabled={!column.selected}
      />
    </div>
  );
}

type ColumnSettingsPanelProps = {
  columns: ColumnState[];
  isAllSelected: boolean;
  onToggle: (key: ExportColumnKey, selected: boolean) => void;
  onUpdateHeader: (key: ExportColumnKey, header: string) => void;
  onToggleAll: (selected: boolean) => void;
  onReorder: (columns: ColumnState[]) => void;
};

function ColumnSettingsPanel({
  columns,
  isAllSelected,
  onToggle,
  onUpdateHeader,
  onToggleAll,
  onReorder,
}: ColumnSettingsPanelProps) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = columns.findIndex(column => column.key === active.id);
    const newIndex = columns.findIndex(column => column.key === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    onReorder(arrayMove(columns, oldIndex, newIndex));
  };

  return (
    <>
      <div className="flex items-center justify-between rounded-md border px-3 py-2">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={isAllSelected}
            onChange={e => onToggleAll(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          全选
        </label>
        <span className="text-xs text-muted-foreground">
          已选 {columns.filter(column => column.selected).length} / {columns.length} 列
        </span>
      </div>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={columns.map(column => column.key)} strategy={verticalListSortingStrategy}>
          <div className="max-h-[50vh] overflow-auto rounded-md border">
            {columns.map(column => (
              <SortableColumnRow
                key={column.key}
                column={column}
                onToggle={onToggle}
                onUpdateHeader={onUpdateHeader}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
    </>
  );
}

export default function Components() {
  const [components, setComponents] = useState<Component[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [loading, setLoading] = useState(false);
  const [searchFilters, setSearchFilters] = useState<ComponentSearchFilters>(EMPTY_SEARCH_FILTERS);
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [categorySearchInput, setCategorySearchInput] = useState('');
  const [manufacturerOptions, setManufacturerOptions] = useState<string[]>([]);
  const [showSearchManufacturerDropdown, setShowSearchManufacturerDropdown] = useState(false);
  const [showSearchSupplierDropdown, setShowSearchSupplierDropdown] = useState(false);
  const [showSearchCategoryDropdown, setShowSearchCategoryDropdown] = useState(false);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [isBatchLocationOpen, setIsBatchLocationOpen] = useState(false);
  const [batchLocation, setBatchLocation] = useState('');
  const [isBatchUpdating, setIsBatchUpdating] = useState(false);
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
  const [formTotalPriceYuan, setFormTotalPriceYuan] = useState('');


  // Stock State
  const [stockForm, setStockForm] = useState({ type: 'in', amount: 1, reason: '', totalPriceYuan: '' });
  const [backfillTarget, setBackfillTarget] = useState<Component | null>(null);
  const [backfillForm, setBackfillForm] = useState({ totalPriceYuan: '', quantity: 1 });
  
  // Logs State
  const [componentLogs, setComponentLogs] = useState<StockLog[]>([]);
  const [revokingLogId, setRevokingLogId] = useState<number | null>(null);

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
    if (component.price && component.price > 0) {
      const qty = quantity && quantity > 0 ? quantity : 1;
      setFormTotalPriceYuan((component.price * qty).toFixed(2));
    }
  };

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
    page = 1,
    pageSize = pagination.page_size,
    filters: ComponentSearchFilters = searchFilters,
    categoryInput: string = categorySearchInput,
    categoryId: string = selectedCategory,
    nextSortBy: ExportColumnKey = sortBy,
    nextSortOrder: ComponentSortOrder = sortOrder,
  ) => {
    const showLoading = components.length === 0;
    if (showLoading) setLoading(true);
    try {
      const params: ComponentSearchParams = {
        page,
        page_size: pageSize,
        sort_by: nextSortBy,
        sort_order: nextSortOrder,
      };
      const resolvedCategoryId = resolveCategoryId(categoryInput, categoryId);
      if (resolvedCategoryId) params.category_id = resolvedCategoryId;
      for (const key of SEARCH_PARAM_KEYS) {
        const value = filters[key].trim();
        if (value) params[key] = value;
      }

      const res = await client.get('/components', { params });
      setComponents(res.data.data || []);
      setPagination(res.data.pagination || { page: 1, page_size: 20, total: 0, total_page: 0 });
      setSelectedIds([]);
    } catch {
      toast.error('加载元件失败');
    } finally {
      if (showLoading) setLoading(false);
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
      setManufacturerOptions(options.manufacturers || []);
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

  const handleSearchFilterChange = (key: keyof ComponentSearchFilters, value: string) => {
    setSearchFilters(prev => ({ ...prev, [key]: value }));
  };

  const handleCategorySearchInputChange = (value: string) => {
    setCategorySearchInput(value);
    const matched = categories.find(c => c.name.toLowerCase() === value.trim().toLowerCase());
    setSelectedCategory(matched ? String(matched.id) : '');
  };

  const filteredManufacturerOptions = manufacturerOptions.filter(option =>
    option.toLowerCase().includes(searchFilters.manufacturer.toLowerCase())
  );

  const filteredSupplierOptions = suppliers.filter(s =>
    s.name.toLowerCase().includes(searchFilters.supplier.toLowerCase())
  );

  const filteredCategoryOptions = categories.filter(c =>
    c.name.toLowerCase().includes(categorySearchInput.toLowerCase())
  );

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
        return <td className="p-4 align-middle font-mono text-xs">{component.component_number || '-'}</td>;
      case 'name':
        return <td className="p-4 align-middle font-medium">{component.name}</td>;
      case 'model':
        return <td className="p-4 align-middle">{component.model || '-'}</td>;
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
            {component.unit_price_cents != null ? formatCents(component.unit_price_cents) : '-'}
          </td>
        );
      case 'location':
        return <td className="p-4 align-middle">{component.location}</td>;
      case 'supplier':
        return <td className="p-4 align-middle">{component.supplier?.name || '-'}</td>;
      case 'supplier_part_number':
        return <td className="p-4 align-middle">{component.supplier_part_number || '-'}</td>;
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

      const response = await fetch(`/api/v1/components/export?${params.toString()}`, {
        credentials: 'include',
      });
      if (!response.ok) {
        const data = await response.json().catch(() => null) as { error?: string } | null;
        throw new Error(data?.error || '导出失败');
      }

      const blob = await response.blob();
      const disposition = response.headers.get('Content-Disposition');
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

      const data: Record<string, unknown> = {
        ...formData,
        category_id: Number(categoryId),
        supplier_id: supplierId,
        supplier: undefined,
        category: undefined,
        component_number: formData.component_number?.trim() || undefined,
        stock_quantity: Number(formData.stock_quantity)
      };

      const totalPriceCents = yuanToCents(formTotalPriceYuan);
      if (!editingComponent && Number(formData.stock_quantity) > 0 && totalPriceCents > 0) {
        data.total_price_cents = totalPriceCents;
      }

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

  const handleBackfillSubmit = async () => {
    if (!backfillTarget) return;
    const totalPriceCents = yuanToCents(backfillForm.totalPriceYuan);
    if (totalPriceCents <= 0) {
      toast.error('请输入采购总价');
      return;
    }
    if (backfillForm.quantity <= 0) {
      toast.error('采购数量必须大于 0');
      return;
    }
    try {
      await client.post(`/components/${backfillTarget.id}/backfill-price`, {
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
  };

  const handleStockSubmit = async () => {
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
      await client.post(`/components/${editingComponent.id}/stock`, payload);
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

  const handleRevokeLog = async (log: StockLog) => {
    if (!confirm('确定撤销此记录？库存将回滚。')) return;
    setRevokingLogId(log.id);
    try {
      await client.post(`/stock-logs/${log.id}/revoke`);
      toast.success('撤销成功');
      if (editingComponent) {
        const [logsRes, compRes] = await Promise.all([
          client.get(`/components/${editingComponent.id}/logs`),
          client.get(`/components/${editingComponent.id}`),
        ]);
        setComponentLogs(logsRes.data.data || []);
        const updated = compRes.data.data as Component;
        setEditingComponent(updated);
        setComponents(prev => prev.map(c => (c.id === updated.id ? updated : c)));
      }
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || '撤销失败');
    } finally {
      setRevokingLogId(null);
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
            <Button onClick={() => openForm()}>
                <Plus className="mr-2 h-4 w-4" /> 添加元件
            </Button>
        </div>
      </div>

      <div className="rounded-md border p-4 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
          {renderSearchField('component_number')}
          {renderSearchField('name')}
          {renderSearchField('model')}
          <div className="space-y-1">
            <Label htmlFor="search-manufacturer" className="text-xs text-muted-foreground">制造商</Label>
            <div className="relative">
              <Input
                id="search-manufacturer"
                placeholder="YAGEO"
                value={searchFilters.manufacturer}
                onChange={e => handleSearchFilterChange('manufacturer', e.target.value)}
                onFocus={() => setShowSearchManufacturerDropdown(true)}
                onBlur={() => setTimeout(() => setShowSearchManufacturerDropdown(false), 200)}
                onKeyDown={handleSearchKeyDown}
              />
              {showSearchManufacturerDropdown && (
                <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                  {filteredManufacturerOptions.map(option => (
                    <div
                      key={option}
                      className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                      onClick={() => {
                        handleSearchFilterChange('manufacturer', option);
                        setShowSearchManufacturerDropdown(false);
                      }}
                    >
                      {option}
                    </div>
                  ))}
                  {filteredManufacturerOptions.length === 0 && (
                    <div className="px-3 py-2 text-sm text-muted-foreground">
                      {searchFilters.manufacturer ? '无匹配制造商' : '无历史制造商'}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
          {renderSearchField('value')}
          <div className="space-y-1">
            <Label htmlFor="search-supplier" className="text-xs text-muted-foreground">供应商</Label>
            <div className="relative">
              <Input
                id="search-supplier"
                placeholder="嘉立创"
                value={searchFilters.supplier}
                onChange={e => handleSearchFilterChange('supplier', e.target.value)}
                onFocus={() => setShowSearchSupplierDropdown(true)}
                onBlur={() => setTimeout(() => setShowSearchSupplierDropdown(false), 200)}
                onKeyDown={handleSearchKeyDown}
              />
              {showSearchSupplierDropdown && (
                <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                  {filteredSupplierOptions.map(s => (
                    <div
                      key={s.id}
                      className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                      onClick={() => {
                        handleSearchFilterChange('supplier', s.name);
                        setShowSearchSupplierDropdown(false);
                      }}
                    >
                      {s.name}
                    </div>
                  ))}
                  {filteredSupplierOptions.length === 0 && (
                    <div className="px-3 py-2 text-sm text-muted-foreground">
                      {searchFilters.supplier ? '无匹配供应商' : '无供应商'}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
          {renderSearchField('supplier_part_number')}
          <div className="space-y-1">
            <Label htmlFor="search-category" className="text-xs text-muted-foreground">分类</Label>
            <div className="relative">
              <Input
                id="search-category"
                placeholder="输入或选择分类"
                value={categorySearchInput}
                onChange={e => handleCategorySearchInputChange(e.target.value)}
                onFocus={() => setShowSearchCategoryDropdown(true)}
                onBlur={() => setTimeout(() => setShowSearchCategoryDropdown(false), 200)}
                onKeyDown={handleSearchKeyDown}
              />
              {showSearchCategoryDropdown && (
                <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
                  {filteredCategoryOptions.map(c => (
                    <div
                      key={c.id}
                      className="px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground"
                      onClick={() => {
                        setCategorySearchInput(c.name);
                        setSelectedCategory(String(c.id));
                        setShowSearchCategoryDropdown(false);
                        setTimeout(() => handleSearch(), 0);
                      }}
                    >
                      {c.name}
                    </div>
                  ))}
                  {filteredCategoryOptions.length === 0 && (
                    <div className="px-3 py-2 text-sm text-muted-foreground">
                      {categorySearchInput ? '无匹配分类' : '无分类'}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
        <div className="flex flex-col sm:flex-row gap-3 sm:items-end">
          <div className="flex flex-col sm:flex-row gap-3 sm:items-end">
            <div className="space-y-1">
              <Label htmlFor="sort-by" className="text-xs text-muted-foreground">排序字段</Label>
              <select
                id="sort-by"
                value={sortBy}
                onChange={e => handleSortByChange(e.target.value as ExportColumnKey)}
                className="h-9 w-full sm:w-40 rounded-md border border-input bg-background px-2 text-sm"
              >
                {EXPORT_COLUMNS.map(column => (
                  <option key={column.key} value={column.key}>
                    {column.defaultHeader}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <Label htmlFor="sort-order" className="text-xs text-muted-foreground">排序方向</Label>
              <select
                id="sort-order"
                value={sortOrder}
                onChange={e => handleSortOrderChange(e.target.value as ComponentSortOrder)}
                className="h-9 w-full sm:w-28 rounded-md border border-input bg-background px-2 text-sm"
              >
                <option value="asc">升序</option>
                <option value="desc">降序</option>
              </select>
            </div>
          </div>
          <div className="flex gap-2 sm:ml-auto">
            <Button onClick={handleSearch} variant="secondary">
              <Search className="h-4 w-4 mr-2" />搜索
            </Button>
            {hasActiveFilters && (
              <Button onClick={handleClearFilters} variant="outline">清空筛选</Button>
            )}
          </div>
        </div>
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
                {visibleTableColumns.map(column => (
                  <th key={column.key} className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">
                    {getColumnHeader(column)}
                  </th>
                ))}
                <th className="h-12 px-4 align-middle font-medium text-muted-foreground whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody className="[&_tr:last-child]:border-0">
              {loading && components.length === 0 ? (
                <tr><td colSpan={2 + visibleTableColumns.length + 1} className="p-4 text-center">加载中...</td></tr>
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
                            }}
                        />
                    </div>
                  </td>
                  {visibleTableColumns.map(column => renderTableCell(column.key, component))}
                  <td className="p-4 align-middle">
                    <div className="flex gap-2">
                        <Button variant="ghost" size="icon" onClick={() => openForm(component)} title="编辑"><Edit className="h-4 w-4" /></Button>
                        <Button variant="ghost" size="icon" onClick={() => openStock(component)} title="库存"><Database className="h-4 w-4 text-blue-500" /></Button>
                        <Button variant="ghost" size="icon" onClick={() => openBackfill(component)} title="补录价格"><Coins className="h-4 w-4 text-amber-600" /></Button>
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
                {!editingComponent && (
                  <div className="space-y-2">
                    <Label>采购总价（元）</Label>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={formTotalPriceYuan}
                      onChange={e => setFormTotalPriceYuan(e.target.value)}
                      placeholder="可选，按库存数量分摊单价"
                    />
                    {(formData.stock_quantity || 0) > 0 && formTotalPriceYuan && yuanToCents(formTotalPriceYuan) > 0 && (
                      <p className="text-xs text-muted-foreground">
                        分摊单价：{formatCents(calcUnitPriceCents(yuanToCents(formTotalPriceYuan), formData.stock_quantity || 0))}
                      </p>
                    )}
                  </div>
                )}
                {editingComponent && (editingComponent.unit_price_cents ?? 0) > 0 && (
                  <div className="space-y-2">
                    <Label>参考单价</Label>
                    <p className="text-sm text-muted-foreground">{formatCents(editingComponent.unit_price_cents)}</p>
                  </div>
                )}
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

      {/* Backfill Price Modal */}
      <Modal
        isOpen={isBackfillOpen}
        onClose={() => setIsBackfillOpen(false)}
        title={backfillTarget ? `补录价格 · ${backfillTarget.name}` : '补录价格'}
        footer={
          <>
            <Button variant="outline" onClick={() => setIsBackfillOpen(false)}>取消</Button>
            <Button onClick={handleBackfillSubmit}>确认</Button>
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
          </div>
          {backfillForm.quantity > 0 && backfillForm.totalPriceYuan && yuanToCents(backfillForm.totalPriceYuan) > 0 && (
            <p className="text-xs text-muted-foreground">
              分摊单价：{formatCents(calcUnitPriceCents(yuanToCents(backfillForm.totalPriceYuan), backfillForm.quantity))}
            </p>
          )}
          {(backfillTarget?.unit_price_cents ?? 0) > 0 && (
            <p className="text-xs text-muted-foreground">
              当前参考单价：{formatCents(backfillTarget!.unit_price_cents)}，补录后将按加权平均更新
            </p>
          )}
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
                    分摊单价：{formatCents(calcUnitPriceCents(yuanToCents(stockForm.totalPriceYuan), stockForm.amount))}
                  </p>
                )}
              </div>
            )}
            {stockForm.type === 'out' && (editingComponent?.unit_price_cents ?? 0) > 0 && stockForm.amount > 0 && (
              <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                <div>参考单价：{formatCents(editingComponent?.unit_price_cents)}</div>
                <div>预估成本：{formatCents(calcTotalPriceCents(editingComponent!.unit_price_cents!, stockForm.amount))}</div>
              </div>
            )}
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
                 <div key={log.id} className={`flex justify-between items-center p-3 rounded-lg bg-secondary/10 gap-3 ${stockLogCardClass(log)}`}>
                     <div className="flex items-center gap-3 min-w-0">
                         <div className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 ${stockLogIconClass(log)}`}>
                             {stockLogIconLabel(log)}
                         </div>
                         <div className="min-w-0">
                             <div className="flex items-center gap-2 flex-wrap">
                               <span className="font-medium">{isBackfillLog(log) ? '补录' : Math.abs(log.change_amount)}</span>
                               {isRevoked(log) && (
                                 <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">已撤销</span>
                               )}
                               {isReversal(log) && (
                                 <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">撤销冲销</span>
                               )}
                             </div>
                             <div className="text-xs text-muted-foreground">{new Date(log.created_at).toLocaleString()}</div>
                         </div>
                     </div>
                     <div className="text-sm text-right shrink-0">
                       <div className={`font-medium ${stockLogAmountClass(log)}`}>
                         {formatStockLogChangeAmount(log)}
                       </div>
                       {(log.total_price_cents ?? 0) > 0 && (
                         <div className="text-foreground">
                           {log.change_amount < 0 ? '成本' : '总价'} {formatCents(log.total_price_cents)}
                           {(log.unit_price_cents ?? 0) > 0 && (
                             <span className="text-muted-foreground"> · 单价 {formatCents(log.unit_price_cents)}</span>
                           )}
                         </div>
                       )}
                       <div className="text-muted-foreground">{log.reason || '无备注'}</div>
                       {canRevoke(log) && (
                         <Button
                           variant="outline"
                           size="sm"
                           className="mt-2"
                           disabled={revokingLogId === log.id}
                           onClick={() => void handleRevokeLog(log)}
                         >
                           撤销
                         </Button>
                       )}
                     </div>
                 </div>
             ))
            }
         </div>
      </Modal>

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

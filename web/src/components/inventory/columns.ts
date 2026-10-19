export type ExportColumnKey =
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

export type ComponentSortOrder = 'asc' | 'desc';

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

export type ColumnState = {
  key: ExportColumnKey;
  selected: boolean;
  header: string;
};

export const EXPORT_COLUMNS: ExportColumnConfig[] = [
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

export const getColumnConfig = (key: ExportColumnKey) =>
  EXPORT_COLUMNS.find(column => column.key === key);

export const getColumnHeader = (column: ColumnState) => {
  const trimmed = column.header.trim();
  if (trimmed) return trimmed;
  return getColumnConfig(column.key)?.defaultHeader || column.key;
};

export const createDefaultExportColumns = (): ColumnState[] =>
  EXPORT_COLUMNS.map(column => ({
    key: column.key,
    selected: column.defaultSelected,
    header: column.defaultHeader,
  }));

export const createDefaultTableColumns = (): ColumnState[] =>
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

export const readStoredExportColumns = (): ColumnState[] => {
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

export const persistExportColumns = (columns: ColumnState[]) => {
  try {
    localStorage.setItem(EXPORT_COLUMNS_STORAGE_KEY, JSON.stringify(columns));
  } catch {
    // ignore quota or privacy mode errors
  }
};

export const readStoredTableColumns = (): ColumnState[] => {
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

export const persistTableColumns = (columns: ColumnState[]) => {
  try {
    localStorage.setItem(TABLE_COLUMNS_STORAGE_KEY, JSON.stringify(columns));
  } catch {
    // ignore quota or privacy mode errors
  }
};

export const VALID_SORT_BY_KEYS = new Set<ExportColumnKey>(EXPORT_COLUMNS.map(column => column.key));

export const readStoredComponentSort = (): StoredComponentSort => {
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

export const persistComponentSort = (sortBy: ExportColumnKey, sortOrder: ComponentSortOrder) => {
  try {
    localStorage.setItem(COMPONENT_SORT_STORAGE_KEY, JSON.stringify({ sortBy, sortOrder }));
  } catch {
    // ignore quota or privacy mode errors
  }
};




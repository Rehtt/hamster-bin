export interface Category {
  id: number;
  name: string;
}

export interface Supplier {
  id: number;
  name: string;
}

export interface Component {
  id: number;
  category_id: number;
  supplier_id?: number | null;
  component_number?: string | null;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  package: string;
  supplier_part_number: string;
  description: string;
  stock_quantity: number;
  unit_price_micro?: number;
  location: string;
  datasheet_url: string;
  image_url: string;
  created_at?: string;
  updated_at?: string;
  category?: Category;
  supplier?: Supplier;
}

export interface StockLog {
  id: number;
  component_id: number;
  change_amount: number;
  unit_price_micro?: number;
  total_price_cents?: number;
  reason: string;
  revoked_at?: string | null;
  reversal_of_id?: number | null;
  created_at: string;
  component?: Component;
}

export type PreStockStatus = 'pending' | 'confirmed';

export interface PreStock {
  id: number;
  category_id: number;
  supplier_id?: number | null;
  component_number?: string | null;
  name: string;
  model: string;
  manufacturer: string;
  value: string;
  package: string;
  supplier_part_number: string;
  description: string;
  expected_quantity: number;
  total_price_cents?: number;
  location: string;
  datasheet_url: string;
  image_url: string;
  status: PreStockStatus;
  component_id?: number | null;
  confirmed_at?: string | null;
  created_at?: string;
  updated_at?: string;
  category?: Category;
  supplier?: Supplier;
  component?: Component;
}

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
  total_page?: number;
}

export interface ApiResponse<T> {
  data: T;
  pagination?: Pagination;
  error?: string;
}

export interface Platform {
  code: string;
  name: string;
}

export interface ComponentOptions {
  packages: string[];
  locations: string[];
  manufacturers: string[];
}

export type StatsRange = 'month' | 'quarter' | 'all';

export interface DashboardStats {
  range: StatsRange;
  range_start: string | null;
  range_end: string | null;
  component_count: number;
  category_count: number;
  total_stock: number;
  inventory_value_cents: number;
  inbound_quantity: number;
  outbound_quantity: number;
  inbound_cost_cents: number;
}

export interface BatchStockOutFailure {
  component_id: number;
  component_name?: string;
  stock_quantity?: number;
  requested: number;
  error: string;
}

export interface BatchStockOutResult {
  updated: Component[];
  total_quantity: number;
  total_cost_cents: number;
}

export type ParseMode = 'standard' | 'llm' | 'agent';
export interface ParseTrace {
  mode: ParseMode;
  final_source?: string;
  fallback: boolean;
  fallback_reason?: string;
  calls: { tool: string; arguments: Record<string, string>; endpoint?: string; started_at: string; duration_ms: number; status: string; summary: string; cached?: boolean }[];
}
export interface SupplierQuote {
  source_name?: string;
  source_url?: string;
  stock_quantity: number | null;
  currency: string | null;
  price_tiers: { min_quantity: number; unit_price: string }[];
  queried_at: string;
}

export interface ParsedComponentInfo {
  supplier_quote?: SupplierQuote;
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
}
export interface TaskResult {
  parse_trace?: ParseTrace;
  component?: ParsedComponentInfo;
  items?: ParsedImageItem[];
  quantity?: number;
  qrcode_info?: { code: string; quantity: number; raw_data: string; platform: string };
}
export interface ParsedImageItem {
  name: string;
  category_name: string;
  model: string;
  manufacturer: string;
  value: string;
  package: string;
  supplier_name: string;
  supplier_part_number: string;
  description: string;
  quantity: number | null;
}
// 核对后数量必填；原始照片任务允许未知数量 null。
export interface ReviewedImageItem extends ParsedImageItem {
  quantity: number;
}
export interface TaskRequest {
  type: 'component_parse' | 'qrcode_parse' | 'image_parse';
  input: { code?: string; qrcode_data?: string; use_llm?: boolean; parse_mode?: ParseMode };
  source: 'components' | 'pre-stocks';
  record_id?: number;
}
export interface ParseTask extends Omit<TaskRequest, 'source'> {
  id: number;
  source: TaskRequest['source'] | '';
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';
  result: TaskResult | null;
  error?: string;
  error_code?: string;
  retry_of_id?: number;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

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

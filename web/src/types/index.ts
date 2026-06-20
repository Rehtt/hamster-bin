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
  unit_price_cents?: number;
  location: string;
  datasheet_url: string;
  image_url: string;
  category?: Category;
  supplier?: Supplier;
}

export interface StockLog {
  id: number;
  component_id: number;
  change_amount: number;
  unit_price_cents?: number;
  total_price_cents?: number;
  reason: string;
  revoked_at?: string | null;
  reversal_of_id?: number | null;
  created_at: string;
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

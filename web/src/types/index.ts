export interface Category {
  id: number;
  name: string;
}

export interface Component {
  id: number;
  category_id: number;
  name: string;
  value: string;
  package: string;
  description: string;
  stock_quantity: number;
  location: string;
  datasheet_url: string;
  image_url: string;
  category?: Category;
}

export interface StockLog {
  id: number;
  component_id: number;
  change_amount: number;
  reason: string;
  created_at: string;
  component?: Component;
}

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
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

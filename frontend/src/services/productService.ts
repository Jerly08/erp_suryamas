import api from './api';
import { API_ENDPOINTS } from '@/config/api';

export interface Product {
  id?: number;
  code: string;
  name: string;
  description?: string;
  category_id?: number;
  warehouse_location_id?: number;
  brand?: string;
  model?: string;
  unit: string;
  purchase_price: number;
  cost_price: number; // ✅ ADDED: Harga Pokok (COGS calculation base)
  sale_price: number;
  pricing_tier?: string;
  stock: number;
  min_stock: number;
  max_stock: number;
  reorder_level: number;
  barcode?: string;
  sku?: string;
  weight?: number;
  dimensions?: string;
  is_active: boolean;
  is_service: boolean;
  is_assembly?: boolean;
  taxable: boolean;
  image_path?: string;
  notes?: string;
  category?: Category;
  warehouse_location?: WarehouseLocation;
  variants?: ProductVariant[];
}

export interface ProductVariant {
  id?: number;
  product_id: number;
  name: string;
  sku?: string;
  price: number;
  stock: number;
  is_active: boolean;
}

export interface Category {
  id?: number;
  code: string;
  name: string;
  description?: string;
  parent_id?: number;
  is_active: boolean;
  parent?: Category;
  children?: Category[];
}

export interface ProductUnit {
  id?: number;
  code: string;
  name: string;
  symbol?: string;
  type?: string;
  description?: string;
  is_active: boolean;
}

export interface WarehouseLocation {
  id?: number;
  code: string;
  name: string;
  description?: string;
  address?: string;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface InventoryMovement {
  id: number;
  product_id: number;
  reference_type: string;
  reference_id: number;
  type: 'IN' | 'OUT';
  quantity: number;
  unit_cost: number;
  total_cost: number;
  notes?: string;
  transaction_date: string;
  product: Product;
}

export interface StockAdjustment {
  product_id: number;
  quantity: number;
  type: 'IN' | 'OUT';
  notes?: string;
}

export interface StockOpname {
  product_id: number;
  new_stock: number;
  notes?: string;
}

export interface BulkPriceUpdate {
  updates: {
    product_id: number;
    purchase_price?: number;
    sale_price?: number;
  }[];
}

class ProductService {
  // Products
  async getProducts(params?: {
    search?: string;
    category?: string;
    warehouse_location?: string;
    status?: string;
    sort_by?: string;
    sort_order?: 'asc' | 'desc';
    page?: number;
    limit?: number;
    is_assembly?: boolean;
  }, token?: string) {
    try {
      // Always use axios api instance which handles auth automatically
      const response = await api.get(API_ENDPOINTS.PRODUCTS, { params });
      return response.data;
    } catch (error: any) {
      console.warn('ProductService: Failed to load products:', error.message);
      // Return empty structure for graceful fallback
      return { data: [] };
    }
  }

  async getProduct(id: number) {
    const response = await api.get(API_ENDPOINTS.PRODUCTS_BY_ID(id));
    return response.data;
  }

  async createProduct(product: Product) {
    const response = await api.post(API_ENDPOINTS.PRODUCTS, product);
    return response.data;
  }

  async updateProduct(id: number, product: Partial<Product>) {
    const response = await api.put(API_ENDPOINTS.PRODUCTS_BY_ID(id), product);
    return response.data;
  }

  async deleteProduct(id: number) {
    const response = await api.delete(API_ENDPOINTS.PRODUCTS_BY_ID(id));
    return response.data;
  }

  async importProducts(products: Partial<Product>[]) {
    const response = await api.post(API_ENDPOINTS.PRODUCTS_IMPORT, products);
    return response.data;
  }

  // Categories
  async getCategories(params?: {
    include_relations?: boolean;
    parent_id?: string;
  }) {
    const response = await api.get(API_ENDPOINTS.CATEGORIES, { params });
    return response.data;
  }

  async getCategory(id: number) {
    const response = await api.get(API_ENDPOINTS.CATEGORIES_BY_ID(id));
    return response.data;
  }

  async createCategory(category: Category) {
    const response = await api.post(API_ENDPOINTS.CATEGORIES, category);
    return response.data;
  }

  async updateCategory(id: number, category: Partial<Category>) {
    const response = await api.put(API_ENDPOINTS.CATEGORIES_BY_ID(id), category);
    return response.data;
  }

  async deleteCategory(id: number) {
    const response = await api.delete(API_ENDPOINTS.CATEGORIES_BY_ID(id));
    return response.data;
  }

  async getCategoryTree() {
    const response = await api.get(API_ENDPOINTS.CATEGORIES_TREE);
    return response.data;
  }

  async getCategoryProducts(id: number, search?: string) {
    const response = await api.get(API_ENDPOINTS.CATEGORIES_PRODUCTS(id), {
      params: { search }
    });
    return response.data;
  }

  // Product Units
  async getProductUnits(params?: {
    search?: string;
    type?: string;
  }) {
    const response = await api.get(API_ENDPOINTS.PRODUCT_UNITS, { params });
    return response.data;
  }

  async getProductUnit(id: number) {
    const response = await api.get(API_ENDPOINTS.PRODUCT_UNITS_BY_ID(id));
    return response.data;
  }

  async createProductUnit(unit: ProductUnit) {
    const response = await api.post(API_ENDPOINTS.PRODUCT_UNITS, unit);
    return response.data;
  }

  async updateProductUnit(id: number, unit: Partial<ProductUnit>) {
    const response = await api.put(API_ENDPOINTS.PRODUCT_UNITS_BY_ID(id), unit);
    return response.data;
  }

  async deleteProductUnit(id: number) {
    const response = await api.delete(API_ENDPOINTS.PRODUCT_UNITS_BY_ID(id));
    return response.data;
  }

  // Warehouse Locations
  async getWarehouseLocations(params?: {
    search?: string;
    is_active?: boolean;
  }) {
    try {
      const response = await api.get(API_ENDPOINTS.WAREHOUSE_LOCATIONS, { params });
      return response.data;
    } catch (error: any) {
      console.warn('ProductService: Warehouse locations API not implemented yet, using mock data');
      // Return mock data structure for development
      return {
        data: [
          {
            id: 1,
            code: 'WH-001',
            name: 'Main Warehouse',
            description: 'Primary storage facility',
            address: 'Jl. Gudang Utama No. 1',
            is_active: true
          },
          {
            id: 2,
            code: 'WH-002',
            name: 'Storage Room A',
            description: 'Small items storage',
            address: 'Jl. Gudang Utama No. 2',
            is_active: true
          },
          {
            id: 3,
            code: 'WH-003',
            name: 'Cold Storage',
            description: 'Temperature controlled storage',
            address: 'Jl. Gudang Utama No. 3',
            is_active: true
          }
        ],
        message: 'Using mock warehouse locations data'
      };
    }
  }

  async getWarehouseLocation(id: number) {
    const response = await api.get(API_ENDPOINTS.WAREHOUSE_LOCATIONS_BY_ID(id));
    return response.data;
  }

  async createWarehouseLocation(location: WarehouseLocation) {
    try {
      const response = await api.post(API_ENDPOINTS.WAREHOUSE_LOCATIONS, location);
      return response.data;
    } catch (error: any) {
      console.warn('ProductService: Warehouse location create API not implemented yet');
      // Return mock success response
      return {
        data: {
          ...location,
          id: Math.floor(Math.random() * 1000) + 100,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString()
        },
        message: 'Mock warehouse location created successfully'
      };
    }
  }

  async updateWarehouseLocation(id: number, location: Partial<WarehouseLocation>) {
    try {
      const response = await api.put(API_ENDPOINTS.WAREHOUSE_LOCATIONS_BY_ID(id), location);
      return response.data;
    } catch (error: any) {
      console.warn('ProductService: Warehouse location update API not implemented yet');
      return {
        data: {
          ...location,
          id,
          updated_at: new Date().toISOString()
        },
        message: 'Mock warehouse location updated successfully'
      };
    }
  }

  async deleteWarehouseLocation(id: number) {
    try {
      const response = await api.delete(API_ENDPOINTS.WAREHOUSE_LOCATIONS_BY_ID(id));
      return response.data;
    } catch (error: any) {
      console.warn('ProductService: Warehouse location delete API not implemented yet');
      return {
        message: 'Mock warehouse location deleted successfully'
      };
    }
  }

  // Inventory
  async getInventoryMovements(params?: {
    product_id?: number;
    start_date?: string;
    end_date?: string;
    type?: 'IN' | 'OUT';
  }) {
    const response = await api.get(API_ENDPOINTS.INVENTORY_MOVEMENTS, { params });
    return response.data;
  }

  async getLowStockProducts() {
    const response = await api.get(API_ENDPOINTS.INVENTORY_LOW_STOCK);
    return response.data;
  }

  async getStockValuation(params?: {
    method?: 'FIFO' | 'LIFO' | 'Average';
    product_id?: number;
  }) {
    const response = await api.get(API_ENDPOINTS.INVENTORY_VALUATION, { params });
    return response.data;
  }

  async getStockReport(params?: {
    category_id?: number;
  }) {
    const response = await api.get(API_ENDPOINTS.INVENTORY_REPORT, { params });
    return response.data;
  }

  async bulkPriceUpdate(data: BulkPriceUpdate) {
    const response = await api.post(API_ENDPOINTS.INVENTORY_BULK_PRICE_UPDATE, data);
    return response.data;
  }

  // Stock Operations
  async adjustStock(data: StockAdjustment) {
    const response = await api.post(API_ENDPOINTS.PRODUCTS_ADJUST_STOCK, data);
    return response.data;
  }

  async stockOpname(data: StockOpname) {
    const response = await api.post(API_ENDPOINTS.PRODUCTS_OPNAME, data);
    return response.data;
  }

  // File Upload
  async uploadProductImage(productId: number, file: File) {
    const formData = new FormData();
    formData.append('image', file);
    formData.append('product_id', productId.toString());

    const response = await api.post(API_ENDPOINTS.PRODUCTS_UPLOAD_IMAGE, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return response.data;
  }
}

export default new ProductService();

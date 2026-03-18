export interface Asset {
  id: number;
  code: string;
  name: string;
  category: string;
  status: 'PENDING' | 'ACTIVE' | 'INACTIVE' | 'SOLD';
  purchase_date: string;
  purchase_price: number;
  salvage_value: number;
  useful_life: number;
  depreciation_method: 'STRAIGHT_LINE' | 'DECLINING_BALANCE';
  accumulated_depreciation: number;
  opening_accumulated_depreciation?: number;
  is_active: boolean;
  notes: string;
  location?: string;
  coordinates?: string;         // "lat,lng" format
  maps_url?: string;           // Generated Google Maps URL
  serial_number?: string;
  condition?: string;
  image_path?: string;          // Path to asset image
  asset_account_id?: number;
  depreciation_account_id?: number;
  accumulated_depreciation_account_id?: number;
  created_at: string;
  updated_at: string;
  
  // Relations (if preloaded)
  asset_account?: {
    id: number;
    code: string;
    name: string;
  };
  depreciation_account?: {
    id: number;
    code: string;
    name: string;
  };
  accumulated_depreciation_account?: {
    id: number;
    code: string;
    name: string;
  };
}

export interface AssetFormData {
  code?: string;
  name: string;
  category: string;
  status?: 'PENDING' | 'ACTIVE' | 'INACTIVE' | 'SOLD';
  purchaseDate: string;
  purchasePrice: number;
  salvageValue?: number;
  usefulLife: number;
  depreciationMethod?: 'STRAIGHT_LINE' | 'DECLINING_BALANCE';
  isActive?: boolean;
  notes?: string;
  location?: string;
  coordinates?: string;         // "lat,lng" format for maps
  serialNumber?: string;
  condition?: string;
  assetAccountId?: number;
  depreciationAccountId?: number;
  userId?: number;
}

export interface AssetsSummary {
  total_assets: number;
  active_assets: number;
  total_value: number;
  total_depreciation: number;
  net_book_value: number;
}

export interface DepreciationEntry {
  year: number;
  date: string;
  depreciation_cost: number;
  accumulated_depreciation: number;
  book_value: number;
}

export interface AssetDepreciationReport {
  asset: Asset;
  annual_depreciation: number;
  monthly_depreciation: number;
  remaining_depreciation: number;
  remaining_years: number;
  current_book_value: number;
}

export interface DepreciationCalculation {
  asset_id: number;
  asset_name: string;
  as_of_date: string;
  purchase_price: number;
  salvage_value: number;
  accumulated_depreciation: number;
  current_book_value: number;
  depreciation_method: string;
  useful_life_years: number;
}

// Account interface for dropdowns
export interface Account {
  id: number;
  code: string;
  name: string;
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  category: string;
  balance: number;
  is_active: boolean;
}

export const ASSET_CATEGORIES = [
  'Fixed Asset',
  'Real Estate',
  'Computer Equipment',
  'Vehicle',
  'Office Equipment',
  'Furniture',
  'IT Infrastructure',
  'Machinery'
] as const;

export const ASSET_STATUS = [
  'PENDING',
  'ACTIVE',
  'INACTIVE',
  'SOLD'
] as const;

export const DEPRECIATION_METHODS = [
  'STRAIGHT_LINE',
  'DECLINING_BALANCE'
] as const;

export const DEPRECIATION_METHOD_LABELS = {
  'STRAIGHT_LINE': 'Straight Line',
  'DECLINING_BALANCE': 'Declining Balance'
} as const;

export type AssetCategory = typeof ASSET_CATEGORIES[number];
export type AssetStatus = typeof ASSET_STATUS[number];
export type DepreciationMethod = typeof DEPRECIATION_METHODS[number];

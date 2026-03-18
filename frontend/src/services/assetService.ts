import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface Asset {
  id: number;
  code: string;
  name: string;
  category: string;
  status: string;
  purchase_date: string;
  purchase_price: number;
  salvage_value: number;
  useful_life: number;
  depreciation_method: string;
  accumulated_depreciation: number;
  opening_accumulated_depreciation?: number;
  is_active: boolean;
  notes: string;
  location?: string;
  coordinates?: string;
  maps_url?: string;
  serial_number?: string;
  condition?: string;
  image_path?: string;
  asset_account_id?: number;
  depreciation_account_id?: number;
  accumulated_depreciation_account_id?: number;
  asset_account?: { id: number; code: string; name: string; balance?: number };
  depreciation_account?: { id: number; code: string; name: string; balance?: number };
  accumulated_depreciation_account?: { id: number; code: string; name: string; balance?: number };
  created_at: string;
  updated_at: string;
}

export interface AssetCreateRequest {
  code?: string;
  name: string;
  category: string;
  status?: string;
  purchase_date: string;
  purchase_price: number;
  salvage_value?: number;
  useful_life: number;
  depreciation_method?: string;
  is_active?: boolean;
  notes?: string;
  location?: string;
  coordinates?: string;
  serial_number?: string;
  condition?: string;
  asset_account_id?: number;
  depreciation_account_id?: number;
  accumulated_depreciation_account_id?: number;
  opening_accumulated_depreciation?: number;
  // Backend AssetCreateRequest requires these additional fields (user_id required)
  user_id?: number;
  payment_method?: string;
  payment_account_id?: number;
  credit_account_id?: number;
}

export interface AssetUpdateRequest {
  name: string;
  category: string;
  status?: string;
  purchase_date: string;
  purchase_price: number;
  salvage_value?: number;
  useful_life: number;
  depreciation_method?: string;
  is_active?: boolean;
  notes?: string;
  location?: string;
  coordinates?: string;
  serial_number?: string;
  condition?: string;
  asset_account_id?: number;
  depreciation_account_id?: number;
  accumulated_depreciation_account_id?: number;
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

export interface MonthlyDepreciationRunResult {
  period: string;
  processed_assets: number;
  skipped_assets: number;
  total_assets: number;
  total_depreciation: number;
  errors?: string[];
}

export interface DepreciationHistoryItem {
  period: string;
  entry_count: number;
  total_depreciation: number;
  last_run_at: string;
  status: string;
}

export interface DepreciationHistoryDetailItem {
  journal_id: number;
  entry_number: string;
  asset_id: number;
  asset_code: string;
  asset_name: string;
  entry_date: string;
  reference: string;
  depreciation_amount: number;
  status: string;
}

class AssetService {
  // Get all assets
  async getAssets(): Promise<{ data: Asset[]; message: string; count: number }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.LIST);
    return response.data;
  }

  // ===== Asset Categories Management =====
  async getAssetCategories(): Promise<{ data: { id: number; code: string; name: string; description?: string; parent_id?: number; is_active: boolean }[]; message: string; count: number }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.CATEGORIES.LIST);
    return response.data;
  }

  async createAssetCategory(category: { code: string; name: string; description?: string; parent_id?: number; is_active?: boolean }): Promise<{ data: any; message: string }> {
    const payload = {
      code: (category.code || '').toUpperCase(),
      name: category.name,
      description: category.description || '',
      parent_id: category.parent_id,
      is_active: category.is_active !== false,
    };
    const response = await api.post(API_ENDPOINTS.ASSETS.CATEGORIES.CREATE, payload);
    return response.data;
  }

  // Get single asset by ID
  async getAsset(id: number): Promise<{ data: Asset; message: string }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.GET_BY_ID(id));
    return response.data;
  }

  // Create new asset
  async createAsset(asset: AssetCreateRequest): Promise<{ data: Asset; message: string }> {
    const response = await api.post(API_ENDPOINTS.ASSETS.CREATE, asset);
    return response.data;
  }

  // Update existing asset
  async updateAsset(id: number, asset: AssetUpdateRequest): Promise<{ data: Asset; message: string }> {
    // Ensure date is properly formatted for backend
    const formattedAsset = {
      ...asset,
      purchase_date: asset.purchase_date.includes('T') ? asset.purchase_date : `${asset.purchase_date}T00:00:00Z`
    };
    const response = await api.put(API_ENDPOINTS.ASSETS.UPDATE(id), formattedAsset);
    return response.data;
  }

  // Delete asset
  async deleteAsset(id: number): Promise<{ message: string }> {
    const response = await api.delete(API_ENDPOINTS.ASSETS.DELETE(id));
    return response.data;
  }

  // Get assets summary
  async getAssetsSummary(): Promise<{ data: AssetsSummary; message: string }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.SUMMARY);
    return response.data;
  }

  // Get depreciation report
  async getDepreciationReport(): Promise<{ data: AssetDepreciationReport[]; message: string; count: number }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.DEPRECIATION_REPORT);
    return response.data;
  }

  // Get depreciation schedule for specific asset
  async getDepreciationSchedule(id: number): Promise<{ 
    data: { asset: Asset; schedule: DepreciationEntry[] }; 
    message: string 
  }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.DEPRECIATION_SCHEDULE(id));
    return response.data;
  }

  // Calculate current depreciation
  async calculateDepreciation(id: number, asOfDate?: string): Promise<{ 
    data: DepreciationCalculation; 
    message: string 
  }> {
    const params = asOfDate ? { as_of_date: asOfDate } : {};
    const response = await api.get(API_ENDPOINTS.ASSETS.CALCULATE_DEPRECIATION(id), { params });
    return response.data;
  }

  async runMonthlyDepreciation(year: number, month: number, userId?: number): Promise<{
    data: MonthlyDepreciationRunResult;
    message: string;
  }> {
    const response = await api.post(API_ENDPOINTS.ASSETS.RUN_DEPRECIATION, {
      year,
      month,
      user_id: userId,
    });
    return response.data;
  }

  async getDepreciationHistory(limit = 6): Promise<{
    data: DepreciationHistoryItem[];
    message: string;
    count: number;
  }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.DEPRECIATION_HISTORY, {
      params: { limit },
    });
    return response.data;
  }

  async getDepreciationHistoryDetail(period: string): Promise<{
    data: {
      period: string;
      entries: DepreciationHistoryDetailItem[];
    };
    message: string;
    count: number;
  }> {
    const response = await api.get(API_ENDPOINTS.ASSETS.DEPRECIATION_HISTORY_DETAIL(period));
    return response.data;
  }

  // Upload asset image
  async uploadAssetImage(assetId: number, imageFile: File): Promise<{
    message: string;
    filename: string;
    path: string;
    asset: Asset;
  }> {
    const formData = new FormData();
    formData.append('asset_id', assetId.toString());
    formData.append('image', imageFile);

    const response = await api.post(API_ENDPOINTS.ASSETS.UPLOAD_IMAGE, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return response.data;
  }

  // Helper method to format currency
  formatCurrency(amount: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  }

  // Helper method to format date
  formatDate(dateString: string): string {
    return new Date(dateString).toLocaleDateString('id-ID', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    });
  }

  // Helper method to get category prefix
  getCategoryPrefix(category: string): string {
    const prefixes: { [key: string]: string } = {
      'Fixed Asset': 'FA',
      'Real Estate': 'RE',
      'Computer Equipment': 'CE',
      'Vehicle': 'VH',
      'Office Equipment': 'OE',
      'Furniture': 'FR',
      'IT Infrastructure': 'IT',
      'Machinery': 'MC',
    };
    return prefixes[category] || 'AS';
  }

  // Helper method to calculate current book value
  calculateBookValue(asset: Asset): number {
    return asset.purchase_price - asset.accumulated_depreciation;
  }

  // Helper method to get asset status color
  getStatusColor(status: string): string {
    switch (status) {
      case 'ACTIVE':
        return 'green';
      case 'INACTIVE':
        return 'gray';
      case 'SOLD':
        return 'red';
      default:
        return 'gray';
    }
  }

  // Helper method to get depreciation method display name
  getDepreciationMethodName(method: string): string {
    switch (method) {
      case 'STRAIGHT_LINE':
        return 'Straight Line';
      case 'DECLINING_BALANCE':
        return 'Declining Balance';
      default:
        return method;
    }
  }

  // Helper method to generate Google Maps URL
  generateMapsURL(coordinates: string): string {
    if (!coordinates) return '';
    return `https://www.google.com/maps?q=${coordinates}`;
  }

  // Helper method to open location in maps
  openInMaps(coordinates: string): void {
    if (!coordinates) return;
    const mapsURL = this.generateMapsURL(coordinates);
    window.open(mapsURL, '_blank');
  }

  // Helper method to validate coordinates format (lat,lng)
  validateCoordinates(coordinates: string): boolean {
    if (!coordinates) return true; // Optional field
    const coordRegex = /^-?\d+\.?\d*,-?\d+\.?\d*$/;
    return coordRegex.test(coordinates.trim());
  }

  // Validate asset data
  validateAsset(asset: AssetCreateRequest | AssetUpdateRequest): string[] {
    const errors: string[] = [];

    if (!asset.name?.trim()) {
      errors.push('Asset name is required');
    }

    if (!asset.category?.trim()) {
      errors.push('Category is required');
    }

    if (!asset.purchase_date) {
      errors.push('Purchase date is required');
    }

    if (!asset.purchase_price || asset.purchase_price <= 0) {
      errors.push('Purchase price must be greater than 0');
    }

    if (!asset.useful_life || asset.useful_life <= 0) {
      errors.push('Useful life must be greater than 0');
    }

    if (asset.salvage_value && asset.salvage_value < 0) {
      errors.push('Salvage value cannot be negative');
    }

    if (asset.salvage_value && asset.purchase_price && asset.salvage_value >= asset.purchase_price) {
      errors.push('Salvage value must be less than purchase price');
    }

    return errors;
  }

  // Get integrated cash & bank accounts (from cash_banks table with COA integration)
  async getBankAccounts(): Promise<{ data: any[]; message: string }> {
    // Use cashbank service to get integrated accounts instead of direct COA
    const response = await api.get(API_ENDPOINTS.CASH_BANK.PAYMENT_ACCOUNTS);
    
    // Transform the response to match expected format for asset form
    if (response.data && response.data.data) {
      return {
        data: response.data.data.map((account: any) => ({
          id: account.id,
          code: account.code,
          name: account.name,
          type: account.type, // CASH or BANK
          balance: account.balance,
          currency: account.currency || 'IDR',
          bank_name: account.bank_name,
          account_no: account.account_no,
          account_id: account.account_id, // Link to COA
          // Enhanced display info for better UX
          display_name: account.bank_name 
            ? `${account.name} - ${account.bank_name} (${account.account_no})`
            : account.name,
          balance_formatted: new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: account.currency || 'IDR',
            minimumFractionDigits: 0
          }).format(account.balance)
        })),
        message: response.data.message || 'Cash & Bank accounts loaded successfully'
      };
    }
    
    return {
      data: [],
      message: 'No cash & bank accounts found'
    };
  }

  // Get liability accounts for credit purchases
  async getLiabilityAccounts(): Promise<{ data: any[]; message: string }> {
    const response = await api.get(API_ENDPOINTS.ACCOUNTS.LIST + '?type=LIABILITY');
    return response.data;
  }

  // Get fixed asset accounts
  async getFixedAssetAccounts(): Promise<{ data: any[]; message: string }> {
    const response = await api.get(API_ENDPOINTS.ACCOUNTS.LIST + '?category=FIXED_ASSET&type=ASSET');
    return response.data;
  }

  // Get depreciation expense accounts
  async getDepreciationExpenseAccounts(): Promise<{ data: any[]; message: string }> {
    // Fetch all EXPENSE type accounts and filter for depreciation expense
    const response = await api.get(API_ENDPOINTS.ACCOUNTS.LIST + '?type=EXPENSE');
    const allExpense = response.data?.data || [];
    const filtered = allExpense.filter((acc: any) => {
      const code = (acc.code || '').toString();
      const name = (acc.name || '').toLowerCase();
      const category = (acc.category || '').toUpperCase();
      
      // Match by category first
      if (category === 'DEPRECIATION_EXPENSE') return true;
      
      // Exclude "akumulasi" accounts (those are contra-asset, not expense)
      if (name.includes('akumulasi')) return false;
      
      // Match beban penyusutan accounts by code or name
      return code.startsWith('6103') || code.startsWith('6104') || 
             (name.includes('penyusutan') && name.includes('beban')) ||
             (name.includes('depreciation') && name.includes('expense'));
    });
    return { data: filtered, message: 'Depreciation expense accounts loaded' };
  }

  // Get accumulated depreciation accounts (1202-xxx)
  async getAccumulatedDepreciationAccounts(): Promise<{ data: any[]; message: string }> {
    // Fetch ASSET type accounts, then filter for 1202-xxx (accumulated depreciation)
    const response = await api.get(API_ENDPOINTS.ACCOUNTS.LIST + '?type=ASSET');
    const data = response.data?.data || [];
    const filtered = data.filter((acc: any) => {
      const code = (acc.code || '').toString();
      const name = (acc.name || '').toLowerCase();
      return code.startsWith('1202') ||
             (name.includes('akumulasi') && name.includes('penyusutan')) ||
             (name.includes('accumulated') && name.includes('depreciation'));
    });
    return { data: filtered, message: 'Accumulated depreciation accounts loaded' };
  }

  // Export assets data (frontend processing)
  exportToCSV(assets: Asset[]): void {
    const headers = [
      'Code',
      'Name',
      'Category',
      'Status',
      'Purchase Date',
      'Purchase Price',
      'Current Value',
      'Depreciation Method',
      'Useful Life',
      'Notes'
    ];

    const csvData = assets.map(asset => [
      asset.code,
      asset.name,
      asset.category,
      asset.status,
      this.formatDate(asset.purchase_date),
      asset.purchase_price,
      this.calculateBookValue(asset),
      this.getDepreciationMethodName(asset.depreciation_method),
      `${asset.useful_life} years`,
      asset.notes || ''
    ]);

    const csvContent = [
      headers.join(','),
      ...csvData.map(row => row.map(cell => `"${cell}"`).join(','))
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `assets_${new Date().getTime()}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }
}

export const assetService = new AssetService();

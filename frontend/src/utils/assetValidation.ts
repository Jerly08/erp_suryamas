import { ASSET_CATEGORIES } from '@/types/asset';

export interface AssetFormData {
  id?: number;
  code?: string;
  name: string;
  category: string;
  status?: 'ACTIVE' | 'INACTIVE' | 'SOLD';
  purchaseDate: string;
  purchasePrice: number;
  salvageValue?: number;
  usefulLife: number;
  depreciationMethod?: 'STRAIGHT_LINE' | 'DECLINING_BALANCE';
  isActive?: boolean;
  notes?: string;
  location?: string;
  serialNumber?: string;
  condition?: string;
  assetAccountId?: number;
  depreciationAccountId?: number;
  accumulatedDepreciationAccountId?: number;
  coordinates?: string;
  entryType?: 'purchase' | 'opening';
  paymentMethod?: string;
  paymentAccountId?: number;
  creditAccountId?: number;
}

export interface ValidationError {
  field: keyof AssetFormData;
  message: string;
}

// Accept optional allowedCategories to support dynamic categories loaded from DB.
// Fallback to the default ASSET_CATEGORIES constant for backward compatibility.
export const validateAssetForm = (
  data: AssetFormData,
  allowedCategories?: readonly string[]
): ValidationError[] => {
  const errors: ValidationError[] = [];

  const categories = allowedCategories && allowedCategories.length > 0
    ? allowedCategories.map((c) => c.trim())
    : [...ASSET_CATEGORIES];

  // Required field validations
  if (!data.name?.trim()) {
    errors.push({ field: 'name', message: 'Asset name is required' });
  } else if (data.name.trim().length < 2) {
    errors.push({ field: 'name', message: 'Asset name must be at least 2 characters' });
  } else if (data.name.trim().length > 100) {
    errors.push({ field: 'name', message: 'Asset name must not exceed 100 characters' });
  }

  if (!data.category?.trim()) {
    errors.push({ field: 'category', message: 'Category is required' });
  } else if (!categories.includes(data.category.trim())) {
    errors.push({ field: 'category', message: 'Please select a valid category' });
  }

  if (!data.purchaseDate) {
    errors.push({ field: 'purchaseDate', message: 'Purchase date is required' });
  } else {
    const purchaseDate = new Date(data.purchaseDate);
    const today = new Date();
    
    if (purchaseDate > today) {
      errors.push({ field: 'purchaseDate', message: 'Purchase date cannot be in the future' });
    }
    
    const minDate = new Date('1900-01-01');
    if (purchaseDate < minDate) {
      errors.push({ field: 'purchaseDate', message: 'Purchase date seems too old' });
    }
  }

  // Purchase price validation
  if (!data.purchasePrice || data.purchasePrice <= 0) {
    errors.push({ field: 'purchasePrice', message: 'Purchase price must be greater than 0' });
  } else if (data.purchasePrice > 999999999999) {
    errors.push({ field: 'purchasePrice', message: 'Purchase price is too large' });
  }

  // Salvage value validation
  if (data.salvageValue !== undefined) {
    if (data.salvageValue < 0) {
      errors.push({ field: 'salvageValue', message: 'Salvage value cannot be negative' });
    } else if (data.salvageValue >= data.purchasePrice) {
      errors.push({ field: 'salvageValue', message: 'Salvage value must be less than purchase price' });
    }
  }

  // Useful life validation
  if (!data.usefulLife || data.usefulLife <= 0) {
    errors.push({ field: 'usefulLife', message: 'Useful life must be greater than 0' });
  } else if (data.usefulLife > 100) {
    errors.push({ field: 'usefulLife', message: 'Useful life cannot exceed 100 years' });
  }

  // Depreciation account validation — required when status is ACTIVE
  // (skip for land/tanah which doesn't depreciate)
  const categoryLower = (data.category || '').toLowerCase().trim();
  const isLand = categoryLower === 'tanah' || categoryLower === 'land';
  if (data.status === 'ACTIVE' && !isLand) {
    if (!data.depreciationAccountId) {
      errors.push({ field: 'depreciationAccountId', message: 'Depreciation expense account is required for active assets' });
    }
    if (!data.accumulatedDepreciationAccountId) {
      errors.push({ field: 'accumulatedDepreciationAccountId', message: 'Accumulated depreciation account is required for active assets' });
    }
  }

  // Serial number validation (if provided)
  if (data.serialNumber && data.serialNumber.length > 50) {
    errors.push({ field: 'serialNumber', message: 'Serial number must not exceed 50 characters' });
  }

  // Location validation (if provided)
  if (data.location && data.location.length > 100) {
    errors.push({ field: 'location', message: 'Location must not exceed 100 characters' });
  }

  // Notes validation (if provided)
  if (data.notes && data.notes.length > 1000) {
    errors.push({ field: 'notes', message: 'Notes must not exceed 1000 characters' });
  }

  return errors;
};

export const getFieldError = (errors: ValidationError[], field: keyof AssetFormData): string | undefined => {
  const error = errors.find(e => e.field === field);
  return error?.message;
};

export const hasFieldError = (errors: ValidationError[], field: keyof AssetFormData): boolean => {
  return errors.some(e => e.field === field);
};

// Helper function to format currency for display
export const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

// Helper function to format date for display
export const formatDate = (dateString: string): string => {
  return new Date(dateString).toLocaleDateString('id-ID', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });
};

// Helper function to calculate book value
export const calculateBookValue = (purchasePrice: number, accumulatedDepreciation: number): number => {
  return Math.max(0, purchasePrice - accumulatedDepreciation);
};

// Helper function to calculate depreciation percentage
export const calculateDepreciationPercentage = (purchasePrice: number, accumulatedDepreciation: number): number => {
  if (purchasePrice === 0) return 0;
  return (accumulatedDepreciation / purchasePrice) * 100;
};

// Helper function to get category prefix for asset codes
export const getCategoryPrefix = (category: string): string => {
  const prefixes: { [key: string]: string } = {
    'Real Estate': 'RE',
    'Computer Equipment': 'CE',
    'Vehicle': 'VH',
    'Office Equipment': 'OE',
    'Furniture': 'FR',
    'IT Infrastructure': 'IT',
    'Machinery': 'MC',
  };
  return prefixes[category] || 'AS';
};

// Helper function to get status display color
export const getStatusColor = (status: string): string => {
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
};

// Helper function to get condition display color
export const getConditionColor = (condition: string): string => {
  switch (condition?.toLowerCase()) {
    case 'excellent':
      return 'green';
    case 'good':
      return 'blue';
    case 'fair':
      return 'yellow';
    case 'poor':
      return 'red';
    default:
      return 'gray';
  }
};

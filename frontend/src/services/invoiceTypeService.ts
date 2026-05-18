import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface InvoiceType {
  id: number;
  name: string;
  code: string;
  description?: string;
  is_active: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
  creator?: {
    id: number;
    username: string;
    first_name?: string;
    last_name?: string;
    name?: string; // Full name for display
  };
}

export interface InvoiceTypeCreateRequest {
  name: string;
  code: string;
  description?: string;
}

export interface InvoiceTypeUpdateRequest {
  name?: string;
  code?: string;
  description?: string;
  is_active?: boolean;
}

export interface InvoiceNumberPreview {
  invoice_number: string;
  counter: number;
  year: number;
  month_roman: string;
  type_code: string;
}

export interface InvoiceNumberPreviewRequest {
  invoice_type_id: number;
  date: string; // ISO date string
}

export interface InvoiceCounter {
  id: number;
  invoice_type_id: number;
  year: number;
  counter: number;
  created_at: string;
  updated_at: string;
  invoice_type?: InvoiceType;
}

export interface ResetCounterRequest {
  year: number;
  counter: number;
}

class InvoiceTypeService {
  // Get all invoice types
  async getInvoiceTypes(activeOnly = false): Promise<InvoiceType[]> {
    try {
      if (activeOnly) {
        return this.getActiveInvoiceTypes();
      }

      const params = activeOnly ? { active_only: 'true' } : {};
      console.log('InvoiceTypeService: Making API call to:', API_ENDPOINTS.INVOICE_TYPES, 'with params:', params);
      const response = await api.get(API_ENDPOINTS.INVOICE_TYPES, { params });
      console.log('InvoiceTypeService: Raw API response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        console.warn('InvoiceTypeService: Received null response from API');
        return [];
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data && Array.isArray(response.data.data)) {
        return response.data.data;
      } else if (Array.isArray(response.data)) {
        return response.data;
      } else {
        console.warn('InvoiceTypeService: Unexpected response structure:', response.data);
        return [];
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error fetching invoice types:', error);
      
      // If it's a 404 or 500 error, or network error, return empty array
      if (error.response?.status === 404 || error.response?.status >= 500 || !error.response) {
        return [];
      }
      
      // Re-throw other errors (like authentication errors)
      throw error;
    }
  }

  // Get active invoice types for dropdowns
  async getActiveInvoiceTypes(): Promise<InvoiceType[]> {
    try {
      const response = await api.get(API_ENDPOINTS.INVOICE_TYPES_ACTIVE);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        console.warn('InvoiceTypeService: Received null response from active invoice types API');
        return [];
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data && Array.isArray(response.data.data)) {
        return response.data.data;
      } else if (Array.isArray(response.data)) {
        return response.data;
      } else {
        console.warn('InvoiceTypeService: Unexpected active invoice types response structure:', response.data);
        return [];
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error fetching active invoice types:', error);
      
      // If it's a 404 or 500 error, or network error, return empty array
      if (error.response?.status === 404 || error.response?.status >= 500 || !error.response) {
        return [];
      }
      
      // Re-throw other errors (like authentication errors)
      throw error;
    }
  }

  // Get single invoice type
  async getInvoiceType(id: number): Promise<InvoiceType> {
    try {
      const response = await api.get(API_ENDPOINTS.INVOICE_TYPES_BY_ID(id));
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('No response received from server');
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data) {
        return response.data.data;
      } else if (response.data.id) {
        return response.data;
      } else {
        throw new Error('Invalid response structure');
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error fetching invoice type:', error);
      throw error;
    }
  }

  // Create new invoice type
  async createInvoiceType(data: InvoiceTypeCreateRequest): Promise<InvoiceType> {
    try {
      console.log('InvoiceTypeService: Creating invoice type with data:', data);
      const response = await api.post(API_ENDPOINTS.INVOICE_TYPES, data);
      console.log('InvoiceTypeService: Create response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('No response received from server');
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data) {
        return response.data.data;
      } else if (response.data.id) {
        return response.data;
      } else {
        throw new Error('Invalid response structure from create endpoint');
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error creating invoice type:', error);
      throw error;
    }
  }

  // Update invoice type
  async updateInvoiceType(id: number, data: InvoiceTypeUpdateRequest): Promise<InvoiceType> {
    try {
      console.log('InvoiceTypeService: Updating invoice type', id, 'with data:', data);
      const response = await api.put(API_ENDPOINTS.INVOICE_TYPES_BY_ID(id), data);
      console.log('InvoiceTypeService: Update response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('No response received from server');
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data) {
        return response.data.data;
      } else if (response.data.id) {
        return response.data;
      } else {
        throw new Error('Invalid response structure from update endpoint');
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error updating invoice type:', error);
      throw error;
    }
  }

  // Delete invoice type
  async deleteInvoiceType(id: number): Promise<void> {
    try {
      console.log('InvoiceTypeService: Deleting invoice type', id);
      const response = await api.delete(API_ENDPOINTS.INVOICE_TYPES_BY_ID(id));
      console.log('InvoiceTypeService: Delete response:', response);
      // Delete operations typically don't return data, just check if successful
    } catch (error: any) {
      console.error('InvoiceTypeService: Error deleting invoice type:', error);
      throw error;
    }
  }

  // Toggle invoice type status
  async toggleInvoiceType(id: number): Promise<InvoiceType> {
    try {
      console.log('InvoiceTypeService: Toggling invoice type', id);
      const response = await api.post(API_ENDPOINTS.INVOICE_TYPES_TOGGLE(id));
      console.log('InvoiceTypeService: Toggle response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('No response received from server');
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data) {
        return response.data.data;
      } else if (response.data.id) {
        return response.data;
      } else {
        throw new Error('Invalid response structure from toggle endpoint');
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error toggling invoice type:', error);
      throw error;
    }
  }

  // Preview next invoice number
  async previewInvoiceNumber(invoiceTypeId: number): Promise<{ preview_number: string }> {
    try {
      // Use the correct overload for preview - just pass the ID
      const response = await api.get(`${API_ENDPOINTS.INVOICE_TYPES_BY_ID(invoiceTypeId)}/preview`);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('Failed to get invoice number preview');
      }
      
      // Handle different response structures
      if (response.data.data) {
        return response.data.data;
      } else if (response.data.preview_number) {
        return response.data;
      } else {
        throw new Error('Invalid preview response structure');
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error previewing invoice number:', error);
      throw error;
    }
  }

  // Get counter history for a specific type
  async getCounterHistory(id: number): Promise<InvoiceCounter[]> {
    try {
      console.log('InvoiceTypeService: Getting counter history for', id);
      const response = await api.get(API_ENDPOINTS.INVOICE_TYPES_COUNTER_HISTORY(id));
      console.log('InvoiceTypeService: Counter history response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        console.warn('InvoiceTypeService: Received null response from counter history API');
        return [];
      }
      
      // Handle different response structures (axios response has response.data directly)
      if (response.data.data && Array.isArray(response.data.data)) {
        return response.data.data;
      } else if (Array.isArray(response.data)) {
        return response.data;
      } else {
        console.warn('InvoiceTypeService: Unexpected counter history response structure:', response.data);
        return [];
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error fetching counter history:', error);
      
      // Return empty array on error to keep UI functional
      return [];
    }
  }

  // Reset counter for a specific invoice type and year
  async resetCounter(id: number, data: ResetCounterRequest): Promise<any> {
    try {
      console.log('InvoiceTypeService: Resetting counter for invoice type', id, 'with data:', data);
      const response = await api.post(API_ENDPOINTS.INVOICE_TYPES_RESET_COUNTER(id), data);
      console.log('InvoiceTypeService: Reset counter response:', response);
      
      // Handle null or undefined response
      if (!response || !response.data) {
        throw new Error('No response received from server');
      }
      
      // Handle different response structures
      if (response.data.data) {
        return response.data.data;
      } else {
        return response.data;
      }
    } catch (error: any) {
      console.error('InvoiceTypeService: Error resetting counter:', error);
      throw error;
    }
  }

  // Validate code uniqueness (optional utility)
  async validateCode(code: string, excludeId?: number): Promise<boolean> {
    try {
      const types = await this.getInvoiceTypes();
      const existingType = types.find(
        type => type.code.toUpperCase() === code.toUpperCase() && type.id !== excludeId
      );
      return !existingType;
    } catch (error) {
      console.error('Error validating code:', error);
      return false;
    }
  }

  // Format display text for dropdown
  formatDisplayText(type: InvoiceType): string {
    return `${type.code} - ${type.name}`;
  }

  // Get invoice type options for Select components
  getSelectOptions(types: InvoiceType[]) {
    return types.map(type => ({
      value: type.id,
      label: this.formatDisplayText(type),
      code: type.code,
      name: type.name,
      isActive: type.is_active
    }));
  }
}

export default new InvoiceTypeService();

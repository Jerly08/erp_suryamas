import api from './api';
import { API_ENDPOINTS } from '@/config/api';

// Types
export interface Contact {
  id: number;
  code?: string;
  name: string;
  type: 'CUSTOMER' | 'VENDOR';
  email?: string;
  phone?: string;
  address?: string;
  payment_terms?: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ContactFilters {
  type?: 'CUSTOMER' | 'VENDOR';
  is_active?: boolean;
  search?: string;
  page?: number;
  limit?: number;
}

export interface ContactResult {
  data: Contact[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

class SearchableSelectService {
  private readonly baseUrl = API_ENDPOINTS.CONTACTS; // '/api/v1/contacts'

  // Get contacts for selection dropdowns
  async getContacts(filters: ContactFilters = {}): Promise<Contact[]> {
    try {
      const params = new URLSearchParams();
      
      if (filters.type) params.append('type', filters.type);
      if (filters.is_active !== undefined) params.append('is_active', filters.is_active.toString());
      if (filters.search) params.append('search', filters.search);
      if (filters.page) params.append('page', filters.page.toString());
      if (filters.limit) params.append('limit', filters.limit.toString());

      const response = await api.get(`${this.baseUrl}?${params}`);
      
      // Handle both array response and paginated response
      if (Array.isArray(response.data)) {
        return response.data;
      } else if (response.data?.data) {
        return response.data.data;
      } else {
        return [];
      }
    } catch (error) {
      console.error('Error fetching contacts:', error);
      // Return empty array as fallback
      return [];
    }
  }

  // Get customers only
  async getCustomers(filters: Omit<ContactFilters, 'type'> = {}): Promise<Contact[]> {
    return this.getContacts({ ...filters, type: 'CUSTOMER' });
  }

  // Get vendors only
  async getVendors(filters: Omit<ContactFilters, 'type'> = {}): Promise<Contact[]> {
    return this.getContacts({ ...filters, type: 'VENDOR' });
  }

  // Search contacts by name
  async searchContacts(query: string, type?: 'CUSTOMER' | 'VENDOR'): Promise<Contact[]> {
    return this.getContacts({ 
      search: query, 
      type,
      limit: 20 // Limit search results for performance
    });
  }

  // Get contact by ID
  async getContactById(id: number): Promise<Contact | null> {
    try {
      const response = await api.get(`${this.baseUrl}/${id}`);
      return response.data;
    } catch (error) {
      console.error('Error fetching contact:', error);
      return null;
    }
  }

  // Format contact name for display
  formatContactName(contact: Contact): string {
    return `${contact.name} (${contact.type})`;
  }

  // Get active contacts only
  async getActiveContacts(filters: Omit<ContactFilters, 'is_active'> = {}): Promise<Contact[]> {
    return this.getContacts({ ...filters, is_active: true });
  }
}

export const searchableSelectService = new SearchableSelectService();
export default searchableSelectService;

import { 
  Contact, 
  ContactAddress,
  ApiResponse,
  ApiError
} from '@/types/contact';
import { API_V1_BASE } from '@/config/api';

// Use centralized API configuration with consistent /api/v1 prefix
const API_BASE_URL = API_V1_BASE;

class ContactService {

  private getHeaders(token?: string): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };
    
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
    
    return headers;
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
      let errorData: ApiError;
      try {
        errorData = await response.json();
      } catch {
        errorData = {
          error: 'Network error',
          code: 'NETWORK_ERROR',
        };
      }

      // Handle specific error cases
      if (response.status === 401) {
        throw new Error('Unauthorized: Authentication token is invalid or expired');
      }
      if (response.status === 403) {
        throw new Error('Forbidden: Insufficient permissions to perform this action');
      }
      if (response.status === 404) {
        throw new Error('Not found: The requested resource does not exist');
      }
      if (response.status === 400) {
        throw new Error(errorData.error || errorData.message || 'Bad request: Invalid data provided');
      }
      if (response.status >= 500) {
        throw new Error(errorData.error || errorData.message || 'Server error: Please try again later');
      }

      throw new Error(errorData.error || errorData.message || `HTTP error! status: ${response.status}`);
    }
    
    // Check if response has content
    const contentLength = response.headers.get('content-length');
    if (contentLength === '0') {
      console.warn('ContactService: Empty response received');
      return {} as T;
    }
    
    try {
      const result = await response.json();
      if (result === null || result === undefined) {
        console.warn('ContactService: Null or undefined response received');
        return {} as T;
      }
      return result;
    } catch (error) {
      console.error('ContactService: Error parsing JSON response:', error);
      throw new Error('Invalid JSON response from server');
    }
  }

  // Get all contacts
  async getContacts(token?: string, type?: string): Promise<Contact[]> {
    if (!token) {
      throw new Error('Authentication token is required to access contacts');
    }
    
    let url: string;
    
    if (type) {
      // Use the type-specific endpoint
      url = `${API_BASE_URL}/contacts/type/${type}`;
    } else {
      // Use the general contacts endpoint
      url = `${API_BASE_URL}/contacts`;
    }
    
    const response = await fetch(url, {
      method: 'GET',
      headers: this.getHeaders(token),
    });
    
    const result = await this.handleResponse<Contact[] | ApiResponse<Contact[]>>(response);
    return Array.isArray(result) ? result : result.data || [];
  }

  // Get single contact by ID
  async getContact(token: string, id: string): Promise<Contact> {
    const response = await fetch(`${API_BASE_URL}/contacts/${id}`, {
      method: 'GET',
      headers: this.getHeaders(token),
    });
    
    const result: ApiResponse<Contact> = await this.handleResponse(response);
    return result.data;
  }

  // Create new contact
  async createContact(token: string, contactData: Partial<Contact>): Promise<Contact> {
    console.log('ContactService: Creating contact with data:', contactData);
    
    if (!token) {
      throw new Error('Authentication token is required to create contacts');
    }
    
    const response = await fetch(`${API_BASE_URL}/contacts`, {
      method: 'POST',
      headers: this.getHeaders(token),
      body: JSON.stringify(contactData),
    });
    
    console.log('ContactService: Response status:', response.status, response.statusText);
    
    const result = await this.handleResponse<Contact | ApiResponse<Contact>>(response);
    console.log('ContactService: Parsed result:', result);
    
    // Handle different response structures
    if (result && typeof result === 'object') {
      // Check if result has data property (wrapped response)
      if ('data' in result && result.data) {
        console.log('ContactService: Returning result.data:', result.data);
        return result.data as Contact;
      }
      
      // Check if result is the contact object directly
      if ('id' in result || 'name' in result) {
        console.log('ContactService: Returning result directly:', result);
        return result as Contact;
      }
    }
    
    console.error('ContactService: Invalid response structure:', result);
    throw new Error('Invalid response format from server');
  }

  // Quick-create a vendor from the Purchase form. This route is authorized by
  // the Purchase module, not by access to the standalone Contact module.
  async createVendorForPurchase(token: string, vendorData: Partial<Contact>): Promise<Contact> {
    if (!token) {
      throw new Error('Authentication token is required to create a vendor');
    }

    const response = await fetch(`${API_BASE_URL}/purchases/vendors`, {
      method: 'POST',
      headers: this.getHeaders(token),
      body: JSON.stringify(vendorData),
    });

    const result = await this.handleResponse<Contact | ApiResponse<Contact>>(response);
    if ('data' in result && result.data) {
      return result.data;
    }
    return result as Contact;
  }

  // Update existing contact
  async updateContact(token: string, id: string, contactData: Partial<Contact>): Promise<Contact> {
    if (!token) {
      throw new Error('Authentication token is required to update contacts');
    }
    
    const response = await fetch(`${API_BASE_URL}/contacts/${id}`, {
      method: 'PUT',
      headers: this.getHeaders(token),
      body: JSON.stringify(contactData),
    });
    
    const result = await this.handleResponse<Contact | ApiResponse<Contact>>(response);
    return 'data' in result ? result.data : result;
  }

  // Delete contact
  async deleteContact(token: string, id: string): Promise<void> {
    if (!token) {
      throw new Error('Authentication token is required to delete contacts');
    }
    
    const response = await fetch(`${API_BASE_URL}/contacts/${id}`, {
      method: 'DELETE',
      headers: this.getHeaders(token),
    });
    
    await this.handleResponse(response);
  }

  // Search contacts
  async searchContacts(token: string, query: string): Promise<Contact[]> {
    if (!token) {
      throw new Error('Authentication token is required to search contacts');
    }
    
    const response = await fetch(`${API_BASE_URL}/contacts/search?q=${encodeURIComponent(query)}`, {
      method: 'GET',
      headers: this.getHeaders(token),
    });
    
    const result = await this.handleResponse<Contact[] | ApiResponse<Contact[]>>(response);
    return Array.isArray(result) ? result : result.data || [];
  }

  // Import contacts
  async importContacts(token: string, contactsData: Contact[]): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/contacts/import`, {
      method: 'POST',
      headers: this.getHeaders(token),
      body: JSON.stringify(contactsData),
    });
    
    await this.handleResponse(response);
  }

  // Export contacts
  async exportContacts(token: string): Promise<Contact[]> {
    const response = await fetch(`${API_BASE_URL}/contacts/export`, {
      method: 'GET',
      headers: this.getHeaders(token),
    });
    
    const result = await this.handleResponse<Contact[] | ApiResponse<Contact[]>>(response);
    return Array.isArray(result) ? result : result.data || [];
  }

  // Helper: Get contact type label
  getContactTypeLabel(type: string): string {
    switch (type) {
      case 'CUSTOMER':
        return 'Pelanggan';
      case 'VENDOR':
        return 'Supplier';
      case 'EMPLOYEE':
        return 'Karyawan';
      default:
        return type;
    }
  }

  // Helper: Get contact type color
  getContactTypeColor(type: string): string {
    switch (type) {
      case 'CUSTOMER':
        return 'blue';
      case 'VENDOR':
        return 'green';
      case 'EMPLOYEE':
        return 'purple';
      default:
        return 'gray';
    }
  }

  // Add contact address
  async addContactAddress(token: string, contactId: string, addressData: Partial<ContactAddress>): Promise<ContactAddress> {
    const response = await fetch(`${API_BASE_URL}/contacts/${contactId}/addresses`, {
      method: 'POST',
      headers: this.getHeaders(token),
      body: JSON.stringify(addressData),
    });
    
    const result: ApiResponse<ContactAddress> = await this.handleResponse(response);
    return result.data;
  }

  // Update contact address
  async updateContactAddress(token: string, contactId: string, addressId: string, addressData: Partial<ContactAddress>): Promise<ContactAddress> {
    const response = await fetch(`${API_BASE_URL}/contacts/${contactId}/addresses/${addressId}`, {
      method: 'PUT',
      headers: this.getHeaders(token),
      body: JSON.stringify(addressData),
    });
    
    const result: ApiResponse<ContactAddress> = await this.handleResponse(response);
    return result.data;
  }

  // Delete contact address
  async deleteContactAddress(token: string, contactId: string, addressId: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/contacts/${contactId}/addresses/${addressId}`, {
      method: 'DELETE',
      headers: this.getHeaders(token),
    });
    
    await this.handleResponse(response);
  }
}

export const contactService = new ContactService();
export default contactService;

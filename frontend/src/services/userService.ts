import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface User {
  id: number;
  username: string;
  email: string;
  full_name: string;
  role: string;
  status: string;
  department?: string;
  position?: string;
  phone?: string;
  created_at: string;
  updated_at: string;
}

export interface UsersFilter {
  page?: number;
  limit?: number;
  role?: string;
  status?: string;
  search?: string;
}

export interface UsersResult {
  data: User[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface UserCreateRequest {
  username: string;
  email: string;
  password: string;
  full_name: string;
  role: string;
  department?: string;
  position?: string;
  phone?: string;
}

export interface UserUpdateRequest {
  username?: string;
  email?: string;
  password?: string;
  full_name?: string;
  role?: string;
  department?: string;
  position?: string;
  phone?: string;
  status?: string;
}

class UserService {
  async getUsers(filter: UsersFilter = {}): Promise<UsersResult> {
    const params = new URLSearchParams();
    Object.entries(filter).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        params.append(key, value.toString());
      }
    });
    
    const response = await api.get(API_ENDPOINTS.USERS.LIST + `?${params}`);
    return response.data;
  }

  async getUser(id: number): Promise<User> {
    const response = await api.get(API_ENDPOINTS.USERS.GET_BY_ID(id));
    return response.data;
  }

  async createUser(data: UserCreateRequest): Promise<User> {
    const response = await api.post(API_ENDPOINTS.USERS.CREATE, data);
    return response.data;
  }

  async updateUser(id: number, data: UserUpdateRequest): Promise<User> {
    const response = await api.put(API_ENDPOINTS.USERS.UPDATE(id), data);
    return response.data;
  }

  async deleteUser(id: number): Promise<void> {
    await api.delete(API_ENDPOINTS.USERS.DELETE(id));
  }

  async getUserProfile(): Promise<User> {
    const response = await api.get(API_ENDPOINTS.AUTH.PROFILE);
    return response.data;
  }

  async updateProfile(data: Partial<UserUpdateRequest>): Promise<User> {
    const response = await api.put(API_ENDPOINTS.AUTH.PROFILE, data);
    return response.data;
  }

  async changePassword(currentPassword: string, newPassword: string): Promise<void> {
    await api.post(API_ENDPOINTS.AUTH.CHANGE_PASSWORD, {
      current_password: currentPassword,
      new_password: newPassword
    });
  }

  // Helper methods
  getRoleLabel(role: string): string {
    const labels: { [key: string]: string } = {
      'admin': 'Administrator',
      'manager': 'Manager',
      'accountant': 'Accountant',
      'finance': 'Finance Manager',
      'finance_manager': 'Finance Manager',
      'finance_staff': 'Finance Staff',
      'accounting': 'Accounting',
      'sales': 'Sales',
      'inventory': 'Inventory',
      'employee': 'Staff'
    };
    return labels[role] || role;
  }

  getStatusColor(status: string): string {
    const colors: { [key: string]: string } = {
      'active': 'green',
      'inactive': 'gray',
      'suspended': 'red',
      'pending': 'yellow'
    };
    return colors[status] || 'gray';
  }

  getStatusLabel(status: string): string {
    const labels: { [key: string]: string } = {
      'active': 'Active',
      'inactive': 'Inactive',
      'suspended': 'Suspended',
      'pending': 'Pending'
    };
    return labels[status] || status;
  }
}

export default new UserService();

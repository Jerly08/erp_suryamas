import api from './api';
import { API_ENDPOINTS } from '@/config/api';

export interface Project {
    id: number;
    code: string;
    name: string;
    description: string;
    status: 'ACTIVE' | 'COMPLETED' | 'ON_HOLD' | 'CANCELLED';
    start_date: string | null;
    end_date: string | null;
    total_budget: number;
    total_actual_cost: number;
    manager_id: number | null;
    created_at: string;
    updated_at: string;
    manager?: {
        id: number;
        username: string;
        first_name: string;
        last_name: string;
    };
}

export interface ProjectCreateRequest {
    code: string;
    name: string;
    description?: string;
    start_date?: string;
    end_date?: string;
    total_budget?: number;
    manager_id?: number;
}

export interface ProjectUpdateRequest {
    name?: string;
    description?: string;
    status?: string;
    start_date?: string;
    end_date?: string;
    total_budget?: number;
    manager_id?: number;
}

// Ensure the endpoint exists in api.ts or config/api.ts
// Assuming API_ENDPOINTS.PROJECTS='projects' or equivalent and api has baseURL defined.
// Using explicit string for safety since it might not be exported in API_ENDPOINTS yet.
const BASE_URL = '/api/v1/projects';

export const projectService = {
    getAll: async (): Promise<Project[]> => {
        const response = await api.get(BASE_URL);
        return response.data.data; // Assuming standard JSON response format with { data: [...] }
    },

    getById: async (id: number): Promise<Project> => {
        const response = await api.get(`${BASE_URL}/${id}`);
        return response.data.data;
    },

    create: async (data: ProjectCreateRequest): Promise<Project> => {
        const response = await api.post(BASE_URL, data);
        return response.data.data;
    },

    update: async (id: number, data: ProjectUpdateRequest): Promise<Project> => {
        const response = await api.put(`${BASE_URL}/${id}`, data);
        return response.data.data;
    },

    delete: async (id: number): Promise<void> => {
        await api.delete(`${BASE_URL}/${id}`);
    },
};

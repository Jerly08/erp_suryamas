import api from './api';

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
    total_sales_revenue: number;
    sales_pipeline_amount?: number;
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
    clear_start_date?: boolean;
    clear_end_date?: boolean;
    total_budget?: number;
    manager_id?: number;
}

const BASE_URL = '/api/v1/projects';

export const projectService = {
    getAll: async (): Promise<Project[]> => {
        const response = await api.get(BASE_URL);
        return response.data.data;
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

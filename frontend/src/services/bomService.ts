import api from './api';
import { BOM, BOMCreateRequest, ProjectBOM, ProjectBOMAddPayload } from '../types/bom';
import { ServiceResponse } from '../types/api';

/**
 * Service to handle Master BOM templates operations
 */
export const bomService = {
    /**
     * Fetch all Master BOM templates
     */
    async getBOMs(params?: { target_product_id?: number }): Promise<ServiceResponse<BOM[]>> {
        const response = await api.get('/api/v1/boms', { params });
        return { data: response.data.data, success: true, status: response.status };
    },

    /**
     * Fetch a single BOM template by ID
     */
    async getBOM(id: number): Promise<ServiceResponse<BOM>> {
        const response = await api.get(`/api/v1/boms/${id}`);
        return { data: response.data.data, success: true, status: response.status };
    },

    /**
     * Create a new Master BOM template
     */
    async createBOM(data: BOMCreateRequest): Promise<ServiceResponse<any>> {
        const response = await api.post('/api/v1/boms', data);
        return { data: response.data, success: true, status: response.status };
    },

    /**
     * Update an existing Master BOM template
     */
    async updateBOM(id: number, data: BOMCreateRequest): Promise<ServiceResponse<any>> {
        const response = await api.put(`/api/v1/boms/${id}`, data);
        return { data: response.data, success: true, status: response.status };
    },

    /**
     * Delete a Master BOM template
     */
    async deleteBOM(id: number): Promise<ServiceResponse<any>> {
        const response = await api.delete(`/api/v1/boms/${id}`);
        return { data: response.data, success: true, status: response.status };
    }
};

/**
 * Service to handle Project BOM attachments and auto-procurement
 */
export const projectBOMService = {
    /**
     * Fetch all BOMs attached to a specific Project
     */
    async getProjectBOMs(projectId: number): Promise<ServiceResponse<ProjectBOM[]>> {
        const response = await api.get(`/api/v1/projects/${projectId}/boms`);
        return { data: response.data.data, success: true, status: response.status };
    },

    /**
     * Attach a Master BOM to a Project
     */
    async addBOMToProject(projectId: number, payload: ProjectBOMAddPayload): Promise<ServiceResponse<any>> {
        const response = await api.post(`/api/v1/projects/${projectId}/boms`, payload);
        return { data: response.data.data, success: true, status: response.status };
    },

    /**
     * Remove a BOM attachment from a Project
     */
    async removeBOMFromProject(projectId: number, projectBomId: number): Promise<ServiceResponse<any>> {
        const response = await api.delete(`/api/v1/projects/${projectId}/boms/${projectBomId}`);
        return { data: response.data.data, success: true, status: response.status };
    },

    /**
     * Trigger auto-procurement to draft a PR for stock shortages in the project's BOMs
     */
    async generatePR(projectId: number): Promise<ServiceResponse<any>> {
        const response = await api.post(`/api/v1/projects/${projectId}/generate-pr`);
        return { data: response.data.data, success: true, status: response.status };
    }
};

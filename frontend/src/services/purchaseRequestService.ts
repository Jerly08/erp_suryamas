import api from './api';

export interface User {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
}

export interface Product {
    id: number;
    code: string;
    name: string;
    unit: string;
    purchase_price: number;
}

export interface PurchaseRequestItem {
    id: number;
    purchase_request_id: number;
    product_id: number;
    quantity: number;
    estimated_price: number;
    total_estimated_price: number;
    notes: string;
    product?: Product;
}

export interface PurchaseRequest {
    id: number;
    pr_number: string;
    requester_id: number;
    department: string;
    project_id: number | null;
    total_estimated_amount: number;
    status: 'DRAFT' | 'PENDING_APPROVAL' | 'APPROVED' | 'REJECTED' | 'COMPLETED';
    notes: string;
    approved_by: number | null;
    approved_at: string | null;
    rejection_reason: string;
    created_at: string;
    updated_at: string;
    requester?: User;
    approver?: User;
    project?: {
        id: number;
        code: string;
        name: string;
    };
    items?: PurchaseRequestItem[];
}

export interface PurchaseRequestItemPayload {
    product_id: number;
    quantity: number;
    estimated_price: number;
    notes?: string;
}

export interface PurchaseRequestCreatePayload {
    department?: string;
    project_id?: number | null;
    notes?: string;
    items: PurchaseRequestItemPayload[];
}

export interface RejectPayload {
    reason: string;
}

const BASE_URL = '/api/v1/purchase-requests';

export const purchaseRequestService = {
    getAll: async (department?: string, status?: string): Promise<PurchaseRequest[]> => {
        let query = '';
        const params = new URLSearchParams();
        if (department) params.append('department', department);
        if (status) params.append('status', status);

        if (params.toString()) query = `?${params.toString()}`;

        const response = await api.get(`${BASE_URL}${query}`);
        return response.data;
    },

    getById: async (id: number): Promise<PurchaseRequest> => {
        const response = await api.get(`${BASE_URL}/${id}`);
        return response.data;
    },

    create: async (data: PurchaseRequestCreatePayload): Promise<PurchaseRequest> => {
        const response = await api.post(BASE_URL, data);
        return response.data;
    },

    approve: async (id: number): Promise<{ message: string }> => {
        const response = await api.put(`${BASE_URL}/${id}/approve`);
        return response.data;
    },

    reject: async (id: number, reason: string): Promise<{ message: string }> => {
        const response = await api.put(`${BASE_URL}/${id}/reject`, { reason });
        return response.data;
    },
};

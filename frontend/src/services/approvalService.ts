import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface ApprovalRequest {
  id: number;
  request_code: string;
  workflow_id: number;
  requester_id: number;
  entity_type: string;
  entity_id: number;
  amount: number;
  status: string;
  priority: string;
  request_title: string;
  request_message?: string;
  reject_reason?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  workflow: {
    id: number;
    name: string;
    module: string;
  };
  requester: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
    name: string;
  };
  approval_steps: ApprovalAction[];
}

export interface ApprovalAction {
  id: number;
  request_id: number;
  step_id: number;
  approver_id?: number;
  status: string;
  comments?: string;
  action_date?: string;
  is_active: boolean;
  step: {
    id: number;
    step_order: number;
    step_name: string;
    approver_role: string;
    time_limit?: number; // hours until due
  };
  // When this step became active; used for SLA countdown if provided
  activated_at?: string;
  approver?: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
  };
}

export interface ApprovalHistory {
  id: number;
  request_id: number;
  user_id: number;
  action: string;
  comments: string;
  created_at: string;
  user: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
    role: string;
  };
}

export interface Purchase {
  id: number;
  code: string;
  vendor_id: number;
  user_id: number;
  date: string;
  due_date?: string;
  total_amount: number;
  discount: number;
  tax: number;
  status: string;
  notes?: string;
  approval_status: string;
  requires_approval: boolean;
  approved_at?: string;
  vendor?: {
    id: number;
    name: string;
  };
  user?: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
    name: string;
  };
}

export interface ApprovalStats {
  pending_approvals: number;
  approved_this_month: number;
  rejected_this_month: number;
  total_amount_pending: number;
}

export interface PendingApprovalsResponse {
  purchases: Purchase[];
  total: number;
  page: number;
  limit: number;
}

class ApprovalService {
  // Get purchases pending approval for current user
  async getPurchasesForApproval(params: { page?: number; limit?: number } = {}): Promise<PendingApprovalsResponse> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_PENDING_APPROVAL, { params });
    return response.data;
  }

  // Approve a purchase
  async approvePurchase(purchaseId: number, data: { comments?: string; escalate_to_director?: boolean }): Promise<{ message: string; purchase_id: number; escalated?: boolean; status?: string; approval_status?: string }> {
    const response = await api.post(API_ENDPOINTS.PURCHASES_APPROVE(purchaseId), data);
    return response.data;
  }

  // Reject a purchase
  async rejectPurchase(purchaseId: number, data: { comments: string }): Promise<{ message: string; purchase_id: number; comments: string }> {
    const response = await api.post(API_ENDPOINTS.PURCHASES_REJECT(purchaseId), data);
    return response.data;
  }

  // Get approval history for a purchase
  async getApprovalHistory(purchaseId: number): Promise<{ purchase_id: number; approval_history: ApprovalHistory[] }> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_APPROVAL_HISTORY(purchaseId));
    return response.data;
  }

  // Get approval statistics
  async getApprovalStats(): Promise<ApprovalStats> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_APPROVAL_STATS);
    return response.data;
  }

  // Submit purchase for approval
  async submitPurchaseForApproval(purchaseId: number): Promise<{ message: string; purchase_id: number }> {
    const response = await api.post(API_ENDPOINTS.PURCHASES_SUBMIT_APPROVAL(purchaseId));
    return response.data;
  }

  // SALES APPROVAL METHODS
  
  // Submit sale for approval
  async submitSaleForApproval(saleId: number, data?: { comments?: string; priority?: string }): Promise<ApprovalRequest> {
    const response = await api.post(API_ENDPOINTS.SALES.SUBMIT_APPROVAL(saleId), data || {});
    return response.data;
  }

  // Get approval status for a sale
  async getSaleApprovalStatus(saleId: number): Promise<ApprovalRequest | null> {
    try {
      const response = await api.get(API_ENDPOINTS.SALES.APPROVAL_STATUS(saleId));
      return response.data;
    } catch (error: any) {
      if (error.response?.status === 404) {
        return null;
      }
      throw error;
    }
  }

  // Get sales pending approval
  async getSalesForApproval(params: { page?: number; limit?: number; status?: string; priority?: string } = {}): Promise<any> {
    const response = await api.get(API_ENDPOINTS.SALES.PENDING_APPROVAL, { params });
    return response.data;
  }

  // Approve a sale step
  async approveSaleStep(approvalId: number, stepId: number, data: { comments?: string }): Promise<void> {
    await api.post(API_ENDPOINTS.APPROVAL.APPROVE_STEP(approvalId, stepId), data);
  }

  // Reject a sale step
  async rejectSaleStep(approvalId: number, stepId: number, data: { comments?: string }): Promise<void> {
    await api.post(API_ENDPOINTS.APPROVAL.REJECT_STEP(approvalId, stepId), data);
  }

  // Get sale approval history
  async getSaleApprovalHistory(saleId: number): Promise<{ sale_id: number; approval_history: ApprovalHistory[] }> {
    const response = await api.get(API_ENDPOINTS.SALES.APPROVAL_HISTORY(saleId));
    return response.data;
  }

  // Get all approval requests (generic)
  async getApprovalRequests(params: { 
    page?: number; 
    limit?: number; 
    status?: string; 
    entity_type?: string;
    priority?: string;
    requester_id?: number;
    my_approvals?: boolean;
    date_from?: string;
    date_to?: string;
  } = {}): Promise<any> {
    const response = await api.get(API_ENDPOINTS.APPROVAL.REQUESTS, { params });
    return response.data;
  }

  // Get approval request by ID
  async getApprovalRequest(requestId: number): Promise<ApprovalRequest> {
    const response = await api.get(API_ENDPOINTS.APPROVAL.GET_REQUEST(requestId));
    return response.data;
  }

  // Cancel approval request
  async cancelApprovalRequest(requestId: number, data: { reason?: string } = {}): Promise<void> {
    await api.post(API_ENDPOINTS.APPROVAL.CANCEL_REQUEST(requestId), data);
  }

  // Escalate approval step
  async escalateApprovalStep(approvalId: number, stepId: number, data: { comments?: string; escalate_to?: number }): Promise<void> {
    await api.post(API_ENDPOINTS.APPROVAL.ESCALATE_STEP(approvalId, stepId), data);
  }

  // Get pending approvals for current user (any entity type)
  async getMyPendingApprovals(): Promise<ApprovalRequest[]> {
    const response = await api.get(API_ENDPOINTS.APPROVAL.MY_PENDING);
    return response.data.requests || [];
  }

  // Get approval summary/dashboard data
  async getApprovalSummary(): Promise<{
    total_pending: number;
    total_approved: number;
    total_rejected: number;
    my_pending: number;
    urgent_count: number;
    overdue_count: number;
    by_entity_type: Record<string, number>;
    by_status: Record<string, number>;
  }> {
    const response = await api.get(API_ENDPOINTS.APPROVAL.SUMMARY);
    return response.data;
  }

  // Get approval workflows
  async getApprovalWorkflows(module?: string): Promise<any[]> {
    const response = await api.get(API_ENDPOINTS.APPROVAL.WORKFLOWS, { params: { module } });
    return response.data.workflows || [];
  }

  // Get notifications
  async getNotifications(params: { page?: number; limit?: number; type?: string } = {}): Promise<any> {
    let endpoint = API_ENDPOINTS.NOTIFICATIONS;
    if (params.type === 'approval') {
      endpoint = API_ENDPOINTS.NOTIFICATIONS_APPROVALS;
    } else if (params.type) {
      endpoint = API_ENDPOINTS.NOTIFICATIONS_BY_TYPE(params.type);
    }
    const response = await api.get(endpoint, { params });
    return response.data;
  }

  // Mark notification as read
  async markNotificationAsRead(notificationId: number): Promise<void> {
    await api.put(API_ENDPOINTS.NOTIFICATIONS_MARK_READ(notificationId));
  }

  // Get unread notification count
  async getUnreadNotificationCount(params: { type?: string; types?: string[] } = {}): Promise<{ count: number }> {
    const queryParams: Record<string, string> = {};

    if (params.type) {
      queryParams.type = params.type;
    }

    if (params.types && params.types.length > 0) {
      queryParams.types = params.types.join(',');
    }

    const response = await api.get(API_ENDPOINTS.NOTIFICATIONS_UNREAD_COUNT, { params: queryParams });
    return response.data;
  }

  // Simplified sales approval methods (for compatibility)
  async approveSale(saleId: number, data: { comments?: string }): Promise<{ message: string; sale_id: number }> {
    const response = await api.post(API_ENDPOINTS.SALES.APPROVE(saleId), data);
    return response.data;
  }

  async rejectSale(saleId: number, data: { comments: string }): Promise<{ message: string; sale_id: number }> {
    const response = await api.post(API_ENDPOINTS.SALES.REJECT(saleId), data);
    return response.data;
  }
}

const approvalService = new ApprovalService();
export default approvalService;

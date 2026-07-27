import api from './api';
import { API_ENDPOINTS } from '@/config/api';
import { getDownloadFilename } from '@/utils/downloadFilename';

export type PostedTransactionCorrectionStatus =
  | 'PENDING_ACCOUNTING'
  | 'PENDING_FINANCE_MANAGER'
  | 'APPROVED_FOR_EXECUTION'
  | 'REJECTED'
  | 'COMPLETED'
  | 'CANCELLED';

export type PostedTransactionCorrectionType =
  | 'MISSING_PAYMENT_PROOF'
  | 'PAYMENT_DATE'
  | 'PAYMENT_AMOUNT'
  | 'SOURCE_BANK'
  | 'DESTINATION_ACCOUNT'
  | 'PURCHASE_DATE'
  | 'OTHER';

export type PostedTransactionCorrectionResolution =
  | 'SUPPLEMENTAL_PROOF'
  | 'JOURNAL_RECLASSIFICATION'
  | 'REVERSE_AND_RECREATE'
  | 'BANK_RECONCILIATION'
  | 'MANUAL_ADJUSTMENT';

export interface CorrectionActor {
  id: number;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
}

export interface PostedTransactionCorrectionDocument {
  id: number;
  correction_id: number;
  file_name: string;
  file_path: string;
  file_size: number;
  mime_type?: string;
  uploaded_by: number;
  uploaded_at: string;
  uploader?: CorrectionActor;
}

export interface PostedTransactionCorrectionEvent {
  id: number;
  correction_id: number;
  event_type: string;
  notes?: string;
  actor_id: number;
  actor_role?: string;
  created_at: string;
  actor?: CorrectionActor;
}

export interface PostedTransactionCorrection {
  id: number;
  target_type: 'PURCHASE' | 'PAYMENT';
  target_code: string;
  purchase_id?: number | null;
  payment_id?: number | null;
  purchase_payment_id?: number | null;
  original_journal_entry_id: number;
  correction_type: PostedTransactionCorrectionType;
  status: PostedTransactionCorrectionStatus;
  reason: string;
  requested_date?: string | null;
  requested_amount?: number | null;
  requested_cash_bank_reference?: string;
  requested_destination_reference?: string;
  requested_reference?: string;
  requested_by: number;
  requested_at: string;
  accounting_resolution?: PostedTransactionCorrectionResolution;
  accounting_notes?: string;
  proposed_account_id?: number | null;
  accounting_reviewed_by?: number | null;
  accounting_reviewed_at?: string | null;
  finance_manager_decision_notes?: string;
  finance_manager_approved_by?: number | null;
  finance_manager_approved_at?: string | null;
  completion_notes?: string;
  completion_journal_entry_id?: number | null;
  replacement_payment_id?: number | null;
  completed_by?: number | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
  purchase?: { id: number; code: string; total_amount?: number; date?: string };
  payment?: { id: number; code: string; amount?: number; date?: string };
  requester?: CorrectionActor;
  accounting_user?: CorrectionActor;
  finance_manager?: CorrectionActor;
  completer?: CorrectionActor;
  proposed_account?: { id: number; code: string; name: string };
  documents?: PostedTransactionCorrectionDocument[];
  events?: PostedTransactionCorrectionEvent[];
}

export interface CreatePostedTransactionCorrectionRequest {
  purchase_id?: number;
  payment_id?: number;
  correction_type: PostedTransactionCorrectionType;
  reason: string;
  requested_date?: string;
  requested_amount?: number;
  requested_cash_bank_reference?: string;
  requested_destination_reference?: string;
  requested_reference?: string;
}

export interface PostedTransactionCorrectionFilters {
  purchase_id?: number;
  payment_id?: number;
  status?: PostedTransactionCorrectionStatus;
  page?: number;
  limit?: number;
}

export interface PostedTransactionCorrectionListResponse {
  data: PostedTransactionCorrection[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    total_pages: number;
  };
}

const authHeaders = (token?: string) =>
  token ? { Authorization: `Bearer ${token}` } : undefined;

const postedTransactionCorrectionService = {
  async list(
    token: string | undefined,
    filters: PostedTransactionCorrectionFilters = {},
  ): Promise<PostedTransactionCorrectionListResponse> {
    const response = await api.get(API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.LIST, {
      headers: authHeaders(token),
      params: filters,
    });
    return response.data;
  },

  async getById(token: string | undefined, id: number): Promise<PostedTransactionCorrection> {
    const response = await api.get(API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.BY_ID(id), {
      headers: authHeaders(token),
    });
    return response.data.data;
  },

  async create(
    token: string | undefined,
    payload: CreatePostedTransactionCorrectionRequest,
  ): Promise<PostedTransactionCorrection> {
    const response = await api.post(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.CREATE,
      payload,
      { headers: authHeaders(token) },
    );
    return response.data.data;
  },

  async reviewAccounting(
    token: string | undefined,
    id: number,
    payload: {
      resolution: PostedTransactionCorrectionResolution;
      notes: string;
      proposed_account_id?: number;
    },
  ): Promise<PostedTransactionCorrection> {
    const response = await api.post(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.ACCOUNTING_REVIEW(id),
      payload,
      { headers: authHeaders(token) },
    );
    return response.data.data;
  },

  async decideFinanceManager(
    token: string | undefined,
    id: number,
    payload: { approve: boolean; notes?: string },
  ): Promise<PostedTransactionCorrection> {
    const response = await api.post(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.FINANCE_MANAGER_DECISION(id),
      payload,
      { headers: authHeaders(token) },
    );
    return response.data.data;
  },

  async complete(
    token: string | undefined,
    id: number,
    payload: {
      notes: string;
      completion_journal_entry_id?: number;
      replacement_payment_id?: number;
    },
  ): Promise<PostedTransactionCorrection> {
    const response = await api.post(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.COMPLETE(id),
      payload,
      { headers: authHeaders(token) },
    );
    return response.data.data;
  },

  async uploadDocument(
    token: string | undefined,
    id: number,
    document: File,
  ): Promise<PostedTransactionCorrectionDocument> {
    const formData = new FormData();
    formData.append('document', document);
    const response = await api.post(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.DOCUMENTS(id),
      formData,
      { headers: authHeaders(token) },
    );
    return response.data.document;
  },

  async downloadDocument(
    token: string | undefined,
    document: PostedTransactionCorrectionDocument,
  ): Promise<void> {
    const response = await api.get(
      API_ENDPOINTS.POSTED_TRANSACTION_CORRECTIONS.DOCUMENT_DOWNLOAD(document.id),
      { headers: authHeaders(token), responseType: 'blob' },
    );
    const filename = getDownloadFilename(
      response.headers?.['content-disposition'],
      document.file_name || 'dokumen-koreksi',
    );
    const url = window.URL.createObjectURL(new Blob([response.data]));
    const link = window.document.createElement('a');
    link.href = url;
    link.setAttribute('download', filename);
    window.document.body.appendChild(link);
    link.click();
    link.remove();
    window.URL.revokeObjectURL(url);
  },
};

export default postedTransactionCorrectionService;

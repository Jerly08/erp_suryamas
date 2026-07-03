import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import type { AxiosResponse } from 'axios';

const PAYROLL_UPLOAD_TIMEOUT_MS = 120000;

export type PayrollStatus = 'PENDING_APPROVAL' | 'APPROVED' | 'REJECTED';
export type PayrollPPhPaymentStatus = 'UNPAID' | 'PARTIAL_PAID' | 'PAID';
export type PayrollSalaryPaymentStatus = 'UNPAID' | 'PARTIAL_PAID' | 'PAID';
export type PayrollBPJSPaymentStatus = 'UNPAID' | 'PARTIAL_PAID' | 'PAID';

export interface PayrollSettlement {
  id: number;
  payroll_run_id: number;
  settlement_type: 'SALARY' | 'PPH21' | 'BPJS';
  amount: number;
  payment_date: string;
  cash_bank_id: number;
  journal_entry_id: number;
  reference?: string;
  notes?: string;
  source: string;
  cash_bank?: { id: number; code: string; name: string };
}

export interface PayrollDocument {
  id: number;
  payroll_run_id: number;
  document_type: string;
  file_name: string;
  file_path: string;
  file_size: number;
  mime_type: string;
  uploaded_by: number;
  created_at: string;
  updated_at: string;
  uploader?: {
    id: number;
    username: string;
    first_name?: string;
    last_name?: string;
  };
}

export interface PayrollItem {
  id?: number;
  payroll_run_id?: number;
  employee_name: string;
  employee_code?: string;
  working_day?: number;
  work_unit?: string;
  payment_via?: string;
  account_no?: string;
  division?: string;
  department?: string;
  position?: string;
  base_salary: number;
  bonus: number;
  attendance_allowance?: number;
  compensation_pkwt?: number;
  overtime?: number;
  pph: number;
  bpjs: number;
  bpjs_tk_jht?: number;
  bpjs_tk_jp?: number;
  bpjs_kesehatan?: number;
  proportional?: number;
  other_deduction?: number;
  employee_loan_deduction?: number;
  net_pay?: number;
  notes?: string;
}

export interface PayrollRun {
  id: number;
  payroll_number: string;
  period_start: string;
  period_end: string;
  payment_date?: string | null;
  notes?: string;
  status: PayrollStatus;
  total_salary: number;
  total_bonus: number;
  total_pph: number;
  total_bpjs: number;
  total_employee_loan_deduction?: number;
  total_overall: number;
  employee_loan_account_id?: number | null;
  salary_payment_status?: PayrollSalaryPaymentStatus | '';
  salary_paid_amount?: number;
  salary_payment_date?: string | null;
  salary_payment_reference?: string;
  salary_payment_notes?: string;
  salary_payment_journal_entry_id?: number | null;
  bpjs_payment_status?: PayrollBPJSPaymentStatus | '';
  bpjs_paid_amount?: number;
  bpjs_payment_date?: string | null;
  bpjs_payment_reference?: string;
  bpjs_payment_notes?: string;
  bpjs_payment_journal_entry_id?: number | null;
  pph_payment_status?: PayrollPPhPaymentStatus | '';
  pph_paid_amount?: number;
  pph_recognition_date?: string | null;
  pph_payment_date?: string | null;
  pph_payment_reference?: string;
  pph_payment_notes?: string;
  pph_payable_journal_entry_id?: number | null;
  pph_payment_journal_entry_id?: number | null;
  created_by_id: number;
  approved_by_id?: number | null;
  approved_at?: string | null;
  rejected_by_id?: number | null;
  rejected_at?: string | null;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
  detail_allowed: boolean;
  created_by?: {
    id: number;
    username: string;
    first_name?: string;
    last_name?: string;
  };
  approved_by?: {
    id: number;
    username: string;
    first_name?: string;
    last_name?: string;
  } | null;
  rejected_by?: {
    id: number;
    username: string;
    first_name?: string;
    last_name?: string;
  } | null;
  items?: PayrollItem[];
  settlements?: PayrollSettlement[];
  documents?: PayrollDocument[];
}

export interface PayrollCreatePayload {
  period_start: string;
  period_end: string;
  payment_date?: string;
  notes?: string;
  items: PayrollItem[];
}

export interface PayrollPPhPaymentPayload {
  payment_date: string;
  amount: number;
  cash_bank_id: number;
  reference?: string;
  notes?: string;
}

export type PayrollSalaryPaymentPayload = PayrollPPhPaymentPayload;
export type PayrollBPJSPaymentPayload = PayrollPPhPaymentPayload;

export interface PayrollImportPreview {
  items: PayrollItem[];
  summary: {
    total_rows: number;
    imported_rows: number;
    total_salary: number;
    total_bonus: number;
    total_pph: number;
    total_bpjs: number;
    total_employee_loan_deduction?: number;
    total_overall: number;
  };
  warnings: string[];
  headers: string[];
  suggested_period_start?: string;
  suggested_period_end?: string;
  suggested_payment_date?: string;
}

interface ApiEnvelope<T> {
  data?: T;
  message?: string;
}

const unwrapData = <T>(response: AxiosResponse<ApiEnvelope<T> | T>): T => {
  const body = response.data;
  if (body && typeof body === 'object' && 'data' in body) {
    return (body as ApiEnvelope<T>).data as T;
  }
  return body as T;
};

const payrollService = {
  async getPayrollRuns(status?: string): Promise<PayrollRun[]> {
    const response = await api.get('/api/v1/payroll', {
      params: status && status !== 'ALL' ? { status } : undefined,
    });
    return unwrapData<PayrollRun[]>(response) || [];
  },

  async getPayrollRun(id: number): Promise<PayrollRun> {
    const response = await api.get(`/api/v1/payroll/${id}`);
    return unwrapData<PayrollRun>(response);
  },

  async createPayrollRun(payload: PayrollCreatePayload, importFile?: File | null): Promise<PayrollRun> {
    if (importFile) {
      const formData = new FormData();
      formData.append('payload', JSON.stringify(payload));
      formData.append('import_file', importFile);

      const response = await api.post('/api/v1/payroll', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      return unwrapData<PayrollRun>(response);
    }

    const response = await api.post('/api/v1/payroll', payload);
    return unwrapData<PayrollRun>(response);
  },

  async previewPayrollImport(file: File): Promise<PayrollImportPreview> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await api.post('/api/v1/payroll/import/preview', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return unwrapData<PayrollImportPreview>(response);
  },

  async downloadPayrollImportTemplate(): Promise<Blob> {
    const response = await api.get('/api/v1/payroll/import/template', {
      responseType: 'blob',
    });
    return response.data;
  },

  async uploadDocuments(
    id: number,
    documents: File[],
  ): Promise<PayrollDocument[]> {
    const formData = new FormData();
    documents.forEach((file) => {
      formData.append('documents', file);
    });

    const response = await api.post(`/api/v1/payroll/${id}/documents`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: PAYROLL_UPLOAD_TIMEOUT_MS,
    });
    const body = response.data as { documents?: PayrollDocument[] };
    return body.documents || [];
  },

  async downloadDocument(documentId: number, fallbackName = 'payroll-document'): Promise<void> {
    const response = await api.get(API_ENDPOINTS.PAYROLL_DOCUMENT_DOWNLOAD(documentId), {
      responseType: 'blob',
    });
    const disposition = response.headers?.['content-disposition'] || '';
    const filenameMatch =
      disposition.match(/filename\*=UTF-8''([^;]+)/i) ||
      disposition.match(/filename="?([^"]+)"?/i);
    const filename = filenameMatch?.[1]
      ? decodeURIComponent(filenameMatch[1])
      : fallbackName;

    const url = window.URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  },

  async downloadPayrollExcel(id: number, fallbackName = 'payroll-pengajuan.xlsx'): Promise<void> {
    const response = await api.get(API_ENDPOINTS.PAYROLL_EXPORT_EXCEL(id), {
      responseType: 'blob',
    });
    const disposition = response.headers?.['content-disposition'] || '';
    const filenameMatch =
      disposition.match(/filename\*=UTF-8''([^;]+)/i) ||
      disposition.match(/filename="?([^"]+)"?/i);
    const filename = filenameMatch?.[1]
      ? decodeURIComponent(filenameMatch[1])
      : fallbackName;

    const url = window.URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  },

  async approvePayrollRun(
    id: number,
    payload?: { expense_account_id?: number; posting_date?: string },
  ): Promise<PayrollRun> {
    const response = await api.post(`/api/v1/payroll/${id}/approve`, payload);
    return unwrapData<PayrollRun>(response);
  },

  async rejectPayrollRun(id: number, reason: string): Promise<PayrollRun> {
    const response = await api.post(`/api/v1/payroll/${id}/reject`, { reason });
    return unwrapData<PayrollRun>(response);
  },

  async payPayrollPPh(
    id: number,
    payload: PayrollPPhPaymentPayload,
  ): Promise<PayrollRun> {
    const response = await api.post(`/api/v1/payroll/${id}/pay-pph`, payload);
    return unwrapData<PayrollRun>(response);
  },

  async payPayrollSalary(
    id: number,
    payload: PayrollSalaryPaymentPayload,
  ): Promise<PayrollRun> {
    const response = await api.post(`/api/v1/payroll/${id}/pay-salary`, payload);
    return unwrapData<PayrollRun>(response);
  },

  async payPayrollBPJS(
    id: number,
    payload: PayrollBPJSPaymentPayload,
  ): Promise<PayrollRun> {
    const response = await api.post(`/api/v1/payroll/${id}/pay-bpjs`, payload);
    return unwrapData<PayrollRun>(response);
  },
};

export default payrollService;

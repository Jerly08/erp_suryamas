import api from '@/services/api';
import type { AxiosResponse } from 'axios';

export type PayrollStatus = 'PENDING_APPROVAL' | 'APPROVED' | 'REJECTED';
export type PayrollPPhPaymentStatus = 'UNPAID' | 'PARTIAL_PAID' | 'PAID';

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
  compensation_pkwt?: number;
  overtime?: number;
  pph: number;
  bpjs: number;
  bpjs_tk_jht?: number;
  bpjs_tk_jp?: number;
  bpjs_kesehatan?: number;
  proportional?: number;
  other_deduction?: number;
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
  total_overall: number;
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

export interface PayrollImportPreview {
  items: PayrollItem[];
  summary: {
    total_rows: number;
    imported_rows: number;
    total_salary: number;
    total_bonus: number;
    total_pph: number;
    total_bpjs: number;
    total_overall: number;
  };
  warnings: string[];
  headers: string[];
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

  async createPayrollRun(payload: PayrollCreatePayload): Promise<PayrollRun> {
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

  async approvePayrollRun(
    id: number,
    payload?: { expense_account_id?: number; cash_bank_account_id?: number; payment_method?: string },
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
};

export default payrollService;

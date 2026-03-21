import api from "./api";
import { API_ENDPOINTS } from "../config/api";

export interface SSOTGeneralLedgerData {
  company?: CompanyInfo;
  start_date: string;
  end_date: string;
  currency: string;
  account?: AccountInfo;
  entries?: GeneralLedgerEntry[]; // Legacy field
  transactions?: GeneralLedgerEntry[]; // New field name from backend
  opening_balance?: number;
  closing_balance?: number;
  total_debits?: number;
  total_credits?: number;
  is_balanced?: boolean;
  net_position_change?: number;
  net_position_status?: string;
  generated_at: string;
}

export interface CompanyInfo {
  name: string;
  address: string;
  city: string;
  state: string;
  phone: string;
  email: string;
  tax_number: string;
}

export interface AccountInfo {
  account_id: number;
  account_code: string;
  account_name: string;
  account_type: string;
}

export interface GeneralLedgerEntry {
  // Fields returned by backend GeneralLedgerEntry struct
  date: string;
  journal_code: string;
  description: string;
  reference: string;
  debit_amount: number;
  credit_amount: number;
  balance: number;
  entry_type: string;

  // Optional/Legacy fields that might not be present or named differently
  journal_id?: number;
  account_code?: string;
  account_name?: string;
  status?: string;
}

export interface SSOTGeneralLedgerParams {
  account_id?: string;
  start_date: string;
  end_date: string;
  format?: "json" | "pdf" | "csv" | "excel";
}

class SSOTGeneralLedgerService {
  async generateSSOTGeneralLedger(
    params: SSOTGeneralLedgerParams,
  ): Promise<SSOTGeneralLedgerData> {
    try {
      const queryParams = new URLSearchParams({
        start_date: params.start_date,
        end_date: params.end_date,
        format: params.format || "json",
      });

      if (params.account_id) {
        queryParams.append("account_id", params.account_id);
      }

      const response = await api.get(
        API_ENDPOINTS.SSOT_REPORTS.GENERAL_LEDGER + `?${queryParams}`,
      );

      if (response.data.status === "success") {
        return response.data.data;
      }

      throw new Error(
        response.data.message || "Failed to generate general ledger",
      );
    } catch (error: any) {
      console.error("Error generating SSOT general ledger:", error);
      if (error.response?.data?.message) {
        throw new Error(error.response.data.message);
      }
      throw error;
    }
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
    }).format(value);
  }

  getEntryTypeColor(sourceType: string): string {
    const colors: { [key: string]: string } = {
      SALES: "green",
      PURCHASE: "blue",
      PAYMENT: "orange",
      RECEIPT: "purple",
      JOURNAL: "gray",
      ADJUSTMENT: "red",
    };
    return colors[sourceType] || "gray";
  }

  calculateRunningBalance(entries: GeneralLedgerEntry[]): GeneralLedgerEntry[] {
    let runningBalance = 0;
    return entries.map((entry) => {
      runningBalance += entry.debit_amount - entry.credit_amount;
      return { ...entry, balance: runningBalance };
    });
  }

  /**
   * Export General Ledger as PDF
   */
  async exportToPDF(params: {
    start_date: string;
    end_date: string;
    account_id?: string;
  }): Promise<void> {
    try {
      const queryParams = new URLSearchParams({
        start_date: params.start_date,
        end_date: params.end_date,
        format: "pdf",
      });

      if (params.account_id) {
        queryParams.append("account_id", params.account_id);
      }

      const url = `${API_ENDPOINTS.SSOT_REPORTS.GENERAL_LEDGER}?${queryParams.toString()}`;

      const response = await fetch(url, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });

      if (!response.ok) {
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        try {
          const errorData = await response.json();
          errorMessage = errorData.message || errorMessage;
        } catch {
          // Use default error message if JSON parsing fails
        }
        throw new Error(errorMessage);
      }

      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error("Empty file received from server");
      }

      const filename = `General_Ledger_${params.start_date}_to_${params.end_date}.pdf`;

      // Trigger download
      const downloadUrl = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = downloadUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(downloadUrl);
    } catch (error) {
      console.error("PDF export error:", error);
      throw new Error(
        `Failed to export General Ledger as PDF: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }
  }

  /**
   * Export General Ledger as CSV
   */
  async exportToCSV(params: {
    start_date: string;
    end_date: string;
    account_id?: string;
  }): Promise<void> {
    try {
      const queryParams = new URLSearchParams({
        start_date: params.start_date,
        end_date: params.end_date,
        format: "csv",
      });

      if (params.account_id) {
        queryParams.append("account_id", params.account_id);
      }

      const url = `${API_ENDPOINTS.SSOT_REPORTS.GENERAL_LEDGER}?${queryParams.toString()}`;

      const response = await fetch(url, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });

      if (!response.ok) {
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        try {
          const errorData = await response.json();
          errorMessage = errorData.message || errorMessage;
        } catch {
          // Use default error message if JSON parsing fails
        }
        throw new Error(errorMessage);
      }

      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error("Empty file received from server");
      }

      const filename = `General_Ledger_${params.start_date}_to_${params.end_date}.csv`;

      // Trigger download
      const downloadUrl = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = downloadUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(downloadUrl);
    } catch (error) {
      console.error("CSV export error:", error);
      throw new Error(
        `Failed to export General Ledger as CSV: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }
  }
}

export const ssotGeneralLedgerService = new SSOTGeneralLedgerService();

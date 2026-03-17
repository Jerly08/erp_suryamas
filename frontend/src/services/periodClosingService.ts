import { API_ENDPOINTS } from '@/config/api';
import { getAuthHeaders } from '@/utils/authTokenUtils';

export interface ClosedPeriod {
  id: number;
  start_date: string;
  end_date: string;
  description: string;
  period_type: string;
  fiscal_year?: number;
  closed_at: string;
  is_closed: boolean;
  is_locked: boolean;
  total_revenue: number;
  total_expense: number;
  net_income: number;
}

export interface PeriodFilterOption {
  value: string;
  label: string;
  period: ClosedPeriod;
  group: string;
}

class PeriodClosingService {
  private getAuthHeaders() {
    return getAuthHeaders();
  }

  /**
   * Quick check apakah sebuah tanggal sudah masuk periode yang ditutup.
   * Digunakan untuk validasi form (misalnya payment) sebelum kirim request berat.
   */
  async isDateInClosedPeriod(date: string): Promise<boolean> {
    try {
      if (!date) return false;

      const response = await fetch(`${API_ENDPOINTS.PERIOD_CLOSING.CHECK_DATE}?date=${date}`, {
        headers: this.getAuthHeaders(),
      });

      if (!response.ok) {
        console.warn('PeriodClosingService.isDateInClosedPeriod - non OK response', response.status);
        // Jangan mem‑block transaksi hanya karena service pengecekan gagal;
        // backend masih punya middleware ValidateTransactionPeriod sebagai pengaman terakhir.
        return false;
      }

      const result = await response.json();
      if (!result.success) {
        console.warn('PeriodClosingService.isDateInClosedPeriod - unsuccessful result', result);
        return false;
      }

      return Boolean(result.is_closed);
    } catch (error) {
      console.error('PeriodClosingService.isDateInClosedPeriod - error', error);
      // Fail-open: biarkan backend yang menolak jika memang tertutup.
      return false;
    }
  }

  /**
   * Get all closed periods for filtering
   */
  async getClosedPeriodsForFilter(): Promise<PeriodFilterOption[]> {
    try {
      const response = await fetch(API_ENDPOINTS.PERIOD_CLOSING.HISTORY, {
        headers: this.getAuthHeaders(),
      });

      if (!response.ok) {
        throw new Error('Failed to fetch closed periods');
      }

      const result = await response.json();
      
      if (!result.success || !result.data) {
        return [];
      }

      return this.mapToFilterOptions(result.data);
    } catch (error) {
      console.error('Error fetching closed periods:', error);
      return [];
    }
  }

  /**
   * Map closed periods to filter options
   */
  private mapToFilterOptions(periods: ClosedPeriod[]): PeriodFilterOption[] {
    return periods
      .sort((a, b) => new Date(b.end_date).getTime() - new Date(a.end_date).getTime())
      .map(period => ({
        value: period.end_date,
        label: this.formatPeriodLabel(period),
        period: period,
        group: this.getPeriodGroup(period)
      }));
  }

  /**
   * Format period label for display
   * Example: "31 Des 2026 - Fiscal Year-End Closing 2026"
   */
  private formatPeriodLabel(period: ClosedPeriod): string {
    const endDate = new Date(period.end_date).toLocaleDateString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric'
    });

    return `${endDate} - ${period.description}`;
  }

  /**
   * Get period group for categorization
   */
  private getPeriodGroup(period: ClosedPeriod): string {
    const currentYear = new Date().getFullYear();
    const periodYear = new Date(period.end_date).getFullYear();

    if (periodYear === currentYear) {
      return 'Current Year';
    } else if (periodYear === currentYear - 1) {
      return 'Last Year';
    } else {
      return `Year ${periodYear}`;
    }
  }

  /**
   * Get last closed period (for default selection)
   */
  async getLastClosedPeriod(): Promise<ClosedPeriod | null> {
    try {
      const response = await fetch(API_ENDPOINTS.PERIOD_CLOSING.LAST_INFO, {
        headers: this.getAuthHeaders(),
      });

      if (!response.ok) {
        return null;
      }

      const result = await response.json();
      
      if (result.success && result.data?.has_previous_closing) {
        // Return the last closed period info
        return {
          end_date: result.data.last_closing_date,
          // Minimal info - will be enriched by full list
        } as ClosedPeriod;
      }

      return null;
    } catch (error) {
      console.error('Error fetching last closed period:', error);
      return null;
    }
  }

  /**
   * Validate period data structure
   */
  validatePeriod(period: any): period is ClosedPeriod {
    return (
      typeof period.id === 'number' &&
      typeof period.end_date === 'string' &&
      typeof period.description === 'string' &&
      /^\d{4}-\d{2}-\d{2}$/.test(period.end_date)
    );
  }

  /**
   * Format currency for display
   */
  formatCurrency(amount: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  }
}

export const periodClosingService = new PeriodClosingService();
export default periodClosingService;

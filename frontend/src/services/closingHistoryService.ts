import api from './api';
import { API_ENDPOINTS } from '@/config/api';
import {
  ClosedPeriod,
  PeriodHistoryEvent,
  formatDateOnly,
  formatPeriodRange,
  parseDateOnly,
} from './periodClosingService';

export interface ClosingHistoryItem {
  id: string;
  code: string;
  description: string;
  entry_date: string;
  created_at?: string;
  total_debit?: number;
  total_credit?: number;
  net_income?: number;
  fiscal_year?: string;
  status?: string;
  source: 'period' | 'fiscal';
  source_label: string;
  is_closed?: boolean;
  can_reopen?: boolean;
  period_start_date?: string;
  period_end_date?: string;
  total_revenue?: number;
  total_expense?: number;
  transaction_count?: number;
  reopen_count?: number;
  notes?: string;
  history_events?: PeriodHistoryEvent[];
}

interface ClosingHistoryResponse {
  success: boolean;
  data: unknown[];
  error?: string;
}

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unknown error';
};

class ClosingHistoryService {
  private toIsoDate(value: unknown): string | null {
    if (!value) return null;

    if (typeof value === 'string') {
      if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
        return value;
      }

      const parsed = new Date(value);
      if (!Number.isNaN(parsed.getTime())) {
        return parsed.toISOString();
      }

      return null;
    }

    if (value instanceof Date && !Number.isNaN(value.getTime())) {
      return value.toISOString();
    }

    return null;
  }

  private toNumber(value: unknown): number | undefined {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === 'string') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }
    return undefined;
  }

  private isPeriodLikeEntry(item: unknown): item is ClosedPeriod & Record<string, unknown> {
    if (!item || typeof item !== 'object') {
      return false;
    }

    const candidate = item as Record<string, unknown>;
    return typeof candidate.start_date === 'string' && typeof candidate.end_date === 'string';
  }

  private mapPeriodHistoryItem(item: ClosedPeriod & Record<string, unknown>, source: 'period' | 'fiscal' = 'period'): ClosingHistoryItem {
    const entryDate =
      this.toIsoDate(item.closed_at) ||
      this.toIsoDate(item.reopened_at) ||
      this.toIsoDate(item.end_date) ||
      new Date().toISOString();

    return {
      id: `period:${item.id}`,
      code: typeof item.closing_journal_id === 'number' ? `PC-${item.closing_journal_id}` : `PC-${item.id}`,
      description: item.description || `Period closing ${formatPeriodRange(item.start_date, item.end_date)}`,
      entry_date: entryDate,
      created_at: this.toIsoDate(item.closed_at) || entryDate,
      total_debit: item.net_income,
      net_income: item.net_income,
      status: item.is_closed ? 'completed' : 'reopened',
      source,
      source_label: source === 'period' ? 'Period Closing' : 'Accounting Period',
      is_closed: item.is_closed,
      can_reopen: item.can_reopen,
      period_start_date: item.start_date,
      period_end_date: item.end_date,
      total_revenue: item.total_revenue,
      total_expense: item.total_expense,
      transaction_count: item.transaction_count,
      reopen_count: item.reopen_count,
      notes: item.notes,
      history_events: item.history_events,
    };
  }

  private mapLegacyHistoryItem(item: Record<string, unknown>): ClosingHistoryItem {
    const entryDate = this.toIsoDate(item.entry_date) || this.toIsoDate(item.created_at) || new Date().toISOString();
    const code = typeof item.code === 'string' && item.code ? item.code : `CLO-${item.id ?? entryDate}`;
    const description = typeof item.description === 'string' && item.description
      ? item.description
      : 'Fiscal Closing';

    return {
      id: `fiscal:${code}:${entryDate}`,
      code,
      description,
      entry_date: entryDate,
      created_at: this.toIsoDate(item.created_at) || entryDate,
      total_debit: this.toNumber(item.total_debit),
      total_credit: this.toNumber(item.total_credit),
      net_income: this.toNumber(item.net_income) ?? this.toNumber(item.total_debit),
      fiscal_year: typeof item.fiscal_year === 'string' ? item.fiscal_year : undefined,
      status: typeof item.status === 'string' ? item.status.toLowerCase() : 'completed',
      source: 'fiscal',
      source_label: 'Fiscal Closing',
      period_start_date: typeof item.start_date === 'string' ? item.start_date : undefined,
      period_end_date: typeof item.end_date === 'string' ? item.end_date : undefined,
    };
  }

  private normalizeHistoryItems(items: unknown[], source: 'period' | 'fiscal'): ClosingHistoryItem[] {
    return items
      .map((item) => {
        if (this.isPeriodLikeEntry(item)) {
          return this.mapPeriodHistoryItem(item, source === 'period' ? 'period' : 'fiscal');
        }

        if (item && typeof item === 'object') {
          return this.mapLegacyHistoryItem(item as Record<string, unknown>);
        }

        return null;
      })
      .filter((item): item is ClosingHistoryItem => item !== null);
  }

  /**
   * Get fiscal year closing history
   */
  async getFiscalClosingHistory(): Promise<ClosingHistoryItem[]> {
    try {
      const response = await api.get<ClosingHistoryResponse>(
        API_ENDPOINTS.FISCAL_CLOSING.HISTORY,
      );

      if (!response.data.success) {
        throw new Error(response.data.error || 'Failed to fetch closing history');
      }

      return this.normalizeHistoryItems(response.data.data || [], 'fiscal');
    } catch (error: unknown) {
      console.error('Error fetching fiscal closing history:', error);
      throw error;
    }
  }

  /**
   * Get canonical period closing history
   */
  async getPeriodClosingHistory(): Promise<ClosingHistoryItem[]> {
    try {
      const response = await api.get<ClosingHistoryResponse>(
        API_ENDPOINTS.PERIOD_CLOSING.HISTORY,
      );

      if (!response.data.success) {
        throw new Error(response.data.error || 'Failed to fetch period closing history');
      }

      return this.normalizeHistoryItems(response.data.data || [], 'period');
    } catch (error: unknown) {
      console.warn('Period closing history not available:', getErrorMessage(error));
      return [];
    }
  }

  /**
   * Get combined closing history (both fiscal and period) without duplicate period rows
   */
  async getAllClosingHistory(): Promise<ClosingHistoryItem[]> {
    try {
      const [fiscalHistory, periodHistory] = await Promise.allSettled([
        this.getFiscalClosingHistory(),
        this.getPeriodClosingHistory(),
      ]);

      const allHistory: ClosingHistoryItem[] = [];

      if (periodHistory.status === 'fulfilled') {
        allHistory.push(...periodHistory.value);
      }

      if (fiscalHistory.status === 'fulfilled') {
        allHistory.push(...fiscalHistory.value);
      }

      const uniqueHistory = new Map<string, ClosingHistoryItem>();
      for (const item of allHistory) {
        const key = item.period_start_date && item.period_end_date
          ? `${item.period_start_date}:${item.period_end_date}`
          : `${item.source}:${item.code}:${item.entry_date}`;

        if (!uniqueHistory.has(key) || item.source === 'period') {
          uniqueHistory.set(key, item);
        }
      }

      return Array.from(uniqueHistory.values()).sort((a, b) => {
        const dateA = parseDateOnly(a.entry_date)?.getTime() ?? new Date(a.entry_date).getTime();
        const dateB = parseDateOnly(b.entry_date)?.getTime() ?? new Date(b.entry_date).getTime();
        return dateB - dateA;
      });
    } catch (error: unknown) {
      console.error('Error fetching closing history:', error);
      return [];
    }
  }

  /**
   * Check if a specific date falls within a closed period
   */
  async isDateInClosedPeriod(date: string): Promise<boolean> {
    try {
      const response = await api.get<{ success: boolean; is_closed: boolean }>(
        API_ENDPOINTS.PERIOD_CLOSING.CHECK_DATE,
        { params: { date } },
      );

      return response.data.is_closed || false;
    } catch (error: unknown) {
      console.error('Error checking closed period:', error);
      return false;
    }
  }

  /**
   * Filter closing history by date range
   */
  filterHistoryByDateRange(
    history: ClosingHistoryItem[],
    startDate?: string,
    endDate?: string,
  ): ClosingHistoryItem[] {
    return history.filter((item) => {
      const dateOnly = parseDateOnly(item.entry_date);
      const itemDateValue = dateOnly ? dateOnly.getTime() : new Date(item.entry_date).getTime();
      if (Number.isNaN(itemDateValue)) {
        return false;
      }

      if (startDate) {
        const start = parseDateOnly(startDate)?.getTime() ?? new Date(`${startDate}T00:00:00`).getTime();
        if (itemDateValue < start) {
          return false;
        }
      }

      if (endDate) {
        const endDateOnly = parseDateOnly(endDate);
        const end = endDateOnly
          ? new Date(endDateOnly.getFullYear(), endDateOnly.getMonth(), endDateOnly.getDate(), 23, 59, 59).getTime()
          : new Date(`${endDate}T23:59:59`).getTime();
        if (itemDateValue > end) {
          return false;
        }
      }

      return true;
    });
  }

  /**
   * Format closing history for display
   */
  formatClosingHistory(item: ClosingHistoryItem): {
    date: string;
    period: string;
    netIncome: string;
    status: string;
    description: string;
    sourceLabel: string;
  } {
    const formattedDate = item.entry_date
      ? formatDateOnly(item.entry_date, 'id-ID', {
          year: 'numeric',
          month: 'long',
          day: 'numeric',
        })
      : '-';

    const period = item.period_start_date && item.period_end_date
      ? formatPeriodRange(item.period_start_date, item.period_end_date)
      : item.fiscal_year || formatDateOnly(item.entry_date);

    const netIncome = item.net_income !== undefined
      ? item.net_income
      : item.total_debit || 0;

    return {
      date: formattedDate,
      period,
      netIncome: this.formatCurrency(netIncome),
      status: item.status || 'completed',
      description: item.description || 'Closing',
      sourceLabel: item.source_label,
    };
  }

  /**
   * Format currency for display
   */
  private formatCurrency(amount: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  }
}

const closingHistoryService = new ClosingHistoryService();

export default closingHistoryService;

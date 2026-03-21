import { API_ENDPOINTS } from "@/config/api";
import { getAuthHeaders } from "@/utils/authTokenUtils";

export interface PeriodHistoryEvent {
  type: "CLOSED" | "REOPENED" | "SNAPSHOT" | "NOTE" | string;
  title: string;
  description: string;
  at?: string;
}

export interface ClosedPeriod {
  id: number;
  start_date: string;
  end_date: string;
  description: string;
  period_type: string;
  fiscal_year?: number;
  closed_at?: string;
  is_closed: boolean;
  is_locked: boolean;
  total_revenue: number;
  total_expense: number;
  net_income: number;
  account_count?: number;
  transaction_count?: number;
  reopen_count?: number;
  notes?: string;
  note_lines?: string[];
  status?: string;
  closed_by_name?: string;
  reopened_at?: string;
  reopened_by_name?: string;
  reopen_reason?: string;
  closing_journal_id?: number;
  reopening_journal_id?: number;
  snapshot_generated_at?: string;
  can_reopen?: boolean;
  effective_closed?: boolean;
  future_closed_pending?: boolean;
  history_events?: PeriodHistoryEvent[];
}

export interface PeriodFilterOption {
  value: string;
  label: string;
  period: ClosedPeriod;
  group: string;
}

export const parseDateOnly = (value?: string | null): Date | null => {
  if (!value) return null;

  const normalized = value.split("T")[0];
  const [year, month, day] = normalized.split("-").map(Number);
  if (!year || !month || !day) {
    return null;
  }

  return new Date(year, month - 1, day);
};

export const formatDateOnly = (
  value?: string | null,
  locale = "id-ID",
  options: Intl.DateTimeFormatOptions = {
    day: "2-digit",
    month: "short",
    year: "numeric",
  },
): string => {
  const parsed = parseDateOnly(value);
  if (!parsed) return "-";
  return parsed.toLocaleDateString(locale, options);
};

export const formatDateTime = (
  value?: string | null,
  locale = "id-ID",
  options: Intl.DateTimeFormatOptions = {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  },
): string => {
  if (!value) return "-";

  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return formatDateOnly(value, locale, {
      day: "2-digit",
      month: "short",
      year: "numeric",
    });
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return parsed.toLocaleString(locale, options);
};

export const formatPeriodRange = (
  startDate?: string | null,
  endDate?: string | null,
  locale = "id-ID",
): string => {
  const startLabel = formatDateOnly(startDate, locale, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
  const endLabel = formatDateOnly(endDate, locale, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
  return `${startLabel} - ${endLabel}`;
};

const isEffectiveClosedPeriod = (period: ClosedPeriod): boolean => {
  if (typeof period.effective_closed === "boolean") {
    return period.effective_closed;
  }

  if (!period.is_closed) return false;

  const endDate = parseDateOnly(period.end_date);
  if (!endDate) return false;

  const today = new Date();
  const todayStart = new Date(
    today.getFullYear(),
    today.getMonth(),
    today.getDate(),
  );
  const allowSameDayClosing =
    process.env.NEXT_PUBLIC_ALLOW_SAME_DAY_PERIOD_CLOSING === "true" ||
    process.env.NODE_ENV !== "production";

  return allowSameDayClosing
    ? endDate.getTime() <= todayStart.getTime()
    : endDate.getTime() < todayStart.getTime();
};

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

      const response = await fetch(
        `${API_ENDPOINTS.PERIOD_CLOSING.CHECK_DATE}?date=${date}`,
        {
          headers: this.getAuthHeaders(),
        },
      );

      if (!response.ok) {
        console.warn(
          "PeriodClosingService.isDateInClosedPeriod - non OK response",
          response.status,
        );
        return false;
      }

      const result = await response.json();
      if (!result.success) {
        console.warn(
          "PeriodClosingService.isDateInClosedPeriod - unsuccessful result",
          result,
        );
        return false;
      }

      return Boolean(result.is_closed);
    } catch (error) {
      console.error("PeriodClosingService.isDateInClosedPeriod - error", error);
      return false;
    }
  }

  async getClosingHistory(limit = 20): Promise<ClosedPeriod[]> {
    try {
      const response = await fetch(
        `${API_ENDPOINTS.PERIOD_CLOSING.HISTORY}?limit=${limit}`,
        {
          headers: this.getAuthHeaders(),
        },
      );

      if (!response.ok) {
        throw new Error("Failed to fetch period closing history");
      }

      const result = await response.json();
      if (!result.success || !Array.isArray(result.data)) {
        return [];
      }

      return result.data.filter((period: unknown) =>
        this.validatePeriod(period),
      );
    } catch (error) {
      console.error("Error fetching period closing history:", error);
      return [];
    }
  }

  /**
   * Get all closed periods for filtering
   */
  async getClosedPeriodsForFilter(): Promise<PeriodFilterOption[]> {
    try {
      const periods = await this.getClosingHistory(100);
      const closedPeriods = periods.filter((period) =>
        isEffectiveClosedPeriod(period),
      );
      return this.mapToFilterOptions(closedPeriods);
    } catch (error) {
      console.error("Error fetching closed periods:", error);
      return [];
    }
  }

  /**
   * Map closed periods to filter options
   */
  private mapToFilterOptions(periods: ClosedPeriod[]): PeriodFilterOption[] {
    return periods
      .sort((a, b) => {
        const left = parseDateOnly(b.end_date)?.getTime() ?? 0;
        const right = parseDateOnly(a.end_date)?.getTime() ?? 0;
        return left - right;
      })
      .map((period) => ({
        value: period.end_date,
        label: this.formatPeriodLabel(period),
        period,
        group: this.getPeriodGroup(period),
      }));
  }

  /**
   * Format period label for display
   * Example: "31 Des 2026 - Fiscal Year-End Closing 2026"
   */
  private formatPeriodLabel(period: ClosedPeriod): string {
    return `${formatDateOnly(period.end_date)} - ${period.description}`;
  }

  /**
   * Get period group for categorization
   */
  private getPeriodGroup(period: ClosedPeriod): string {
    const currentYear = new Date().getFullYear();
    const periodYear =
      parseDateOnly(period.end_date)?.getFullYear() ?? currentYear;

    if (periodYear === currentYear) {
      return "Current Year";
    }
    if (periodYear === currentYear - 1) {
      return "Last Year";
    }
    return `Year ${periodYear}`;
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

      if (
        result.success &&
        result.data?.has_previous_closing &&
        !result.data?.future_closed_pending &&
        !result.data?.requires_reclose
      ) {
        return {
          end_date: result.data.last_closing_date,
        } as ClosedPeriod;
      }

      return null;
    } catch (error) {
      console.error("Error fetching last closed period:", error);
      return null;
    }
  }

  /**
   * Validate period data structure
   */
  validatePeriod(period: unknown): period is ClosedPeriod {
    if (!period || typeof period !== "object") {
      return false;
    }

    const candidate = period as Partial<ClosedPeriod>;
    return (
      typeof candidate.id === "number" &&
      typeof candidate.start_date === "string" &&
      typeof candidate.end_date === "string" &&
      typeof candidate.description === "string" &&
      /^\d{4}-\d{2}-\d{2}$/.test(candidate.start_date) &&
      /^\d{4}-\d{2}-\d{2}$/.test(candidate.end_date)
    );
  }

  /**
   * Format currency for display
   */
  formatCurrency(amount: number): string {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  }
}

export const periodClosingService = new PeriodClosingService();
export default periodClosingService;

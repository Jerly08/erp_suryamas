import api from "./api";
import { API_ENDPOINTS } from "../config/api";
import { filterOperationalCashBankAccounts } from "@/utils/accountFilters";

// Types
export interface CashBank {
  id: number;
  code: string;
  name: string;
  type: "CASH" | "BANK";
  account_id: number;
  bank_name?: string;
  account_no?: string;
  account_holder_name?: string;
  branch?: string;
  currency: string;
  balance: number;
  book_balance_idr?: number;
  last_exchange_rate?: number;
  min_balance: number;
  max_balance: number;
  daily_limit: number;
  monthly_limit: number;
  is_active: boolean;
  is_restricted: boolean;
  is_holding_account?: boolean;
  allow_direct_transactions?: boolean;
  frozen_amount: number;
  frozen_reason?: string;
  frozen_at?: string;
  frozen_by?: number;
  user_id: number;
  description?: string;
  created_at: string;
  updated_at: string;
  account?: {
    id: number;
    code: string;
    name: string;
    is_active?: boolean;
    is_header?: boolean;
  };
}

// Computed helper for available balance
export function getAvailableBalance(account: CashBank): number {
  const available = account.balance - (account.frozen_amount || 0);
  return available > 0 ? available : 0;
}

export function getBookBalanceInIDR(account: CashBank): number {
  if ((account.currency || "IDR").toUpperCase() === "IDR") {
    return account.balance || 0;
  }

  if (typeof account.book_balance_idr === "number") {
    return account.book_balance_idr;
  }

  const rate =
    account.last_exchange_rate && account.last_exchange_rate > 0
      ? account.last_exchange_rate
      : 1;
  return (account.balance || 0) * rate;
}

export function formatCashBankAmount(
  account: CashBank,
  amount: number,
): string {
  return `${(account.currency || "IDR").toUpperCase()} ${Math.abs(amount || 0).toLocaleString("id-ID")}`;
}

export interface CashBankTransaction {
  id: number;
  cash_bank_id: number;
  reference_type: string;
  reference_id: number;
  amount: number;
  currency?: string;
  exchange_rate?: number;
  book_amount_idr?: number;
  balance_after: number;
  transaction_date: string;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface BalanceSummary {
  total_cash: number;
  total_bank: number;
  total_balance: number;
  by_account: AccountBalance[];
  by_currency: Record<string, number>;
}

export interface AccountBalance {
  account_id: number;
  account_name: string;
  account_type: string;
  balance: number;
  currency: string;
}

export interface CashBankCreateRequest {
  name: string;
  type: "CASH" | "BANK";
  account_id?: number;
  bank_name?: string;
  account_no?: string;
  account_holder_name?: string;
  branch?: string;
  currency?: string;
  opening_balance?: number;
  opening_exchange_rate?: number;
  opening_date?: string;
  description?: string;
  is_holding_account?: boolean;
  allow_direct_transactions?: boolean;
  source_account_id?: number;
}

export interface CashBankUpdateRequest {
  name?: string;
  bank_name?: string;
  account_no?: string;
  account_holder_name?: string;
  branch?: string;
  description?: string;
  is_active?: boolean;
  is_holding_account?: boolean;
  allow_direct_transactions?: boolean;
}

export interface TransferRequest {
  from_account_id: number;
  to_account_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  reference?: string;
  notes?: string;
}

export interface DepositRequest {
  account_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  reference?: string;
  notes?: string;
  source_account_id?: number;
}

export interface WithdrawalRequest {
  account_id: number;
  date: string;
  amount: number;
  reference?: string;
  notes?: string;
  // Optional: expense/target GL account for automatic journal integration
  target_account_id?: number;
}

export interface TransactionFilter {
  page?: number;
  limit?: number;
  start_date?: string;
  end_date?: string;
  sort_order?: "asc" | "desc";
}

export interface TransactionResult {
  data: CashBankTransaction[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface ReconciliationRequest {
  date: string;
  statement_balance: number;
  items: ReconciliationItemRequest[];
}

export interface ReconciliationItemRequest {
  transaction_id: number;
  is_cleared: boolean;
  notes?: string;
}

export interface BankReconciliation {
  id: number;
  cash_bank_id: number;
  reconcile_date: string;
  statement_balance: number;
  system_balance: number;
  difference: number;
  status: string;
  user_id: number;
  created_at: string;
  updated_at: string;
}

export interface ReferenceExchangeRate {
  base_currency: string;
  target_currency: string;
  exchange_rate: number;
  source: string;
  provider: string;
  fetched_at: string;
  cached_until: string;
  message?: string;
}

interface ApiServiceErrorShape {
  response?: {
    status?: number;
  };
  code?: string;
}

export interface CashBankCounterpartyAccount {
  id: number;
  code: string;
  name: string;
  type: string;
  category?: string;
  is_active?: boolean;
  is_header?: boolean;
  balance: number;
}

export interface CashBankTransferActor {
  id: number;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  role?: string;
}

export interface CashBankTransfer {
  id: number;
  transfer_number: string;
  from_account_id: number;
  to_account_id: number;
  date: string;
  amount: number;
  exchange_rate: number;
  converted_amount: number;
  source_book_amount_idr?: number;
  destination_book_amount_idr?: number;
  forex_gain_loss_amount?: number;
  forex_gain_loss_type?: string;
  reference?: string;
  notes?: string;
  status: string;
  user_id: number;
  approval_required?: boolean;
  approved_by?: number | null;
  approved_at?: string | null;
  rejected_by?: number | null;
  rejected_at?: string | null;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
  from_account?: CashBank;
  to_account?: CashBank;
  requester?: CashBankTransferActor;
  approver?: CashBankTransferActor | null;
  rejector?: CashBankTransferActor | null;
}

export interface TransferOperationResponse {
  transfer: CashBankTransfer;
  message: string;
}

class CashBankService {
  // Use API_ENDPOINTS instead of hardcoded baseUrl

  private normalizeAccountList(data: unknown): CashBankCounterpartyAccount[] {
    const responseData =
      typeof data === "object" && data !== null && "data" in data
        ? (data as { data?: unknown }).data
        : undefined;
    const accounts = Array.isArray(responseData)
      ? responseData
      : Array.isArray(data)
        ? data
        : [];
    return accounts.filter(
      (account: CashBankCounterpartyAccount) => account && account.id,
    );
  }

  private isActiveLeafAccount(account: CashBankCounterpartyAccount): boolean {
    return account.is_active !== false && account.is_header !== true;
  }

  private isNonOperatingRevenueAccount(
    account: CashBankCounterpartyAccount,
  ): boolean {
    const code = (account.code || "").toUpperCase();
    const category = (account.category || "").toUpperCase();
    return (
      code.startsWith("7") ||
      [
        "NON_OPERATING_REVENUE",
        "INTEREST_INCOME",
        "DIVIDEND_INCOME",
        "GAIN_ON_SALE",
        "OTHER_INCOME",
      ].includes(category)
    );
  }

  private isOperatingExpenseAccount(
    account: CashBankCounterpartyAccount,
  ): boolean {
    const category = (account.category || "").toUpperCase();
    return [
      "OPERATING_EXPENSE",
      "ADMINISTRATIVE_EXPENSE",
      "SELLING_EXPENSE",
      "MARKETING_EXPENSE",
      "GENERAL_EXPENSE",
      "DEPRECIATION_EXPENSE",
      "AMORTIZATION_EXPENSE",
      "BAD_DEBT_EXPENSE",
    ].includes(category);
  }

  private isNonOperatingExpenseAccount(
    account: CashBankCounterpartyAccount,
  ): boolean {
    const code = (account.code || "").toUpperCase();
    const category = (account.category || "").toUpperCase();
    return (
      code.startsWith("8") ||
      [
        "NON_OPERATING_EXPENSE",
        "INTEREST_EXPENSE",
        "FINANCIAL_EXPENSE",
        "LOSS_ON_SALE",
        "OTHER_EXPENSE",
      ].includes(category)
    );
  }

  // Get all cash and bank accounts
  async getCashBankAccounts(options?: {
    includeHolding?: boolean;
  }): Promise<CashBank[]> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.ACCOUNTS, {
        params: {
          _t: Date.now(),
          ...(options?.includeHolding ? { include_holding: true } : {}),
        },
      });
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error fetching cash bank accounts:", error);
      throw error;
    }
  }

  // Get account by ID
  async getCashBankById(id: number): Promise<CashBank> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.GET_BY_ID(id), {
        params: { _t: Date.now() },
      });
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error fetching cash bank account:", error);
      throw error;
    }
  }

  // Create new cash/bank account
  async createCashBankAccount(data: CashBankCreateRequest): Promise<CashBank> {
    try {
      const response = await api.post(API_ENDPOINTS.CASH_BANK.CREATE, data);
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error creating cash bank account:", error);
      throw error;
    }
  }

  // Update cash/bank account
  async updateCashBankAccount(
    id: number,
    data: CashBankUpdateRequest,
  ): Promise<CashBank> {
    try {
      const response = await api.put(API_ENDPOINTS.CASH_BANK.UPDATE(id), data);
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error updating cash bank account:", error);
      throw error;
    }
  }

  // Delete cash/bank account
  async deleteCashBankAccount(id: number): Promise<void> {
    try {
      await api.delete(API_ENDPOINTS.CASH_BANK.DELETE(id));
    } catch (error) {
      console.error("Error deleting cash bank account:", error);
      throw error;
    }
  }

  // Process transfer
  async processTransfer(
    data: TransferRequest,
  ): Promise<TransferOperationResponse> {
    try {
      const response = await api.post(API_ENDPOINTS.CASH_BANK.TRANSFER, data);
      return response.data?.data ?? response.data;
    } catch (error) {
      const normalizedError = error as ApiServiceErrorShape;
      const status = normalizedError?.response?.status;
      if (!status || status >= 500) {
        console.error("Error processing transfer:", error);
      }
      throw error;
    }
  }

  // Process deposit
  async processDeposit(data: DepositRequest): Promise<CashBankTransaction> {
    try {
      const response = await api.post(API_ENDPOINTS.CASH_BANK.DEPOSIT, data, {
        timeout: 60000, // 60 seconds timeout for deposit operations
      });
      // Handler returns { status, message, data: { transaction, message } }
      return (
        response.data?.data?.transaction ?? response.data?.data ?? response.data
      );
    } catch (error) {
      console.error("Error processing deposit:", error);
      throw error;
    }
  }

  // Process withdrawal
  async processWithdrawal(
    data: WithdrawalRequest,
  ): Promise<CashBankTransaction> {
    try {
      const response = await api.post(
        API_ENDPOINTS.CASH_BANK.WITHDRAWAL,
        data,
        {
          timeout: 60000, // 60 seconds timeout for withdrawal operations
        },
      );
      return (
        response.data?.data?.transaction ?? response.data?.data ?? response.data
      );
    } catch (error) {
      console.error("Error processing withdrawal:", error);
      throw error;
    }
  }

  // Get transactions for account
  async getTransactions(
    accountId: number,
    filter: TransactionFilter = {},
  ): Promise<TransactionResult> {
    try {
      const params: Record<string, string | number> = { _t: Date.now() };
      if (filter.page) params.page = filter.page;
      if (filter.limit) params.limit = filter.limit;
      if (filter.start_date) params.start_date = filter.start_date;
      if (filter.end_date) params.end_date = filter.end_date;
      if (filter.sort_order) params.sort_order = filter.sort_order;

      const response = await api.get(
        API_ENDPOINTS.CASH_BANK.TRANSACTIONS(accountId),
        {
          params,
        },
      );
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error fetching transactions:", error);
      throw error;
    }
  }

  // Get balance summary
  async getBalanceSummary(): Promise<BalanceSummary> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.BALANCE_SUMMARY, {
        params: { _t: Date.now() },
      });
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error fetching balance summary:", error);
      throw error;
    }
  }

  // Get payment accounts (for dropdowns)
  async getPaymentAccounts(): Promise<CashBank[]> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.PAYMENT_ACCOUNTS, {
        params: { _t: Date.now() },
      });
      return filterOperationalCashBankAccounts(response.data.data || response.data || []);
    } catch (error) {
      console.error("Error fetching payment accounts:", error);
      throw error;
    }
  }

  // Get transaction history for an account with filtering
  async getTransactionHistory(
    accountId: number,
    filter: TransactionFilter = {},
  ): Promise<TransactionResult> {
    try {
      const params: Record<string, string | number> = { _t: Date.now() };
      if (filter.page) params.page = filter.page;
      if (filter.limit) params.limit = filter.limit;
      if (filter.start_date) params.start_date = filter.start_date;
      if (filter.end_date) params.end_date = filter.end_date;

      const response = await api.get(
        API_ENDPOINTS.CASH_BANK.TRANSACTIONS(accountId),
        {
          params,
        },
      );
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error(
        `Error fetching transaction history for account ${accountId}:`,
        error,
      );
      throw error;
    }
  }

  // Reconcile bank account
  async reconcileAccount(
    accountId: number,
    data: ReconciliationRequest,
  ): Promise<BankReconciliation> {
    try {
      const response = await api.post(
        API_ENDPOINTS.CASH_BANK.RECONCILE(accountId),
        data,
      );
      return response.data;
    } catch (error) {
      console.error("Error reconciling account:", error);
      throw error;
    }
  }

  // Check GL account links status
  async checkGLAccountLinks(): Promise<unknown> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.CHECK_GL_LINKS, {
        params: { _t: Date.now() },
      });
      return response.data;
    } catch (error) {
      console.error("Error checking GL account links:", error);
      throw error;
    }
  }

  async getTransfers(params?: {
    status?: string;
    limit?: number;
  }): Promise<CashBankTransfer[]> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.TRANSFERS, {
        params: {
          _t: Date.now(),
          ...(params?.status ? { status: params.status } : {}),
          ...(params?.limit ? { limit: params.limit } : {}),
        },
      });
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error fetching transfer requests:", error);
      throw error;
    }
  }

  async approveTransfer(id: number): Promise<TransferOperationResponse> {
    try {
      const response = await api.post(
        API_ENDPOINTS.CASH_BANK.APPROVE_TRANSFER(id),
      );
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error approving transfer request:", error);
      throw error;
    }
  }

  async rejectTransfer(
    id: number,
    reason?: string,
  ): Promise<TransferOperationResponse> {
    try {
      const response = await api.post(
        API_ENDPOINTS.CASH_BANK.REJECT_TRANSFER(id),
        {
          reason: reason || "",
        },
      );
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error rejecting transfer request:", error);
      throw error;
    }
  }

  async fixGLAccountLinks(): Promise<{
    fixed_count: number;
    message?: string;
  }> {
    const result = await this.checkGLAccountLinks();
    return (result ?? { fixed_count: 0 }) as {
      fixed_count: number;
      message?: string;
    };
  }

  // Get revenue accounts for deposit form
  async getRevenueAccounts(): Promise<CashBankCounterpartyAccount[]> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.REVENUE_ACCOUNTS, {
        params: { _t: Date.now() },
      });
      return this.normalizeAccountList(response.data);
    } catch (error) {
      console.error("Error fetching revenue accounts:", error);
      throw error;
    }
  }

  // Get deposit source accounts for deposit form
  async getDepositSourceAccounts(): Promise<{
    revenue: CashBankCounterpartyAccount[];
    equity: CashBankCounterpartyAccount[];
    liability: CashBankCounterpartyAccount[];
  }> {
    try {
      const [revenueResponse, equityResponse, liabilityResponse] =
        await Promise.all([
          api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
            params: { type: "REVENUE", _t: Date.now() },
          }),
          api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
            params: { type: "EQUITY", _t: Date.now() },
          }),
          api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
            params: { type: "LIABILITY", _t: Date.now() },
          }),
        ]);

      const revenueAccounts = this.normalizeAccountList(
        revenueResponse.data,
      ).filter(
        (account) =>
          this.isActiveLeafAccount(account) &&
          this.isNonOperatingRevenueAccount(account),
      );
      const equityAccounts = this.normalizeAccountList(
        equityResponse.data,
      ).filter((account) => this.isActiveLeafAccount(account));
      const liabilityAccounts = this.normalizeAccountList(
        liabilityResponse.data,
      ).filter((account) => this.isActiveLeafAccount(account));

      return {
        revenue: revenueAccounts,
        equity: equityAccounts,
        liability: liabilityAccounts,
      };
    } catch (error) {
      console.error("Error fetching deposit source accounts:", error);
      throw error;
    }
  }

  async getWithdrawalTargetAccounts(): Promise<{
    operatingExpense: CashBankCounterpartyAccount[];
    nonOperatingExpense: CashBankCounterpartyAccount[];
    liability: CashBankCounterpartyAccount[];
    asset: CashBankCounterpartyAccount[];
  }> {
    try {
      const [
        expenseResponse,
        liabilityResponse,
        assetResponse,
        cashBankAccounts,
      ] = await Promise.all([
        api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
          params: { type: "EXPENSE", _t: Date.now() },
        }),
        api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
          params: { type: "LIABILITY", _t: Date.now() },
        }),
        api.get(API_ENDPOINTS.ACCOUNTS.LIST, {
          params: { type: "ASSET", _t: Date.now() },
        }),
        this.getCashBankAccounts({ includeHolding: true }),
      ]);

      const expenseAccounts = this.normalizeAccountList(
        expenseResponse.data,
      ).filter((account) => this.isActiveLeafAccount(account));
      const cashBankLinkedAccountIDs = new Set(
        cashBankAccounts
          .map((account) => account.account_id)
          .filter(
            (accountID): accountID is number =>
              typeof accountID === "number" && accountID > 0,
          ),
      );

      return {
        operatingExpense: expenseAccounts.filter((account) =>
          this.isOperatingExpenseAccount(account),
        ),
        nonOperatingExpense: expenseAccounts.filter((account) =>
          this.isNonOperatingExpenseAccount(account),
        ),
        liability: this.normalizeAccountList(liabilityResponse.data).filter(
          (account) => this.isActiveLeafAccount(account),
        ),
        asset: this.normalizeAccountList(assetResponse.data).filter(
          (account) =>
            this.isActiveLeafAccount(account) &&
            !cashBankLinkedAccountIDs.has(account.id),
        ),
      };
    } catch (error) {
      console.error("Error fetching withdrawal target accounts:", error);
      throw error;
    }
  }

  async getReferenceExchangeRate(
    base = "USD",
    target = "IDR",
  ): Promise<ReferenceExchangeRate | null> {
    try {
      const response = await api.get(API_ENDPOINTS.CASH_BANK.REFERENCE_RATE, {
        params: {
          base,
          target,
          _t: Date.now(),
        },
      });
      return response.data?.data ?? response.data;
    } catch (error) {
      const normalizedError = error as ApiServiceErrorShape;
      const status = normalizedError.response?.status || 0;
      const isTransientFailure =
        status >= 500 ||
        normalizedError.code === "ERR_NETWORK" ||
        normalizedError.code === "ECONNABORTED";
      if (isTransientFailure) {
        return null;
      }
      throw error;
    }
  }

  // Get available GL accounts for cash/bank creation
  // Returns GL accounts that are:
  // 1. Not already linked to any cash/bank account
  // 2. Filtered by type (CASH shows accounts with "KAS" in name, BANK shows accounts with "BANK" in name)
  async getAvailableGLAccounts(
    type: "CASH" | "BANK",
  ): Promise<AvailableGLAccount[]> {
    try {
      const response = await api.get(
        "/api/v1/cash-bank/available-gl-accounts",
        {
          params: { type, _t: Date.now() },
        },
      );
      // Backend returns { status, message, data } where data is the array
      const result = response.data?.data ?? response.data ?? [];
      // Ensure we always return an array
      return Array.isArray(result) ? result : [];
    } catch (error) {
      console.error(
        `Error fetching available GL accounts for type ${type}:`,
        error,
      );
      throw error;
    }
  }

  // Freeze funds in a cash/bank account
  async freezeAmount(id: number, data: FreezeAmountRequest): Promise<CashBank> {
    try {
      const response = await api.post(API_ENDPOINTS.CASH_BANK.FREEZE(id), data);
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error freezing funds:", error);
      throw error;
    }
  }

  // Unfreeze funds in a cash/bank account
  async unfreezeAmount(
    id: number,
    data: UnfreezeAmountRequest,
  ): Promise<CashBank> {
    try {
      const response = await api.post(
        API_ENDPOINTS.CASH_BANK.UNFREEZE(id),
        data,
      );
      return response.data?.data ?? response.data;
    } catch (error) {
      console.error("Error unfreezing funds:", error);
      throw error;
    }
  }

  // Get frozen history for an account
  async getFrozenHistory(id: number): Promise<FrozenHistory[]> {
    try {
      const response = await api.get(
        API_ENDPOINTS.CASH_BANK.FROZEN_HISTORY(id),
        {
          params: { _t: Date.now() },
        },
      );
      return response.data?.data ?? response.data ?? [];
    } catch (error: unknown) {
      const normalizedError = error as { response?: { status?: number } };
      // Silently return empty array if endpoint not available (e.g. backend not restarted yet)
      if (normalizedError.response?.status === 404) {
        return [];
      }
      console.error("Error fetching frozen history:", error);
      throw error;
    }
  }
}

// Type for available GL accounts
export interface AvailableGLAccount {
  id: number;
  code: string;
  name: string;
  balance: number;
  category: string;
}

// Frozen fund request/response types
export interface FreezeAmountRequest {
  amount: number;
  reason: string;
}

export interface UnfreezeAmountRequest {
  amount: number;
  reason: string;
}

export interface FrozenHistory {
  id: number;
  cash_bank_id: number;
  action: "FREEZE" | "UNFREEZE";
  amount: number;
  previous_frozen: number;
  new_frozen: number;
  reason: string;
  user_id: number;
  created_at: string;
}

const cashBankService = new CashBankService();

export default cashBankService;

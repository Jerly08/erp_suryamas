type MaybeLinkedAccount = {
  active?: boolean;
  is_active?: boolean;
  is_header?: boolean;
  isHeader?: boolean;
};

type AccountLike = MaybeLinkedAccount & {
  id?: number;
  account?: MaybeLinkedAccount | null;
};

type CashBankCurrencyLike = AccountLike & {
  currency?: string | null;
};

const OPERATIONAL_CASH_BANK_CURRENCIES = new Set(["IDR", "USD"]);

const isTruthyHeader = (value: unknown): boolean => value === true;

export const isOperationalCashBankCurrency = (
  currency?: string | null,
): boolean => {
  const normalizedCurrency = (currency || "IDR").trim().toUpperCase();
  return OPERATIONAL_CASH_BANK_CURRENCIES.has(normalizedCurrency);
};

export const isActiveLeafAccount = <T extends AccountLike>(account: T): boolean => {
  if (!account || !account.id) {
    return false;
  }

  const active = account.active ?? account.is_active ?? true;
  const isHeader = isTruthyHeader(account.is_header ?? account.isHeader);
  const linkedActive = account.account
    ? account.account.active ?? account.account.is_active ?? true
    : true;
  const linkedIsHeader = account.account
    ? isTruthyHeader(account.account.is_header ?? account.account.isHeader)
    : false;

  return active !== false && linkedActive !== false && !isHeader && !linkedIsHeader;
};

export const filterActiveLeafAccounts = <T extends AccountLike>(accounts: T[] = []): T[] =>
  accounts.filter(isActiveLeafAccount);

export const filterOperationalCashBankAccounts = <
  T extends CashBankCurrencyLike,
>(
  accounts: T[] = [],
): T[] =>
  filterActiveLeafAccounts(accounts).filter((account) =>
    isOperationalCashBankCurrency(account.currency),
  );

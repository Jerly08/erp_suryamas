export const CASH_BANK_CURRENCY_OPTIONS = [
  { code: 'IDR', label: 'IDR - Indonesian Rupiah' },
  { code: 'USD', label: 'USD - US Dollar' },
  { code: 'CNY', label: 'CNY - Chinese Yuan (RMB)' },
] as const;

export const normalizeCashBankCurrency = (value?: string): string => {
  const normalized = (value || 'IDR').trim().toUpperCase();
  return normalized || 'IDR';
};

export const isCashBankForeignCurrency = (value?: string): boolean => normalizeCashBankCurrency(value) !== 'IDR';

export const getCashBankAmountPrecision = (value?: string): number => (isCashBankForeignCurrency(value) ? 2 : 0);

export const supportsIdrForeignTransferPair = (sourceCurrency: string, destinationCurrency: string): boolean => {
  const source = normalizeCashBankCurrency(sourceCurrency);
  const destination = normalizeCashBankCurrency(destinationCurrency);

  if (source === destination) {
    return true;
  }

  return (source === 'IDR' && destination !== 'IDR') || (source !== 'IDR' && destination === 'IDR');
};

export const getForeignCurrencyInTransferPair = (sourceCurrency: string, destinationCurrency: string): string | null => {
  const source = normalizeCashBankCurrency(sourceCurrency);
  const destination = normalizeCashBankCurrency(destinationCurrency);

  if (source !== 'IDR') {
    return source;
  }

  if (destination !== 'IDR') {
    return destination;
  }

  return null;
};

import { Account } from '@/types/account';

const EMPHASIZED_REVENUE_ACCOUNT_CODES = new Set([
  '4101-003',
  '4101-004',
  '4101-005',
]);

// These liability groups remain postable accounts, but should stand out alongside
// the other primary short-term liability groups in the COA views.
const EMPHASIZED_LIABILITY_ACCOUNT_CODES = new Set([
  '2104',
  '2105',
]);

export function shouldEmphasizeAccountRow(
  account: Pick<Account, 'code' | 'is_header' | 'type'>,
): boolean {
  return (
    account.is_header ||
    (account.type === 'REVENUE' && EMPHASIZED_REVENUE_ACCOUNT_CODES.has(account.code)) ||
    (account.type === 'LIABILITY' && EMPHASIZED_LIABILITY_ACCOUNT_CODES.has(account.code))
  );
}

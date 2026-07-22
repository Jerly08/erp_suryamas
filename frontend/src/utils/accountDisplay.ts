import { Account } from '@/types/account';

const EMPHASIZED_REVENUE_ACCOUNT_CODES = new Set([
  '4101-003',
  '4101-004',
  '4101-005',
]);

export function shouldEmphasizeAccountRow(
  account: Pick<Account, 'code' | 'is_header' | 'type'>,
): boolean {
  return (
    account.is_header ||
    (account.type === 'REVENUE' && EMPHASIZED_REVENUE_ACCOUNT_CODES.has(account.code))
  );
}

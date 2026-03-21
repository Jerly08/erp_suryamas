import { normalizeRole, humanizeRole } from './roles';

export interface UserDisplaySource {
  id?: string | number | null;
  name?: string | null;
  full_name?: string | null;
  username?: string | null;
  first_name?: string | null;
  last_name?: string | null;
  role?: string | null;
  email?: string | null;
  company_type?: string | null;
  active?: boolean;
  is_active?: boolean;
}

interface DisplayNameOptions {
  fallback?: string;
  allowRoleFallback?: boolean;
}

const cleanText = (value?: string | number | null): string => {
  if (typeof value === 'number') return value.toString();
  if (typeof value !== 'string') return '';
  return value.trim();
};

export const getUserDisplayName = (
  user?: UserDisplaySource | null,
  options: DisplayNameOptions = {},
): string => {
  const {
    fallback = 'Unknown User',
    allowRoleFallback = true,
  } = options;

  if (!user) return fallback;

  const fullName = [cleanText(user.first_name), cleanText(user.last_name)]
    .filter(Boolean)
    .join(' ')
    .trim();

  if (fullName) return fullName;

  const directName = cleanText(user.full_name) || cleanText(user.name);
  if (directName) return directName;

  const username = cleanText(user.username);
  if (username) return username;

  if (allowRoleFallback) {
    const role = cleanText(user.role);
    if (role) return humanizeRole(role);
  }

  return fallback;
};

export const normalizeAuthUser = <T extends UserDisplaySource>(
  user?: T | null,
  companyType?: string,
): (T & {
  id: string;
  role: string;
  name: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  company_type?: string;
  active: boolean;
}) | null => {
  if (!user) return null;

  const normalizedRole = normalizeRole(cleanText(user.role));
  const displayName = getUserDisplayName(user, {
    fallback: '',
    allowRoleFallback: false,
  });
  const username = cleanText(user.username) || undefined;
  const firstName = cleanText(user.first_name) || undefined;
  const lastName = cleanText(user.last_name) || undefined;
  const active =
    typeof user.active === 'boolean'
      ? user.active
      : typeof user.is_active === 'boolean'
        ? user.is_active
        : true;

  return {
    ...user,
    id: cleanText(user.id),
    role: normalizedRole || cleanText(user.role),
    name: displayName,
    username,
    first_name: firstName,
    last_name: lastName,
    company_type: cleanText(companyType) || cleanText(user.company_type) || undefined,
    active,
  };
};

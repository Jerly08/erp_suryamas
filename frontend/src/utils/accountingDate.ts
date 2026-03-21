const DATE_ONLY_REGEX = /^(\d{4})-(\d{2})-(\d{2})/;

const parseAccountingDate = (value: string): Date | null => {
  if (!value) return null;

  const match = value.match(DATE_ONLY_REGEX);
  if (!match) {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  }

  const [, year, month, day] = match;
  return new Date(Number(year), Number(month) - 1, Number(day));
};

export const formatAccountingDate = (
  value: string,
  locale = "id-ID",
  options: Intl.DateTimeFormatOptions = {
    day: "2-digit",
    month: "short",
    year: "numeric",
  },
): string => {
  const parsed = parseAccountingDate(value);
  if (!parsed) return "-";
  return parsed.toLocaleDateString(locale, options);
};

export const formatAccountingDateShort = (value: string): string =>
  formatAccountingDate(value, "id-ID", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });

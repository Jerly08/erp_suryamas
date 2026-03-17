export const roundUsdIdrRate = (rate?: number | null, fallback = 0): number => {
  const numericRate = Number(rate);

  if (!Number.isFinite(numericRate) || numericRate <= 0) {
    return fallback;
  }

  return Math.round(numericRate);
};

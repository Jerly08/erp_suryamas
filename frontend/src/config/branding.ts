export const DEFAULT_APP_NAME =
  process.env.NEXT_PUBLIC_DEFAULT_APP_NAME?.trim() || "Finance Management";
export const DEFAULT_LOGO_PATH = "/api/v1/branding/logo";
export const DISABLED_BROWSER_ICON_PATH = "/favicon.svg?v=no-avery";
export const DEFAULT_ICON_PATH = DISABLED_BROWSER_ICON_PATH;
export const DEFAULT_LOGO_SCALE = 100;
export const MIN_LOGO_SCALE = 50;
export const MAX_LOGO_SCALE = 180;
export const POWERED_BY_TEXT = "Powered by PT SURYA HEBAT BERBAKAT";

export const normalizeLogoScale = (value: unknown): number => {
  const scale = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(scale) || scale <= 0) return DEFAULT_LOGO_SCALE;
  return Math.min(MAX_LOGO_SCALE, Math.max(MIN_LOGO_SCALE, Math.round(scale)));
};

export const getScaledLogoSize = (basePixels: number, scale: number): string =>
  `${Math.round((basePixels * normalizeLogoScale(scale)) / 100)}px`;

export const DEFAULT_APP_NAME = "Avery";
export const DEFAULT_LOGO_PATH = "/logo_avery.png";
export const DEFAULT_ICON_PATH = "/avery-icon.png";
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

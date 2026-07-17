export const DEFAULT_APP_NAME =
  process.env.NEXT_PUBLIC_DEFAULT_APP_NAME?.trim() || "Finance Management";
export const DEFAULT_LOGO_PATH = "/api/v1/branding/logo";
export const FALLBACK_ICON_PATH = "/favicon.svg?v=no-branding";
export const HEAD_ICON_PATH = "/api/v1/branding/icon?size=192&source=head-static&v=no-avery-20260711";
export const HEAD_APPLE_TOUCH_ICON_PATH = "/api/v1/branding/icon?size=180&source=head-static&v=no-avery-20260711";
export const DEFAULT_ICON_PATH = "/api/v1/branding/icon?size=192&source=runtime-branding";
export const APPLE_TOUCH_ICON_PATH = "/api/v1/branding/icon?size=180&source=runtime-branding";
export const DEFAULT_LOGO_SCALE = 100;
export const MIN_LOGO_SCALE = 50;
export const MAX_LOGO_SCALE = 180;
export const DEFAULT_POWERED_BY_TEXT = "Powered by";

export const normalizeLogoScale = (value: unknown): number => {
  const scale = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(scale) || scale <= 0) return DEFAULT_LOGO_SCALE;
  return Math.min(MAX_LOGO_SCALE, Math.max(MIN_LOGO_SCALE, Math.round(scale)));
};

export const getScaledLogoSize = (basePixels: number, scale: number): string =>
  `${Math.round((basePixels * normalizeLogoScale(scale)) / 100)}px`;

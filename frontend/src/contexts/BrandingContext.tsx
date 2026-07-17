'use client';

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { API_ENDPOINTS } from '@/config/api';
import {
  APPLE_TOUCH_ICON_PATH,
  DEFAULT_APP_NAME,
  DEFAULT_ICON_PATH,
  FALLBACK_ICON_PATH,
  DEFAULT_LOGO_PATH,
  DEFAULT_LOGO_SCALE,
  DEFAULT_POWERED_BY_TEXT,
  normalizeLogoScale,
} from '@/config/branding';
import { getImageUrl } from '@/utils/imageUrl';

interface PublicBrandingResponse {
  success?: boolean;
  data?: {
    company_name?: string;
    company_logo?: string;
    company_logo_scale?: number;
    company_icon?: string;
    powered_by_text?: string;
    updated_at?: string;
  };
}

interface BrandingContextValue {
  companyName: string;
  logoUrl: string;
  logoScale: number;
  iconUrl: string;
  poweredByText: string;
  refreshBranding: () => Promise<void>;
}

interface BrandingState {
  companyName: string;
  logoUrl: string;
  logoScale: number;
  iconUrl: string;
  poweredByText: string;
  updatedAt: string;
}

const defaultBranding: BrandingState = {
  companyName: DEFAULT_APP_NAME,
  logoUrl: DEFAULT_LOGO_PATH,
  logoScale: DEFAULT_LOGO_SCALE,
  iconUrl: DEFAULT_ICON_PATH,
  poweredByText: DEFAULT_POWERED_BY_TEXT,
  updatedAt: '',
};

const BrandingContext = createContext<BrandingContextValue | undefined>(undefined);

const appendVersion = (url: string, version?: string): string => {
  if (!version) return url;
  return `${url}${url.includes('?') ? '&' : '?'}v=${encodeURIComponent(version)}`;
};

const setNamedMetaContent = (name: string, content: string) => {
  let meta = document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`);
  if (!meta) {
    meta = document.createElement('meta');
    meta.name = name;
    document.head.appendChild(meta);
  }
  meta.content = content;
};

const getIconType = (url: string): string | undefined => {
  const cleanPath = url.split('?')[0].toLowerCase();
  if (cleanPath === '/api/v1/branding/icon') return 'image/png';
  if (cleanPath.endsWith('.svg')) return 'image/svg+xml';
  if (cleanPath.endsWith('.png')) return 'image/png';
  if (cleanPath.endsWith('.jpg') || cleanPath.endsWith('.jpeg')) return 'image/jpeg';
  if (cleanPath.endsWith('.webp')) return 'image/webp';
  return undefined;
};

export const BrandingProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [branding, setBranding] = useState<BrandingState>(defaultBranding);

  const refreshBranding = useCallback(async () => {
    try {
      const response = await fetch(API_ENDPOINTS.BRANDING_PUBLIC, {
        cache: 'no-store',
        credentials: 'omit',
      });

      if (!response.ok) {
        throw new Error(`Branding request failed with status ${response.status}`);
      }

      const payload = (await response.json()) as PublicBrandingResponse;
      const data = payload.data;
      if (!payload.success || !data) return;

      const version = data.updated_at || '';
      const logoUrl = getImageUrl(data.company_logo) || DEFAULT_LOGO_PATH;
      const iconUrl = data.company_icon?.trim() ? DEFAULT_ICON_PATH : FALLBACK_ICON_PATH;

      setBranding({
        companyName: data.company_name?.trim() || DEFAULT_APP_NAME,
        logoUrl: appendVersion(logoUrl, version),
        logoScale: normalizeLogoScale(data.company_logo_scale),
        iconUrl: appendVersion(iconUrl, version),
        poweredByText: data.powered_by_text?.trim() || DEFAULT_POWERED_BY_TEXT,
        updatedAt: version,
      });
    } catch (error) {
      console.warn('Unable to load public branding, using defaults.', error);
    }
  }, []);

  useEffect(() => {
    void refreshBranding();
  }, [refreshBranding]);

  useEffect(() => {
    const iconHref = branding.iconUrl || FALLBACK_ICON_PATH;
    const appleIconHref = appendVersion(APPLE_TOUCH_ICON_PATH, branding.updatedAt);
    const iconType = getIconType(iconHref);
    const iconDefinitions = [
      { key: 'favicon', rel: 'icon', sizes: '192x192', href: iconHref },
      { key: 'shortcut', rel: 'shortcut icon', sizes: '192x192', href: iconHref },
      { key: 'apple-touch', rel: 'apple-touch-icon', sizes: '180x180', href: appleIconHref },
    ];

    document.title = branding.companyName;
    setNamedMetaContent('application-name', branding.companyName);
    setNamedMetaContent('apple-mobile-web-app-title', branding.companyName);

    iconDefinitions.forEach(({ key, rel, sizes, href }) => {
      let link = document.head.querySelector<HTMLLinkElement>(`link[data-branding-icon="${key}"]`);
      if (!link) {
        link = document.createElement('link');
        link.setAttribute('data-branding-icon', key);
        document.head.appendChild(link);
      }

      link.rel = rel;
      link.href = href;
      link.sizes = sizes;
      if (iconType) {
        link.type = iconType;
      } else {
        link.removeAttribute('type');
      }
    });

    const manifest = document.head.querySelector<HTMLLinkElement>('link[rel="manifest"]');
    if (manifest && branding.updatedAt) {
      manifest.href = appendVersion(API_ENDPOINTS.BRANDING_MANIFEST, branding.updatedAt);
    }

    return () => {
      document.head
        .querySelectorAll<HTMLLinkElement>('link[data-branding-icon]')
        .forEach((link) => {
          if (link.parentNode) {
            link.parentNode.removeChild(link);
          }
        });
    };
  }, [branding.companyName, branding.iconUrl, branding.updatedAt]);

  const value = useMemo<BrandingContextValue>(
    () => ({
      companyName: branding.companyName,
      logoUrl: branding.logoUrl,
      logoScale: branding.logoScale,
      iconUrl: branding.iconUrl,
      poweredByText: branding.poweredByText,
      refreshBranding,
    }),
    [
      branding.companyName,
      branding.iconUrl,
      branding.logoScale,
      branding.logoUrl,
      branding.poweredByText,
      refreshBranding,
    ],
  );

  return <BrandingContext.Provider value={value}>{children}</BrandingContext.Provider>;
};

export const useBranding = (): BrandingContextValue => {
  const context = useContext(BrandingContext);
  if (!context) {
    throw new Error('useBranding must be used within a BrandingProvider');
  }
  return context;
};

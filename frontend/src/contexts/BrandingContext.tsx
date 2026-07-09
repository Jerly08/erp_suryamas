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
  DEFAULT_APP_NAME,
  DEFAULT_ICON_PATH,
  DEFAULT_LOGO_PATH,
  DEFAULT_LOGO_SCALE,
  POWERED_BY_TEXT,
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
  updatedAt: string;
}

const defaultBranding: BrandingState = {
  companyName: DEFAULT_APP_NAME,
  logoUrl: DEFAULT_LOGO_PATH,
  logoScale: DEFAULT_LOGO_SCALE,
  iconUrl: DEFAULT_ICON_PATH,
  updatedAt: '',
};

const BrandingContext = createContext<BrandingContextValue | undefined>(undefined);

const appendVersion = (url: string, version?: string): string => {
  if (!version) return url;
  return `${url}${url.includes('?') ? '&' : '?'}v=${encodeURIComponent(version)}`;
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
      const iconUrl = getImageUrl(data.company_icon) || DEFAULT_ICON_PATH;

      setBranding({
        companyName: data.company_name?.trim() || DEFAULT_APP_NAME,
        logoUrl: appendVersion(logoUrl, version),
        logoScale: normalizeLogoScale(data.company_logo_scale),
        iconUrl: appendVersion(iconUrl, version),
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
    const iconSelector =
      'link[rel="icon"], link[rel="shortcut icon"], link[rel="apple-touch-icon"]';
    const iconDefinitions = ['icon', 'shortcut icon', 'apple-touch-icon'];
    const absoluteIconUrl = new URL(branding.iconUrl, document.baseURI).href;

    const syncBrandingMetadata = () => {
      if (document.title !== branding.companyName) {
        document.title = branding.companyName;
      }

      const iconLinks = Array.from(
        document.querySelectorAll<HTMLLinkElement>(iconSelector),
      );
      const hasExpectedIcons =
        iconLinks.length === iconDefinitions.length &&
        iconDefinitions.every((rel) =>
          iconLinks.some((link) => link.rel === rel && link.href === absoluteIconUrl),
        );

      if (!hasExpectedIcons) {
        iconLinks.forEach((link) => link.remove());
        iconDefinitions.forEach((rel) => {
          const link = document.createElement('link');
          link.rel = rel;
          link.href = branding.iconUrl;
          link.setAttribute('data-branding-icon', 'true');
          document.head.appendChild(link);
        });
      }

      const appleTitle = document.querySelector<HTMLMetaElement>(
        'meta[name="apple-mobile-web-app-title"]',
      );
      if (appleTitle?.content !== branding.companyName) {
        appleTitle?.setAttribute('content', branding.companyName);
      }

      const manifest = document.querySelector<HTMLLinkElement>('link[rel="manifest"]');
      const manifestUrl = appendVersion(API_ENDPOINTS.BRANDING_MANIFEST, branding.updatedAt);
      if (manifest && manifest.getAttribute('href') !== manifestUrl) {
        manifest.href = manifestUrl;
      }
    };

    let syncScheduled = false;
    const scheduleSync = () => {
      if (syncScheduled) return;
      syncScheduled = true;
      queueMicrotask(() => {
        syncScheduled = false;
        syncBrandingMetadata();
      });
    };

    syncBrandingMetadata();

    const headObserver = new MutationObserver(scheduleSync);
    headObserver.observe(document.head, {
      childList: true,
      subtree: true,
      characterData: true,
    });

    return () => headObserver.disconnect();
  }, [branding]);

  const value = useMemo<BrandingContextValue>(
    () => ({
      companyName: branding.companyName,
      logoUrl: branding.logoUrl,
      logoScale: branding.logoScale,
      iconUrl: branding.iconUrl,
      poweredByText: POWERED_BY_TEXT,
      refreshBranding,
    }),
    [
      branding.companyName,
      branding.iconUrl,
      branding.logoScale,
      branding.logoUrl,
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

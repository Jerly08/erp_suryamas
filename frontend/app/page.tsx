'use client';

import React from 'react';
import { API_ENDPOINTS } from '@/config/api';
import { useBranding } from '@/contexts/BrandingContext';
import { getScaledLogoSize } from '@/config/branding';
import Image from 'next/image';

// Simple redirect page - let Next.js handle the redirect
export default function Home() {
  const { companyName, logoUrl, logoScale, poweredByText } = useBranding();

  // Check if we're on client side and redirect accordingly
  React.useEffect(() => {
    const redirectBySession = async () => {
      try {
        const response = await fetch(API_ENDPOINTS.VALIDATE_TOKEN, {
          method: 'GET',
          credentials: 'include',
          headers: {
            'Content-Type': 'application/json',
          },
        });
        window.location.replace(response.ok ? '/dashboard' : '/login');
      } catch {
        window.location.replace('/login');
      }
    };

    redirectBySession();
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <Image
          src={logoUrl}
          alt={companyName}
          width={260}
          height={56}
          className="max-w-64 object-contain mx-auto mb-4"
          style={{ height: getScaledLogoSize(56, logoScale), width: 'auto' }}
        />
        <p className="text-gray-600 mb-6">Please wait while we redirect you...</p>
        <p className="text-gray-500 text-xs mb-6">{poweredByText}</p>
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-blue-500 mx-auto"></div>
      </div>
    </div>
  );
}

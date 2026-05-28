'use client';

import React from 'react';
import { AVERY_LOGO_ALT, AVERY_LOGO_PATH, POWERED_BY_TEXT } from '@/config/branding';
import Image from 'next/image';

// Simple redirect page - let Next.js handle the redirect
export default function Home() {
  // Check if we're on client side and redirect accordingly
  React.useEffect(() => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    if (token) {
      window.location.replace('/dashboard');
    } else {
      window.location.replace('/login');
    }
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <Image
          src={AVERY_LOGO_PATH}
          alt={AVERY_LOGO_ALT}
          width={260}
          height={56}
          className="h-14 max-w-64 object-contain mx-auto mb-4"
        />
        <p className="text-gray-600 mb-6">Please wait while we redirect you...</p>
        <p className="text-gray-500 text-xs mb-6">{POWERED_BY_TEXT}</p>
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-blue-500 mx-auto"></div>
      </div>
    </div>
  );
}

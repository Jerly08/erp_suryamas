'use client';

import React, { useEffect, useState } from 'react';
import { ChakraProvider, ColorModeScript, localStorageManager } from '@chakra-ui/react';
import { ThemeProvider } from '@/contexts/SimpleThemeContext';
import { AuthProvider } from '@/contexts/AuthContext';
import { LanguageProvider } from '@/contexts/LanguageContext';
import theme from '@/theme';
import { AVERY_LOGO_ALT, AVERY_LOGO_PATH, POWERED_BY_TEXT } from '@/config/branding';
import Image from 'next/image';

interface ClientProvidersProps {
  children: React.ReactNode;
}

export default function ClientProviders({ children }: ClientProvidersProps) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    // Use neutral styles during SSR to avoid hydration mismatches
    // We'll use a light theme by default on server to match most common case
    return (
      <div style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: '#f7f7f7',
        color: '#1a1d23',
        transition: 'background-color 0.3s ease, color 0.3s ease'
      }}>
        <div style={{
          textAlign: 'center',
          padding: '2rem'
        }}>
          <Image
            src={AVERY_LOGO_PATH}
            alt={AVERY_LOGO_ALT}
            width={260}
            height={56}
            style={{
              height: '56px',
              maxWidth: '260px',
              objectFit: 'contain',
              marginBottom: '0.75rem'
            }}
          />
          <div style={{
            fontSize: '0.875rem',
            opacity: 0.7
          }}>Loading...</div>
          <div style={{
            fontSize: '0.75rem',
            opacity: 0.6,
            marginTop: '0.5rem'
          }}>{POWERED_BY_TEXT}</div>
        </div>
      </div>
    );
  }

  return (
    <ChakraProvider theme={theme} colorModeManager={localStorageManager}>
      <ColorModeScript initialColorMode={theme.config.initialColorMode} />
      <ThemeProvider>
        <LanguageProvider>
          <AuthProvider>
            {children}
          </AuthProvider>
        </LanguageProvider>
      </ThemeProvider>
    </ChakraProvider>
  );
}

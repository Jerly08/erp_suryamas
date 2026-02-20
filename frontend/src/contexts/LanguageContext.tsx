'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import idTranslations from '../translations/id.json';
import enTranslations from '../translations/en.json';
// Translation files updated: 2026-02-19

export type Language = 'id' | 'en';

interface LanguageContextType {
  language: Language;
  setLanguage: (lang: Language) => void;
  t: (key: string, interpolations?: Record<string, string | number>) => string;
}

const LanguageContext = createContext<LanguageContextType | undefined>(undefined);

interface LanguageProviderProps {
  children: ReactNode;
}

// Pre-load all translations
const translationsMap = {
  id: idTranslations,
  en: enTranslations
};

export const LanguageProvider: React.FC<LanguageProviderProps> = ({ children }) => {
  const [language, setLanguageState] = useState<Language>('id');
  const [translations, setTranslations] = useState<any>(translationsMap['id']);

  // Load saved language preference from localStorage
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const savedLanguage = localStorage.getItem('preferredLanguage') as Language;
      if (savedLanguage && (savedLanguage === 'id' || savedLanguage === 'en')) {
        setLanguageState(savedLanguage);
        setTranslations(translationsMap[savedLanguage]);
      }
    }
  }, []);

  // Update translations when language changes
  useEffect(() => {
    setTranslations(translationsMap[language]);
  }, [language]);

  const setLanguage = (lang: Language) => {
    setLanguageState(lang);
    if (typeof window !== 'undefined') {
      localStorage.setItem('preferredLanguage', lang);
    }
  };

  // Translation function with interpolation support
  const t = (key: string, interpolations?: Record<string, string | number>): string => {
    // Support nested keys with dot notation
    const keys = key.split('.');
    let value: any = translations;

    for (const k of keys) {
      if (value && typeof value === 'object' && k in value) {
        value = value[k];
      } else {
        // Return the key itself if translation not found
        console.warn(`Translation not found for key: ${key}`);
        return key;
      }
    }

    let result = typeof value === 'string' ? value : key;

    // Apply interpolations if provided
    if (interpolations && typeof result === 'string') {
      Object.entries(interpolations).forEach(([placeholder, replacement]) => {
        result = result.replace(new RegExp(`{{${placeholder}}}`, 'g'), String(replacement));
      });
    }

    return result;
  };

  const value: LanguageContextType = {
    language,
    setLanguage,
    t
  };

  return (
    <LanguageContext.Provider value={value}>
      {children}
    </LanguageContext.Provider>
  );
};

export const useLanguage = (): LanguageContextType => {
  const context = useContext(LanguageContext);
  if (!context) {
    throw new Error('useLanguage must be used within a LanguageProvider');
  }
  return context;
};

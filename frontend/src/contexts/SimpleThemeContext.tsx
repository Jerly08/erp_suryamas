'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';

type Theme = 'light' | 'dark';
const THEME_STORAGE_KEY = 'theme';
const CHAKRA_COLOR_MODE_KEY = 'chakra-ui-color-mode';

interface ThemeContextType {
  theme: Theme;
  toggleTheme: () => void;
  mounted: boolean;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export const useTheme = () => {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return context;
};

interface ThemeProviderProps {
  children: React.ReactNode;
}

export const ThemeProvider: React.FC<ThemeProviderProps> = ({ children }) => {
  const [theme, setTheme] = useState<Theme>('light');
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // Initialize theme
    const initTheme = () => {
      try {
        const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as Theme;
        const savedChakraTheme = localStorage.getItem(CHAKRA_COLOR_MODE_KEY) as Theme;
        if (savedTheme === 'dark' || savedTheme === 'light') {
          setTheme(savedTheme);
          applyTheme(savedTheme);
        } else if (savedChakraTheme === 'dark' || savedChakraTheme === 'light') {
          setTheme(savedChakraTheme);
          applyTheme(savedChakraTheme);
        } else {
          // Check system preference
          const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
          const initialTheme = prefersDark ? 'dark' : 'light';
          setTheme(initialTheme);
          applyTheme(initialTheme);
        }
      } catch (error) {
        setTheme('light');
        applyTheme('light');
      }
      setMounted(true);
    };

    initTheme();
  }, []);

  const applyTheme = (newTheme: Theme) => {
    if (typeof document === 'undefined') return;
    
    const root = document.documentElement;
    root.classList.remove('light', 'dark');
    root.classList.add(newTheme);
    root.setAttribute('data-theme', newTheme);
    
    // Store in localStorage
    try {
      localStorage.setItem(THEME_STORAGE_KEY, newTheme);
      localStorage.setItem(CHAKRA_COLOR_MODE_KEY, newTheme);
    } catch (error) {
      console.warn('Cannot save theme to localStorage');
    }
  };

  const toggleTheme = () => {
    const newTheme = theme === 'light' ? 'dark' : 'light';
    setTheme(newTheme);
    applyTheme(newTheme);
  };

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme, mounted }}>
      {children}
    </ThemeContext.Provider>
  );
};

import type { Metadata } from "next";
import "./globals.css";
import ClientProviders from './ClientProviders';

export const metadata: Metadata = {
  title: "Accounting System - User Friendly Dark Mode",
  description: "Professional accounting application with user-friendly dark mode",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `
              // Global error handler to prevent external extension errors
              window.addEventListener('error', function(e) {
                if (e.error && e.error.message && e.error.message.includes('MetaMask')) {
                  e.preventDefault();
                  console.warn('MetaMask error suppressed:', e.error.message);
                  return false;
                }
              });
              
              // Theme initialization
              try {
                const theme = localStorage.getItem('theme');
                if (theme === 'dark' || (!theme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
                  document.documentElement.classList.add('dark');
                  document.documentElement.setAttribute('data-theme', 'dark');
                } else {
                  document.documentElement.classList.add('light');
                  document.documentElement.setAttribute('data-theme', 'light');
                }
              } catch (e) {
                console.warn('Theme initialization error:', e);
              }
            `,
          }}
        />
      </head>
      <body suppressHydrationWarning>
        <ClientProviders>{children}</ClientProviders>
      </body>
    </html>
  );
}

import type { Metadata, Viewport } from "next";
import "./globals.css";
import ClientProviders from './ClientProviders';
import { DEFAULT_APP_NAME, HEAD_APPLE_TOUCH_ICON_PATH, HEAD_ICON_PATH } from '@/config/branding';

export const metadata: Metadata = {
  title: DEFAULT_APP_NAME,
  description: "Accounting and finance management application",
  applicationName: DEFAULT_APP_NAME,
  manifest: "/api/v1/branding/manifest.webmanifest",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: DEFAULT_APP_NAME,
  },
  formatDetection: {
    telephone: false,
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: "#2196F3",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="icon" href={HEAD_ICON_PATH} sizes="192x192" type="image/png" />
        <link rel="shortcut icon" href={HEAD_ICON_PATH} sizes="192x192" type="image/png" />
        <link rel="apple-touch-icon" href={HEAD_APPLE_TOUCH_ICON_PATH} sizes="180x180" type="image/png" />
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
                const theme = localStorage.getItem('theme') || localStorage.getItem('chakra-ui-color-mode');
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

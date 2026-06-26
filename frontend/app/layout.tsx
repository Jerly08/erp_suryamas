import type { Metadata, Viewport } from "next";
import "./globals.css";
import ClientProviders from './ClientProviders';

export const metadata: Metadata = {
  title: "Avery",
  description: "Avery accounting application",
  applicationName: "Avery",
  manifest: "/manifest.webmanifest",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "Avery",
  },
  formatDetection: {
    telephone: false,
  },
  icons: {
    icon: [
      { url: "/avery-icon.png?v=20260529", type: "image/png", sizes: "256x256" },
      { url: "/favicon.ico?v=20260529", type: "image/x-icon" },
    ],
    shortcut: [{ url: "/avery-icon.png?v=20260529", type: "image/png", sizes: "256x256" }],
    apple: [{ url: "/avery-icon.png?v=20260529", type: "image/png", sizes: "256x256" }],
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

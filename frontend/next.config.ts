import type { NextConfig } from "next";

const nextConfig: NextConfig = {
	poweredByHeader: false,
	env: {
		NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
	},
	async headers() {
		return [
			{
				source: '/:path*',
				headers: [
					{
						key: 'Content-Security-Policy',
						value: [
							"default-src 'self'",
							"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
							"style-src 'self' 'unsafe-inline'",
							"img-src 'self' data: blob:",
							"font-src 'self' data:",
							"connect-src 'self'",
							"media-src 'self' blob:",
							"object-src 'none'",
							"base-uri 'self'",
							"frame-ancestors 'self'",
							"form-action 'self'",
						].join('; '),
					},
				],
			},
		];
	},
	typescript: {
		// Next validator menghasilkan referensi .next/types yang salah arah
		// pada repo ini karena struktur campuran app/ dan src/app/.
		// Gate type dijalankan eksplisit lewat npm prebuild.
		ignoreBuildErrors: true,
	},
	eslint: {
		// Lint dijalankan eksplisit lewat npm prebuild agar build tetap stabil
		// dan tidak bergantung pada checker internal Next.
		ignoreDuringBuilds: true,
	},
	output: 'standalone',
	// Konfigurasi Webpack hanya untuk mode non-Turbopack
	...(!process.env.TURBOPACK && {
		webpack: (config, { isServer }) => {
			// Handle client-side module resolution for jsPDF
			if (!isServer) {
				config.resolve.fallback = {
					...config.resolve.fallback,
					fs: false,
					path: false,
				};

				// Handle jsPDF ES modules
				config.module.rules.push({
					test: /\.m?js$/,
					resolve: {
						fullySpecified: false
					}
				});
			}
			return config;
		},
	}),
	async rewrites() {
		// Use specific environment variable for internal Docker routing, fallback to public API URL, then localhost
		const apiBaseUrl = process.env.API_INTERNAL_URL || process.env.NEXT_PUBLIC_API_URL || 'http://backend:8080';
		return [
			{
				source: '/api/:path*',
				destination: `${apiBaseUrl}/api/:path*`,
			},
			{
				source: '/uploads/company/:path*',
				destination: `${apiBaseUrl}/uploads/company/:path*`,
			},
			{
				source: '/uploads/products/:path*',
				destination: `${apiBaseUrl}/uploads/products/:path*`,
			},
			{
				source: '/uploads/assets/:path*',
				destination: `${apiBaseUrl}/uploads/assets/:path*`,
			},
		];
	},
};

export default nextConfig;

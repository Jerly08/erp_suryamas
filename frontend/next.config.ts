import type { NextConfig } from "next";

const nextConfig: NextConfig = {
	env: {
		NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
	},
	typescript: {
		ignoreBuildErrors: true,
	},
	eslint: {
		ignoreDuringBuilds: true,
	},
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
		// Use environment variable for API destination, fallback to localhost for development
		const apiBaseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
		return [
			{
				source: '/api/:path*',
				destination: `${apiBaseUrl}/api/:path*`,
			},
			{
				// Proxy /uploads/ to backend so static files (logos, etc.) are accessible
				source: '/uploads/:path*',
				destination: `${apiBaseUrl}/uploads/:path*`,
			},
		];
	},
};

export default nextConfig;

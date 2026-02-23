/**
 * Image URL utilities for handling product and asset images
 * Fixes issues with undefined base URLs and ensures proper image loading
 */

import API_BASE_URL from '@/config/api';

// Get the API base URL from the global config
const getApiBaseUrl = (): string => {
  // Remove /api/v1 suffix if present since we're accessing static files directly
  return API_BASE_URL.replace(/\/api\/v1\/?$/, '');
};

// Get the full static file base URL
const getStaticBaseUrl = (): string => {
  const baseUrl = getApiBaseUrl();
  // Ensure no trailing slash to avoid double slashes when concatenating with paths
  const normalizedBase = baseUrl.replace(/\/+$/, '');
  return normalizedBase;
};

/**
 * Constructs a complete URL for an image from a relative path
 * @param imagePath - The relative image path from database (e.g., "/uploads/products/1_123456_image.jpg")
 * @returns Complete image URL or null if path is invalid
 */
export const getImageUrl = (imagePath: string | null | undefined): string | null => {
  if (!imagePath || imagePath.trim() === '') {
    return null;
  }

  // Clean up the path
  let cleanPath = imagePath.trim();

  // If the path already starts with http, return it as-is (full URL)
  if (cleanPath.startsWith('http://') || cleanPath.startsWith('https://')) {
    return cleanPath;
  }

  // Ensure path starts with /
  if (!cleanPath.startsWith('/')) {
    cleanPath = `/${cleanPath}`;
  }

  // Remove duplicate slashes
  cleanPath = cleanPath.replace(/\/+/g, '/');

  // Build complete URL
  const baseUrl = getStaticBaseUrl();
  const fullUrl = `${baseUrl}${cleanPath}`;

  return fullUrl;
};

/**
 * Get a product image URL with fallback to placeholder
 * @param imagePath - The product image path
 * @param fallbackToPlaceholder - Whether to return a placeholder if no image
 * @returns Image URL or placeholder
 */
export const getProductImageUrl = (
  imagePath: string | null | undefined,
  fallbackToPlaceholder: boolean = true
): string | null => {
  const imageUrl = getImageUrl(imagePath);

  if (imageUrl) {
    return imageUrl;
  }

  if (fallbackToPlaceholder) {
    // Return a simple data URL placeholder
    return 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgdmlld0JveD0iMCAwIDIwMCAyMDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxyZWN0IHdpZHRoPSIyMDAiIGhlaWdodD0iMjAwIiBmaWxsPSIjRjdGQUZDIi8+CjxwYXRoIGQ9Ik02NyA4M0MxMDEuNzk0IDgzIDEzMyA1MS43OTQgMTMzIDUwQzEzMyA0OC4yMDYgMTAxLjc5NCA4MyA2NyA4M1oiIGZpbGw9IiNFMkU4RjAiLz4KPHBhdGggZD0iTTEwMCAxMDBMMTMzIDEzM0g2N0wxMDAgMTAwWiIgZmlsbD0iI0UyRThGMCIvPgo8L3N2Zz4K';
  }

  return null;
};

/**
 * Get an asset image URL with fallback to placeholder
 * @param imagePath - The asset image path
 * @param fallbackToPlaceholder - Whether to return a placeholder if no image
 * @returns Image URL or placeholder
 */
export const getAssetImageUrl = (
  imagePath: string | null | undefined,
  fallbackToPlaceholder: boolean = true
): string | null => {
  const imageUrl = getImageUrl(imagePath);

  if (imageUrl) {
    return imageUrl;
  }

  if (fallbackToPlaceholder) {
    // Return a simple data URL placeholder for assets
    return 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgdmlld0JveD0iMCAwIDIwMCAyMDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxyZWN0IHdpZHRoPSIyMDAiIGhlaWdodD0iMjAwIiBmaWxsPSIjRjdGQUZDIi8+CjxjaXJjbGUgY3g9IjEwMCIgY3k9IjEwMCIgcj0iNDAiIGZpbGw9IiNFMkU4RjAiLz4KPC9zdmc+';
  }

  return null;
};

/**
 * Debug function to log image URL construction details
 * @param imagePath - The original image path
 * @returns Object with debug information
 */
export const debugImageUrl = (imagePath: string | null | undefined) => {
  const debug = {
    originalPath: imagePath,
    apiBaseUrl: getApiBaseUrl(),
    staticBaseUrl: getStaticBaseUrl(),
    finalUrl: getImageUrl(imagePath),
    envVariables: {
      NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
      NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
      API_URL: process.env.API_URL,
    }
  };

  console.log('Image URL Debug:', debug);
  return debug;
};

/**
 * Test if an image URL is accessible
 * @param imageUrl - The image URL to test
 * @returns Promise that resolves to boolean
 */
export const testImageUrl = (imageUrl: string | null): Promise<boolean> => {
  return new Promise((resolve) => {
    if (!imageUrl) {
      resolve(false);
      return;
    }

    const img = new Image();
    img.onload = () => resolve(true);
    img.onerror = () => resolve(false);
    img.src = imageUrl;
  });
};
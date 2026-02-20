"use client";

import jsPDF from 'jspdf';
import { SSOTCashFlowData } from '../services/ssotCashFlowReportService';
import { exportCashFlowToPDF, CompanyLetterhead } from './cashFlowExportUtils';

/**
 * Load an image URL and convert it to a base64 data URL.
 * Uses fetch() + FileReader to avoid CORS/tainted canvas issues
 * that occur when backend and frontend run on different ports.
 */
async function loadImageAsBase64(url: string): Promise<string | null> {
  try {
    const response = await fetch(url);
    if (!response.ok) {
      console.warn(`Failed to fetch logo: ${response.status} ${response.statusText}`);
      return null;
    }
    const blob = await response.blob();
    return new Promise<string | null>((resolve) => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result as string);
      reader.onerror = () => resolve(null);
      reader.readAsDataURL(blob);
    });
  } catch (e) {
    console.warn('Failed to load logo image:', e);
    return null;
  }
}

// Download PDF in the browser
export function downloadCashFlowPDF(doc: jsPDF, filename?: string): void {
  const pdfName = filename || `Arus_Kas_${new Date().toISOString().split('T')[0]}.pdf`;
  doc.save(pdfName);
}

// Export Cash Flow as PDF and trigger download (client-only)
// Made async to support logo pre-loading
export async function exportAndDownloadCashFlowPDF(
  data: SSOTCashFlowData,
  options?: {
    companyName?: string;
    logoUrl?: string;
    includeAccountDetails?: boolean;
    filename?: string;
    letterhead?: CompanyLetterhead;
  }
): Promise<void> {
  // Pre-load company logo if URL is provided
  if (options?.logoUrl && options.letterhead && !options.letterhead.companyLogoBase64) {
    const base64 = await loadImageAsBase64(options.logoUrl);
    if (base64) {
      options.letterhead.companyLogoBase64 = base64;
    }
  }

  const pdfDoc = exportCashFlowToPDF(data, options);
  downloadCashFlowPDF(pdfDoc, options?.filename);
}
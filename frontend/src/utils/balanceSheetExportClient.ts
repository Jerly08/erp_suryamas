"use client";

import jsPDF from 'jspdf';
import { SSOTBalanceSheetData } from '../services/ssotBalanceSheetReportService';
import { 
  exportBalanceSheetToCSV,
  exportBalanceSheetToPDF
} from './balanceSheetExportUtils';
import { cleanCsvContent } from './exportText';

// Download CSV in the browser
export function downloadCSV(csvContent: string, filename?: string): void {
  const blob = new Blob([cleanCsvContent(csvContent)], { type: 'text/csv;charset=utf-8;' });
  const link = document.createElement('a');

  if (link.download !== undefined) {
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', filename || `balance_sheet_${new Date().toISOString().split('T')[0]}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }
}

// Download PDF in the browser
export function downloadPDF(doc: jsPDF, filename?: string): void {
  const pdfName = filename || `balance_sheet_${new Date().toISOString().split('T')[0]}.pdf`;
  doc.save(pdfName);
}

// Export Balance Sheet as CSV and trigger download (client-only)
export function exportAndDownloadCSV(
  data: SSOTBalanceSheetData,
  options?: {
    includeAccountDetails?: boolean;
    companyName?: string;
    filename?: string;
  }
): void {
  const csvContent = exportBalanceSheetToCSV(data, options);
  downloadCSV(csvContent, options?.filename);
}

// Export Balance Sheet as PDF and trigger download (client-only)
export function exportAndDownloadPDF(
  data: SSOTBalanceSheetData,
  options?: {
    companyName?: string;
    logoUrl?: string;
    includeAccountDetails?: boolean;
    filename?: string;
  }
): void {
  const doc = exportBalanceSheetToPDF(data, options);
  downloadPDF(doc, options?.filename);
}

/**
 * Balance Sheet Export Utilities
 * 
 * Provides enhanced CSV and PDF export functionality for Balance Sheet reports
 * Supports both SSOT and legacy balance sheet data formats
 */

import jsPDF from 'jspdf';
import autoTable from 'jspdf-autotable';
import { SSOTBalanceSheetData } from '../services/ssotBalanceSheetReportService';
import { cleanCsvCell } from './exportText';

const getBalanceSheetTotals = (data: SSOTBalanceSheetData) => ({
  totalAssets: data.assets?.total_assets || 0,
  totalLiabilities: data.liabilities?.total_liabilities || 0,
  totalEquity: data.equity?.total_equity || 0,
});


/**
 * Enhanced CSV Export for Balance Sheet
 * Generates professional CSV format with proper formatting and structure
 */
export function exportBalanceSheetToCSV(
  data: SSOTBalanceSheetData,
  options: {
    includeAccountDetails?: boolean;
    companyName?: string;
  } = {}
): string {
  const { includeAccountDetails = true, companyName } = options;
  
  const csvLines: string[] = [];
  
  // Helper function to escape CSV values
  const escapeCSV = (value: string | number | null | undefined): string => {
    if (value === null || value === undefined) return '';
    const str = cleanCsvCell(value);
    if (str.includes(',') || str.includes('"') || str.includes('\n')) {
      return `"${str.replace(/"/g, '""')}"`;
    }
    return str;
  };
  
  // Helper function to format currency
  const formatCurrencyForCSV = (amount: number | null | undefined): string => {
    if (amount === null || amount === undefined || isNaN(Number(amount))) {
      return '0';
    }
    return Number(amount).toLocaleString('id-ID');
  };
  
  // Header section
  csvLines.push(escapeCSV(companyName || data.company?.name || 'PT. Sistem Akuntansi'));
  csvLines.push('BALANCE SHEET');
  csvLines.push(`As of: ${data.as_of_date || new Date().toISOString().split('T')[0]}`);
  csvLines.push(`Generated on: ${new Date().toLocaleString('id-ID')}`);
  csvLines.push(''); // Empty line
  
  // Summary section
  const totals = getBalanceSheetTotals(data);
  csvLines.push('FINANCIAL SUMMARY');
  csvLines.push('Category,Amount');
  csvLines.push(`Total Assets,${formatCurrencyForCSV(totals.totalAssets)}`);
  csvLines.push(`Total Liabilities,${formatCurrencyForCSV(totals.totalLiabilities)}`);
  csvLines.push(`Total Equity,${formatCurrencyForCSV(totals.totalEquity)}`);
  csvLines.push(`Total Liabilities + Equity,${formatCurrencyForCSV(data.total_liabilities_and_equity || 0)}`);
  csvLines.push(`Balanced,${data.is_balanced ? 'Yes' : 'No'}`);
  if (!data.is_balanced && data.balance_difference) {
    csvLines.push(`Balance Difference,${formatCurrencyForCSV(data.balance_difference)}`);
  }
  csvLines.push(''); // Empty line
  
  if (includeAccountDetails) {
    // Detailed account breakdown
    csvLines.push('DETAILED BREAKDOWN');
    csvLines.push('Account Code,Account Name,Category,Amount');
    
    // Assets section
    csvLines.push('ASSETS,,,');
    
    // Current Assets
    if (data.assets?.current_assets?.items && data.assets.current_assets.items.length > 0) {
      csvLines.push('Current Assets,,,');
      data.assets.current_assets.items.forEach(item => {
        csvLines.push([
          escapeCSV(item.account_code || ''),
          escapeCSV(item.account_name || ''),
          'Current Asset',
          formatCurrencyForCSV(item.amount || 0)
        ].join(','));
      });
      csvLines.push(`Subtotal Current Assets,,,${formatCurrencyForCSV(data.assets.current_assets.total_current_assets || 0)}`);
      csvLines.push('');
    }
    
    // Non-Current Assets
    if (data.assets?.non_current_assets?.items && data.assets.non_current_assets.items.length > 0) {
      csvLines.push('Non-Current Assets,,,');
      data.assets.non_current_assets.items.forEach(item => {
        csvLines.push([
          escapeCSV(item.account_code || ''),
          escapeCSV(item.account_name || ''),
          'Non-Current Asset',
          formatCurrencyForCSV(item.amount || 0)
        ].join(','));
      });
      csvLines.push(`Subtotal Non-Current Assets,,,${formatCurrencyForCSV(data.assets.non_current_assets.total_non_current_assets || 0)}`);
      csvLines.push('');
    }
    
    csvLines.push(`TOTAL ASSETS,,,${formatCurrencyForCSV(data.assets?.total_assets || 0)}`);
    csvLines.push(''); // Empty line
    
    // Liabilities section
    csvLines.push('LIABILITIES,,,');
    
    // Current Liabilities
    if (data.liabilities?.current_liabilities?.items && data.liabilities.current_liabilities.items.length > 0) {
      csvLines.push('Current Liabilities,,,');
      data.liabilities.current_liabilities.items.forEach(item => {
        csvLines.push([
          escapeCSV(item.account_code || ''),
          escapeCSV(item.account_name || ''),
          'Current Liability',
          formatCurrencyForCSV(item.amount || 0)
        ].join(','));
      });
      csvLines.push(`Subtotal Current Liabilities,,,${formatCurrencyForCSV(data.liabilities.current_liabilities.total_current_liabilities || 0)}`);
      csvLines.push('');
    }
    
    // Non-Current Liabilities
    if (data.liabilities?.non_current_liabilities?.items && data.liabilities.non_current_liabilities.items.length > 0) {
      csvLines.push('Non-Current Liabilities,,,');
      data.liabilities.non_current_liabilities.items.forEach(item => {
        csvLines.push([
          escapeCSV(item.account_code || ''),
          escapeCSV(item.account_name || ''),
          'Non-Current Liability',
          formatCurrencyForCSV(item.amount || 0)
        ].join(','));
      });
      csvLines.push(`Subtotal Non-Current Liabilities,,,${formatCurrencyForCSV(data.liabilities.non_current_liabilities.total_non_current_liabilities || 0)}`);
      csvLines.push('');
    }
    
    csvLines.push(`TOTAL LIABILITIES,,,${formatCurrencyForCSV(data.liabilities?.total_liabilities || 0)}`);
    csvLines.push(''); // Empty line
    
    // Equity section
    if (data.equity?.items && data.equity.items.length > 0) {
      csvLines.push('EQUITY,,,');
      data.equity.items.forEach(item => {
        csvLines.push([
          escapeCSV(item.account_code || ''),
          escapeCSV(item.account_name || ''),
          'Equity',
          formatCurrencyForCSV(item.amount || 0)
        ].join(','));
      });
      csvLines.push('');
    }
    
    csvLines.push(`TOTAL EQUITY,,,${formatCurrencyForCSV(data.equity?.total_equity || 0)}`);
    csvLines.push(''); // Empty line
    csvLines.push(`TOTAL LIABILITIES + EQUITY,,,${formatCurrencyForCSV(data.total_liabilities_and_equity || 0)}`);
  }
  
  // Footer
  csvLines.push('');
  csvLines.push('Generated by Sistem Akuntansi');
  csvLines.push(`Report Date: ${new Date().toLocaleString('id-ID')}`);
  csvLines.push(`Data Source: ${data.enhanced ? 'SSOT Enhanced' : 'SSOT Standard'}`);
  
  return csvLines.join('\n');
}

/**
 * Enhanced PDF Export for Balance Sheet
 * Generates professional PDF report with proper formatting and branding
 */
export function exportBalanceSheetToPDF(
  data: SSOTBalanceSheetData,
  options: {
    companyName?: string;
    logoUrl?: string;
    includeAccountDetails?: boolean;
  } = {}
): jsPDF {
  try {
    const { companyName, includeAccountDetails = true } = options;
    
    // Initialize PDF
    const doc = new jsPDF('portrait', 'mm', 'a4');
    const pageWidth = doc.internal.pageSize.width;
    const pageHeight = doc.internal.pageSize.height;
    
    // Helper function to format currency for PDF
    const formatCurrencyForPDF = (amount: number | null | undefined): string => {
      if (amount === null || amount === undefined || isNaN(Number(amount))) {
        return 'Rp 0';
      }
      return new Intl.NumberFormat('id-ID', {
        style: 'currency',
        currency: 'IDR',
        minimumFractionDigits: 0
      }).format(Number(amount));
    };
    
    // Set fonts
    doc.setFont('helvetica');
    
    // Header section
    let yPosition = 20;
    
    // Company name
    doc.setFontSize(16);
    doc.setFont('helvetica', 'bold');
    const company = companyName || data.company?.name || 'PT. Sistem Akuntansi';
    doc.text(company, pageWidth / 2, yPosition, { align: 'center' });
    
    yPosition += 10;
    
    // Report title
    doc.setFontSize(14);
    doc.text('BALANCE SHEET', pageWidth / 2, yPosition, { align: 'center' });
    
    yPosition += 8;
    
    // Date and status
    doc.setFontSize(12);
    doc.setFont('helvetica', 'normal');
    const asOfDate = data.as_of_date || new Date().toISOString().split('T')[0];
    doc.text(`As of: ${new Date(asOfDate).toLocaleDateString('id-ID')}`, pageWidth / 2, yPosition, { align: 'center' });
    
    yPosition += 6;
    
    // Balance status
    const balanceStatus = data.is_balanced ? '✓ Balanced' : '✗ Not Balanced';
    const statusColor = data.is_balanced ? [0, 128, 0] : [255, 0, 0];
    doc.setTextColor(statusColor[0], statusColor[1], statusColor[2]);
    doc.text(balanceStatus, pageWidth / 2, yPosition, { align: 'center' });
    doc.setTextColor(0, 0, 0); // Reset to black
    
    yPosition += 15;
    
    // Summary table
    doc.setFontSize(12);
    doc.setFont('helvetica', 'bold');
    doc.text('FINANCIAL SUMMARY', 20, yPosition);
    yPosition += 5;
    
    const totals = getBalanceSheetTotals(data);
    const summaryData = [
      ['Total Assets', formatCurrencyForPDF(totals.totalAssets)],
      ['Total Liabilities', formatCurrencyForPDF(totals.totalLiabilities)],
      ['Total Equity', formatCurrencyForPDF(totals.totalEquity)],
      ['Total Liabilities + Equity', formatCurrencyForPDF(data.total_liabilities_and_equity || 0)]
    ];
    
    if (!data.is_balanced && data.balance_difference) {
      summaryData.push(['Balance Difference', formatCurrencyForPDF(data.balance_difference)]);
    }
    
    autoTable(doc, {
      startY: yPosition,
      head: [['Category', 'Amount']],
      body: summaryData,
      theme: 'grid',
      headStyles: { fillColor: [66, 139, 202], textColor: 255, fontStyle: 'bold' },
      bodyStyles: { fontSize: 11 },
      alternateRowStyles: { fillColor: [245, 245, 245] },
      margin: { left: 20, right: 20 },
      columnStyles: {
        1: { halign: 'right' }
      }
    } as any);
    
    yPosition = (doc as any).lastAutoTable.finalY + 10;
    
    if (includeAccountDetails) {
      // Detailed breakdown
      doc.setFontSize(12);
      doc.setFont('helvetica', 'bold');
      doc.text('DETAILED BREAKDOWN', 20, yPosition);
      yPosition += 10;
    
    // Assets section
    if (data.assets) {
      doc.setFontSize(11);
      doc.setFont('helvetica', 'bold');
      doc.text('ASSETS', 20, yPosition);
      yPosition += 5;
      
      const assetsData: any[] = [];
      
      // Current Assets
      if (data.assets.current_assets?.items && data.assets.current_assets.items.length > 0) {
        assetsData.push(['CURRENT ASSETS', '', '']);
        data.assets.current_assets.items.forEach(item => {
          assetsData.push([
            `  ${item.account_code || ''}`,
            item.account_name || '',
            formatCurrencyForPDF(item.amount || 0)
          ]);
        });
        assetsData.push([
          'Subtotal Current Assets',
          '',
          formatCurrencyForPDF(data.assets.current_assets.total_current_assets || 0)
        ]);
        assetsData.push(['', '', '']); // Empty row
      }
      
      // Non-Current Assets
      if (data.assets.non_current_assets?.items && data.assets.non_current_assets.items.length > 0) {
        assetsData.push(['NON-CURRENT ASSETS', '', '']);
        data.assets.non_current_assets.items.forEach(item => {
          assetsData.push([
            `  ${item.account_code || ''}`,
            item.account_name || '',
            formatCurrencyForPDF(item.amount || 0)
          ]);
        });
        assetsData.push([
          'Subtotal Non-Current Assets',
          '',
          formatCurrencyForPDF(data.assets.non_current_assets.total_non_current_assets || 0)
        ]);
        assetsData.push(['', '', '']); // Empty row
      }
      
      assetsData.push([
        'TOTAL ASSETS',
        '',
        formatCurrencyForPDF(data.assets.total_assets || 0)
      ]);
      
      if (assetsData.length > 0) {
        autoTable(doc, {
          startY: yPosition,
          head: [['Account Code', 'Account Name', 'Amount']],
          body: assetsData,
          theme: 'grid',
          headStyles: { fillColor: [40, 167, 69], textColor: 255, fontStyle: 'bold' },
          bodyStyles: { fontSize: 9 },
          alternateRowStyles: { fillColor: [248, 249, 250] },
          margin: { left: 20, right: 20 },
          columnStyles: {
            2: { halign: 'right' }
          }
        } as any);
        
        yPosition = (doc as any).lastAutoTable.finalY + 10;
      }
    }
    
    // Check if we need a new page
    if (yPosition > pageHeight - 80) {
      doc.addPage();
      yPosition = 20;
    }
    
    // Liabilities section
    if (data.liabilities) {
      doc.setFontSize(11);
      doc.setFont('helvetica', 'bold');
      doc.text('LIABILITIES', 20, yPosition);
      yPosition += 5;
      
      const liabilitiesData: any[] = [];
      
      // Current Liabilities
      if (data.liabilities.current_liabilities?.items && data.liabilities.current_liabilities.items.length > 0) {
        liabilitiesData.push(['CURRENT LIABILITIES', '', '']);
        data.liabilities.current_liabilities.items.forEach(item => {
          liabilitiesData.push([
            `  ${item.account_code || ''}`,
            item.account_name || '',
            formatCurrencyForPDF(item.amount || 0)
          ]);
        });
        liabilitiesData.push([
          'Subtotal Current Liabilities',
          '',
          formatCurrencyForPDF(data.liabilities.current_liabilities.total_current_liabilities || 0)
        ]);
        liabilitiesData.push(['', '', '']); // Empty row
      }
      
      // Non-Current Liabilities
      if (data.liabilities.non_current_liabilities?.items && data.liabilities.non_current_liabilities.items.length > 0) {
        liabilitiesData.push(['NON-CURRENT LIABILITIES', '', '']);
        data.liabilities.non_current_liabilities.items.forEach(item => {
          liabilitiesData.push([
            `  ${item.account_code || ''}`,
            item.account_name || '',
            formatCurrencyForPDF(item.amount || 0)
          ]);
        });
        liabilitiesData.push([
          'Subtotal Non-Current Liabilities',
          '',
          formatCurrencyForPDF(data.liabilities.non_current_liabilities.total_non_current_liabilities || 0)
        ]);
        liabilitiesData.push(['', '', '']); // Empty row
      }
      
      liabilitiesData.push([
        'TOTAL LIABILITIES',
        '',
        formatCurrencyForPDF(data.liabilities.total_liabilities || 0)
      ]);
      
      if (liabilitiesData.length > 0) {
        autoTable(doc, {
          startY: yPosition,
          head: [['Account Code', 'Account Name', 'Amount']],
          body: liabilitiesData,
          theme: 'grid',
          headStyles: { fillColor: [255, 193, 7], textColor: [33, 37, 41], fontStyle: 'bold' },
          bodyStyles: { fontSize: 9 },
          alternateRowStyles: { fillColor: [248, 249, 250] },
          margin: { left: 20, right: 20 },
          columnStyles: {
            2: { halign: 'right' }
          }
        } as any);
        
        yPosition = (doc as any).lastAutoTable.finalY + 10;
      }
    }
    
    // Check if we need a new page
    if (yPosition > pageHeight - 60) {
      doc.addPage();
      yPosition = 20;
    }
    
    // Equity section
    if (data.equity?.items && data.equity.items.length > 0) {
      doc.setFontSize(11);
      doc.setFont('helvetica', 'bold');
      doc.text('EQUITY', 20, yPosition);
      yPosition += 5;
      
      const equityData: any[] = [];
      
      data.equity.items.forEach(item => {
        equityData.push([
          item.account_code || '',
          item.account_name || '',
          formatCurrencyForPDF(item.amount || 0)
        ]);
      });
      
      equityData.push([
        'TOTAL EQUITY',
        '',
        formatCurrencyForPDF(data.equity.total_equity || 0)
      ]);
      
      autoTable(doc, {
        startY: yPosition,
        head: [['Account Code', 'Account Name', 'Amount']],
        body: equityData,
        theme: 'grid',
        headStyles: { fillColor: [23, 162, 184], textColor: 255, fontStyle: 'bold' },
        bodyStyles: { fontSize: 9 },
        alternateRowStyles: { fillColor: [248, 249, 250] },
        margin: { left: 20, right: 20 },
        columnStyles: {
          2: { halign: 'right' }
        }
      } as any);
      
      yPosition = (doc as any).lastAutoTable.finalY + 10;
    }
    } // End of includeAccountDetails
    
    // Footer
    const footerY = pageHeight - 20;
    doc.setFontSize(8);
    doc.setFont('helvetica', 'normal');
    doc.text('Generated by Sistem Akuntansi', 20, footerY);
    doc.text(`Generated on: ${new Date().toLocaleString('id-ID')}`, pageWidth - 20, footerY, { align: 'right' });
    doc.text(`Data Source: ${data.enhanced ? 'SSOT Enhanced' : 'SSOT Standard'}`, pageWidth / 2, footerY, { align: 'center' });
    
    // Add page numbers if multiple pages
    const pageCount = doc.internal.pages.length - 1;
    if (pageCount > 1) {
      for (let i = 1; i <= pageCount; i++) {
        doc.setPage(i);
        doc.setFontSize(8);
        doc.text(`Page ${i} of ${pageCount}`, pageWidth - 20, footerY - 5, { align: 'right' });
      }
    }
    
    return doc;
  } catch (err: any) {
    // Provide an informative error that surfaces in the toast
    const hint = 'Pastikan dependensi PDF terpasang dan kompatibel: jspdf >= 3 dan jspdf-autotable >= 5. Gunakan import autoTable dari "jspdf-autotable".';
    const message = `Gagal mengekspor PDF Balance Sheet: ${err?.message || err}. ${hint}`;
    throw new Error(message);
  }
}

// Note: Client-side download helpers moved to balanceSheetExportClient.ts
// This module now only contains pure generation functions (no DOM access)

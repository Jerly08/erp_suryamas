/**
 * Cash Flow Export Utilities
 * 
 * Provides enhanced CSV and PDF export functionality for Cash Flow reports
 * Supports SSOT cash flow data formats with detailed account information
 * Uses functional autoTable(doc, {...}) pattern for Turbopack/Next.js compatibility
 */

import jsPDF from 'jspdf';
import autoTable from 'jspdf-autotable';
import { SSOTCashFlowData } from '../services/ssotCashFlowReportService';

// ─── Theme colours ───────────────────────────────────────────────────────────
const COLORS = {
  primary: [25, 55, 109] as [number, number, number], // dark navy
  accent: [0, 119, 182] as [number, number, number], // cerulean
  operating: [0, 128, 94] as [number, number, number], // teal
  investing: [106, 27, 154] as [number, number, number], // purple
  financing: [230, 126, 34] as [number, number, number], // warm orange
  dark: [33, 37, 41] as [number, number, number],
  muted: [108, 117, 125] as [number, number, number],
  lightBg: [245, 247, 250] as [number, number, number],
  white: [255, 255, 255] as [number, number, number],
  red: [220, 53, 69] as [number, number, number],
  green: [40, 167, 69] as [number, number, number],
  line: [200, 210, 220] as [number, number, number],
};

// ─── Types ───────────────────────────────────────────────────────────────────
export interface CompanyLetterhead {
  companyName?: string;
  companyAddress?: string;
  companyPhone?: string;
  companyEmail?: string;
  companyWebsite?: string;
  taxNumber?: string;
  companyLogoBase64?: string; // pre-loaded logo as base64 data URL
}

// ─── Helpers ─────────────────────────────────────────────────────────────────
const fmt = (amount: number | null | undefined): string => {
  if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
  return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(Number(amount));
};

const fmtCSV = (amount: number | null | undefined): string => {
  if (amount === null || amount === undefined || isNaN(Number(amount))) return '0';
  return Number(amount).toLocaleString('id-ID');
};

const escapeCSV = (value: string | number | null | undefined): string => {
  if (value === null || value === undefined) return '';
  const str = String(value);
  return (str.includes(',') || str.includes('"') || str.includes('\n')) ? `"${str.replace(/"/g, '""')}"` : str;
};

const formatDateID = (dateStr: string): string => {
  try {
    return new Date(dateStr).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });
  } catch { return dateStr; }
};

// ─── PDF Letterhead ──────────────────────────────────────────────────────────
function drawLetterhead(doc: jsPDF, info: CompanyLetterhead, pageWidth: number): number {
  const margin = 20;
  const rightX = pageWidth - margin;
  let y = 15;

  // Logo — top-left (matching P&L Go backend layout)
  let logoBottomY = y;
  if (info.companyLogoBase64) {
    try {
      const logoW = 35; // mm, same as Go backend
      doc.addImage(info.companyLogoBase64, 'AUTO', margin, y - 3, logoW, 0);
      logoBottomY = y - 3 + logoW + 2; // approximate: logo height ≈ width for square logos
    } catch (e) {
      console.warn('Failed to add logo to PDF:', e);
    }
  }

  // Company name — bold, right-aligned
  doc.setFont('helvetica', 'bold');
  doc.setFontSize(14);
  doc.setTextColor(...COLORS.primary);
  const name = info.companyName || 'PT. Sistem Akuntansi';
  doc.text(name, rightX, y, { align: 'right' });
  y += 6;

  // Address — right-aligned
  if (info.companyAddress) {
    doc.setFont('helvetica', 'normal');
    doc.setFontSize(9);
    doc.setTextColor(...COLORS.dark);
    doc.text(info.companyAddress, rightX, y, { align: 'right' });
    y += 5;
  }

  // Phone — right-aligned
  if (info.companyPhone) {
    doc.setFont('helvetica', 'normal');
    doc.setFontSize(9);
    doc.setTextColor(...COLORS.dark);
    doc.text(`Phone: ${info.companyPhone}`, rightX, y, { align: 'right' });
    y += 5;
  }

  // Email — right-aligned
  if (info.companyEmail) {
    doc.setFont('helvetica', 'normal');
    doc.setFontSize(9);
    doc.setTextColor(...COLORS.dark);
    doc.text(`Email: ${info.companyEmail}`, rightX, y, { align: 'right' });
    y += 5;
  }

  // Ensure y is below logo area
  if (y < logoBottomY) y = logoBottomY;

  // Separator line
  y += 3;
  doc.setDrawColor(238, 238, 238);
  doc.setLineWidth(0.3);
  doc.line(margin, y, pageWidth - margin, y);
  y += 8;

  doc.setTextColor(0, 0, 0);
  return y;
}

// ─── PDF Page Footer ─────────────────────────────────────────────────────────
function drawFooter(doc: jsPDF, pageWidth: number, pageHeight: number) {
  const totalPages = (doc as any).getNumberOfPages();
  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    const footerY = pageHeight - 10;
    doc.setDrawColor(...COLORS.line);
    doc.setLineWidth(0.3);
    doc.line(20, footerY - 3, pageWidth - 20, footerY - 3);
    doc.setFontSize(7);
    doc.setFont('helvetica', 'normal');
    doc.setTextColor(...COLORS.muted);
    doc.text('Dicetak oleh Sistem Akuntansi', 20, footerY);
    doc.text(`Halaman ${i} dari ${totalPages}`, pageWidth - 20, footerY, { align: 'right' });
    doc.text(new Date().toLocaleString('id-ID'), pageWidth / 2, footerY, { align: 'center' });
  }
}

// ─── Ensure new page if near bottom ──────────────────────────────────────────
function ensureSpace(doc: jsPDF, y: number, needed: number, pageHeight: number): number {
  if (y + needed > pageHeight - 20) { doc.addPage(); return 20; }
  return y;
}

// ─── Section title ───────────────────────────────────────────────────────────
function sectionTitle(doc: jsPDF, text: string, y: number, color: [number, number, number]): number {
  doc.setFont('helvetica', 'bold');
  doc.setFontSize(11);
  doc.setTextColor(...color);
  doc.text(text, 20, y);
  doc.setTextColor(0, 0, 0);
  return y + 6;
}

// ═══════════════════════════════════════════════════════════════════════════════
// CSV EXPORT
// ═══════════════════════════════════════════════════════════════════════════════
export function exportCashFlowToCSV(
  data: SSOTCashFlowData,
  options: { includeAccountDetails?: boolean; companyName?: string } = {}
): string {
  const { includeAccountDetails = true, companyName } = options;
  const L: string[] = [];

  L.push(escapeCSV(companyName || data.company?.name || 'PT. Sistem Akuntansi'));
  L.push('LAPORAN ARUS KAS');
  L.push(`Periode: ${data.start_date} s/d ${data.end_date}`);
  L.push(`Dicetak: ${new Date().toLocaleString('id-ID')}`);
  L.push('');
  L.push('POSISI KAS');
  L.push('Metrik,Jumlah');
  L.push(`Arus Kas Bersih,${fmtCSV(data.net_cash_flow)}`);
  L.push(`Kas Awal Periode,${fmtCSV(data.cash_at_beginning)}`);
  L.push(`Kas Akhir Periode,${fmtCSV(data.cash_at_end)}`);
  L.push('');

  // Operating
  if (data.operating_activities) {
    const op = data.operating_activities;
    L.push('AKTIVITAS OPERASI');
    L.push('Kode Akun,Nama Akun,Jumlah');
    L.push(`-,Laba Bersih,${fmtCSV(op.net_income)}`);
    if (op.adjustments?.items?.length) {
      L.push('PENYESUAIAN NON-KAS,,');
      op.adjustments.items.forEach(i => L.push(`${escapeCSV(i.account_code)},${escapeCSV(i.account_name)},${fmtCSV(i.amount)}`));
      L.push(`,Total Penyesuaian,${fmtCSV(op.adjustments.total_adjustments)}`);
    }
    if (op.working_capital_changes?.items?.length) {
      L.push('PERUBAHAN MODAL KERJA,,');
      op.working_capital_changes.items.forEach(i => L.push(`${escapeCSV(i.account_code)},${escapeCSV(i.account_name)},${fmtCSV(i.amount)}`));
      L.push(`,Total Perubahan Modal Kerja,${fmtCSV(op.working_capital_changes.total_working_capital_changes)}`);
    }
    L.push(`TOTAL ARUS KAS OPERASI,,${fmtCSV(op.total_operating_cash_flow)}`);
    L.push('');
  }

  // Investing
  if (data.investing_activities) {
    const inv = data.investing_activities;
    L.push('AKTIVITAS INVESTASI');
    L.push('Kode Akun,Nama Akun,Jumlah');
    if (inv.items?.length) inv.items.forEach(i => L.push(`${escapeCSV(i.account_code)},${escapeCSV(i.account_name)},${fmtCSV(i.amount)}`));
    L.push(`TOTAL ARUS KAS INVESTASI,,${fmtCSV(inv.total_investing_cash_flow)}`);
    L.push('');
  }

  // Financing
  if (data.financing_activities) {
    const fin = data.financing_activities;
    L.push('AKTIVITAS PENDANAAN');
    L.push('Kode Akun,Nama Akun,Jumlah');
    if (fin.items?.length) fin.items.forEach(i => L.push(`${escapeCSV(i.account_code)},${escapeCSV(i.account_name)},${fmtCSV(i.amount)}`));
    L.push(`TOTAL ARUS KAS PENDANAAN,,${fmtCSV(fin.total_financing_cash_flow)}`);
    L.push('');
  }

  // Ratios
  if (data.cash_flow_ratios) {
    L.push('RASIO ARUS KAS');
    L.push('Rasio,Nilai');
    if (data.cash_flow_ratios.operating_cash_flow_ratio !== undefined) L.push(`Operating CF Ratio,${data.cash_flow_ratios.operating_cash_flow_ratio}`);
    if (data.cash_flow_ratios.free_cash_flow !== undefined) L.push(`Free Cash Flow,${fmtCSV(data.cash_flow_ratios.free_cash_flow)}`);
    if (data.cash_flow_ratios.cash_flow_to_debt_ratio !== undefined) L.push(`CF to Debt Ratio,${data.cash_flow_ratios.cash_flow_to_debt_ratio}`);
    L.push('');
  }

  L.push('Dicetak oleh Sistem Akuntansi');
  return L.join('\n');
}

// ═══════════════════════════════════════════════════════════════════════════════
// PDF EXPORT
// ═══════════════════════════════════════════════════════════════════════════════
export function exportCashFlowToPDF(
  data: SSOTCashFlowData,
  options: {
    companyName?: string;
    logoUrl?: string;
    includeAccountDetails?: boolean;
    letterhead?: CompanyLetterhead;
  } = {}
): jsPDF {
  const { companyName, includeAccountDetails = true, letterhead } = options;
  const doc = new jsPDF('portrait', 'mm', 'a4');
  const pw = doc.internal.pageSize.width;
  const ph = doc.internal.pageSize.height;

  // ── Letterhead ──
  const lh: CompanyLetterhead = letterhead || { companyName: companyName || data.company?.name };
  let y = drawLetterhead(doc, lh, pw);

  // ── Report title ──
  doc.setFont('helvetica', 'bold');
  doc.setFontSize(14);
  doc.setTextColor(...COLORS.primary);
  doc.text('LAPORAN ARUS KAS', pw / 2, y, { align: 'center' });
  y += 6;
  doc.setFont('helvetica', 'normal');
  doc.setFontSize(9);
  doc.setTextColor(...COLORS.muted);
  doc.text(`Periode: ${formatDateID(data.start_date)} s/d ${formatDateID(data.end_date)}`, pw / 2, y, { align: 'center' });
  y += 10;

  doc.setTextColor(0, 0, 0);

  // ════════════════════════════════════════════════════════════════════════════
  // 1. RINGKASAN POSISI KAS
  // ════════════════════════════════════════════════════════════════════════════
  y = sectionTitle(doc, 'RINGKASAN POSISI KAS', y, COLORS.primary);
  autoTable(doc, {
    startY: y,
    head: [['Keterangan', 'Jumlah']],
    body: [
      ['Kas Awal Periode', fmt(data.cash_at_beginning)],
      ['Arus Kas Bersih', fmt(data.net_cash_flow)],
      ['Kas Akhir Periode', fmt(data.cash_at_end)],
    ],
    theme: 'grid',
    headStyles: { fillColor: COLORS.primary, textColor: 255, fontStyle: 'bold', fontSize: 9 },
    bodyStyles: { fontSize: 9 },
    alternateRowStyles: { fillColor: COLORS.lightBg },
    margin: { left: 20, right: 20 },
    columnStyles: { 1: { halign: 'right' } },
  } as any);
  y = (doc as any).lastAutoTable.finalY + 10;

  // ════════════════════════════════════════════════════════════════════════════
  // 2. AKTIVITAS OPERASI
  // ════════════════════════════════════════════════════════════════════════════
  if (data.operating_activities) {
    const op = data.operating_activities;
    y = ensureSpace(doc, y, 40, ph);
    y = sectionTitle(doc, 'A. AKTIVITAS OPERASI', y, COLORS.operating);

    // Net Income row
    autoTable(doc, {
      startY: y,
      head: [['Keterangan', 'Jumlah']],
      body: [['Laba Bersih (dari Laporan Laba Rugi)', fmt(op.net_income)]],
      theme: 'grid',
      headStyles: { fillColor: COLORS.operating, textColor: 255, fontStyle: 'bold', fontSize: 9 },
      bodyStyles: { fontSize: 9, fontStyle: 'bold' },
      margin: { left: 20, right: 20 },
      columnStyles: { 1: { halign: 'right' } },
    } as any);
    y = (doc as any).lastAutoTable.finalY + 4;

    // Adjustments
    if (op.adjustments) {
      const adjItems: any[] = [];
      if (op.adjustments.items?.length) {
        op.adjustments.items.forEach(i => adjItems.push([i.account_code || '', i.account_name || '', fmt(i.amount)]));
      } else {
        if (op.adjustments.depreciation) adjItems.push(['', 'Penyusutan', fmt(op.adjustments.depreciation)]);
        if (op.adjustments.amortization) adjItems.push(['', 'Amortisasi', fmt(op.adjustments.amortization)]);
        if (op.adjustments.bad_debt_expense) adjItems.push(['', 'Beban Piutang Tak Tertagih', fmt(op.adjustments.bad_debt_expense)]);
        if (op.adjustments.gain_loss_on_asset_disposal) adjItems.push(['', 'Laba/Rugi Pelepasan Aset', fmt(op.adjustments.gain_loss_on_asset_disposal)]);
        if (op.adjustments.other_non_cash_items) adjItems.push(['', 'Item Non-Kas Lainnya', fmt(op.adjustments.other_non_cash_items)]);
      }
      if (adjItems.length) {
        adjItems.push([{ content: 'Total Penyesuaian', colSpan: 2, styles: { fontStyle: 'bold' } }, { content: fmt(op.adjustments.total_adjustments), styles: { fontStyle: 'bold', halign: 'right' } }]);
        autoTable(doc, {
          startY: y,
          head: [['Kode', 'Penyesuaian Non-Kas', 'Jumlah']],
          body: adjItems,
          theme: 'striped',
          headStyles: { fillColor: COLORS.dark, textColor: 255, fontSize: 8 },
          bodyStyles: { fontSize: 8 },
          margin: { left: 25, right: 20 },
          columnStyles: { 0: { cellWidth: 25 }, 2: { halign: 'right', cellWidth: 40 } },
        } as any);
        y = (doc as any).lastAutoTable.finalY + 3;
      }
    }

    // Working Capital Changes
    if (op.working_capital_changes) {
      const wcItems: any[] = [];
      if (op.working_capital_changes.items?.length) {
        op.working_capital_changes.items.forEach(i => wcItems.push([i.account_code || '', i.account_name || '', fmt(i.amount)]));
      } else {
        if (op.working_capital_changes.accounts_receivable_change) wcItems.push(['', 'Perub. Piutang Usaha', fmt(op.working_capital_changes.accounts_receivable_change)]);
        if (op.working_capital_changes.inventory_change) wcItems.push(['', 'Perub. Persediaan', fmt(op.working_capital_changes.inventory_change)]);
        if (op.working_capital_changes.prepaid_expenses_change) wcItems.push(['', 'Perub. Beban Dibayar Dimuka', fmt(op.working_capital_changes.prepaid_expenses_change)]);
        if (op.working_capital_changes.accounts_payable_change) wcItems.push(['', 'Perub. Utang Usaha', fmt(op.working_capital_changes.accounts_payable_change)]);
        if (op.working_capital_changes.accrued_liabilities_change) wcItems.push(['', 'Perub. Utang Akrual', fmt(op.working_capital_changes.accrued_liabilities_change)]);
        if (op.working_capital_changes.other_working_capital_change) wcItems.push(['', 'Perub. Modal Kerja Lainnya', fmt(op.working_capital_changes.other_working_capital_change)]);
      }
      if (wcItems.length) {
        wcItems.push([{ content: 'Total Perubahan Modal Kerja', colSpan: 2, styles: { fontStyle: 'bold' } }, { content: fmt(op.working_capital_changes.total_working_capital_changes), styles: { fontStyle: 'bold', halign: 'right' } }]);
        y = ensureSpace(doc, y, 30, ph);
        autoTable(doc, {
          startY: y,
          head: [['Kode', 'Perubahan Modal Kerja', 'Jumlah']],
          body: wcItems,
          theme: 'striped',
          headStyles: { fillColor: COLORS.dark, textColor: 255, fontSize: 8 },
          bodyStyles: { fontSize: 8 },
          margin: { left: 25, right: 20 },
          columnStyles: { 0: { cellWidth: 25 }, 2: { halign: 'right', cellWidth: 40 } },
        } as any);
        y = (doc as any).lastAutoTable.finalY + 3;
      }
    }

    // Total Operating
    autoTable(doc, {
      startY: y,
      body: [['TOTAL ARUS KAS DARI AKTIVITAS OPERASI', fmt(op.total_operating_cash_flow)]],
      theme: 'plain',
      bodyStyles: { fontSize: 10, fontStyle: 'bold', textColor: COLORS.operating },
      margin: { left: 20, right: 20 },
      columnStyles: { 1: { halign: 'right', cellWidth: 50 } },
    } as any);
    y = (doc as any).lastAutoTable.finalY + 8;
  }

  // ════════════════════════════════════════════════════════════════════════════
  // 3. AKTIVITAS INVESTASI
  // ════════════════════════════════════════════════════════════════════════════
  if (data.investing_activities) {
    const inv = data.investing_activities;
    y = ensureSpace(doc, y, 40, ph);
    y = sectionTitle(doc, 'B. AKTIVITAS INVESTASI', y, COLORS.investing);

    const invItems: any[] = [];
    if (inv.items?.length) {
      inv.items.forEach(i => invItems.push([i.account_code || '', i.account_name || '', fmt(i.amount)]));
    } else {
      if (inv.purchase_of_fixed_assets) invItems.push(['', 'Pembelian Aset Tetap', fmt(inv.purchase_of_fixed_assets)]);
      if (inv.sale_of_fixed_assets) invItems.push(['', 'Penjualan Aset Tetap', fmt(inv.sale_of_fixed_assets)]);
      if (inv.purchase_of_investments) invItems.push(['', 'Pembelian Investasi', fmt(inv.purchase_of_investments)]);
      if (inv.sale_of_investments) invItems.push(['', 'Penjualan Investasi', fmt(inv.sale_of_investments)]);
      if (inv.intangible_asset_purchases) invItems.push(['', 'Pembelian Aset Tak Berwujud', fmt(inv.intangible_asset_purchases)]);
      if (inv.other_investing_activities) invItems.push(['', 'Aktivitas Investasi Lainnya', fmt(inv.other_investing_activities)]);
    }

    if (invItems.length) {
      autoTable(doc, {
        startY: y,
        head: [['Kode', 'Aktivitas Investasi', 'Jumlah']],
        body: invItems,
        theme: 'striped',
        headStyles: { fillColor: COLORS.investing, textColor: 255, fontSize: 8 },
        bodyStyles: { fontSize: 8 },
        alternateRowStyles: { fillColor: COLORS.lightBg },
        margin: { left: 20, right: 20 },
        columnStyles: { 0: { cellWidth: 25 }, 2: { halign: 'right', cellWidth: 40 } },
      } as any);
      y = (doc as any).lastAutoTable.finalY + 3;
    }

    autoTable(doc, {
      startY: y,
      body: [['TOTAL ARUS KAS DARI AKTIVITAS INVESTASI', fmt(inv.total_investing_cash_flow)]],
      theme: 'plain',
      bodyStyles: { fontSize: 10, fontStyle: 'bold', textColor: COLORS.investing },
      margin: { left: 20, right: 20 },
      columnStyles: { 1: { halign: 'right', cellWidth: 50 } },
    } as any);
    y = (doc as any).lastAutoTable.finalY + 8;
  }

  // ════════════════════════════════════════════════════════════════════════════
  // 4. AKTIVITAS PENDANAAN
  // ════════════════════════════════════════════════════════════════════════════
  if (data.financing_activities) {
    const fin = data.financing_activities;
    y = ensureSpace(doc, y, 40, ph);
    y = sectionTitle(doc, 'C. AKTIVITAS PENDANAAN', y, COLORS.financing);

    const finItems: any[] = [];
    if (fin.items?.length) {
      fin.items.forEach(i => finItems.push([i.account_code || '', i.account_name || '', fmt(i.amount)]));
    } else {
      if (fin.share_capital_increase) finItems.push(['', 'Penambahan Modal Saham', fmt(fin.share_capital_increase)]);
      if (fin.share_capital_decrease) finItems.push(['', 'Pengurangan Modal Saham', fmt(fin.share_capital_decrease)]);
      if (fin.long_term_debt_increase) finItems.push(['', 'Penambahan Utang Jangka Panjang', fmt(fin.long_term_debt_increase)]);
      if (fin.long_term_debt_decrease) finItems.push(['', 'Pelunasan Utang Jangka Panjang', fmt(fin.long_term_debt_decrease)]);
      if (fin.short_term_debt_increase) finItems.push(['', 'Penambahan Utang Jangka Pendek', fmt(fin.short_term_debt_increase)]);
      if (fin.short_term_debt_decrease) finItems.push(['', 'Pelunasan Utang Jangka Pendek', fmt(fin.short_term_debt_decrease)]);
      if (fin.dividends_paid) finItems.push(['', 'Dividen Dibayarkan', fmt(fin.dividends_paid)]);
      if (fin.other_financing_activities) finItems.push(['', 'Aktivitas Pendanaan Lainnya', fmt(fin.other_financing_activities)]);
    }

    if (finItems.length) {
      autoTable(doc, {
        startY: y,
        head: [['Kode', 'Aktivitas Pendanaan', 'Jumlah']],
        body: finItems,
        theme: 'striped',
        headStyles: { fillColor: COLORS.financing, textColor: 255, fontSize: 8 },
        bodyStyles: { fontSize: 8 },
        alternateRowStyles: { fillColor: COLORS.lightBg },
        margin: { left: 20, right: 20 },
        columnStyles: { 0: { cellWidth: 25 }, 2: { halign: 'right', cellWidth: 40 } },
      } as any);
      y = (doc as any).lastAutoTable.finalY + 3;
    }

    autoTable(doc, {
      startY: y,
      body: [['TOTAL ARUS KAS DARI AKTIVITAS PENDANAAN', fmt(fin.total_financing_cash_flow)]],
      theme: 'plain',
      bodyStyles: { fontSize: 10, fontStyle: 'bold', textColor: COLORS.financing },
      margin: { left: 20, right: 20 },
      columnStyles: { 1: { halign: 'right', cellWidth: 50 } },
    } as any);
    y = (doc as any).lastAutoTable.finalY + 10;
  }

  // ════════════════════════════════════════════════════════════════════════════
  // 5. RINGKASAN AKHIR
  // ════════════════════════════════════════════════════════════════════════════
  y = ensureSpace(doc, y, 50, ph);
  y = sectionTitle(doc, 'RINGKASAN ARUS KAS', y, COLORS.primary);

  const op = data.operating_activities;
  const inv = data.investing_activities;
  const fin = data.financing_activities;

  autoTable(doc, {
    startY: y,
    head: [['Keterangan', 'Jumlah']],
    body: [
      ['Kas Awal Periode', fmt(data.cash_at_beginning)],
      ['Arus Kas dari Aktivitas Operasi (A)', fmt(op?.total_operating_cash_flow || 0)],
      ['Arus Kas dari Aktivitas Investasi (B)', fmt(inv?.total_investing_cash_flow || 0)],
      ['Arus Kas dari Aktivitas Pendanaan (C)', fmt(fin?.total_financing_cash_flow || 0)],
      [{ content: 'Kenaikan (Penurunan) Bersih Kas', styles: { fontStyle: 'bold' } }, { content: fmt(data.net_cash_flow), styles: { fontStyle: 'bold', halign: 'right' } }],
      [{ content: 'Kas Akhir Periode', styles: { fontStyle: 'bold', fillColor: [232, 245, 233] } }, { content: fmt(data.cash_at_end), styles: { fontStyle: 'bold', halign: 'right', fillColor: [232, 245, 233] } }],
    ],
    theme: 'grid',
    headStyles: { fillColor: COLORS.primary, textColor: 255, fontStyle: 'bold', fontSize: 9 },
    bodyStyles: { fontSize: 9 },
    alternateRowStyles: { fillColor: COLORS.lightBg },
    margin: { left: 20, right: 20 },
    columnStyles: { 1: { halign: 'right', cellWidth: 50 } },
  } as any);
  y = (doc as any).lastAutoTable.finalY + 10;

  // ════════════════════════════════════════════════════════════════════════════
  // 6. RASIO ARUS KAS (if available)
  // ════════════════════════════════════════════════════════════════════════════
  if (data.cash_flow_ratios) {
    y = ensureSpace(doc, y, 40, ph);
    y = sectionTitle(doc, 'RASIO ARUS KAS', y, COLORS.accent);

    const ratiosBody: any[] = [];
    if (data.cash_flow_ratios.operating_cash_flow_ratio !== undefined)
      ratiosBody.push(['Operating Cash Flow Ratio', `${data.cash_flow_ratios.operating_cash_flow_ratio.toFixed(2)}x`]);
    if (data.cash_flow_ratios.free_cash_flow !== undefined)
      ratiosBody.push(['Free Cash Flow', fmt(data.cash_flow_ratios.free_cash_flow)]);
    if (data.cash_flow_ratios.cash_flow_to_debt_ratio !== undefined)
      ratiosBody.push(['Cash Flow to Debt Ratio', `${data.cash_flow_ratios.cash_flow_to_debt_ratio.toFixed(2)}x`]);
    if (data.cash_flow_ratios.cash_flow_per_share !== undefined)
      ratiosBody.push(['Cash Flow per Share', `${data.cash_flow_ratios.cash_flow_per_share}`]);

    if (ratiosBody.length) {
      autoTable(doc, {
        startY: y,
        head: [['Rasio', 'Nilai']],
        body: ratiosBody,
        theme: 'grid',
        headStyles: { fillColor: COLORS.accent, textColor: 255, fontSize: 9 },
        bodyStyles: { fontSize: 9 },
        alternateRowStyles: { fillColor: COLORS.lightBg },
        margin: { left: 20, right: 20 },
        columnStyles: { 1: { halign: 'right' } },
      } as any);
      y = (doc as any).lastAutoTable.finalY + 10;
    }
  }

  // ════════════════════════════════════════════════════════════════════════════
  // 7. RINCIAN AKUN (if includeAccountDetails)
  // ════════════════════════════════════════════════════════════════════════════
  if (includeAccountDetails && data.account_details?.length) {
    y = ensureSpace(doc, y, 40, ph);
    y = sectionTitle(doc, 'RINCIAN AKUN', y, COLORS.dark);

    const detailBody: any[] = [];
    data.account_details.forEach(a => {
      detailBody.push([a.account_code || '', a.account_name || '', a.account_type || '', fmt(a.debit_total), fmt(a.credit_total), fmt(a.net_balance)]);
    });

    autoTable(doc, {
      startY: y,
      head: [['Kode', 'Nama Akun', 'Tipe', 'Debit', 'Kredit', 'Saldo Bersih']],
      body: detailBody,
      theme: 'grid',
      headStyles: { fillColor: COLORS.dark, textColor: 255, fontSize: 7 },
      bodyStyles: { fontSize: 7 },
      alternateRowStyles: { fillColor: COLORS.lightBg },
      margin: { left: 20, right: 20 },
      columnStyles: { 3: { halign: 'right' }, 4: { halign: 'right' }, 5: { halign: 'right' } },
    } as any);
  }

  // ── Footer on all pages ──
  drawFooter(doc, pw, ph);

  return doc;
}

// ─── Convenience wrappers ────────────────────────────────────────────────────
export function downloadCashFlowPDF(doc: jsPDF, filename?: string): void {
  doc.save(filename || `Arus_Kas_${new Date().toISOString().split('T')[0]}.pdf`);
}

export function exportAndDownloadCashFlowPDF(
  data: SSOTCashFlowData,
  options?: { companyName?: string; logoUrl?: string; includeAccountDetails?: boolean; filename?: string; letterhead?: CompanyLetterhead }
): void {
  const doc = exportCashFlowToPDF(data, options);
  downloadCashFlowPDF(doc, options?.filename);
}
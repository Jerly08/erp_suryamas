import jsPDF from 'jspdf';
import autoTable from 'jspdf-autotable';
import { SSOTJournalEntry } from '@/services/ssotJournalService';
import { SSOTGeneralLedgerData } from '@/services/ssotGeneralLedgerService';
import { SSOTBalanceSheetData } from '@/services/ssotBalanceSheetReportService';

// ===== HELPERS =====
const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
};

const formatDate = (dateString: string): string => {
  if (!dateString) return '-';
  return new Date(dateString).toLocaleDateString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric'
  });
};

const formatDateShort = (dateString: string): string => {
  if (!dateString) return '-';
  return new Date(dateString).toLocaleDateString('id-ID', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric'
  });
};

const addHeader = (doc: jsPDF, title: string, subtitle: string, yStart: number = 20): number => {
  let yPos = yStart;

  doc.setFontSize(14);
  doc.setFont('helvetica', 'bold');
  doc.text(title, doc.internal.pageSize.width / 2, yPos, { align: 'center' });

  yPos += 7;
  doc.setFontSize(11);
  doc.setFont('helvetica', 'normal');
  doc.text(subtitle, doc.internal.pageSize.width / 2, yPos, { align: 'center' });

  yPos += 10;
  return yPos;
};

const addFooter = (doc: jsPDF) => {
  const pageCount = (doc as any).getNumberOfPages();
  for (let i = 1; i <= pageCount; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.setFont('helvetica', 'normal');
    doc.text(
      `Halaman ${i} dari ${pageCount}`,
      doc.internal.pageSize.width / 2,
      doc.internal.pageSize.height - 10,
      { align: 'center' }
    );
    doc.text(
      `Digenerate pada: ${new Date().toLocaleString('id-ID')}`,
      14,
      doc.internal.pageSize.height - 10
    );
  }
};

const triggerDownload = (blob: Blob, filename: string) => {
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// ===== JURNAL UMUM (General Journal) PDF =====
export const exportGeneralJournalToPDF = (
  entries: SSOTJournalEntry[],
  startDate: string,
  endDate: string
): void => {
  const doc = new jsPDF({ orientation: 'landscape' });

  let yPos = addHeader(doc, 'Jurnal Umum', `Periode: ${formatDateShort(startDate)} - ${formatDateShort(endDate)}`);

  // Summary
  const totalDebit = entries.reduce((sum, e) => sum + (Number(e.total_debit) || 0), 0);
  const totalCredit = entries.reduce((sum, e) => sum + (Number(e.total_credit) || 0), 0);
  doc.setFontSize(9);
  doc.text(`Total Entri: ${entries.length}  |  Total Debit: ${formatCurrency(totalDebit)}  |  Total Kredit: ${formatCurrency(totalCredit)}`, 14, yPos);
  yPos += 8;

  // Build table rows - flatten entries with their lines
  const tableData: any[][] = [];
  entries.forEach((entry) => {
    const lines = entry.lines || [];
    lines.forEach((line, index) => {
      tableData.push([
        index === 0 ? formatDate(entry.entry_date) : '',
        index === 0 ? entry.entry_number : '',
        line.account ? `${line.account.code} - ${line.account.name}` : '-',
        line.description || '',
        index === 0 ? (entry.reference || '-') : '',
        line.debit_amount > 0 ? formatCurrency(line.debit_amount) : '-',
        line.credit_amount > 0 ? formatCurrency(line.credit_amount) : '-',
        index === 0 ? entry.status : '',
      ]);
    });
  });

  autoTable(doc, {
    startY: yPos,
    head: [['Tanggal', 'Nomor', 'Akun', 'Deskripsi', 'Ref', 'Debit', 'Kredit', 'Status']],
    body: tableData,
    styles: { fontSize: 8, cellPadding: 2, overflow: 'linebreak' },
    headStyles: { fillColor: [41, 128, 185], textColor: 255, fontSize: 8, fontStyle: 'bold' },
    alternateRowStyles: { fillColor: [245, 245, 245] },
    columnStyles: {
      0: { cellWidth: 25 },
      1: { cellWidth: 35 },
      2: { cellWidth: 45 },
      3: { cellWidth: 60 },
      4: { cellWidth: 28 },
      5: { halign: 'right', cellWidth: 32 },
      6: { halign: 'right', cellWidth: 32 },
      7: { halign: 'center', cellWidth: 20 },
    },
    margin: { left: 14, right: 14 },
    theme: 'striped',
  });

  addFooter(doc);

  const fileName = `Jurnal_Umum_${startDate}_${endDate}.pdf`;
  doc.save(fileName);
};

// ===== JURNAL UMUM (General Journal) CSV =====
export const exportGeneralJournalToCSV = (
  entries: SSOTJournalEntry[],
  startDate: string,
  endDate: string
): void => {
  const headers = ['Tanggal', 'Nomor', 'Sumber', 'Akun Kode', 'Akun Nama', 'Deskripsi', 'Ref', 'Debit', 'Kredit', 'Status'];
  const rows: string[][] = [];

  entries.forEach((entry) => {
    const lines = entry.lines || [];
    lines.forEach((line, index) => {
      rows.push([
        index === 0 ? formatDateShort(entry.entry_date) : '',
        index === 0 ? entry.entry_number : '',
        index === 0 ? entry.source_type : '',
        line.account?.code || '',
        line.account?.name || '',
        line.description || '',
        index === 0 ? (entry.reference || '') : '',
        line.debit_amount > 0 ? line.debit_amount.toString() : '0',
        line.credit_amount > 0 ? line.credit_amount.toString() : '0',
        index === 0 ? entry.status : '',
      ]);
    });
  });

  const csvContent = [
    headers.join(','),
    ...rows.map(row => row.map(cell => `"${cell.replace(/"/g, '""')}"`).join(','))
  ].join('\n');

  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' });
  triggerDownload(blob, `Jurnal_Umum_${startDate}_${endDate}.csv`);
};

// ===== BUKU BESAR (General Ledger) PDF - Client-side fallback =====
export const exportGeneralLedgerToPDF = (
  data: SSOTGeneralLedgerData,
  startDate: string,
  endDate: string
): void => {
  const doc = new jsPDF();
  const isAllAccountsMode = !data.account?.account_code;

  const accountLabel = data.account?.account_code ? `${data.account.account_code} - ${data.account.account_name}` : 'Semua Akun';
  let yPos = addHeader(doc, 'Buku Besar', `${accountLabel}\nPeriode: ${formatDateShort(startDate)} - ${formatDateShort(endDate)}`);
  yPos += 4;

  // Summary
  doc.setFontSize(9);
  if (isAllAccountsMode) {
    doc.text(`Mutasi Debit Periode: ${formatCurrency(data.total_debits || 0)}`, 14, yPos);
    doc.text(`Mutasi Kredit Periode: ${formatCurrency(data.total_credits || 0)}`, 108, yPos);
    yPos += 5;
    doc.setTextColor(100);
    doc.text('Mode Semua Akun menampilkan total mutasi jurnal periode, bukan saldo akhir neraca.', 14, yPos);
    doc.setTextColor(0);
    yPos += 8;
  } else {
    doc.text(`Saldo Awal: ${formatCurrency(data.opening_balance || 0)}`, 14, yPos);
    doc.text(`Total Debit: ${formatCurrency(data.total_debits || 0)}`, 80, yPos);
    doc.text(`Total Kredit: ${formatCurrency(data.total_credits || 0)}`, 140, yPos);
    yPos += 5;
    doc.setFont('helvetica', 'bold');
    doc.text(`Saldo Akhir: ${formatCurrency(data.closing_balance || 0)}`, 14, yPos);
    doc.setFont('helvetica', 'normal');
    yPos += 8;
  }

  const transactions = data.transactions || data.entries || [];
  const tableData = transactions.map((t) => (
    isAllAccountsMode
      ? [
          formatDate(t.date),
          t.journal_code || '',
          t.description || '',
          t.debit_amount > 0 ? formatCurrency(t.debit_amount) : '-',
          t.credit_amount > 0 ? formatCurrency(t.credit_amount) : '-',
        ]
      : [
          formatDate(t.date),
          t.journal_code || '',
          t.description || '',
          t.debit_amount > 0 ? formatCurrency(t.debit_amount) : '-',
          t.credit_amount > 0 ? formatCurrency(t.credit_amount) : '-',
          formatCurrency(t.balance),
        ]
  ));

  autoTable(doc, {
    startY: yPos,
    head: [isAllAccountsMode
      ? ['Tanggal', 'Ref', 'Deskripsi', 'Debit', 'Kredit']
      : ['Tanggal', 'Ref', 'Deskripsi', 'Debit', 'Kredit', 'Saldo']],
    body: tableData,
    styles: { fontSize: 8, cellPadding: 2, overflow: 'linebreak' },
    headStyles: { fillColor: [41, 128, 185], textColor: 255, fontSize: 8, fontStyle: 'bold' },
    alternateRowStyles: { fillColor: [245, 245, 245] },
    columnStyles: isAllAccountsMode
      ? {
          0: { cellWidth: 25 },
          1: { cellWidth: 35 },
          2: { cellWidth: 75 },
          3: { halign: 'right', cellWidth: 28 },
          4: { halign: 'right', cellWidth: 28 },
        }
      : {
          0: { cellWidth: 25 },
          1: { cellWidth: 30 },
          2: { cellWidth: 55 },
          3: { halign: 'right', cellWidth: 28 },
          4: { halign: 'right', cellWidth: 28 },
          5: { halign: 'right', cellWidth: 28 },
        },
    margin: { left: 14, right: 14 },
    theme: 'striped',
  });

  addFooter(doc);
  doc.save(`Buku_Besar_${startDate}_${endDate}.pdf`);
};

// ===== BUKU BESAR (General Ledger) CSV - Client-side fallback =====
export const exportGeneralLedgerToCSV = (
  data: SSOTGeneralLedgerData,
  startDate: string,
  endDate: string
): void => {
  const isAllAccountsMode = !data.account?.account_code;
  const accountCode = data.account?.account_code || '';
  const accountName = data.account?.account_name || 'Semua Akun';
  const transactions = data.transactions || data.entries || [];

  const headers = isAllAccountsMode
    ? ['Tanggal', 'Ref', 'Akun Kode', 'Akun Nama', 'Deskripsi', 'Debit', 'Kredit']
    : ['Tanggal', 'Ref', 'Akun Kode', 'Akun Nama', 'Deskripsi', 'Debit', 'Kredit', 'Saldo'];
  const rows: string[][] = [];

  transactions.forEach((t) => {
    rows.push(
      isAllAccountsMode
        ? [
            formatDateShort(t.date),
            t.journal_code || '',
            t.account_code || accountCode,
            t.account_name || accountName,
            t.description || '',
            t.debit_amount > 0 ? t.debit_amount.toString() : '0',
            t.credit_amount > 0 ? t.credit_amount.toString() : '0',
          ]
        : [
            formatDateShort(t.date),
            t.journal_code || '',
            t.account_code || accountCode,
            t.account_name || accountName,
            t.description || '',
            t.debit_amount > 0 ? t.debit_amount.toString() : '0',
            t.credit_amount > 0 ? t.credit_amount.toString() : '0',
            (Number(t.balance) || 0).toString(),
          ]
    );
  });

  // Summary rows at the end
  rows.push([]);
  if (isAllAccountsMode) {
    rows.push(['', '', '', '', 'Mutasi Debit Periode', (Number(data.total_debits) || 0).toString(), '', '']);
    rows.push(['', '', '', '', 'Mutasi Kredit Periode', '', (Number(data.total_credits) || 0).toString(), '']);
    rows.push(['', '', '', '', 'Catatan', 'Mode Semua Akun menampilkan total mutasi jurnal periode, bukan saldo akhir neraca.', '', '']);
  } else {
    rows.push(['', '', '', '', 'Saldo Awal', (Number(data.opening_balance) || 0).toString(), '', '']);
    rows.push(['', '', '', '', 'Total Debit', (Number(data.total_debits) || 0).toString(), '', '']);
    rows.push(['', '', '', '', 'Total Kredit', '', (Number(data.total_credits) || 0).toString(), '']);
    rows.push(['', '', '', '', 'Saldo Akhir', '', '', (Number(data.closing_balance) || 0).toString()]);
  }

  const csvContent = [
    headers.join(','),
    ...rows.map(row => row.map(cell => `"${String(cell || '').replace(/"/g, '""')}"`).join(','))
  ].join('\n');

  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' });
  triggerDownload(blob, `Buku_Besar_${startDate}_${endDate}.csv`);
};

// ===== NERACA (Balance Sheet) PDF - Client-side fallback =====
export const exportBalanceSheetToPDF = (
  data: SSOTBalanceSheetData,
  asOfDate: string
): void => {
  const doc = new jsPDF();

  let yPos = 20;
  doc.setFontSize(16);
  doc.setFont('helvetica', 'bold');
  doc.text(data.company?.name || 'Perusahaan', doc.internal.pageSize.width / 2, yPos, { align: 'center' });
  yPos += 7;
  doc.setFontSize(13);
  doc.text('Neraca', doc.internal.pageSize.width / 2, yPos, { align: 'center' });
  yPos += 6;
  doc.setFontSize(10);
  doc.setFont('helvetica', 'normal');
  doc.text(`Per ${new Date(asOfDate).toLocaleDateString('id-ID', { dateStyle: 'long' })}`, doc.internal.pageSize.width / 2, yPos, { align: 'center' });
  yPos += 12;

  const pageWidth = doc.internal.pageSize.width;
  const marginLeft = 14;
  const marginRight = 14;
  const colWidth = (pageWidth - marginLeft - marginRight);

  const addSection = (title: string, items: { account_code: string; account_name: string; amount: number }[], total: number, totalLabel: string) => {
    doc.setFontSize(10);
    doc.setFont('helvetica', 'bold');
    doc.text(title, marginLeft, yPos);
    yPos += 6;

    doc.setFont('helvetica', 'normal');
    doc.setFontSize(9);
    items.forEach(item => {
      doc.text(`  ${item.account_code} - ${item.account_name}`, marginLeft, yPos);
      doc.text(formatCurrency(item.amount), pageWidth - marginRight, yPos, { align: 'right' });
      yPos += 5;
    });

    doc.setFont('helvetica', 'bold');
    doc.setFillColor(240, 240, 240);
    doc.rect(marginLeft, yPos - 3.5, colWidth, 6, 'F');
    doc.text(`  ${totalLabel}`, marginLeft, yPos);
    doc.text(formatCurrency(total), pageWidth - marginRight, yPos, { align: 'right' });
    yPos += 8;
  };

  // ASSETS
  doc.setFontSize(12);
  doc.setFont('helvetica', 'bold');
  doc.setTextColor(0, 128, 0);
  doc.text('ASET', marginLeft, yPos);
  doc.setTextColor(0, 0, 0);
  yPos += 7;

  addSection('Aset Lancar', data.assets.current_assets.items, data.assets.current_assets.total_current_assets, 'Total Aset Lancar');
  addSection('Aset Tidak Lancar', data.assets.non_current_assets.items, data.assets.non_current_assets.total_non_current_assets, 'Total Aset Tidak Lancar');

  doc.setFontSize(11);
  doc.setFont('helvetica', 'bold');
  doc.setFillColor(200, 230, 200);
  doc.rect(marginLeft, yPos - 3.5, colWidth, 7, 'F');
  doc.text('TOTAL ASET', marginLeft + 2, yPos);
  doc.text(formatCurrency(data.assets.total_assets), pageWidth - marginRight, yPos, { align: 'right' });
  yPos += 12;

  // Check if we need a new page
  if (yPos > doc.internal.pageSize.height - 80) {
    doc.addPage();
    yPos = 20;
  }

  // LIABILITIES & EQUITY
  doc.setFontSize(12);
  doc.setFont('helvetica', 'bold');
  doc.setTextColor(200, 0, 0);
  doc.text('LIABILITAS & EKUITAS', marginLeft, yPos);
  doc.setTextColor(0, 0, 0);
  yPos += 7;

  addSection('Liabilitas Jangka Pendek', data.liabilities.current_liabilities.items, data.liabilities.current_liabilities.total_current_liabilities, 'Total Liabilitas Jangka Pendek');
  addSection('Liabilitas Jangka Panjang', data.liabilities.non_current_liabilities.items, data.liabilities.non_current_liabilities.total_non_current_liabilities, 'Total Liabilitas Jangka Panjang');

  doc.setFont('helvetica', 'bold');
  doc.setFontSize(10);
  doc.text('  Total Liabilitas', marginLeft, yPos);
  doc.text(formatCurrency(data.liabilities.total_liabilities), pageWidth - marginRight, yPos, { align: 'right' });
  yPos += 8;

  addSection('Ekuitas', data.equity.items, data.equity.total_equity, 'Total Ekuitas');

  doc.setFontSize(11);
  doc.setFont('helvetica', 'bold');
  doc.setFillColor(220, 200, 240);
  doc.rect(marginLeft, yPos - 3.5, colWidth, 7, 'F');
  doc.text('TOTAL LIABILITAS & EKUITAS', marginLeft + 2, yPos);
  doc.text(formatCurrency(data.total_liabilities_and_equity), pageWidth - marginRight, yPos, { align: 'right' });
  yPos += 10;

  if (!data.is_balanced) {
    doc.setTextColor(200, 0, 0);
    doc.text(`Selisih: ${formatCurrency(data.balance_difference)}`, marginLeft, yPos);
    doc.setTextColor(0, 0, 0);
  }

  addFooter(doc);
  doc.save(`Neraca_${asOfDate}.pdf`);
};

// ===== NERACA (Balance Sheet) CSV - Client-side fallback =====
export const exportBalanceSheetToCSV = (
  data: SSOTBalanceSheetData,
  asOfDate: string
): void => {
  const rows: string[][] = [];

  rows.push([`Neraca - ${data.company?.name || 'Perusahaan'}`]);
  rows.push([`Per ${new Date(asOfDate).toLocaleDateString('id-ID', { dateStyle: 'long' })}`]);
  rows.push([]);

  rows.push(['Kategori', 'Kode Akun', 'Nama Akun', 'Jumlah']);
  rows.push([]);

  // Assets
  rows.push(['ASET', '', '', '']);
  rows.push(['Aset Lancar', '', '', '']);
  data.assets.current_assets.items.forEach(item => {
    rows.push(['', item.account_code, item.account_name, item.amount.toString()]);
  });
  rows.push(['', '', 'Total Aset Lancar', data.assets.current_assets.total_current_assets.toString()]);
  rows.push([]);

  rows.push(['Aset Tidak Lancar', '', '', '']);
  data.assets.non_current_assets.items.forEach(item => {
    rows.push(['', item.account_code, item.account_name, item.amount.toString()]);
  });
  rows.push(['', '', 'Total Aset Tidak Lancar', data.assets.non_current_assets.total_non_current_assets.toString()]);
  rows.push(['', '', 'TOTAL ASET', data.assets.total_assets.toString()]);
  rows.push([]);

  // Liabilities
  rows.push(['LIABILITAS', '', '', '']);
  rows.push(['Liabilitas Jangka Pendek', '', '', '']);
  data.liabilities.current_liabilities.items.forEach(item => {
    rows.push(['', item.account_code, item.account_name, item.amount.toString()]);
  });
  rows.push(['', '', 'Total Liabilitas Jangka Pendek', data.liabilities.current_liabilities.total_current_liabilities.toString()]);
  rows.push([]);

  rows.push(['Liabilitas Jangka Panjang', '', '', '']);
  data.liabilities.non_current_liabilities.items.forEach(item => {
    rows.push(['', item.account_code, item.account_name, item.amount.toString()]);
  });
  rows.push(['', '', 'Total Liabilitas Jangka Panjang', data.liabilities.non_current_liabilities.total_non_current_liabilities.toString()]);
  rows.push(['', '', 'Total Liabilitas', data.liabilities.total_liabilities.toString()]);
  rows.push([]);

  // Equity
  rows.push(['EKUITAS', '', '', '']);
  data.equity.items.forEach(item => {
    rows.push(['', item.account_code, item.account_name, item.amount.toString()]);
  });
  rows.push(['', '', 'Total Ekuitas', data.equity.total_equity.toString()]);
  rows.push([]);

  rows.push(['', '', 'TOTAL LIABILITAS & EKUITAS', data.total_liabilities_and_equity.toString()]);

  if (!data.is_balanced) {
    rows.push([]);
    rows.push(['', '', 'Selisih', data.balance_difference.toString()]);
  }

  const csvContent = rows.map(row => row.map(cell => `"${String(cell || '').replace(/"/g, '""')}"`).join(',')).join('\n');
  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' });
  triggerDownload(blob, `Neraca_${asOfDate}.csv`);
};

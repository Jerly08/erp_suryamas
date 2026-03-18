import jsPDF from 'jspdf';
import 'jspdf-autotable';
import { Payment, PaymentFilters } from '../services/paymentService';

// Interface for PDF export options
export interface PDFExportOptions {
  title?: string;
  subtitle?: string;
  companyName?: string;
  companyAddress?: string;
  reportDate?: string;
  includeFilters?: boolean;
  filters?: PaymentFilters;
  statusFilter?: string;
  methodFilter?: string;
  startDate?: string;
  endDate?: string;
}

// Format currency for display - consistent with sales format
const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
};

// Format date for display
const formatDate = (dateString: string): string => {
  return new Date(dateString).toLocaleDateString('id-ID', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric'
  });
};

// Format method display name
const getMethodDisplayName = (method: string): string => {
  const methodMap: Record<string, string> = {
    'CASH': 'Tunai',
    'BANK_TRANSFER': 'Transfer Bank',
    'CHECK': 'Cek',
    'CREDIT_CARD': 'Kartu Kredit',
    'DEBIT_CARD': 'Kartu Debit',
    'OTHER': 'Lainnya'
  };
  
  return methodMap[method] || method;
};

// Export payments to PDF
export const exportPaymentsToPDF = (
  payments: Payment[], 
  options: PDFExportOptions = {}
): void => {
  try {
    // Create new PDF document
    const doc = new jsPDF();
    
    // Set default options
    const {
      title = 'Laporan Pembayaran',
      subtitle = 'Daftar Transaksi Pembayaran',
      companyName = 'PT. Sistem Akuntansi',
      companyAddress = 'Jakarta, Indonesia',
      reportDate = new Date().toLocaleDateString('id-ID'),
      includeFilters = true,
      statusFilter,
      methodFilter,
      startDate,
      endDate
    } = options;

    let yPos = 20;
    
    // Add company header
    doc.setFontSize(16);
    doc.setFont('helvetica', 'bold');
    doc.text(companyName, 20, yPos);
    
    yPos += 6;
    doc.setFontSize(10);
    doc.setFont('helvetica', 'normal');
    doc.text(companyAddress, 20, yPos);
    
    // Add report title
    yPos += 15;
    doc.setFontSize(14);
    doc.setFont('helvetica', 'bold');
    doc.text(title, 20, yPos);
    
    yPos += 6;
    doc.setFontSize(12);
    doc.setFont('helvetica', 'normal');
    doc.text(subtitle, 20, yPos);
    
    // Add report date
    yPos += 10;
    doc.setFontSize(10);
    doc.text(`Tanggal Laporan: ${reportDate}`, 20, yPos);
    
    // Add filters information if requested
    if (includeFilters) {
      yPos += 8;
      let filterText = 'Filter: ';
      const filterParts: string[] = [];
      
      if (statusFilter && statusFilter !== 'ALL') {
        filterParts.push(`Status: ${statusFilter}`);
      }
      
      if (methodFilter && methodFilter !== 'ALL') {
        filterParts.push(`Metode: ${getMethodDisplayName(methodFilter)}`);
      }
      
      if (startDate) {
        filterParts.push(`Dari: ${formatDate(startDate)}`);
      }
      
      if (endDate) {
        filterParts.push(`Sampai: ${formatDate(endDate)}`);
      }
      
      if (filterParts.length === 0) {
        filterText += 'Semua Data';
      } else {
        filterText += filterParts.join(', ');
      }
      
      doc.text(filterText, 20, yPos);
      yPos += 5;
    }
    
    // Add summary information
    yPos += 10;
    const totalAmount = payments.reduce((sum, payment) => sum + payment.amount, 0);
    const completedPayments = payments.filter(p => p.status === 'COMPLETED').length;
    const pendingPayments = payments.filter(p => p.status === 'PENDING').length;
    
    doc.setFontSize(10);
    doc.setFont('helvetica', 'bold');
    doc.text('Ringkasan:', 20, yPos);
    
    yPos += 6;
    doc.setFont('helvetica', 'normal');
    doc.text(`Total Transaksi: ${payments.length}`, 20, yPos);
    doc.text(`Total Nilai: ${formatCurrency(totalAmount)}`, 120, yPos);
    
    yPos += 6;
    doc.text(`Selesai: ${completedPayments}`, 20, yPos);
    doc.text(`Menunggu: ${pendingPayments}`, 120, yPos);
    
    yPos += 10;
    
    // Prepare table data
    const tableData = payments.map((payment, index) => [
      (index + 1).toString(),
      payment.code,
      payment.contact?.name || '-',
      formatDate(payment.date),
      formatCurrency(payment.amount),
      getMethodDisplayName(payment.method),
      payment.status,
      payment.notes || '-'
    ]);
    
    // Add table
    (doc as any).autoTable({
      startY: yPos,
      head: [['No', 'Kode', 'Kontak', 'Tanggal', 'Jumlah', 'Metode', 'Status', 'Catatan']],
      body: tableData,
      styles: {
        fontSize: 8,
        cellPadding: 3,
        overflow: 'linebreak'
      },
      headStyles: {
        fillColor: [41, 128, 185],
        textColor: 255,
        fontSize: 9,
        fontStyle: 'bold'
      },
      alternateRowStyles: {
        fillColor: [245, 245, 245]
      },
      columnStyles: {
        0: { halign: 'center', cellWidth: 15 }, // No
        1: { cellWidth: 25 }, // Kode
        2: { cellWidth: 35 }, // Kontak
        3: { halign: 'center', cellWidth: 25 }, // Tanggal
        4: { halign: 'right', cellWidth: 30 }, // Jumlah
        5: { halign: 'center', cellWidth: 25 }, // Metode
        6: { halign: 'center', cellWidth: 20 }, // Status
        7: { cellWidth: 35 } // Catatan
      },
      margin: { left: 20, right: 20 },
      theme: 'striped'
    });
    
    // Add footer with page numbers
    const pageCount = doc.internal.pages.length - 1;
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
      
      // Add generation timestamp
      doc.text(
        `Digenerate pada: ${new Date().toLocaleString('id-ID')}`,
        20,
        doc.internal.pageSize.height - 10
      );
    }
    
    // Save the PDF
    const fileName = `payments-report-${new Date().toISOString().split('T')[0]}.pdf`;
    doc.save(fileName);
    
  } catch (error) {
    console.error('Error generating PDF:', error);
    throw new Error('Gagal menggenerate PDF. Silakan coba lagi.');
  }
};

// Export payment detail to PDF
export const exportPaymentDetailToPDF = (payment: Payment): void => {
  try {
    const doc = new jsPDF();
    
    let yPos = 20;
    
    // Header
    doc.setFontSize(16);
    doc.setFont('helvetica', 'bold');
    doc.text('Detail Pembayaran', 20, yPos);
    
    yPos += 15;
    
    // Payment information
    const details = [
      ['Kode Pembayaran', payment.code],
      ['Kontak', payment.contact?.name || '-'],
      ['Tanggal', formatDate(payment.date)],
      ['Jumlah', formatCurrency(payment.amount)],
      ['Metode Pembayaran', getMethodDisplayName(payment.method)],
      ['Referensi', payment.reference || '-'],
      ['Status', payment.status],
      ['Catatan', payment.notes || '-'],
      ['Dibuat pada', formatDate(payment.created_at)],
      ['Diperbarui pada', formatDate(payment.updated_at)]
    ];
    
    details.forEach(([label, value]) => {
      doc.setFont('helvetica', 'bold');
      doc.text(`${label}:`, 20, yPos);
      doc.setFont('helvetica', 'normal');
      doc.text(value, 80, yPos);
      yPos += 8;
    });
    
    // Add footer
    doc.setFontSize(8);
    doc.text(
      `Digenerate pada: ${new Date().toLocaleString('id-ID')}`,
      20,
      doc.internal.pageSize.height - 10
    );
    
    // Save the PDF
    const fileName = `payment-detail-${payment.code}.pdf`;
    doc.save(fileName);
    
  } catch (error) {
    console.error('Error generating payment detail PDF:', error);
    throw new Error('Gagal menggenerate PDF detail pembayaran.');
  }
};

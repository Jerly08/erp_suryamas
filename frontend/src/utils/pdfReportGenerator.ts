import jsPDF from 'jspdf';
import 'jspdf-autotable';
import api from '@/services/api';
import { getImageUrl } from '@/utils/imageUrl';


export interface CompanyInfo {
  name: string;
  address: string;
  phone: string;
  email: string;
  logo?: string; // base64 string or path
  website?: string;
  tax_number?: string;
}

// Settings interface matching the backend structure
export interface SystemSettings {
  id?: number;
  company_name: string;
  company_address: string;
  company_phone: string;
  company_email: string;
  company_website?: string;
  company_logo?: string;
  tax_number?: string;
  currency: string;
  date_format: string;
  fiscal_year_start: string;
  default_tax_rate: number;
  language: string;
  timezone: string;
  thousand_separator: string;
  decimal_separator: string;
  decimal_places: number;
  invoice_prefix: string;
  invoice_next_number: number;
  quote_prefix: string;
  quote_next_number: number;
  purchase_prefix: string;
  purchase_next_number: number;
  email_notifications: boolean;
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_from?: string;
}

export interface ReportConfig {
  title: string;
  subtitle?: string;
  date?: string;
  reportNumber?: string;
  companyInfo: CompanyInfo;
}

export interface TableColumn {
  header: string;
  dataKey: string;
  width?: number;
}

export interface ReportData {
  columns: TableColumn[];
  data: any[];
  summary?: {
    subtotal?: number;
    tax?: number;
    taxRate?: number;
    total?: number;
  };
}

// Function to fetch company settings from API
export const fetchCompanySettings = async (): Promise<SystemSettings | null> => {
  try {
    const response = await api.get('/api/v1/settings/public');
    if (response.data.success) {
      return response.data.data;
    }
  } catch (error) {
    console.error('Error fetching company settings:', error);
  }
  return null;
};

// Convert system settings to company info for PDF
export const settingsToCompanyInfo = (settings: SystemSettings): CompanyInfo => {
  return {
    name: settings.company_name,
    address: settings.company_address,
    phone: settings.company_phone,
    email: settings.company_email,
    website: settings.company_website,
    tax_number: settings.tax_number,
    logo: settings.company_logo
  };
};

// Convert image to base64 for PDF embedding
const imageToBase64 = async (imagePath: string): Promise<string | null> => {
  try {
    const imageUrl = getImageUrl(imagePath);
    if (!imageUrl) return null;

    const response = await fetch(imageUrl);
    const blob = await response.blob();

    return new Promise((resolve) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = () => resolve(null);
      reader.readAsDataURL(blob);
    });
  } catch (error) {
    console.error('Error converting image to base64:', error);
    return null;
  }
};

export class PDFReportGenerator {
  private doc: jsPDF;
  private pageWidth: number;
  private pageHeight: number;
  private margin: number = 20;
  private settings: SystemSettings | null = null;

  constructor(orientation: 'portrait' | 'landscape' = 'portrait') {
    this.doc = new jsPDF(orientation);
    this.pageWidth = this.doc.internal.pageSize.width;
    this.pageHeight = this.doc.internal.pageSize.height;
  }

  /**
   * Load company settings from API
   */
  async loadSettings(): Promise<void> {
    this.settings = await fetchCompanySettings();
  }

  /**
   * Add company header with logo (similar to the invoice layout)
   */
  private async addCompanyHeader(config: ReportConfig): Promise<number> {
    let yPos = this.margin;

    // Logo section (left side)
    let logoBase64: string | null = null;

    if (config.companyInfo.logo) {
      // If logo is already base64, use it directly
      if (config.companyInfo.logo.startsWith('data:')) {
        logoBase64 = config.companyInfo.logo;
      } else {
        // Convert from path to base64
        logoBase64 = await imageToBase64(config.companyInfo.logo);
      }
    }

    if (logoBase64) {
      try {
        // Add logo - adjust size as needed
        this.doc.addImage(
          logoBase64,
          'JPEG', // Changed to JPEG as it's more compatible
          this.margin,
          yPos,
          40,
          40
        );
      } catch (error) {
        console.warn('Could not add logo:', error);
        this.addLogoPlaceholder(yPos);
      }
    } else {
      this.addLogoPlaceholder(yPos);
    }

    // Company information (right side)
    const companyInfoX = this.pageWidth - this.margin - 120;
    this.doc.setFontSize(16);
    this.doc.setTextColor(0);
    this.doc.setFont('helvetica', 'bold');
    this.doc.text(config.companyInfo.name, companyInfoX, yPos + 15);

    this.doc.setFontSize(10);
    this.doc.setFont('helvetica', 'normal');

    // Handle multi-line address
    const addressLines = this.splitTextIntoLines(config.companyInfo.address, 35);
    let addressY = yPos + 25;
    addressLines.forEach(line => {
      this.doc.text(line, companyInfoX, addressY);
      addressY += 5;
    });

    this.doc.text(`Phone: ${config.companyInfo.phone}`, companyInfoX, addressY);
    this.doc.text(`Email: ${config.companyInfo.email}`, companyInfoX, addressY + 5);

    // Add website if available
    if (config.companyInfo.website) {
      this.doc.text(`Website: ${config.companyInfo.website}`, companyInfoX, addressY + 10);
      addressY += 5;
    }

    // Add tax number if available
    if (config.companyInfo.tax_number) {
      this.doc.text(`Tax No: ${config.companyInfo.tax_number}`, companyInfoX, addressY + 10);
    }

    return yPos + 70; // Return next available Y position
  }

  /**
   * Add logo placeholder when logo is not available
   */
  private addLogoPlaceholder(yPos: number): void {
    // Add placeholder logo area
    this.doc.setDrawColor(200);
    this.doc.setFillColor(240, 240, 240);
    this.doc.roundedRect(this.margin, yPos, 40, 40, 5, 5, 'FD');

    // Add coding icon placeholder (similar to invoice)
    this.doc.setFontSize(20);
    this.doc.setTextColor(100);
    this.doc.text('</>', this.margin + 12, yPos + 25);
  }

  /**
   * Split long text into multiple lines
   */
  private splitTextIntoLines(text: string, maxLength: number): string[] {
    if (!text) return [''];

    const words = text.split(' ');
    const lines: string[] = [];
    let currentLine = '';

    words.forEach(word => {
      if ((currentLine + word).length <= maxLength) {
        currentLine += (currentLine ? ' ' : '') + word;
      } else {
        if (currentLine) {
          lines.push(currentLine);
        }
        currentLine = word;
      }
    });

    if (currentLine) {
      lines.push(currentLine);
    }

    return lines.length ? lines : [''];
  }

  /**
   * Add report title and information
   */
  private addReportTitle(config: ReportConfig, startY: number): number {
    let yPos = startY + 10;

    // Main title
    this.doc.setFontSize(24);
    this.doc.setFont('helvetica', 'bold');
    this.doc.setTextColor(0);
    this.doc.text(config.title, this.margin, yPos);

    yPos += 20;

    // Report details in two columns
    this.doc.setFontSize(10);
    this.doc.setFont('helvetica', 'normal');

    if (config.reportNumber) {
      this.doc.setFont('helvetica', 'bold');
      this.doc.text('Report Number:', this.margin, yPos);
      this.doc.setFont('helvetica', 'normal');
      this.doc.text(config.reportNumber, this.margin + 50, yPos);
    }

    if (config.date) {
      const dateX = this.pageWidth - this.margin - 60;
      this.doc.setFont('helvetica', 'bold');
      this.doc.text('Date:', dateX, yPos);
      this.doc.setFont('helvetica', 'normal');
      this.doc.text(config.date, dateX + 20, yPos);
    }

    if (config.subtitle) {
      yPos += 10;
      this.doc.setFont('helvetica', 'normal');
      this.doc.text(config.subtitle, this.margin, yPos);
    }

    return yPos + 15;
  }

  /**
   * Add data table
   */
  private addTable(data: ReportData, startY: number): number {
    const tableConfig = {
      startY: startY,
      head: [data.columns.map(col => col.header)],
      body: data.data.map(row =>
        data.columns.map(col => {
          const value = row[col.dataKey];
          // Format currency values
          if (typeof value === 'number' && col.dataKey.toLowerCase().includes('price') ||
            col.dataKey.toLowerCase().includes('total') ||
            col.dataKey.toLowerCase().includes('amount')) {
            return new Intl.NumberFormat('id-ID', {
              style: 'currency',
              currency: 'IDR',
              minimumFractionDigits: 0
            }).format(value);
          }
          return value;
        })
      ),
      styles: {
        fontSize: 10,
        cellPadding: 5
      },
      headStyles: {
        fillColor: [66, 139, 202],
        textColor: 255,
        fontSize: 11,
        fontStyle: 'bold'
      },
      alternateRowStyles: {
        fillColor: [245, 245, 245]
      },
      columnStyles: {} as any
    };

    // Set column widths if specified
    data.columns.forEach((col, index) => {
      if (col.width) {
        tableConfig.columnStyles[index] = { cellWidth: col.width };
      }
    });

    (this.doc as any).autoTable(tableConfig);

    return (this.doc as any).lastAutoTable.finalY;
  }

  /**
   * Add summary section (like subtotal, tax, total)
   */
  private addSummary(summary: any, startY: number): number {
    if (!summary) return startY;

    const summaryX = this.pageWidth - this.margin - 80;
    let yPos = startY + 20;

    this.doc.setFontSize(11);

    if (summary.subtotal !== undefined) {
      this.doc.setFont('helvetica', 'normal');
      this.doc.text('Subtotal:', summaryX - 30, yPos);
      this.doc.text(this.formatCurrency(summary.subtotal), summaryX, yPos);
      yPos += 8;
    }

    if (summary.tax !== undefined && summary.taxRate !== undefined) {
      this.doc.text(`PPN (${summary.taxRate}%):`, summaryX - 30, yPos);
      this.doc.text(this.formatCurrency(summary.tax), summaryX, yPos);
      yPos += 8;
    }

    if (summary.total !== undefined) {
      // Draw line above total
      this.doc.setDrawColor(0);
      this.doc.line(summaryX - 30, yPos - 2, summaryX + 50, yPos - 2);

      this.doc.setFont('helvetica', 'bold');
      this.doc.setFontSize(12);
      this.doc.text('TOTAL:', summaryX - 30, yPos + 5);
      this.doc.text(this.formatCurrency(summary.total), summaryX, yPos + 5);
    }

    return yPos + 15;
  }

  /**
   * Format currency based on system settings or fallback to Indonesian Rupiah
   */
  private formatCurrency(amount: number): string {
    const currency = this.settings?.currency || 'IDR';
    const locale = this.settings?.language === 'en' ? 'en-US' : 'id-ID';
    const decimalPlaces = this.settings?.decimal_places ?? 0;

    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency: currency,
      minimumFractionDigits: decimalPlaces,
      maximumFractionDigits: decimalPlaces
    }).format(amount);
  }

  /**
   * Add footer with generation info
   */
  private addFooter(): void {
    const footerY = this.pageHeight - this.margin;
    this.doc.setFontSize(8);
    this.doc.setTextColor(128);
    this.doc.setFont('helvetica', 'italic');

    const generateTime = new Date().toLocaleString('id-ID', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });

    this.doc.text(`Generated on ${generateTime}`, this.margin, footerY);
  }

  /**
   * Generate complete PDF report
   */
  public async generateReport(config: ReportConfig, data: ReportData): Promise<jsPDF> {
    // Load settings if not already loaded
    if (!this.settings) {
      await this.loadSettings();
    }

    // Add company header
    let currentY = await this.addCompanyHeader(config);

    // Add report title
    currentY = this.addReportTitle(config, currentY);

    // Add table
    currentY = this.addTable(data, currentY);

    // Add summary if provided
    if (data.summary) {
      currentY = this.addSummary(data.summary, currentY);
    }

    // Add footer
    this.addFooter();

    return this.doc;
  }

  /**
   * Generate report from system settings (convenience method)
   */
  public static async generateFromSettings(
    reportTitle: string,
    data: ReportData,
    options?: {
      reportNumber?: string;
      subtitle?: string;
      date?: string;
      orientation?: 'portrait' | 'landscape';
    }
  ): Promise<jsPDF> {
    // Fetch settings
    const settings = await fetchCompanySettings();
    if (!settings) {
      throw new Error('Could not load company settings');
    }

    // Create PDF generator
    const generator = new PDFReportGenerator(options?.orientation);
    generator.settings = settings;

    // Create config from settings
    const config: ReportConfig = {
      title: reportTitle,
      subtitle: options?.subtitle,
      date: options?.date || new Date().toLocaleDateString('id-ID'),
      reportNumber: options?.reportNumber,
      companyInfo: settingsToCompanyInfo(settings)
    };

    return generator.generateReport(config, data);
  }

  /**
   * Save PDF to file
   */
  public save(filename: string): void {
    this.doc.save(filename);
  }

  /**
   * Get PDF as blob for further processing
   */
  public getBlob(): Blob {
    return this.doc.output('blob');
  }

  /**
   * Get PDF as data URL
   */
  public getDataURL(): string {
    return this.doc.output('dataurlstring');
  }
}

// Utility function to convert image file to base64
export const imageFileToBase64 = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
};

// Generate report number based on system settings
export const generateReportNumber = async (type: 'invoice' | 'quote' | 'purchase' = 'invoice'): Promise<string> => {
  const settings = await fetchCompanySettings();
  if (!settings) {
    const date = new Date();
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const randomNum = Math.floor(Math.random() * 10000).toString().padStart(4, '0');
    return `${type.toUpperCase()}/${year}/${month}/${randomNum}`;
  }

  const date = new Date();
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');

  let prefix: string;
  let nextNumber: number;

  switch (type) {
    case 'quote':
      prefix = settings.quote_prefix || 'QUO';
      nextNumber = settings.quote_next_number || 1;
      break;
    case 'purchase':
      prefix = settings.purchase_prefix || 'PUR';
      nextNumber = settings.purchase_next_number || 1;
      break;
    default:
      prefix = settings.invoice_prefix || 'INV';
      nextNumber = settings.invoice_next_number || 1;
  }

  const paddedNumber = nextNumber.toString().padStart(4, '0');
  return `${prefix}/${year}/${month}/${paddedNumber}`;
};

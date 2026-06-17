import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const outputPath = path.join(__dirname, "erp_contact_import_template_customer_vendor_employee.xlsx");

const wb = Workbook.create();
const guide = wb.worksheets.add("Panduan");
const input = wb.worksheets.add("Contact_Import");
const mapping = wb.worksheets.add("Mapping_JSON");
const lists = wb.worksheets.add("Lists");

const navy = "#1F4E78";
const blue = "#D9EAF7";
const pale = "#F7FAFC";
const green = "#E2F0D9";
const orange = "#FCE4D6";
const gray = "#666666";

for (const sheet of [guide, input, mapping, lists]) {
  sheet.showGridLines = false;
}

guide.getRange("A1:H1").merge();
guide.getRange("A1").values = [["Template Import Contact - Customer, Vendor, Employee"]];
guide.getRange("A1").format = {
  fill: navy,
  font: { bold: true, color: "#FFFFFF", size: 16 },
  horizontalAlignment: "center",
  verticalAlignment: "center",
};
guide.getRange("A1").format.rowHeightPx = 36;

guide.getRange("A3:H3").merge();
guide.getRange("A3").values = [["Gunakan sheet Contact_Import untuk mengisi data kontak. Sheet ini disusun agar field-nya sesuai payload /api/v1/contacts/import."]];
guide.getRange("A3").format = {
  fill: blue,
  font: { color: "#1F2937" },
  wrapText: true,
  verticalAlignment: "center",
};
guide.getRange("A3").format.rowHeightPx = 42;

const guideRows = [
  ["Langkah", "Instruksi"],
  ["1", "Isi data pada sheet Contact_Import mulai baris contoh, lalu hapus/ganti contoh sesuai data asli."],
  ["2", "Kolom wajib minimal: name dan type. Untuk konsistensi form aplikasi, email dan phone sebaiknya tetap diisi."],
  ["3", "type hanya boleh CUSTOMER, VENDOR, atau EMPLOYEE. Gunakan dropdown agar tidak salah ketik."],
  ["4", "code boleh dikosongkan. Backend akan membuat kode otomatis CUST-xxxx, VEND-xxxx, atau EMP-xxxx. Jika diisi, kode harus unik."],
  ["5", "is_active pakai TRUE/FALSE. Jika kosong, sistem default aktif."],
  ["6", "default_expense_account_id hanya relevan untuk vendor dan hanya diisi kalau sudah tahu ID COA biaya default."],
  ["7", "Saat ini endpoint import contact menerima JSON array; sheet Mapping_JSON menunjukkan nama key JSON untuk proses konversi/upload."],
];
guide.getRange(`A5:B${5 + guideRows.length - 1}`).values = guideRows;
guide.getRange("A5:B5").format = {
  fill: navy,
  font: { bold: true, color: "#FFFFFF" },
};
guide.getRange("A6:A12").format = { horizontalAlignment: "center", font: { bold: true } };
guide.getRange("B6:B12").format = { wrapText: true };
guide.getRange("A5:B12").format.borders = {
  insideHorizontal: { style: "continuous", color: "#D0D7DE" },
  insideVertical: { style: "continuous", color: "#D0D7DE" },
  edgeBottom: { style: "continuous", color: "#D0D7DE" },
  edgeTop: { style: "continuous", color: "#D0D7DE" },
  edgeLeft: { style: "continuous", color: "#D0D7DE" },
  edgeRight: { style: "continuous", color: "#D0D7DE" },
};

const fieldRows = [
  ["Field", "Wajib", "Contoh", "Catatan"],
  ["name", "Ya", "PT Global Tech", "Nama customer/vendor/employee."],
  ["type", "Ya", "CUSTOMER", "Pilihan: CUSTOMER, VENDOR, EMPLOYEE."],
  ["email", "Disarankan", "finance@globaltech.co.id", "Form aplikasi menandai email sebagai wajib."],
  ["phone", "Disarankan", "021-555-0101", "Form aplikasi menandai phone sebagai wajib."],
  ["code", "Opsional", "CUST-2026-0001", "Kosongkan jika ingin auto-generate."],
  ["external_id", "Opsional", "CUST-JAN-001", "ID referensi lama/customer/vendor/employee."],
  ["pic_name", "Opsional", "Budi Santoso", "Umumnya untuk CUSTOMER dan VENDOR."],
  ["category", "Opsional", "WHOLESALE", "RETAIL, WHOLESALE, DISTRIBUTOR, MANUFACTURER, SERVICE_PROVIDER."],
  ["credit_limit", "Opsional", "50000000", "Angka tanpa Rp/titik/koma."],
  ["payment_terms", "Opsional", "30", "Jumlah hari termin pembayaran."],
  ["address", "Opsional", "Jl. Sudirman No. 10, Jakarta", "Alamat sederhana 1 kolom."],
];
guide.getRange(`D5:G${5 + fieldRows.length - 1}`).values = fieldRows;
guide.getRange("D5:G5").format = {
  fill: navy,
  font: { bold: true, color: "#FFFFFF" },
};
guide.getRange("D6:G16").format = { wrapText: true, verticalAlignment: "top" };
guide.getRange("D5:G16").format.borders = {
  insideHorizontal: { style: "continuous", color: "#D0D7DE" },
  insideVertical: { style: "continuous", color: "#D0D7DE" },
  edgeBottom: { style: "continuous", color: "#D0D7DE" },
  edgeTop: { style: "continuous", color: "#D0D7DE" },
  edgeLeft: { style: "continuous", color: "#D0D7DE" },
  edgeRight: { style: "continuous", color: "#D0D7DE" },
};

guide.getRange("A14:B14").merge();
guide.getRange("A14").values = [["Catatan: jangan ubah nama header di sheet Contact_Import agar proses mapping/import tetap konsisten."]];
guide.getRange("A14").format = {
  fill: orange,
  font: { bold: true, color: "#7C2D12" },
  wrapText: true,
};

const headers = [
  "code",
  "name",
  "type",
  "external_id",
  "pic_name",
  "category",
  "email",
  "phone",
  "mobile",
  "fax",
  "website",
  "tax_number",
  "credit_limit",
  "payment_terms",
  "is_active",
  "address",
  "default_expense_account_id",
  "notes",
];

const examples = [
  [
    "CUST-2026-0001",
    "PT Global Tech",
    "CUSTOMER",
    "CUST-JAN-001",
    "Budi Santoso",
    "WHOLESALE",
    "finance@globaltech.co.id",
    "021-555-0101",
    "0812-1000-1001",
    "",
    "https://globaltech.example",
    "01.234.567.8-901.000",
    50000000,
    30,
    "TRUE",
    "Jl. Sudirman No. 10, Jakarta Pusat",
    "",
    "Contoh customer untuk transaksi sales bulan Januari 2026",
  ],
  [
    "VEND-2026-0001",
    "Toko Elektronik Sejati",
    "VENDOR",
    "VEND-JAN-001",
    "Siti Rahma",
    "DISTRIBUTOR",
    "sales@elektroniksejati.co.id",
    "022-555-0202",
    "0812-2000-2002",
    "",
    "",
    "02.345.678.9-012.000",
    0,
    14,
    "TRUE",
    "Jl. Asia Afrika No. 20, Bandung",
    "",
    "Contoh vendor untuk transaksi purchase bulan Januari 2026",
  ],
  [
    "EMP-2026-0001",
    "Ahmad Fauzi",
    "EMPLOYEE",
    "EMP-JAN-001",
    "",
    "",
    "ahmad.fauzi@company.example",
    "021-555-0303",
    "0812-3000-3003",
    "",
    "",
    "",
    0,
    0,
    "TRUE",
    "Jl. Melati No. 5, Jakarta Selatan",
    "",
    "Contoh employee bila kontak karyawan juga perlu diimport",
  ],
];

input.getRange("A1:R1").values = [headers];
input.getRange("A2:R4").values = examples;
input.getRange("A1:R1").format = {
  fill: navy,
  font: { bold: true, color: "#FFFFFF" },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  wrapText: true,
};
input.getRange("A2:R501").format = {
  fill: pale,
  verticalAlignment: "top",
  wrapText: true,
};
input.getRange("A2:R4").format.fill = green;
input.getRange("A1:R501").format.borders = {
  insideHorizontal: { style: "continuous", color: "#E5E7EB" },
  insideVertical: { style: "continuous", color: "#E5E7EB" },
  edgeBottom: { style: "continuous", color: "#D0D7DE" },
  edgeTop: { style: "continuous", color: "#D0D7DE" },
  edgeLeft: { style: "continuous", color: "#D0D7DE" },
  edgeRight: { style: "continuous", color: "#D0D7DE" },
};
input.tables.add("A1:R501", true, "ContactImportTable");
input.freezePanes.freezeRows(1);
input.getRange("M2:M501").format.numberFormat = '#,##0';
input.getRange("N2:N501").format.numberFormat = '0';
input.getRange("Q2:Q501").format.numberFormat = '0';

lists.getRange("A1").values = [["type"]];
lists.getRange("A2:A4").values = [["CUSTOMER"], ["VENDOR"], ["EMPLOYEE"]];
lists.getRange("C1").values = [["category"]];
lists.getRange("C2:C6").values = [["RETAIL"], ["WHOLESALE"], ["DISTRIBUTOR"], ["MANUFACTURER"], ["SERVICE_PROVIDER"]];
lists.getRange("E1").values = [["is_active"]];
lists.getRange("E2:E3").values = [["TRUE"], ["FALSE"]];
lists.getRange("A1:E1").format = { fill: navy, font: { bold: true, color: "#FFFFFF" } };

input.getRange("C2:C501").dataValidation = { rule: { type: "list", formula1: "Lists!$A$2:$A$4" } };
input.getRange("F2:F501").dataValidation = { rule: { type: "list", formula1: "Lists!$C$2:$C$6" } };
input.getRange("O2:O501").dataValidation = { rule: { type: "list", formula1: "Lists!$E$2:$E$3" } };

input.getRange("A1:R1").format.rowHeightPx = 34;
input.getRange("A2:R501").format.rowHeightPx = 48;

const mappingRows = [
  ["Excel Header", "JSON Key", "Backend Field", "Import Note"],
  ["code", "code", "Contact.Code", "Opsional; kosongkan untuk auto-generate."],
  ["name", "name", "Contact.Name", "Wajib."],
  ["type", "type", "Contact.Type", "Wajib; CUSTOMER/VENDOR/EMPLOYEE."],
  ["external_id", "external_id", "Contact.ExternalID", "Opsional ID referensi lama."],
  ["pic_name", "pic_name", "Contact.PICName", "Opsional PIC customer/vendor."],
  ["category", "category", "Contact.Category", "Opsional kategori kontak."],
  ["email", "email", "Contact.Email", "Disarankan karena form aplikasi wajib."],
  ["phone", "phone", "Contact.Phone", "Disarankan karena form aplikasi wajib."],
  ["mobile", "mobile", "Contact.Mobile", "Opsional."],
  ["fax", "fax", "Contact.Fax", "Opsional."],
  ["website", "website", "Contact.Website", "Opsional."],
  ["tax_number", "tax_number", "Contact.TaxNumber", "Opsional NPWP/Tax ID."],
  ["credit_limit", "credit_limit", "Contact.CreditLimit", "Angka tanpa simbol mata uang."],
  ["payment_terms", "payment_terms", "Contact.PaymentTerms", "Jumlah hari."],
  ["is_active", "is_active", "Contact.IsActive", "TRUE/FALSE."],
  ["address", "address", "Contact.Address", "Alamat sederhana."],
  ["default_expense_account_id", "default_expense_account_id", "Contact.DefaultExpenseAccountID", "Opsional; vendor only bila ada."],
  ["notes", "notes", "Contact.Notes", "Opsional."],
];
mapping.getRange(`A1:D${mappingRows.length}`).values = mappingRows;
mapping.getRange("A1:D1").format = {
  fill: navy,
  font: { bold: true, color: "#FFFFFF" },
  horizontalAlignment: "center",
};
mapping.getRange(`A2:D${mappingRows.length}`).format = {
  wrapText: true,
  verticalAlignment: "top",
};
mapping.getRange(`A1:D${mappingRows.length}`).format.borders = {
  insideHorizontal: { style: "continuous", color: "#D0D7DE" },
  insideVertical: { style: "continuous", color: "#D0D7DE" },
  edgeBottom: { style: "continuous", color: "#D0D7DE" },
  edgeTop: { style: "continuous", color: "#D0D7DE" },
  edgeLeft: { style: "continuous", color: "#D0D7DE" },
  edgeRight: { style: "continuous", color: "#D0D7DE" },
};
mapping.freezePanes.freezeRows(1);

const widths = {
  Panduan: [70, 520, 20, 140, 110, 220, 290, 20],
  Contact_Import: [135, 220, 120, 140, 160, 155, 230, 145, 145, 110, 210, 170, 125, 115, 95, 320, 190, 440],
  Mapping_JSON: [170, 170, 220, 360],
  Lists: [120, 20, 160, 20, 120],
};

for (const [sheetName, widthList] of Object.entries(widths)) {
  const sheet = wb.worksheets.getItem(sheetName);
  widthList.forEach((width, index) => {
    sheet.getCell(0, index).format.columnWidthPx = width;
  });
}

// Keep the values list visible; users can inspect allowed values without touching the import sheet.
lists.getRange("A1:E6").format.borders = {
  insideHorizontal: { style: "continuous", color: "#D0D7DE" },
  insideVertical: { style: "continuous", color: "#D0D7DE" },
  edgeBottom: { style: "continuous", color: "#D0D7DE" },
  edgeTop: { style: "continuous", color: "#D0D7DE" },
  edgeLeft: { style: "continuous", color: "#D0D7DE" },
  edgeRight: { style: "continuous", color: "#D0D7DE" },
};
lists.getRange("A2:E6").format = { font: { color: gray } };

const preview1 = await wb.render({ sheetName: "Contact_Import", range: "A1:R8", scale: 1, format: "png" });
await fs.writeFile(path.join(__dirname, "preview_contact_import.png"), new Uint8Array(await preview1.arrayBuffer()));
const preview2 = await wb.render({ sheetName: "Panduan", range: "A1:H16", scale: 1, format: "png" });
await fs.writeFile(path.join(__dirname, "preview_panduan.png"), new Uint8Array(await preview2.arrayBuffer()));
const preview3 = await wb.render({ sheetName: "Mapping_JSON", range: "A1:D19", scale: 1, format: "png" });
await fs.writeFile(path.join(__dirname, "preview_mapping.png"), new Uint8Array(await preview3.arrayBuffer()));
const preview4 = await wb.render({ sheetName: "Lists", range: "A1:E6", scale: 1, format: "png" });
await fs.writeFile(path.join(__dirname, "preview_lists.png"), new Uint8Array(await preview4.arrayBuffer()));

const inspectImport = await wb.inspect({
  kind: "table",
  range: "Contact_Import!A1:R4",
  include: "values",
  tableMaxRows: 6,
  tableMaxCols: 20,
});
console.log(inspectImport.ndjson);
const errors = await wb.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 100 },
  summary: "formula error scan",
});
console.log(errors.ndjson);

await fs.mkdir(__dirname, { recursive: true });
const xlsx = await SpreadsheetFile.exportXlsx(wb);
await xlsx.save(outputPath);
console.log(`saved=${outputPath}`);

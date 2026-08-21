# 📚 DOKUMENTASI LENGKAP API SISTEM AKUNTANSI

## 🌐 Akses Swagger UI
- **URL**: http://localhost:8080/swagger
- **Alternative**: http://localhost:8080/docs
- **JSON Spec**: http://localhost:8080/openapi/enhanced-doc.json

---

## 🔐 AUTENTIKASI

### 1. Login
**Endpoint**: `POST /api/v1/auth/login`

**Deskripsi**: Login untuk mendapatkan JWT token

**Request Body**:
```json
{
  "email": "admin@company.com",
  "password": "admin123"
}
```

**Response**:
```json
{
  "status": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "email": "admin@company.com",
      "role": "admin"
    }
  }
}
```

**Cara Penggunaan**:
1. Kirim request dengan email dan password
2. Simpan token dari response
3. Gunakan token untuk authorize request lain dengan header: `Authorization: Bearer <token>`

---

## 👥 USER MANAGEMENT

### 1. Get All Users
**Endpoint**: `GET /api/v1/users`
**Authorization**: Required (Bearer Token)

**Response**:
```json
{
  "status": "success",
  "data": [
    {
      "id": 1,
      "email": "admin@company.com",
      "role": "admin",
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

### 2. Create User
**Endpoint**: `POST /api/v1/users`
**Authorization**: Required (Admin only)

**Request Body**:
```json
{
  "email": "user@company.com",
  "password": "password123",
  "role": "finance",
  "permissions": ["view_accounts", "create_transactions"]
}
```

### 3. Update User
**Endpoint**: `PUT /api/v1/users/{id}`
**Authorization**: Required (Admin only)

**Request Body**:
```json
{
  "email": "updated@company.com",
  "role": "director",
  "is_active": true
}
```

---

## 📊 CHART OF ACCOUNTS (COA)

### 1. Get All Accounts
**Endpoint**: `GET /api/v1/accounts`
**Authorization**: Required

**Query Parameters**:
- `type`: Filter by account type (ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE)
- `category`: Filter by category
- `is_active`: Filter active accounts (true/false)
- `page`: Page number for pagination
- `limit`: Items per page

**Example**: `GET /api/v1/accounts?type=ASSET&is_active=true&page=1&limit=10`

**Response**:
```json
{
  "status": "success",
  "data": [
    {
      "id": 1,
      "code": "1-1000",
      "name": "Cash in Bank",
      "type": "ASSET",
      "category": "Current Assets",
      "balance": 1000000.00,
      "is_active": true
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 50,
    "total_pages": 5
  }
}
```

### 2. Create Account
**Endpoint**: `POST /api/v1/accounts`
**Authorization**: Required (Admin/Finance)

**Request Body**:
```json
{
  "code": "1-1001",
  "name": "Petty Cash",
  "type": "ASSET",
  "category": "Current Assets",
  "description": "Small cash for daily expenses",
  "is_active": true
}
```

### 3. Update Account
**Endpoint**: `PUT /api/v1/accounts/{id}`

**Request Body**:
```json
{
  "name": "Updated Account Name",
  "description": "Updated description",
  "is_active": false
}
```

### 4. Account Balance History
**Endpoint**: `GET /api/v1/accounts/{id}/balance-history`

**Query Parameters**:
- `start_date`: Start date (YYYY-MM-DD)
- `end_date`: End date (YYYY-MM-DD)

---

## 👤 CONTACTS (CUSTOMERS & VENDORS)

### 1. Get All Contacts
**Endpoint**: `GET /api/v1/contacts`

**Query Parameters**:
- `type`: customer, vendor, both
- `search`: Search by name, email, or phone
- `is_active`: true/false

**Example**: `GET /api/v1/contacts?type=customer&search=john&is_active=true`

### 2. Create Contact
**Endpoint**: `POST /api/v1/contacts`

**Request Body**:
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "phone": "+62812345678",
  "type": "customer",
  "address": "Jakarta, Indonesia",
  "tax_number": "12.345.678.9-012.000",
  "payment_terms": 30,
  "credit_limit": 5000000.00
}
```

### 3. Customer Transactions
**Endpoint**: `GET /api/v1/contacts/{id}/transactions`

**Query Parameters**:
- `start_date`: Filter from date
- `end_date`: Filter to date
- `type`: sales, purchase, payment

---

## 📦 PRODUCT MANAGEMENT

### 1. Get All Products
**Endpoint**: `GET /api/v1/products`

**Query Parameters**:
- `search`: Search by name or SKU
- `category`: Filter by category
- `is_active`: true/false
- `low_stock`: Show products with low stock (true/false)

### 2. Create Product
**Endpoint**: `POST /api/v1/products`

**Request Body**:
```json
{
  "sku": "PROD-001",
  "name": "Product Name",
  "description": "Product description",
  "category": "Electronics",
  "unit": "pcs",
  "purchase_price": 100000.00,
  "selling_price": 150000.00,
  "stock_quantity": 100,
  "min_stock": 10,
  "is_active": true,
  "tax_rate": 0.11
}
```

### 3. Stock Adjustment
**Endpoint**: `POST /api/v1/products/{id}/stock-adjustment`

**Request Body**:
```json
{
  "adjustment_type": "increase", // or "decrease"
  "quantity": 50,
  "reason": "Stock correction",
  "reference": "ADJ-001"
}
```

### 4. Product Stock History
**Endpoint**: `GET /api/v1/products/{id}/stock-history`

---

## 🏢 ASSETS MANAGEMENT

### 1. Get All Assets
**Endpoint**: `GET /api/v1/assets`

**Query Parameters**:
- `category`: Filter by category
- `status`: active, disposed, under_maintenance
- `search`: Search by name or asset code

### 2. Create Asset
**Endpoint**: `POST /api/v1/assets`

**Request Body**:
```json
{
  "name": "Office Computer",
  "asset_code": "AST-001",
  "category": "IT Equipment",
  "purchase_date": "2025-01-01",
  "purchase_price": 10000000.00,
  "useful_life": 3,
  "depreciation_method": "straight_line",
  "salvage_value": 1000000.00,
  "location": "Office Floor 1",
  "condition": "good",
  "status": "active"
}
```

### 3. Asset Depreciation Schedule
**Endpoint**: `GET /api/v1/assets/{id}/depreciation-schedule`

**Response**:
```json
{
  "status": "success",
  "data": {
    "asset": {
      "id": 1,
      "name": "Office Computer",
      "purchase_price": 10000000.00,
      "useful_life": 3
    },
    "schedule": [
      {
        "year": 1,
        "opening_value": 10000000.00,
        "depreciation": 3000000.00,
        "closing_value": 7000000.00
      }
    ]
  }
}
```

### 4. Capitalize Asset
**Endpoint**: `POST /api/v1/assets/{id}/capitalize`

**Request Body**:
```json
{
  "capitalize_date": "2025-01-01",
  "notes": "Asset ready for use"
}
```

### 5. Assets Summary
**Endpoint**: `GET /api/v1/assets/summary`

**Response**:
```json
{
  "total_assets": 15,
  "total_value": 150000000.00,
  "total_depreciation": 45000000.00,
  "net_book_value": 105000000.00,
  "by_category": [
    {
      "category": "IT Equipment",
      "count": 5,
      "total_value": 50000000.00
    }
  ]
}
```

---

## ⚙️ SETTINGS MANAGEMENT

### 1. Get System Settings
**Endpoint**: `GET /api/v1/settings`

**Response**:
```json
{
  "status": "success",
  "data": {
    "company_name": "PT. Sistem Akuntansi",
    "company_address": "Jakarta, Indonesia",
    "company_phone": "+62211234567",
    "company_email": "admin@company.com",
    "tax_number": "12.345.678.9-012.000",
    "fiscal_year_start": "01-01",
    "currency": "IDR",
    "decimal_places": 2
  }
}
```

### 2. Update Settings
**Endpoint**: `PUT /api/v1/settings`

**Request Body**:
```json
{
  "company_name": "Updated Company Name",
  "company_address": "New Address",
  "fiscal_year_start": "01-04",
  "currency": "IDR"
}
```

### 3. Update Company Info
**Endpoint**: `PUT /api/v1/settings/company`

**Request Body**:
```json
{
  "company_name": "PT. New Company Name",
  "company_address": "New Company Address",
  "company_phone": "+62211111111",
  "company_email": "new@company.com",
  "tax_number": "99.888.777.6-543.000"
}
```

### 4. Upload Company Logo
**Endpoint**: `POST /api/v1/settings/company/logo`
**Content-Type**: `multipart/form-data`

**Form Data**:
- `logo`: File upload (image)

### 5. Reset to Defaults
**Endpoint**: `POST /api/v1/settings/reset`

---

## 🏦 TAX ACCOUNTS SETTINGS

### 1. Get Current Tax Settings
**Endpoint**: `GET /api/v1/tax-accounts/current`

**Response**:
```json
{
  "status": "success",
  "data": {
    "id": 1,
    "name": "Standard Tax Configuration",
    "input_tax_account_id": 15,
    "output_tax_account_id": 25,
    "is_active": true,
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

### 2. Create Tax Settings
**Endpoint**: `POST /api/v1/tax-accounts`

**Request Body**:
```json
{
  "name": "New Tax Configuration",
  "input_tax_account_id": 15,
  "output_tax_account_id": 25,
  "description": "Tax configuration for 2025"
}
```

### 3. Get Available Accounts
**Endpoint**: `GET /api/v1/tax-accounts/accounts`

**Query Parameters**:
- `type`: Filter by account type
- `category`: Filter by category

### 4. Validate Tax Configuration
**Endpoint**: `POST /api/v1/tax-accounts/validate`

**Request Body**:
```json
{
  "input_tax_account_id": 15,
  "output_tax_account_id": 25
}
```

### 5. Get Account Suggestions
**Endpoint**: `GET /api/v1/tax-accounts/suggestions`

**Response**:
```json
{
  "status": "success",
  "data": {
    "suggested_input_tax_accounts": [
      {
        "id": 15,
        "code": "1-1150",
        "name": "PPN Masukan",
        "type": "ASSET"
      }
    ],
    "suggested_output_tax_accounts": [
      {
        "id": 25,
        "code": "2-2100",
        "name": "PPN Keluaran",
        "type": "LIABILITY"
      }
    ]
  }
}
```

### 6. Refresh Cache
**Endpoint**: `POST /api/v1/tax-accounts/refresh-cache`
**Authorization**: Admin only

### 7. Activate Tax Settings
**Endpoint**: `POST /api/v1/tax-accounts/{id}/activate`

---

## 💰 SALES MANAGEMENT

### 1. Get All Sales
**Endpoint**: `GET /api/v1/sales`

**Query Parameters**:
- `start_date`: Filter from date
- `end_date`: Filter to date
- `customer_id`: Filter by customer
- `status`: draft, confirmed, invoiced, paid, cancelled
- `search`: Search by invoice number or customer name

### 2. Create Sale
**Endpoint**: `POST /api/v1/sales`

**Request Body**:
```json
{
  "customer_id": 1,
  "sale_date": "2025-01-01",
  "due_date": "2025-01-31",
  "items": [
    {
      "product_id": 1,
      "quantity": 10,
      "unit_price": 150000.00,
      "discount_percent": 5.0,
      "tax_rate": 0.11
    }
  ],
  "notes": "Sale notes",
  "payment_terms": 30
}
```

### 3. Confirm Sale
**Endpoint**: `POST /api/v1/sales/{id}/confirm`

### 4. Generate Invoice
**Endpoint**: `POST /api/v1/sales/{id}/invoice`

### 5. Export Invoice PDF
**Endpoint**: `GET /api/v1/sales/{id}/invoice/pdf`

### 6. Sales Summary
**Endpoint**: `GET /api/v1/sales/summary`

**Query Parameters**:
- `period`: daily, weekly, monthly, yearly
- `start_date`: Start date
- `end_date`: End date

**Response**:
```json
{
  "status": "success",
  "data": {
    "total_sales": 50000000.00,
    "total_invoices": 125,
    "average_sale": 400000.00,
    "top_customers": [
      {
        "customer_name": "John Doe",
        "total_sales": 5000000.00
      }
    ],
    "sales_by_period": [
      {
        "period": "2025-01",
        "total": 15000000.00,
        "count": 45
      }
    ]
  }
}
```

---

## 🛒 PURCHASE MANAGEMENT

### 1. Get All Purchases
**Endpoint**: `GET /api/v1/purchases`

**Query Parameters**:
- `start_date`: Filter from date
- `end_date`: Filter to date
- `vendor_id`: Filter by vendor
- `status`: draft, pending_approval, approved, received, paid

### 2. Create Purchase
**Endpoint**: `POST /api/v1/purchases`

**Request Body**:
```json
{
  "vendor_id": 1,
  "purchase_date": "2025-01-01",
  "expected_date": "2025-01-15",
  "items": [
    {
      "product_id": 1,
      "quantity": 50,
      "unit_price": 100000.00,
      "tax_rate": 0.11
    }
  ],
  "notes": "Purchase order notes",
  "payment_terms": 30
}
```

### 3. Submit for Approval
**Endpoint**: `POST /api/v1/purchases/{id}/submit-approval`

### 4. Approve Purchase
**Endpoint**: `POST /api/v1/purchases/{id}/approve`

**Request Body**:
```json
{
  "notes": "Approved by director"
}
```

### 5. Purchase Summary
**Endpoint**: `GET /api/v1/purchases/summary`

---

## 💳 PAYMENT MANAGEMENT (SSOT)

### 1. Create Receivable Payment
**Endpoint**: `POST /api/v1/payments/ssot/receivable`

**Request Body**:
```json
{
  "customer_id": 1,
  "amount": 1000000.00,
  "payment_date": "2025-01-01",
  "payment_method": "bank_transfer",
  "bank_account_id": 5,
  "reference": "TF-001",
  "invoices": [
    {
      "invoice_id": 10,
      "amount": 1000000.00
    }
  ],
  "notes": "Payment received"
}
```

### 2. Create Payable Payment
**Endpoint**: `POST /api/v1/payments/ssot/payable`

**Request Body**:
```json
{
  "vendor_id": 2,
  "amount": 2000000.00,
  "payment_date": "2025-01-01",
  "payment_method": "bank_transfer",
  "bank_account_id": 5,
  "reference": "PAY-001",
  "bills": [
    {
      "bill_id": 15,
      "amount": 2000000.00
    }
  ],
  "notes": "Payment to vendor"
}
```

### 3. Get Payment with Journal
**Endpoint**: `GET /api/v1/payments/ssot/{id}`

### 4. Reverse Payment
**Endpoint**: `POST /api/v1/payments/ssot/{id}/reverse`

**Request Body**:
```json
{
  "reason": "Payment error",
  "reversal_date": "2025-01-02"
}
```

---

## 🏦 CASH & BANK MANAGEMENT

### 1. Get Cash Bank Accounts
**Endpoint**: `GET /api/v1/cash-bank/accounts`

**Response**:
```json
{
  "status": "success",
  "data": [
    {
      "id": 1,
      "account_id": 5,
      "account_name": "Cash in Bank - BCA",
      "account_number": "1234567890",
      "bank_name": "Bank BCA",
      "balance": 50000000.00,
      "is_active": true
    }
  ]
}
```

### 2. Create Cash Bank Account
**Endpoint**: `POST /api/v1/cash-bank/accounts`

**Request Body**:
```json
{
  "account_id": 5,
  "account_number": "9876543210",
  "bank_name": "Bank Mandiri",
  "branch": "Jakarta Pusat",
  "account_holder": "PT. Company Name",
  "opening_balance": 10000000.00,
  "is_active": true
}
```

### 3. Process Deposit
**Endpoint**: `POST /api/v1/cash-bank/deposit`

**Request Body**:
```json
{
  "cash_bank_account_id": 1,
  "amount": 5000000.00,
  "transaction_date": "2025-01-01",
  "description": "Initial deposit",
  "reference": "DEP-001",
  "source_account_id": 10
}
```

### 4. Process Owner Drawing / Prive Withdrawal
**Endpoint**: `POST /api/v1/cash-bank/transactions/withdrawal`

Endpoint ini hanya untuk pengambilan pemilik/Prive. Untuk beban gunakan Pembayaran COA, untuk hutang gunakan pembayaran Purchase/termin, dan untuk aset gunakan Purchases.

**Request Body**:
```json
{
  "account_id": 1,
  "date": "2026-08-21",
  "amount": 2000000.00,
  "reference": "PRV-20260821-001",
  "notes": "Pengambilan pemilik sesuai persetujuan",
  "purpose": "OWNER_DRAWING",
  "target_account_id": 3102
}
```

### 5. Bank Transfer
**Endpoint**: `POST /api/v1/cash-bank/transfer`

**Request Body**:
```json
{
  "from_account_id": 1,
  "to_account_id": 2,
  "amount": 3000000.00,
  "transaction_date": "2025-01-01",
  "description": "Inter-bank transfer",
  "reference": "TF-001"
}
```

---

## 📊 REPORTING

### 1. Balance Sheet
**Endpoint**: `GET /api/v1/reports/balance-sheet`

**Query Parameters**:
- `as_of_date`: Date for balance sheet (YYYY-MM-DD)
- `format`: json, pdf, excel

### 2. Profit & Loss
**Endpoint**: `GET /api/v1/reports/profit-loss`

**Query Parameters**:
- `start_date`: Start date
- `end_date`: End date
- `format`: json, pdf, excel

### 3. Cash Flow Statement
**Endpoint**: `GET /api/v1/reports/cash-flow`

**Query Parameters**:
- `start_date`: Start date
- `end_date`: End date
- `method`: direct, indirect

### 4. Trial Balance
**Endpoint**: `GET /api/v1/reports/trial-balance`

**Query Parameters**:
- `as_of_date`: Date for trial balance
- `include_zero_balance`: true/false

---

## 🔍 TIPS PENGGUNAAN SWAGGER UI

### 1. **Login dan Authorization**
1. Buka Swagger UI: http://localhost:8080/swagger
2. Klik "Authorize" button di pojok kanan atas
3. Gunakan endpoint `POST /api/v1/auth/login` untuk login
4. Copy token dari response
5. Paste di field Authorization dengan format: `Bearer <token>`
6. Klik "Authorize"

### 2. **Testing Endpoints**
1. Pilih endpoint yang ingin dicoba
2. Klik "Try it out"
3. Isi parameter yang diperlukan
4. Klik "Execute"
5. Lihat response di bagian "Response body"

### 3. **Common Headers**
```
Authorization: Bearer <your_jwt_token>
Content-Type: application/json
Accept: application/json
```

### 4. **Error Responses**
- `400`: Bad Request - Invalid request data
- `401`: Unauthorized - Missing or invalid token
- `403`: Forbidden - Insufficient permissions
- `404`: Not Found - Resource not found
- `422`: Validation Error - Invalid input data
- `500`: Internal Server Error - Server error

### 5. **Pagination**
Untuk endpoint yang mendukung pagination:
```
GET /api/v1/accounts?page=1&limit=20
```

Response akan include pagination info:
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "total_pages": 5,
    "has_next": true,
    "has_prev": false
  }
}
```

---

## 📋 WORKFLOW EXAMPLES

### Complete Sales Process
```
1. POST /api/v1/sales (create sale)
2. POST /api/v1/sales/{id}/confirm (confirm sale)
3. POST /api/v1/sales/{id}/invoice (generate invoice)
4. POST /api/v1/payments/ssot/receivable (record payment)
5. GET /api/v1/sales/{id}/invoice/pdf (print invoice)
```

### Complete Purchase Process
```
1. POST /api/v1/purchases (create purchase)
2. POST /api/v1/purchases/{id}/submit-approval (submit for approval)
3. POST /api/v1/purchases/{id}/approve (approve)
4. POST /api/v1/payments/ssot/payable (record payment)
```

### Asset Management Process
```
1. POST /api/v1/assets (create asset)
2. POST /api/v1/assets/{id}/capitalize (capitalize)
3. GET /api/v1/assets/{id}/depreciation-schedule (view schedule)
```

---

**🎉 Dokumentasi ini mencakup semua API yang tersedia di sistem akuntansi. Gunakan Swagger UI untuk testing dan eksplorasi lebih lanjut!**

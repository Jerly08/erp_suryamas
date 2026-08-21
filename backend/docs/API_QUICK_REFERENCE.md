# 📋 API QUICK REFERENCE - SISTEM AKUNTANSI

## 🔗 Swagger UI Access
- **URL**: http://localhost:8080/swagger
- **JSON**: http://localhost:8080/openapi/enhanced-doc.json

## 🔐 Authentication
```bash
POST /api/v1/auth/login
Body: {"email": "admin@company.com", "password": "admin123"}
Header: Authorization: Bearer <token>
```

---

## 🗂️ API ENDPOINTS REFERENCE

### 👥 USERS
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/users` | List users |
| GET | `/api/v1/users/{id}` | Get user by ID |
| POST | `/api/v1/users` | Create user |
| PUT | `/api/v1/users/{id}` | Update user |
| DELETE | `/api/v1/users/{id}` | Delete user |
| GET | `/api/v1/profile` | Current user profile |

### 📊 ACCOUNTS (Chart of Accounts)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/accounts` | List accounts |
| GET | `/api/v1/accounts/{id}` | Get account by ID |
| POST | `/api/v1/accounts` | Create account |
| PUT | `/api/v1/accounts/{id}` | Update account |
| DELETE | `/api/v1/accounts/{id}` | Delete account |
| GET | `/api/v1/accounts/{id}/balance-history` | Account balance history |
| POST | `/api/v1/accounts/export` | Export accounts |

### 👤 CONTACTS (Customers & Vendors)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/contacts` | List contacts |
| GET | `/api/v1/contacts/{id}` | Get contact by ID |
| POST | `/api/v1/contacts` | Create contact |
| PUT | `/api/v1/contacts/{id}` | Update contact |
| DELETE | `/api/v1/contacts/{id}` | Delete contact |
| GET | `/api/v1/contacts/{id}/transactions` | Contact transactions |
| POST | `/api/v1/contacts/export` | Export contacts |

### 📦 PRODUCTS
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/products` | List products |
| GET | `/api/v1/products/{id}` | Get product by ID |
| POST | `/api/v1/products` | Create product |
| PUT | `/api/v1/products/{id}` | Update product |
| DELETE | `/api/v1/products/{id}` | Delete product |
| POST | `/api/v1/products/{id}/stock-adjustment` | Stock adjustment |
| GET | `/api/v1/products/{id}/stock-history` | Stock history |
| POST | `/api/v1/products/export` | Export products |

### 🏢 ASSETS (Fixed Assets)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/assets` | List assets |
| GET | `/api/v1/assets/{id}` | Get asset by ID |
| POST | `/api/v1/assets` | Create asset |
| PUT | `/api/v1/assets/{id}` | Update asset |
| DELETE | `/api/v1/assets/{id}` | Delete asset |
| POST | `/api/v1/assets/{id}/capitalize` | Capitalize asset |
| GET | `/api/v1/assets/{id}/depreciation-schedule` | Depreciation schedule |
| GET | `/api/v1/assets/categories` | Asset categories |
| POST | `/api/v1/assets/categories` | Create category |
| GET | `/api/v1/assets/summary` | Assets summary |

### ⚙️ SETTINGS
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/settings` | Get system settings |
| PUT | `/api/v1/settings` | Update settings |
| PUT | `/api/v1/settings/company` | Update company info |
| PUT | `/api/v1/settings/system` | Update system config |
| POST | `/api/v1/settings/company/logo` | Upload company logo |
| POST | `/api/v1/settings/reset` | Reset to defaults |
| GET | `/api/v1/settings/validation-rules` | Validation rules |
| GET | `/api/v1/settings/history` | Settings history |

### 🏦 TAX ACCOUNTS
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tax-accounts` | List tax accounts |
| GET | `/api/v1/tax-accounts/current` | Current tax settings |
| POST | `/api/v1/tax-accounts` | Create tax settings |
| PUT | `/api/v1/tax-accounts/{id}` | Update tax settings |
| POST | `/api/v1/tax-accounts/{id}/activate` | Activate settings |
| GET | `/api/v1/tax-accounts/accounts` | Available accounts |
| POST | `/api/v1/tax-accounts/validate` | Validate configuration |
| POST | `/api/v1/tax-accounts/refresh-cache` | Refresh cache |
| GET | `/api/v1/tax-accounts/suggestions` | Account suggestions |
| GET | `/api/v1/tax-accounts/status` | Tax accounts status |

### 💰 SALES
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/sales` | List sales |
| GET | `/api/v1/sales/{id}` | Get sale by ID |
| POST | `/api/v1/sales` | Create sale |
| PUT | `/api/v1/sales/{id}` | Update sale |
| DELETE | `/api/v1/sales/{id}` | Delete sale |
| POST | `/api/v1/sales/{id}/confirm` | Confirm sale |
| POST | `/api/v1/sales/{id}/invoice` | Generate invoice |
| POST | `/api/v1/sales/{id}/cancel` | Cancel sale |
| GET | `/api/v1/sales/{id}/payments` | Sale payments |
| POST | `/api/v1/sales/{id}/payments` | Create payment |
| GET | `/api/v1/sales/{id}/invoice/pdf` | Invoice PDF |
| GET | `/api/v1/sales/summary` | Sales summary |
| GET | `/api/v1/sales/analytics` | Sales analytics |

### 🛒 PURCHASES
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/purchases` | List purchases |
| GET | `/api/v1/purchases/{id}` | Get purchase by ID |
| POST | `/api/v1/purchases` | Create purchase |
| PUT | `/api/v1/purchases/{id}` | Update purchase |
| DELETE | `/api/v1/purchases/{id}` | Delete purchase |
| POST | `/api/v1/purchases/{id}/submit-approval` | Submit for approval |
| POST | `/api/v1/purchases/{id}/approve` | Approve purchase |
| POST | `/api/v1/purchases/{id}/reject` | Reject purchase |
| GET | `/api/v1/purchases/{id}/payments` | Purchase payments |
| POST | `/api/v1/purchases/{id}/payments` | Create payment |
| GET | `/api/v1/purchases/summary` | Purchase summary |

### 💳 PAYMENTS (SSOT)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/payments/ssot` | List payments |
| GET | `/api/v1/payments/ssot/{id}` | Get payment with journal |
| POST | `/api/v1/payments/ssot/receivable` | Create receivable payment |
| POST | `/api/v1/payments/ssot/payable` | Create payable payment |
| POST | `/api/v1/payments/ssot/{id}/reverse` | Reverse payment |
| POST | `/api/v1/payments/ssot/preview-journal` | Preview journal |
| GET | `/api/v1/payments/ssot/{id}/balance-updates` | Account balance updates |
| GET | `/api/v1/payments/summary` | Payment summary |

### 🏦 CASH & BANK
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/cash-bank/accounts` | List cash bank accounts |
| GET | `/api/v1/cash-bank/accounts/{id}` | Get cash bank by ID |
| POST | `/api/v1/cash-bank/accounts` | Create cash bank account |
| PUT | `/api/v1/cash-bank/accounts/{id}` | Update cash bank account |
| DELETE | `/api/v1/cash-bank/accounts/{id}` | Delete cash bank account |
| GET | `/api/v1/cash-bank/accounts/{id}/transactions` | Account transactions |
| POST | `/api/v1/cash-bank/deposit` | Process deposit |
| POST | `/api/v1/cash-bank/transactions/withdrawal` | Controlled owner drawing / Prive withdrawal |
| POST | `/api/v1/cash-bank/transfer` | Bank transfer |
| GET | `/api/v1/cash-bank/balance-summary` | Balance summary |

### 📊 REPORTS
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/reports/balance-sheet` | Balance sheet |
| GET | `/api/v1/reports/profit-loss` | Profit & loss |
| GET | `/api/v1/reports/cash-flow` | Cash flow statement |
| GET | `/api/v1/reports/trial-balance` | Trial balance |
| GET | `/api/v1/reports/general-ledger` | General ledger |
| GET | `/api/v1/reports/accounts-receivable` | Accounts receivable |
| GET | `/api/v1/reports/accounts-payable` | Accounts payable |
| GET | `/api/v1/reports/inventory-report` | Inventory report |
| GET | `/api/v1/reports/financial-ratios` | Financial ratios |
| GET | `/api/v1/reports/financial-dashboard` | Financial dashboard |

### 🔍 MONITORING & ADMIN
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/monitoring/status` | System status |
| GET | `/api/v1/monitoring/performance/metrics` | Performance metrics |
| GET | `/api/v1/admin/balance-health/check` | Balance health check |
| POST | `/api/v1/admin/balance-health/auto-heal` | Auto-heal balances |

---

## 🔑 COMMON PARAMETERS

### Query Parameters
- `page`: Page number (default: 1)
- `limit`: Items per page (default: 20)
- `search`: Search term
- `start_date`: Start date (YYYY-MM-DD)
- `end_date`: End date (YYYY-MM-DD)
- `is_active`: Filter active items (true/false)
- `type`: Filter by type
- `status`: Filter by status

### Response Codes
- `200`: Success
- `201`: Created
- `204`: No Content
- `400`: Bad Request
- `401`: Unauthorized
- `403`: Forbidden
- `404`: Not Found
- `422`: Validation Error
- `500`: Internal Server Error

---

## 📝 EXAMPLE REQUESTS

### Login
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@company.com", "password": "admin123"}'
```

### Create Account
```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "code": "1-1001",
    "name": "Petty Cash",
    "type": "ASSET",
    "category": "Current Assets"
  }'
```

### Get Assets Summary
```bash
curl -X GET http://localhost:8080/api/v1/assets/summary \
  -H "Authorization: Bearer <token>"
```

### Create Fixed Asset
```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "Office Computer",
    "asset_code": "IT-001",
    "category": "IT Equipment",
    "purchase_date": "2025-01-01",
    "purchase_price": 10000000.00,
    "useful_life": 3,
    "depreciation_method": "straight_line"
  }'
```

---

## 🎯 TESTING WORKFLOW

### 1. Authentication Flow
```
1. POST /api/v1/auth/login
2. Copy token from response
3. Use token in Authorization header for all requests
```

### 2. Master Data Setup
```
1. PUT /api/v1/settings/company (company profile)
2. POST /api/v1/tax-accounts (tax configuration)
3. POST /api/v1/accounts (chart of accounts)
4. POST /api/v1/contacts (customers/vendors)
5. POST /api/v1/products (product catalog)
```

### 3. Asset Management Flow
```
1. POST /api/v1/assets (create asset)
2. POST /api/v1/assets/{id}/capitalize (capitalize)
3. GET /api/v1/assets/{id}/depreciation-schedule (view schedule)
4. GET /api/v1/assets/summary (summary report)
```

### 4. Transaction Flow
```
1. POST /api/v1/sales (create sale)
2. POST /api/v1/sales/{id}/confirm (confirm)
3. POST /api/v1/sales/{id}/invoice (generate invoice)
4. POST /api/v1/payments/ssot/receivable (record payment)
```

---

## 💡 PRO TIPS

### 1. **Use Swagger UI for Testing**
- Interactive documentation
- Built-in request/response examples
- Authorization management
- Real-time API testing

### 2. **Common Filters**
```
GET /api/v1/accounts?type=ASSET&is_active=true
GET /api/v1/contacts?type=customer&search=john
GET /api/v1/products?category=Electronics&low_stock=true
GET /api/v1/assets?status=active&category=IT Equipment
```

### 3. **Pagination**
```
GET /api/v1/accounts?page=2&limit=50
```

### 4. **Export Data**
```
GET /api/v1/reports/balance-sheet?format=pdf
GET /api/v1/sales/{id}/invoice/pdf
POST /api/v1/accounts/export
```

---

**🔗 Bookmark**: http://localhost:8080/swagger untuk akses cepat ke dokumentasi API!

**📚 Full Documentation**: Lihat `COMPLETE_API_DOCUMENTATION.md` untuk penjelasan lengkap setiap endpoint.

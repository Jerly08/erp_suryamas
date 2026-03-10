const fs = require('fs');
const file = 'D:/Project/erp_akuntansi/frontend/app/purchases/page.tsx';
let content = fs.readFileSync(file, 'utf8');

const regex = /if \(\(!item\.product_id && item\.item_type === 'INVENTORY'\) \|\| \(!item\.expense_account_id && \(isServiceCompany \|\| item\.item_type === 'SERVICE' \|\| item\.item_type === 'ASSET'\)\)\) \{/;

// Add better error handling logic to checking ASSET too. First, I'll see where the handleSave and loop of items validation is.

cat << 'INNER_EOF' > frontend_patch.js
const fs = require('fs');
const file = 'D:/Project/erp_akuntansi/frontend/app/purchases/page.tsx';
let content = fs.readFileSync(file, 'utf8');

// 1. Add fixedAssetAccounts state and loadingFixedAssetAccounts
content = content.replace(
  /const \[expenseAccounts, setExpenseAccounts\] = useState<GLAccount\[\]>\(\[\]\);/,
  "const [expenseAccounts, setExpenseAccounts] = useState<GLAccount[]>([]);\n  const [fixedAssetAccounts, setFixedAssetAccounts] = useState<GLAccount[]>([]);\n  const [loadingFixedAssetAccounts, setLoadingFixedAssetAccounts] = useState(false);"
);

// 2. Add fetchFixedAssetAccounts() logic 
content = content.replace(
  /const fetchExpenseAccounts = async \(\) => \{[\s\S]*?catch \(error\) \{[\s\S]*?\} finally \{[\s\S]*?\}\n  \};/,
  `$&

  const fetchFixedAssetAccounts = async () => {
    try {
      setLoadingFixedAssetAccounts(true);
      const accounts = await assetService.getFixedAssetAccounts();
      setFixedAssetAccounts(accounts);
    } catch (error) {
      console.error('Failed to fetch fixed asset accounts:', error);
      toast({
        title: 'Error',
        description: 'Failed to load fixed asset accounts. Please try again.',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoadingFixedAssetAccounts(false);
    }
  };`
);

// 3. Call fetchFixedAssetAccounts() inside handleCreate(...)
content = content.replace(
  /fetchProducts\(\);\n      fetchExpenseAccounts\(\);/,
  "fetchProducts();\n      fetchExpenseAccounts();\n      fetchFixedAssetAccounts();"
);

// 4. Update validation to require expense_account_id for ASSET item types too
content = content.replace(
  /if \(!item\.expense_account_id && isServiceCompany\) \{/,
  "if (!item.expense_account_id && (isServiceCompany || item.item_type === 'ASSET')) {"
);

// 5. In UI table row, restore ExpenseAccountID selection for ASSET items
const dropdownRegex = /\{\(isServiceCompany || item\.item_type === 'SERVICE'\) && \([\s\S]*?<\/[A-Za-z]+>\n\s*\)\}/;
const newDropdown = `{(isServiceCompany || item.item_type === 'SERVICE' || item.item_type === 'ASSET') && (
  canListExpenseAccounts ? (
    <Box maxW="240px">
      <SearchableSelect
        options={item.item_type === 'ASSET' ? fixedAssetAccounts.map(acc => ({
          id: acc.id!,
          code: acc.code,
          name: acc.name,
          active: acc.is_active
        })) : expenseAccounts.map(acc => ({
          id: acc.id!,
          code: acc.code,
          name: acc.name,
          active: acc.is_active
        }))}
        value={item.expense_account_id ? parseInt(item.expense_account_id) : undefined}
        onChange={(val) => {
          const newItems = [...formData.items];
          newItems[index].expense_account_id = val ? val.toString() : '';
          setFormData({ ...formData, items: newItems });
        }}
        placeholder={item.item_type === 'ASSET' ? "Pilih Akun Aset Tetap..." : "Pilih Akun Beban..."}
        isDisabled={!isEditing}
        isLoading={item.item_type === 'ASSET' ? loadingFixedAssetAccounts : loadingExpenseAccounts}
      />
    </Box>
  ) : (
    <Tooltip label="You don't have permission to view expense accounts">
      <Badge colorScheme="red">Akun Tidak Tersedia</Badge>
    </Tooltip>
  )
)}`;

content = content.replace(/\{\(isServiceCompany \|\| item\.item_type === 'SERVICE'\) && \([\s\S]*?:\s*\([\s\S]*?<\/[A-Za-z]+>\s*\)\s*\)?\}\n/g, newDropdown + '\n');
content = content.replace(/\{\(isServiceCompany \|\| item\.item_type === 'SERVICE'\) \? \([\s\S]*?:\s*\([\s\S]*?<\/[A-Za-z]+>\s*\)\s*\)?\}/g, newDropdown);
content = content.replace(/\{\(isServiceCompany \|\| item\.item_type === 'SERVICE'\) && \([\s\S]*?<\/[A-Za-z]+>\s*\)\s*\}/g, newDropdown);

// The exact string matching from page.tsx for the cell
const oldCell = `{!isAssetMode ? (
                                        <Text fontSize="sm" w="full" isTruncated>{item.description}</Text>
                                      ) : (
                                        // When in ASSET item type, expense account isn't needed, just description/config
                                        <HStack spacing={2} w="full">
                                          <Text fontSize="sm" isTruncated flex="1">{item.description}</Text>
                                          <Badge colorScheme="purple" fontSize="xs">ASET</Badge>
                                        </HStack>
                                      )}
                                    </Box>
                                  ) : (
                                    <VStack align="stretch" spacing={2} w="full">
                                      <AutoResizeTextarea
                                        value={item.description || ''}
                                        onChange={(e) => {
                                          const newItems = [...formData.items];
                                          newItems[index].description = e.target.value;
                                          setFormData({ ...formData, items: newItems });
                                        }}
                                        placeholder={isAssetMode ? 'Nama atau deskripsi aset' : 'Deskripsi Barang/Jasa'}
                                        isDisabled={!isEditing}
                                        rows={1}
                                        minH="40px"
                                      />
                                      {/* Legacy branch for Service routing mapping removed */}`;

// Actually I'll use a safer string replacing to catch what it actually looks like in page.tsx
content = content.replace(
  /\{\(isServiceCompany \|\| item\.item_type === 'SERVICE'\)\s*\?\s*\([\s\S]*?<\/[a-zA-Z]+>\s*\)\s*:\s*canListExpenseAccounts\s*\?\s*\([\s\S]*?<\/[a-zA-Z]+>\s*\)\s*:\s*\([\s\S]*?<\/[a-zA-Z]+>\s*\)\s*\}/g,
  newDropdown
);

fs.writeFileSync(file, content);
INNER_EOF
node frontend_patch.js

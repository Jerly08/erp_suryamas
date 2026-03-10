cat << 'INNER_EOF' > auto_create_patch.js
const fs = require('fs');
const file = 'D:/Project/erp_akuntansi/backend/services/purchase_service.go';
let content = fs.readFileSync(file, 'utf8');

const regex = /func \(s \*PurchaseService\) autoCreateAssetsFromPurchase\(purchase \*models\.Purchase\) \{[\s\S]*?\n\}/;

const newFunc = `func (s *PurchaseService) autoCreateAssetsFromPurchase(purchase *models.Purchase) {
	if len(purchase.PurchaseItems) == 0 {
		return
	}

	assetRepo := repositories.NewAssetRepository(s.db)
	assetService := NewAssetService(assetRepo, s.db)
	assetJournalSvc := NewAssetJournalServiceSSOT(s.db)

	for _, item := range purchase.PurchaseItems {
		// Determine if this item should create an asset
		isAssetFromFlag := item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset
		
		if !isAssetFromFlag {
			continue // Not an asset item, skip
		}

		// Check if asset already exists for this purchase item (prevent duplicates)
		var existingCount int64
		s.db.Model(&models.Asset{}).Where(
			"notes LIKE ? AND deleted_at IS NULL",
			fmt.Sprintf("%%PurchaseItem:%d%%", item.ID),
		).Count(&existingCount)
		if existingCount > 0 {
			fmt.Printf("ℹ️ Asset already exists for purchase item %d, skipping\n", item.ID)
			continue
		}

		// Determine asset name
		assetName := item.Description
		if assetName == "" {
			// Fallback: use product name if available
			if item.ProductID != nil && *item.ProductID > 0 {
				var product models.Product
				if err := s.db.First(&product, *item.ProductID).Error; err == nil {
					assetName = product.Name
				}
			}
			if assetName == "" {
				assetName = fmt.Sprintf("Aset dari PO %s", purchase.Code)
			}
		}

		// Determine category from AssetCategoryID (user-selected)
		category := "Peralatan" // Default
		var assetCategoryID *uint
		var assetAccountID *uint

		if item.AssetCategoryID != nil && *item.AssetCategoryID > 0 {
			// User selected asset category from UI
			assetCategoryID = item.AssetCategoryID
			var assetCat models.AssetCategory
			if err := s.db.First(&assetCat, *item.AssetCategoryID).Error; err == nil {
				category = assetCat.Name
			}
		}

		// Firmly use ExpenseAccountID as the debit Asset Account
		if item.ExpenseAccountID != 0 {
			assetAccountID = &item.ExpenseAccountID
		}

		// Resolve depreciation expense account (Beban Penyusutan)
		var depExpAccount models.Account
		var depExpID *uint
		if err := s.db.Where("category = ? AND type = 'EXPENSE' AND deleted_at IS NULL", models.CategoryDepreciationExp).First(&depExpAccount).Error; err == nil {
			depExpID = &depExpAccount.ID
		} else if err := s.db.Where("code LIKE '6103%' AND type = 'EXPENSE' AND deleted_at IS NULL").First(&depExpAccount).Error; err == nil {
			depExpID = &depExpAccount.ID
		}

		// Resolve accumulated depreciation account (1202-xxx mirrors 1201-xxx)
		var accumDepID *uint
		if assetAccountID != nil {
			var assetAcct models.Account
			if err := s.db.First(&assetAcct, *assetAccountID).Error; err == nil {
				accumDepCode := strings.Replace(assetAcct.Code, "1201", "1202", 1)
				var accumDepAccount models.Account
				if err := s.db.Where("code = ? AND deleted_at IS NULL", accumDepCode).First(&accumDepAccount).Error; err == nil {
					accumDepID = &accumDepAccount.ID
				}
			}
		}

		// Set default useful life and depreciation method based on category
		usefulLife := 5 // Default 5 years
		depMethod := models.DepreciationMethodStraightLine
		salvageValue := 0.0

		switch category {
		case "Tanah":
			usefulLife = 0 // Tanah tidak disusutkan
			depMethod = ""
		case "Bangunan":
			usefulLife = 20 // 20 tahun
			salvageValue = item.TotalPrice * 0.10
		case "Kendaraan":
			usefulLife = 5 // 5 tahun
			salvageValue = item.TotalPrice * 0.10
		case "Peralatan":
			usefulLife = 4 // 4 tahun
			salvageValue = item.TotalPrice * 0.05
		case "Inventaris Kantor":
			usefulLife = 4 // 4 tahun
			salvageValue = item.TotalPrice * 0.05
		}

		// Create asset for each unit quantity
		for q := 0; q < item.Quantity; q++ {
			unitAssetName := assetName
			if item.Quantity > 1 {
				unitAssetName = fmt.Sprintf("%s (#%d)", assetName, q+1)
			}

			asset := &models.Asset{
				Name:               unitAssetName,
				Category:           category,
				CategoryID:         assetCategoryID,
				Status:             models.AssetStatusPendingSetup,
				PurchaseDate:       purchase.Date,
				PurchasePrice:      item.UnitPrice, // Per-unit price
				SalvageValue:       salvageValue / float64(item.Quantity),
				UsefulLife:         usefulLife,
				DepreciationMethod: depMethod,
				IsActive:           false, // Inactive until user completes setup
				Notes: fmt.Sprintf(
					"Auto-created from Purchase %s (PurchaseItem:%d). Silakan lengkapi masa manfaat, nilai sisa, dan metode penyusutan lalu aktifkan aset.",
					purchase.Code, item.ID,
				),
				AssetAccountID:                   assetAccountID,
				DepreciationAccountID:            depExpID,
				AccumulatedDepreciationAccountID: accumDepID,
			}

			// Create asset via service (generates code automatically)
			if err := assetService.CreateAsset(asset); err != nil {
				fmt.Printf("⚠️ [AUTO-ASSET] Failed to create asset for item %d (%s): %v\n", item.ID, unitAssetName, err)
				continue
			}

			fmt.Printf("✅ [AUTO-ASSET] Created asset %s (%s) from purchase %s - Rp %.2f, useful life: %d years\n",
				asset.Code, asset.Name, purchase.Code, asset.PurchasePrice, asset.UsefulLife)
		}

		// NOTE: SSOT journal for this asset is NOT created here because the purchase
		// journal (step 2) already handles: Dr 1201-xxx / Cr Hutang/Kas/Bank
		_ = assetJournalSvc // Available if needed for future use
	}
}`;

content = content.replace(regex, newFunc);
fs.writeFileSync(file, content);
INNER_EOF
node auto_create_patch.js

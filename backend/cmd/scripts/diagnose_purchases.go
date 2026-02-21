package main

import (
	"encoding/json"
	"log"
	"os"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var result map[string]interface{} = make(map[string]interface{})

	err := db.Transaction(func(tx *gorm.DB) error {
		var purchases []models.Purchase
		tx.Where("code IN (?, ?)", "PO/2026/02/0011", "PO/2026/02/0012").Find(&purchases)

		var pList []map[string]interface{}
		for _, p := range purchases {
			var jes []models.SSOTJournalEntry
			tx.Where("source_type = 'PURCHASE' AND source_id = ?", p.ID).Find(&jes)

			var jList []map[string]interface{}
			for _, je := range jes {
				jList = append(jList, map[string]interface{}{
					"id":           je.ID,
					"entry_number": je.EntryNumber,
					"status":       je.Status,
					"deleted":      je.DeletedAt != nil,
				})
			}

			pList = append(pList, map[string]interface{}{
				"id":              p.ID,
				"code":            p.Code,
				"status":          p.Status,
				"approval_status": p.ApprovalStatus,
				"total":           p.TotalAmount,
				"paid":            p.PaidAmount,
				"deleted":         p.DeletedAt.Valid,
				"journals":        jList,
			})
		}
		result["purchases"] = pList

		var pps []models.PurchasePayment
		tx.Order("id desc").Limit(3).Find(&pps)
		var ppList []map[string]interface{}
		for _, pp := range pps {
			ppList = append(ppList, map[string]interface{}{
				"id":          pp.ID,
				"purchase_id": pp.PurchaseID,
				"amount":      pp.Amount,
				"deleted":     pp.DeletedAt.Valid,
			})
		}
		result["payments"] = ppList

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("diagnose.json", b, 0644)
}

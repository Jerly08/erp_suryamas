package services

import (
	"app-sistem-akuntansi/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func (s *PurchaseService) validateLinkedPurchaseRequestSelection(purchase *models.Purchase, allowReservedByCurrentPurchase bool) (*models.PurchaseRequest, float64, error) {
	if purchase == nil {
		return nil, 0, nil
	}
	if purchase.PurchaseRequestID == nil {
		for _, purchaseItem := range purchase.PurchaseItems {
			if purchaseItem.PurchaseRequestItemID != nil {
				return nil, 0, fmt.Errorf("purchase_request_id is required when purchase item is linked to purchase request item %d", *purchaseItem.PurchaseRequestItemID)
			}
		}
		return nil, 0, nil
	}

	var pr models.PurchaseRequest
	if err := s.db.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
		return nil, 0, fmt.Errorf("linked purchase request not found: %v", err)
	}

	if pr.Status != models.PRStatusApproved {
		return nil, 0, fmt.Errorf("cannot create purchase from unapproved request (Status: %s)", pr.Status)
	}

	var existingPurchase models.Purchase
	query := s.db.
		Select("id", "code", "status").
		Where("purchase_request_id = ? AND deleted_at IS NULL AND status <> ?", pr.ID, models.PurchaseStatusCancelled)
	if allowReservedByCurrentPurchase && purchase.ID > 0 {
		query = query.Where("id <> ?", purchase.ID)
	}
	err := query.Order("id ASC").First(&existingPurchase).Error
	if err == nil {
		return nil, 0, fmt.Errorf("purchase request %s is already linked to purchase %s", pr.PRNumber, existingPurchase.Code)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, fmt.Errorf("failed to validate purchase request usage: %v", err)
	}

	linkedByItemQuery := s.db.
		Table("purchases AS linked_purchases").
		Select("linked_purchases.id, linked_purchases.code, linked_purchases.status").
		Where("linked_purchases.deleted_at IS NULL AND linked_purchases.status <> ?", models.PurchaseStatusCancelled).
		Where(`(
			EXISTS (
				SELECT 1
				FROM purchase_request_items linked_request_items
				WHERE linked_request_items.purchase_request_id = ?
					AND linked_request_items.purchase_id = linked_purchases.id
					AND linked_request_items.deleted_at IS NULL
			)
			OR EXISTS (
				SELECT 1
				FROM purchase_items linked_purchase_items
				JOIN purchase_request_items linked_purchase_request_items
					ON linked_purchase_request_items.id = linked_purchase_items.purchase_request_item_id
					AND linked_purchase_request_items.deleted_at IS NULL
				WHERE linked_purchase_items.purchase_id = linked_purchases.id
					AND linked_purchase_items.deleted_at IS NULL
					AND linked_purchase_request_items.purchase_request_id = ?
			)
		)`, pr.ID, pr.ID)
	if allowReservedByCurrentPurchase && purchase.ID > 0 {
		linkedByItemQuery = linkedByItemQuery.Where("linked_purchases.id <> ?", purchase.ID)
	}
	err = linkedByItemQuery.Order("linked_purchases.id ASC").First(&existingPurchase).Error
	if err == nil {
		return nil, 0, fmt.Errorf("purchase request %s is already linked to purchase %s", pr.PRNumber, existingPurchase.Code)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, fmt.Errorf("failed to validate purchase request item usage: %v", err)
	}

	if len(purchase.PurchaseItems) == 0 {
		return &pr, 0, nil
	}

	requestItemsByID := make(map[uint]models.PurchaseRequestItem, len(pr.Items))
	for _, item := range pr.Items {
		requestItemsByID[item.ID] = item
	}

	selectedBudget := 0.0
	selectedIDs := make(map[uint]struct{}, len(purchase.PurchaseItems))

	for _, purchaseItem := range purchase.PurchaseItems {
		if purchaseItem.PurchaseRequestItemID == nil {
			return nil, 0, fmt.Errorf("purchase items linked to a purchase request must include purchase_request_item_id")
		}

		requestItemID := *purchaseItem.PurchaseRequestItemID
		if _, exists := selectedIDs[requestItemID]; exists {
			return nil, 0, fmt.Errorf("purchase request item %d is selected more than once", requestItemID)
		}
		selectedIDs[requestItemID] = struct{}{}

		requestItem, exists := requestItemsByID[requestItemID]
		if !exists {
			return nil, 0, fmt.Errorf("purchase request item %d was not found in request %s", requestItemID, pr.PRNumber)
		}

		if requestItem.ApprovalStatus != models.PRItemDecisionApproved {
			return nil, 0, fmt.Errorf("purchase request item %d is not approved", requestItemID)
		}

		if requestItem.PurchaseID != nil {
			if !allowReservedByCurrentPurchase || purchase.ID == 0 || *requestItem.PurchaseID != purchase.ID {
				return nil, 0, fmt.Errorf("purchase request item %d is already linked to another purchase", requestItemID)
			}
		}

		if purchaseItem.ProductID == nil || *purchaseItem.ProductID != requestItem.ProductID {
			return nil, 0, fmt.Errorf("purchase item linked to request item %d must use the same product", requestItemID)
		}

		selectedBudget += requestItem.TotalEstimatedPrice
	}

	if len(selectedIDs) == 0 {
		return nil, 0, fmt.Errorf("please select at least one approved purchase request item")
	}

	return &pr, selectedBudget, nil
}

func (s *PurchaseService) syncLinkedPurchaseRequestItems(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	selectedItemIDs := make(map[uint]struct{})
	for _, item := range purchase.PurchaseItems {
		if item.PurchaseRequestItemID != nil {
			selectedItemIDs[*item.PurchaseRequestItemID] = struct{}{}
		}
	}

	if len(selectedItemIDs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		now := time.Now()
		for index := range pr.Items {
			item := &pr.Items[index]

			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				if _, keepReservation := selectedItemIDs[item.ID]; !keepReservation {
					item.PurchaseID = nil
					item.PurchasedAt = nil
				}
			}

			if _, shouldReserve := selectedItemIDs[item.ID]; shouldReserve {
				if item.ApprovalStatus != models.PRItemDecisionApproved {
					return fmt.Errorf("purchase request item %d is not approved", item.ID)
				}
				if item.PurchaseID != nil && *item.PurchaseID != purchase.ID {
					return fmt.Errorf("purchase request item %d is already linked to another purchase", item.ID)
				}

				item.PurchaseID = &purchase.ID
				item.PurchasedAt = &now
			}
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

func (s *PurchaseService) releaseLinkedPurchaseRequestItems(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		changed := false
		for index := range pr.Items {
			item := &pr.Items[index]
			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				item.PurchaseID = nil
				item.PurchasedAt = nil
				changed = true
			}
		}

		if !changed {
			return nil
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

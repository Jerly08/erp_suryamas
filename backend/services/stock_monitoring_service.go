package services

import (
	"app-sistem-akuntansi/models"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type StockMonitoringService struct {
	db                  *gorm.DB
	notificationService *NotificationService
}

func NewStockMonitoringService(db *gorm.DB, notificationService *NotificationService) *StockMonitoringService {
	return &StockMonitoringService{
		db:                  db,
		notificationService: notificationService,
	}
}

// CheckMinimumStock checks all products for minimum stock levels
func (s *StockMonitoringService) CheckMinimumStock() error {
	var products []models.Product

	// Get products where current stock is <= minimum stock
	err := s.db.Where("stock <= min_stock AND min_stock > 0 AND is_active = ? AND is_assembly = ?", true, false).
		Preload("Category").Find(&products).Error
	if err != nil {
		return err
	}

	for _, product := range products {
		if err := s.createMinimumStockNotification(&product); err != nil {
			log.Printf("Failed to create minimum stock notification for product %s: %v", product.Code, err)
		}
	}

	return nil
}

// CheckReorderLevel checks all products for reorder level
func (s *StockMonitoringService) CheckReorderLevel() error {
	var products []models.Product

	// Get products where current stock is <= reorder level
	err := s.db.Where("stock <= reorder_level AND reorder_level > 0 AND is_active = ? AND is_assembly = ?", true, false).
		Preload("Category").Find(&products).Error
	if err != nil {
		return err
	}

	for _, product := range products {
		if err := s.createReorderNotification(&product); err != nil {
			log.Printf("Failed to create reorder notification for product %s: %v", product.Code, err)
		}
	}

	return nil
}

// CheckOutOfStock checks all products for zero stock (out of stock)
func (s *StockMonitoringService) CheckOutOfStock() error {
	var products []models.Product

	// Get products where current stock is 0 and is not a service or assembly
	err := s.db.Where("stock = 0 AND is_service = ? AND is_active = ? AND is_assembly = ?", false, true, false).
		Preload("Category").Find(&products).Error
	if err != nil {
		return err
	}

	for _, product := range products {
		if err := s.createOutOfStockNotification(&product); err != nil {
			log.Printf("Failed to create out of stock notification for product %s: %v", product.Code, err)
		}
	}

	return nil
}

// CheckSingleProductStock checks stock for a specific product after stock update
func (s *StockMonitoringService) CheckSingleProductStock(productID uint) error {
	var product models.Product
	err := s.db.Preload("Category").First(&product, productID).Error
	if err != nil {
		return err
	}

	// Ignore service and assembly products for stock monitoring
	if product.IsService || product.IsAssembly {
		return nil
	}

	// Priority Check 1: Out of Stock
	if product.Stock == 0 {
		if err := s.createOutOfStockNotification(&product); err != nil {
			log.Printf("Failed to create out of stock notification: %v", err)
		}
	} else if product.MinStock > 0 && product.Stock <= product.MinStock {
		// Priority Check 2: Check minimum stock
		if err := s.createMinimumStockNotification(&product); err != nil {
			log.Printf("Failed to create minimum stock notification: %v", err)
		}
	}

	// Check reorder level
	if product.ReorderLevel > 0 && product.Stock <= product.ReorderLevel {
		if err := s.createReorderNotification(&product); err != nil {
			log.Printf("Failed to create reorder notification: %v", err)
		}
	}

	return nil
}

// SyncProductAlerts refreshes alert state for a set of affected products and then
// resolves alerts that are no longer applicable after the stock change.
func (s *StockMonitoringService) SyncProductAlerts(productIDs ...uint) error {
	if s == nil || len(productIDs) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(productIDs))
	var firstErr error

	for _, productID := range productIDs {
		if productID == 0 {
			continue
		}
		if _, exists := seen[productID]; exists {
			continue
		}
		seen[productID] = struct{}{}

		if err := s.CheckSingleProductStock(productID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if err := s.ResolveStockAlerts(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// GetLowStockProducts returns list of products with low stock
func (s *StockMonitoringService) GetLowStockProducts() ([]models.Product, error) {
	var products []models.Product

	err := s.db.Where("(stock <= min_stock AND min_stock > 0) AND is_active = ? AND is_assembly = ?", true, false).
		Preload("Category").Find(&products).Error

	return products, err
}

// GetReorderProducts returns list of products that need reordering
func (s *StockMonitoringService) GetReorderProducts() ([]models.Product, error) {
	var products []models.Product

	err := s.db.Where("(stock <= reorder_level AND reorder_level > 0) AND is_active = ? AND is_assembly = ?", true, false).
		Preload("Category").Find(&products).Error

	return products, err
}

// GetStockAlerts returns comprehensive stock alerts
func (s *StockMonitoringService) GetStockAlerts() (map[string]interface{}, error) {
	alerts := make(map[string]interface{})

	// Get low stock products
	lowStockProducts, err := s.GetLowStockProducts()
	if err != nil {
		return nil, err
	}
	alerts["low_stock_products"] = lowStockProducts
	alerts["low_stock_count"] = len(lowStockProducts)

	// Get reorder products
	reorderProducts, err := s.GetReorderProducts()
	if err != nil {
		return nil, err
	}
	alerts["reorder_products"] = reorderProducts
	alerts["reorder_count"] = len(reorderProducts)

	// Get out of stock products
	var outOfStockCount int64
	err = s.db.Model(&models.Product{}).Where("stock = 0 AND is_active = ? AND is_service = ? AND is_assembly = ?", true, false, false).Count(&outOfStockCount).Error
	if err != nil {
		return nil, err
	}
	alerts["out_of_stock_count"] = outOfStockCount

	return alerts, nil
}

// Private helper methods

func (s *StockMonitoringService) createMinimumStockNotification(product *models.Product) error {
	stockAlert, created, err := s.upsertActiveStockAlert(product, models.StockAlertTypeLowStock, product.MinStock)
	if err != nil {
		log.Printf("[STOCK-ALERT-ERROR] Failed to upsert low stock alert for product %d: %v", product.ID, err)
		return err
	}
	if !created {
		log.Printf("[STOCK-ALERT] Updated existing low stock alert for product '%s' (ID: %d) - Current: %d, Min: %d",
			product.Name, product.ID, product.Stock, product.MinStock)
		return nil
	}
	log.Printf("[STOCK-ALERT] Created new low stock alert for product '%s' (ID: %d) - Current: %d, Min: %d",
		product.Name, product.ID, product.Stock, product.MinStock)

	// Get all inventory managers and admins
	userIDs, err := s.getInventoryManagers()
	if err != nil {
		return err
	}

	title := "🚨 Minimum Stock Alert"
	message := fmt.Sprintf("Product '%s' has reached minimum stock level. Current: %d, Minimum: %d",
		product.Name, product.Stock, product.MinStock)

	data := map[string]interface{}{
		"product_id":     product.ID,
		"product_code":   product.Code,
		"product_name":   product.Name,
		"current_stock":  product.Stock,
		"minimum_stock":  product.MinStock,
		"category_name":  "",
		"alert_type":     "minimum_stock",
		"urgency":        "high",
		"stock_alert_id": stockAlert.ID,
	}

	if product.Category != nil {
		data["category_name"] = product.Category.Name
	}

	dataJSON, _ := json.Marshal(data)

	// Send notification to all inventory managers
	log.Printf("[STOCK-NOTIFICATION] Sending low stock notifications for product '%s' (ID: %d) to %d users",
		product.Name, product.ID, len(userIDs))
	for _, userID := range userIDs {
		if err := s.saveOrUpdateStockNotification(
			userID,
			models.NotificationTypeLowStock,
			product.ID,
			title,
			message,
			string(dataJSON),
			models.NotificationPriorityHigh,
		); err != nil {
			log.Printf("[STOCK-NOTIFICATION-ERROR] Failed to create notification for user %d, product %d: %v",
				userID, product.ID, err)
		}
	}

	return nil
}

func (s *StockMonitoringService) createOutOfStockNotification(product *models.Product) error {
	stockAlert, created, err := s.upsertActiveStockAlert(product, models.StockAlertTypeOutOfStock, 0)
	if err != nil {
		log.Printf("[STOCK-ALERT-ERROR] Failed to upsert out of stock alert for product %d: %v", product.ID, err)
		return err
	}
	if !created {
		log.Printf("[STOCK-ALERT] Updated existing out of stock alert for product '%s' (ID: %d)",
			product.Name, product.ID)
		return nil
	}
	log.Printf("[STOCK-ALERT] Created new out of stock alert for product '%s' (ID: %d)",
		product.Name, product.ID)

	// Get all inventory managers and admins
	userIDs, err := s.getInventoryManagers()
	if err != nil {
		return err
	}

	title := "🚨 Out of Stock Alert"
	message := fmt.Sprintf("CRITICAL: Product '%s' is completely out of stock!", product.Name)

	data := map[string]interface{}{
		"product_id":     product.ID,
		"product_code":   product.Code,
		"product_name":   product.Name,
		"current_stock":  product.Stock,
		"minimum_stock":  product.MinStock,
		"category_name":  "",
		"alert_type":     "out_of_stock",
		"urgency":        "urgent",
		"stock_alert_id": stockAlert.ID,
	}

	if product.Category != nil {
		data["category_name"] = product.Category.Name
	}

	dataJSON, _ := json.Marshal(data)

	// Send notification to all inventory managers
	log.Printf("[STOCK-NOTIFICATION] Sending out of stock notifications for product '%s' (ID: %d) to %d users",
		product.Name, product.ID, len(userIDs))
	for _, userID := range userIDs {
		if err := s.saveOrUpdateStockNotification(
			userID,
			models.NotificationTypeStockOut,
			product.ID,
			title,
			message,
			string(dataJSON),
			models.NotificationPriorityUrgent,
		); err != nil {
			log.Printf("[STOCK-NOTIFICATION-ERROR] Failed to create notification for user %d, product %d: %v",
				userID, product.ID, err)
		}
	}

	return nil
}

func (s *StockMonitoringService) createReorderNotification(product *models.Product) error {
	userIDs, err := s.getInventoryManagers()
	if err != nil {
		return err
	}

	log.Printf("[REORDER-ALERT] Product '%s' (ID: %d) needs reordering - Current: %d, Reorder Level: %d",
		product.Name, product.ID, product.Stock, product.ReorderLevel)

	title := "📋 Reorder Alert"
	message := fmt.Sprintf("Product '%s' needs reordering. Current: %d, Reorder Level: %d",
		product.Name, product.Stock, product.ReorderLevel)

	data := map[string]interface{}{
		"product_id":    product.ID,
		"product_code":  product.Code,
		"product_name":  product.Name,
		"current_stock": product.Stock,
		"reorder_level": product.ReorderLevel,
		"category_name": "",
		"alert_type":    "reorder_needed",
		"urgency":       "medium",
		"suggested_qty": product.MaxStock - product.Stock, // Suggest to fill up to max stock
	}

	if product.Category != nil {
		data["category_name"] = product.Category.Name
	}

	dataJSON, _ := json.Marshal(data)

	// Send notification to all inventory managers
	for _, userID := range userIDs {
		if err := s.saveOrUpdateStockNotification(
			userID,
			models.NotificationTypeReorderAlert,
			product.ID,
			title,
			message,
			string(dataJSON),
			models.NotificationPriorityMedium,
		); err != nil {
			log.Printf("[REORDER-NOTIFICATION-ERROR] Failed to create notification for user %d, product %d: %v",
				userID, product.ID, err)
		}
	}

	return nil
}

func (s *StockMonitoringService) getInventoryManagers() ([]uint, error) {
	var users []models.User
	var userIDs []uint

	// Get users with inventory_manager or admin roles
	err := s.db.Where("role IN ? AND is_active = ?", []string{"inventory_manager", "admin"}, true).
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	return userIDs, nil
}

// ResolveStockAlerts checks and resolves alerts for products with restored stock
func (s *StockMonitoringService) ResolveStockAlerts() error {
	// Get all active stock alerts
	var activeAlerts []models.StockAlert
	err := s.db.Where("status = ?", models.StockAlertStatusActive).
		Preload("Product").Find(&activeAlerts).Error
	if err != nil {
		return err
	}

	for _, alert := range activeAlerts {
		// Check if stock is now sufficient (>= minimum stock) or min_stock was removed (=0)
		isResolved := false

		if alert.Product.IsService || alert.Product.IsAssembly {
			isResolved = true
		} else if alert.AlertType == models.StockAlertTypeLowStock {
			// Resolve Low Stock if current stock >= min stock OR min stock is 0 (disabled)
			if alert.Product.Stock >= alert.Product.MinStock || alert.Product.MinStock == 0 {
				isResolved = true
			}
		} else if alert.AlertType == models.StockAlertTypeReorderAlert {
			// Resolve Reorder Alert if current stock > reorder level OR reorder level is 0 (disabled)
			// (Assuming > reorder level is sufficient, usually reorder level is the trigger point)
			if alert.Product.Stock > alert.Product.ReorderLevel || alert.Product.ReorderLevel == 0 {
				isResolved = true
			}
		} else if alert.AlertType == models.StockAlertTypeOutOfStock {
			// Resolve Out of Stock Alert if current stock > 0
			if alert.Product.Stock > 0 {
				isResolved = true
			}
		}

		if isResolved {
			// Resolve the alert
			alert.Status = models.StockAlertStatusResolved
			s.db.Save(&alert)

			// Mark related notifications as read
			s.markStockNotificationsAsRead(alert.ProductID)
		}
	}

	return nil
}

// markStockNotificationsAsRead marks all stock-related notifications for a product as read
func (s *StockMonitoringService) markStockNotificationsAsRead(productID uint) error {
	return s.db.Model(&models.Notification{}).
		Where("type IN ? AND data->>'product_id' = ? AND is_read = ?",
			[]string{models.NotificationTypeLowStock, models.NotificationTypeReorderAlert, models.NotificationTypeStockOut},
			fmt.Sprintf("%d", productID),
			false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

func (s *StockMonitoringService) upsertActiveStockAlert(product *models.Product, alertType string, threshold int) (*models.StockAlert, bool, error) {
	var existingAlert models.StockAlert
	err := s.db.Where("product_id = ? AND alert_type = ? AND status = ?",
		product.ID, alertType, models.StockAlertStatusActive).
		First(&existingAlert).Error

	if err == nil {
		existingAlert.CurrentStock = product.Stock
		existingAlert.ThresholdStock = threshold
		existingAlert.LastAlertAt = time.Now()
		return &existingAlert, false, s.db.Save(&existingAlert).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	stockAlert := models.StockAlert{
		ProductID:      product.ID,
		AlertType:      alertType,
		CurrentStock:   product.Stock,
		ThresholdStock: threshold,
		Status:         models.StockAlertStatusActive,
		LastAlertAt:    time.Now(),
	}
	if err := s.db.Create(&stockAlert).Error; err != nil {
		if isPGUniqueViolation(err) {
			err = s.db.Where("product_id = ? AND alert_type = ? AND status = ?",
				product.ID, alertType, models.StockAlertStatusActive).
				First(&existingAlert).Error
			if err != nil {
				return nil, false, err
			}

			existingAlert.CurrentStock = product.Stock
			existingAlert.ThresholdStock = threshold
			existingAlert.LastAlertAt = time.Now()
			return &existingAlert, false, s.db.Save(&existingAlert).Error
		}

		return nil, false, err
	}

	return &stockAlert, true, nil
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *StockMonitoringService) saveOrUpdateStockNotification(userID uint, notificationType string, productID uint, title, message, data, priority string) error {
	var existingNotif models.Notification
	err := s.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Where("user_id = ? AND type = ? AND data->>'product_id' = ? AND is_read = ?",
			userID, notificationType, fmt.Sprintf("%d", productID), false).
		First(&existingNotif).Error

	if err == nil {
		existingNotif.Title = title
		existingNotif.Message = message
		existingNotif.Data = data
		existingNotif.Priority = priority
		existingNotif.UpdatedAt = time.Now()
		return s.db.Save(&existingNotif).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	notification := models.Notification{
		UserID:   userID,
		Type:     notificationType,
		Title:    title,
		Message:  message,
		Data:     data,
		Priority: priority,
		IsRead:   false,
	}

	if err := s.db.Create(&notification).Error; err != nil {
		if !isPGUniqueViolation(err) {
			return err
		}

		err = s.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
			Where("user_id = ? AND type = ? AND data->>'product_id' = ? AND is_read = ?",
				userID, notificationType, fmt.Sprintf("%d", productID), false).
			First(&existingNotif).Error
		if err != nil {
			return err
		}

		existingNotif.Title = title
		existingNotif.Message = message
		existingNotif.Data = data
		existingNotif.Priority = priority
		existingNotif.UpdatedAt = time.Now()
		return s.db.Save(&existingNotif).Error
	}

	return nil
}

// GetActiveStockAlerts returns only active stock alerts for dashboard
func (s *StockMonitoringService) GetActiveStockAlerts() ([]models.StockAlert, error) {
	var alerts []models.StockAlert
	err := s.db.Where("status = ?", models.StockAlertStatusActive).
		Preload("Product").
		Preload("Product.Category").
		Order("last_alert_at DESC").
		Find(&alerts).Error
	return alerts, err
}

// ScheduledStockCheck runs periodic stock monitoring (call this from a cron job)
func (s *StockMonitoringService) ScheduledStockCheck() error {
	log.Println("[STOCK-MONITOR] Starting scheduled stock monitoring check...")

	// Check minimum stock
	if err := s.CheckMinimumStock(); err != nil {
		log.Printf("[STOCK-MONITOR-ERROR] Error checking minimum stock: %v", err)
		return err
	}

	// Check out of stock
	if err := s.CheckOutOfStock(); err != nil {
		log.Printf("[STOCK-MONITOR-ERROR] Error checking out of stock: %v", err)
		return err
	}

	// Check reorder levels
	if err := s.CheckReorderLevel(); err != nil {
		log.Printf("[STOCK-MONITOR-ERROR] Error checking reorder levels: %v", err)
		return err
	}

	// Resolve alerts for products with restored stock
	if err := s.ResolveStockAlerts(); err != nil {
		log.Printf("[STOCK-MONITOR-ERROR] Error resolving stock alerts: %v", err)
	}

	log.Println("[STOCK-MONITOR] Scheduled stock monitoring check completed")
	return nil
}

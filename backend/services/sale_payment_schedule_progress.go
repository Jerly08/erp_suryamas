package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"
)

const (
	salePaymentScheduleStatusPending = "PENDING"
	salePaymentScheduleStatusPartial = "PARTIAL"
	salePaymentScheduleStatusPaid    = "PAID"
)

func roundScheduleMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func hydrateSalePaymentScheduleProgress(sale *models.Sale) {
	if sale == nil || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" || len(sale.PaymentSchedules) == 0 {
		return
	}

	remainingPaid := math.Max(sale.PaidAmount, 0)

	for index := range sale.PaymentSchedules {
		schedule := &sale.PaymentSchedules[index]
		plannedAmount := math.Max(schedule.Amount, 0)
		paidAmount := math.Min(remainingPaid, plannedAmount)
		if paidAmount < 0 {
			paidAmount = 0
		}

		outstandingAmount := plannedAmount - paidAmount
		if outstandingAmount < 0.005 {
			outstandingAmount = 0
		}

		status := salePaymentScheduleStatusPending
		switch {
		case paidAmount <= 0.005:
			status = salePaymentScheduleStatusPending
		case outstandingAmount <= 0.005:
			status = salePaymentScheduleStatusPaid
		default:
			status = salePaymentScheduleStatusPartial
		}

		schedule.PaidAmount = roundScheduleMoney(paidAmount)
		schedule.Outstanding = roundScheduleMoney(outstandingAmount)
		schedule.Status = status

		remainingPaid -= paidAmount
		if remainingPaid < 0 {
			remainingPaid = 0
		}
	}
}

func syncSalePaymentScheduleProgress(tx *gorm.DB, sale *models.Sale) error {
	if sale == nil {
		return fmt.Errorf("sale is required")
	}

	if sale.ID == 0 || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" {
		hydrateSalePaymentScheduleProgress(sale)
		return nil
	}

	var schedules []models.SalePaymentSchedule
	if err := tx.Where("sale_id = ?", sale.ID).Order("sequence ASC").Find(&schedules).Error; err != nil {
		return fmt.Errorf("failed to load sale payment schedules: %v", err)
	}

	sale.PaymentSchedules = schedules
	hydrateSalePaymentScheduleProgress(sale)

	for _, schedule := range sale.PaymentSchedules {
		if err := tx.Model(&models.SalePaymentSchedule{}).
			Where("id = ?", schedule.ID).
			Update("status", schedule.Status).Error; err != nil {
			return fmt.Errorf("failed to update sale payment schedule status: %v", err)
		}
	}

	return nil
}

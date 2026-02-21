package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type OverdueManagementService struct {
	db                  *gorm.DB
	salesRepo           *repositories.SalesRepository
	accountRepo         repositories.AccountRepository
	notificationService NotificationServiceInterface
	emailService        EmailServiceInterface
}

type NotificationServiceInterface interface {
	SendOverdueNotification(sale *models.Sale, daysOverdue int) error
	SendReminderNotification(sale *models.Sale, daysBefore int) error
}

type EmailServiceInterface interface {
	SendOverdueEmail(customerEmail string, sale *models.Sale, daysOverdue int) error
	SendReminderEmail(customerEmail string, sale *models.Sale, daysBefore int) error
}

// OverdueConfig holds configuration for overdue management
type OverdueConfig struct {
	ReminderDays      []int   // Days before due date to send reminders [7, 3, 1]
	OverdueGraceDays  int     // Grace period before marking as overdue (default: 1)
	AutoWriteOffDays  int     // Days after which to suggest write-off (default: 90)
	InterestRate      float64 // Annual interest rate for overdue (default: 12%)
	MaxCollectionDays int     // Maximum days for collection efforts (default: 180)
	AutoLegalDays     int     // Days to trigger legal action (default: 120)
}

func NewOverdueManagementService(
	db *gorm.DB,
	salesRepo *repositories.SalesRepository,
	accountRepo repositories.AccountRepository,
	notificationService NotificationServiceInterface,
	emailService EmailServiceInterface,
) *OverdueManagementService {
	return &OverdueManagementService{
		db:                  db,
		salesRepo:           salesRepo,
		accountRepo:         accountRepo,
		notificationService: notificationService,
		emailService:        emailService,
	}
}

// GetDefaultConfig returns default overdue configuration
func (s *OverdueManagementService) GetDefaultConfig() OverdueConfig {
	return OverdueConfig{
		ReminderDays:      []int{7, 3, 1},
		OverdueGraceDays:  1,
		AutoWriteOffDays:  90,
		InterestRate:      12.0, // 12% per annum
		MaxCollectionDays: 180,
		AutoLegalDays:     120,
	}
}

// ProcessOverdueInvoices - Main automated process (run daily via cron)
func (s *OverdueManagementService) ProcessOverdueInvoices() error {
	log.Println("🕐 Starting automated overdue invoice processing...")

	config := s.GetDefaultConfig()

	// 1. Send payment reminders BEFORE due date
	if err := s.SendPaymentReminders(config); err != nil {
		log.Printf("Error sending reminders: %v", err)
	}

	// 2. Mark invoices as overdue AFTER due date + grace period
	if err := s.MarkInvoicesAsOverdue(config); err != nil {
		log.Printf("Error marking overdue: %v", err)
	}

	// 3. Calculate and apply interest charges
	if err := s.ApplyInterestCharges(config); err != nil {
		log.Printf("Error applying interest: %v", err)
	}

	// 4. Escalate collection efforts
	if err := s.EscalateCollectionEfforts(config); err != nil {
		log.Printf("Error escalating collection: %v", err)
	}

	// 5. Suggest write-offs for very old debts
	if err := s.SuggestWriteOffs(config); err != nil {
		log.Printf("Error suggesting write-offs: %v", err)
	}

	log.Println("✅ Completed overdue invoice processing")
	return nil
}

// SendPaymentReminders sends reminders before due date
func (s *OverdueManagementService) SendPaymentReminders(config OverdueConfig) error {
	log.Println("📧 Processing payment reminders...")

	for _, daysBefore := range config.ReminderDays {
		reminderDate := time.Now().AddDate(0, 0, daysBefore)

		// Find invoices due on reminderDate that haven't been reminded yet
		invoices, err := s.salesRepo.FindInvoicesDueOn(reminderDate)
		if err != nil {
			return fmt.Errorf("failed to find invoices due on %v: %v", reminderDate, err)
		}

		for _, invoice := range invoices {
			if invoice.OutstandingAmount > 0 {
				// Check if reminder already sent
				reminderSent, err := s.checkReminderSent(invoice.ID, daysBefore)
				if err != nil || reminderSent {
					continue
				}

				// Send reminder notification
				if s.notificationService != nil {
					s.notificationService.SendReminderNotification(&invoice, daysBefore)
				}

				// Send reminder email
				if s.emailService != nil && invoice.Customer.Email != "" {
					s.emailService.SendReminderEmail(invoice.Customer.Email, &invoice, daysBefore)
				}

				// Log reminder sent
				s.logReminderSent(invoice.ID, daysBefore)
				log.Printf("📧 Reminder sent for invoice %s (%d days before due)", invoice.InvoiceNumber, daysBefore)
			}
		}
	}

	return nil
}

// MarkInvoicesAsOverdue marks invoices as overdue after grace period
func (s *OverdueManagementService) MarkInvoicesAsOverdue(config OverdueConfig) error {
	log.Println("⚠️ Marking overdue invoices...")

	overdueDate := time.Now().AddDate(0, 0, -config.OverdueGraceDays)

	// Find invoices that are past due date + grace period
	invoices, err := s.salesRepo.FindInvoicesOverdueAsOf(overdueDate)
	if err != nil {
		return fmt.Errorf("failed to find overdue invoices: %v", err)
	}

	for _, invoice := range invoices {
		if invoice.Status == models.SaleStatusInvoiced && invoice.OutstandingAmount > 0 {
			// Update status to overdue
			invoice.Status = models.SaleStatusOverdue

			// Calculate days overdue
			daysOverdue := int(time.Since(invoice.DueDate).Hours() / 24)

			// Update invoice
			if _, err := s.salesRepo.Update(&invoice); err != nil {
				log.Printf("Failed to update invoice %s to overdue: %v", invoice.InvoiceNumber, err)
				continue
			}

			// Send overdue notification
			if s.notificationService != nil {
				s.notificationService.SendOverdueNotification(&invoice, daysOverdue)
			}

			// Send overdue email
			if s.emailService != nil && invoice.Customer.Email != "" {
				s.emailService.SendOverdueEmail(invoice.Customer.Email, &invoice, daysOverdue)
			}

			// Create overdue tracking record
			s.createOverdueRecord(&invoice, daysOverdue)

			log.Printf("⚠️ Invoice %s marked as overdue (%d days)", invoice.InvoiceNumber, daysOverdue)
		}
	}

	return nil
}

// ApplyInterestCharges applies interest to overdue invoices
func (s *OverdueManagementService) ApplyInterestCharges(config OverdueConfig) error {
	log.Println("💰 Applying interest charges...")

	// Find overdue invoices that haven't had interest applied today
	overdueInvoices, err := s.salesRepo.FindOverdueInvoicesForInterest()
	if err != nil {
		return fmt.Errorf("failed to find overdue invoices for interest: %v", err)
	}

	for _, invoice := range overdueInvoices {
		daysOverdue := int(time.Since(invoice.DueDate).Hours() / 24)

		// Calculate daily interest (annual rate / 365)
		dailyRate := config.InterestRate / 100 / 365
		interestAmount := invoice.OutstandingAmount * dailyRate

		if interestAmount > 0.01 { // Only apply if interest > 1 cent
			// Create interest charge record
			interestCharge := &models.InterestCharge{
				SaleID:       invoice.ID,
				ChargeDate:   time.Now(),
				DaysOverdue:  daysOverdue,
				Principal:    invoice.OutstandingAmount,
				InterestRate: config.InterestRate,
				Amount:       interestAmount,
				Status:       "APPLIED",
			}

			// Save interest charge record to database
			if err := s.db.Create(interestCharge).Error; err != nil {
				log.Printf("Failed to create interest charge record for invoice %s: %v", invoice.InvoiceNumber, err)
			}

			// Add interest to outstanding amount
			invoice.OutstandingAmount += interestAmount
			invoice.TotalAmount += interestAmount

			// Update invoice
			if _, err := s.salesRepo.Update(&invoice); err != nil {
				log.Printf("Failed to update invoice %s with interest: %v", invoice.InvoiceNumber, err)
				continue
			}

			// Create journal entry for interest income
			s.createInterestJournalEntry(&invoice, interestAmount)

			log.Printf("💰 Interest applied to invoice %s: %.2f", invoice.InvoiceNumber, interestAmount)
		}
	}

	return nil
}

// EscalateCollectionEfforts escalates collection based on overdue period
func (s *OverdueManagementService) EscalateCollectionEfforts(config OverdueConfig) error {
	log.Println("📞 Escalating collection efforts...")

	// Find invoices at different overdue stages
	overdueInvoices, err := s.salesRepo.FindOverdueInvoicesWithDays()
	if err != nil {
		return fmt.Errorf("failed to find overdue invoices: %v", err)
	}

	for _, invoice := range overdueInvoices {
		daysOverdue := int(time.Since(invoice.DueDate).Hours() / 24)

		// Determine collection level based on days overdue
		var collectionLevel string
		var action string

		switch {
		case daysOverdue >= config.AutoLegalDays:
			collectionLevel = "LEGAL"
			action = "Initiate legal proceedings"
		case daysOverdue >= 90:
			collectionLevel = "AGGRESSIVE"
			action = "Final demand before legal action"
		case daysOverdue >= 60:
			collectionLevel = "INTENSIVE"
			action = "Collection agency referral"
		case daysOverdue >= 30:
			collectionLevel = "ACTIVE"
			action = "Direct phone collection calls"
		case daysOverdue >= 15:
			collectionLevel = "MODERATE"
			action = "Formal collection letters"
		default:
			collectionLevel = "GENTLE"
			action = "Friendly payment reminders"
		}

		// Create collection task
		collectionTask := &models.CollectionTask{
			SaleID:      invoice.ID,
			Level:       collectionLevel,
			Action:      action,
			DaysOverdue: daysOverdue,
			Priority:    s.calculatePriority(invoice.OutstandingAmount, daysOverdue),
			CreatedAt:   time.Now(),
			Status:      "PENDING",
			AssignedTo:  s.getCollectionAgent(collectionLevel),
		}

		// Save collection task
		if err := s.db.Create(collectionTask).Error; err != nil {
			log.Printf("Failed to create collection task for invoice %s: %v", invoice.InvoiceNumber, err)
			continue
		}

		log.Printf("📞 Collection task created for invoice %s: %s (%d days overdue)",
			invoice.InvoiceNumber, collectionLevel, daysOverdue)
	}

	return nil
}

// SuggestWriteOffs suggests write-offs for very old debts
func (s *OverdueManagementService) SuggestWriteOffs(config OverdueConfig) error {
	log.Println("📋 Suggesting write-offs...")

	writeOffDate := time.Now().AddDate(0, 0, -config.AutoWriteOffDays)

	// Find very old overdue invoices
	candidateInvoices, err := s.salesRepo.FindInvoicesOverdueAsOf(writeOffDate)
	if err != nil {
		return fmt.Errorf("failed to find write-off candidates: %v", err)
	}

	for _, invoice := range candidateInvoices {
		if invoice.OutstandingAmount > 0 {
			daysOverdue := int(time.Since(invoice.DueDate).Hours() / 24)

			// Create write-off suggestion
			suggestion := &models.WriteOffSuggestion{
				SaleID:            invoice.ID,
				OutstandingAmount: invoice.OutstandingAmount,
				DaysOverdue:       daysOverdue,
				Reason:            fmt.Sprintf("Overdue for %d days - exceeds collection threshold", daysOverdue),
				Status:            "PENDING_APPROVAL",
				CreatedAt:         time.Now(),
				Priority:          s.calculateWriteOffPriority(invoice.OutstandingAmount, daysOverdue),
			}

			// Save suggestion
			if err := s.db.Create(suggestion).Error; err != nil {
				log.Printf("Failed to create write-off suggestion for invoice %s: %v", invoice.InvoiceNumber, err)
				continue
			}

			log.Printf("📋 Write-off suggested for invoice %s: %.2f (%d days overdue)",
				invoice.InvoiceNumber, invoice.OutstandingAmount, daysOverdue)
		}
	}

	return nil
}

// Helper functions

func (s *OverdueManagementService) checkReminderSent(saleID uint, daysBefore int) (bool, error) {
	var count int64
	err := s.db.Model(&models.ReminderLog{}).
		Where("sale_id = ? AND days_before = ? AND DATE(created_at) = DATE(?)", saleID, daysBefore, time.Now()).
		Count(&count).Error
	return count > 0, err
}

func (s *OverdueManagementService) logReminderSent(saleID uint, daysBefore int) error {
	reminderLog := &models.ReminderLog{
		SaleID:     saleID,
		DaysBefore: daysBefore,
		CreatedAt:  time.Now(),
	}
	return s.db.Create(reminderLog).Error
}

func (s *OverdueManagementService) createOverdueRecord(invoice *models.Sale, daysOverdue int) error {
	overdueRecord := &models.OverdueRecord{
		SaleID:            invoice.ID,
		OverdueDate:       time.Now(),
		DaysOverdue:       daysOverdue,
		OutstandingAmount: invoice.OutstandingAmount,
		Status:            "ACTIVE",
	}
	return s.db.Create(overdueRecord).Error
}

func (s *OverdueManagementService) createInterestJournalEntry(invoice *models.Sale, interestAmount float64) error {
	// Get interest income account
	interestAccount, err := s.accountRepo.GetAccountByCode("4900") // Other Income
	if err != nil {
		return fmt.Errorf("interest income account not found: %v", err)
	}

	// Get accounts receivable account (Try 1102-001 first, then 1102, then 1201)
	arAccount, err := s.accountRepo.GetAccountByCode("1102-001")
	if err != nil {
		arAccount, err = s.accountRepo.GetAccountByCode("1102")
		if err != nil {
			arAccount, err = s.accountRepo.GetAccountByCode("1201")
			if err != nil {
				return fmt.Errorf("accounts receivable account not found: %v", err)
			}
		}
	}

	// Create journal entry for interest
	journalEntry := &models.JournalEntry{
		Code:            s.generateJournalCode("INT"),
		EntryDate:       time.Now(),
		Description:     fmt.Sprintf("Interest charge on overdue invoice %s", invoice.InvoiceNumber),
		Reference:       invoice.InvoiceNumber,
		ReferenceType:   models.JournalRefInterest,
		ReferenceID:     &invoice.ID,
		UserID:          1, // System user
		Status:          models.JournalStatusApproved,
		IsAutoGenerated: true,
		TotalDebit:      interestAmount,
		TotalCredit:     interestAmount,
	}

	// Journal lines
	journalEntry.JournalLines = []models.JournalLine{
		{
			AccountID:    arAccount.ID,
			Description:  fmt.Sprintf("Interest on overdue invoice %s", invoice.InvoiceNumber),
			DebitAmount:  interestAmount,
			CreditAmount: 0,
			LineNumber:   1,
		},
		{
			AccountID:    interestAccount.ID,
			Description:  fmt.Sprintf("Interest income from invoice %s", invoice.InvoiceNumber),
			DebitAmount:  0,
			CreditAmount: interestAmount,
			LineNumber:   2,
		},
	}

	// Save journal entry
	if err := s.db.Create(journalEntry).Error; err != nil {
		return fmt.Errorf("failed to create interest journal entry: %v", err)
	}

	// Update account balances
	for _, line := range journalEntry.JournalLines {
		if err := s.accountRepo.UpdateBalance(context.Background(), line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
			log.Printf("Failed to update balance for account %d: %v", line.AccountID, err)
		}
	}

	return nil
}

func (s *OverdueManagementService) calculatePriority(amount float64, daysOverdue int) string {
	score := (amount / 1000000) + float64(daysOverdue/30) // Weight by amount and days

	switch {
	case score >= 3:
		return "CRITICAL"
	case score >= 2:
		return "HIGH"
	case score >= 1:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func (s *OverdueManagementService) calculateWriteOffPriority(amount float64, daysOverdue int) string {
	if amount > 10000000 || daysOverdue > 180 { // > 10M or > 180 days
		return "HIGH"
	} else if amount > 1000000 || daysOverdue > 120 { // > 1M or > 120 days
		return "MEDIUM"
	}
	return "LOW"
}

func (s *OverdueManagementService) getCollectionAgent(level string) *uint {
	// Logic to assign collection agent based on level
	// This could be from configuration or database
	switch level {
	case "LEGAL":
		return nil // Legal team
	case "AGGRESSIVE", "INTENSIVE":
		return nil // Senior collection agent
	default:
		return nil // Regular collection agent
	}
}

func (s *OverdueManagementService) generateJournalCode(prefix string) string {
	year := time.Now().Year()
	month := time.Now().Month()
	// This should use a proper counter from database
	count := 1 // Simplified
	return fmt.Sprintf("%s/%04d/%02d/%04d", prefix, year, month, count)
}

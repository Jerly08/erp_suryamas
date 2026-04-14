package services

import (
	"errors"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var ErrPrepaidHasLinkedJournal = errors.New("prepaid expense cannot be deleted because amortization or termination journals already exist")

type PrepaidService struct {
	db         *gorm.DB
	journalSvc *UnifiedJournalService
}

func normalizePrepaidDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func NewPrepaidService(db *gorm.DB, journalSvc *UnifiedJournalService) *PrepaidService {
	return &PrepaidService{
		db:         db,
		journalSvc: journalSvc,
	}
}

// GetAllPrepaid retrieves all prepaid expenses with optional status filter
func (s *PrepaidService) GetAllPrepaid(status string) ([]models.PrepaidExpense, error) {
	var prepaids []models.PrepaidExpense
	query := s.db.Preload("Purchase").Preload("Schedules")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at desc").Find(&prepaids).Error
	return prepaids, err
}

// GetPrepaidByID retrieves a specific prepaid expense with its schedules
func (s *PrepaidService) GetPrepaidByID(id uint) (*models.PrepaidExpense, error) {
	var prepaid models.PrepaidExpense
	err := s.db.Preload("Purchase").Preload("Schedules", func(db *gorm.DB) *gorm.DB {
		return db.Order("month_number asc")
	}).First(&prepaid, id).Error

	if err != nil {
		return nil, err
	}
	return &prepaid, nil
}

// DeletePrepaid removes a prepaid expense and its schedules if it has never produced journals.
func (s *PrepaidService) DeletePrepaid(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var prepaid models.PrepaidExpense
		if err := tx.First(&prepaid, id).Error; err != nil {
			return err
		}

		var linkedJournalCount int64
		if err := tx.Model(&models.PrepaidSchedule{}).
			Where("prepaid_id = ? AND journal_entry_id IS NOT NULL", id).
			Count(&linkedJournalCount).Error; err != nil {
			return err
		}

		if linkedJournalCount > 0 {
			return ErrPrepaidHasLinkedJournal
		}

		if err := tx.Where("prepaid_id = ?", id).Delete(&models.PrepaidSchedule{}).Error; err != nil {
			return err
		}

		return tx.Delete(&models.PrepaidExpense{}, id).Error
	})
}

// SetExpenseAccount updates the target expense account for amortization
func (s *PrepaidService) SetExpenseAccount(id uint, expenseAccountID uint) error {
	return s.db.Model(&models.PrepaidExpense{}).Where("id = ?", id).
		Update("expense_account_id", expenseAccountID).Error
}

// UpdateSchedule updates the scheduled date of a pending amortization schedule.
func (s *PrepaidService) UpdateSchedule(prepaidID uint, scheduleID uint, scheduledDate time.Time) (*models.PrepaidSchedule, error) {
	var updatedSchedule models.PrepaidSchedule
	scheduledDate = normalizePrepaidDate(scheduledDate)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var prepaid models.PrepaidExpense
		if err := tx.First(&prepaid, prepaidID).Error; err != nil {
			return err
		}

		if prepaid.Status != "ACTIVE" {
			return errors.New("only ACTIVE prepaid expenses can be edited")
		}

		var schedule models.PrepaidSchedule
		if err := tx.Where("prepaid_id = ?", prepaidID).First(&schedule, scheduleID).Error; err != nil {
			return err
		}

		if schedule.Status != "PENDING" || schedule.JournalEntryID != nil {
			return errors.New("only PENDING schedules can be edited")
		}

		if scheduledDate.Before(normalizePrepaidDate(prepaid.StartDate)) {
			return errors.New("scheduled date cannot be earlier than prepaid start date")
		}

		var previousSchedule models.PrepaidSchedule
		prevErr := tx.Where("prepaid_id = ? AND month_number < ?", prepaidID, schedule.MonthNumber).
			Order("month_number desc").
			First(&previousSchedule).Error
		if prevErr == nil && scheduledDate.Before(normalizePrepaidDate(previousSchedule.ScheduledDate)) {
			return errors.New("scheduled date cannot be earlier than the previous schedule")
		}
		if prevErr != nil && !errors.Is(prevErr, gorm.ErrRecordNotFound) {
			return prevErr
		}

		var followingSchedules []models.PrepaidSchedule
		if err := tx.Where("prepaid_id = ? AND month_number > ?", prepaidID, schedule.MonthNumber).
			Order("month_number asc").
			Find(&followingSchedules).Error; err != nil {
			return err
		}

		for _, followingSchedule := range followingSchedules {
			if followingSchedule.Status != "PENDING" || followingSchedule.JournalEntryID != nil {
				return errors.New("cannot shift following schedules because one or more schedules are already locked")
			}
		}

		schedule.ScheduledDate = scheduledDate
		if err := tx.Save(&schedule).Error; err != nil {
			return err
		}

		latestScheduledDate := schedule.ScheduledDate
		for i := range followingSchedules {
			monthOffset := followingSchedules[i].MonthNumber - schedule.MonthNumber
			followingSchedules[i].ScheduledDate = normalizePrepaidDate(schedule.ScheduledDate.AddDate(0, monthOffset, 0))
			if err := tx.Save(&followingSchedules[i]).Error; err != nil {
				return err
			}
			latestScheduledDate = followingSchedules[i].ScheduledDate
		}

		if err := tx.Model(&models.PrepaidExpense{}).
			Where("id = ?", prepaidID).
			Update("end_date", normalizePrepaidDate(latestScheduledDate)).Error; err != nil {
			return err
		}

		updatedSchedule = schedule
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &updatedSchedule, nil
}

// AmortizeSchedule executes one month of amortization (Debit Expense, Credit Prepaid Asset)
func (s *PrepaidService) AmortizeSchedule(prepaidID uint, scheduleID uint, userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Dapatkan Schedule & Prepaid Header
		var schedule models.PrepaidSchedule
		if err := tx.First(&schedule, scheduleID).Error; err != nil {
			return fmt.Errorf("schedule not found: %v", err)
		}

		if schedule.PrepaidID != prepaidID {
			return errors.New("schedule does not belong to this prepaid expense")
		}

		if schedule.Status == "POSTED" {
			return errors.New("this schedule is already amortized (POSTED)")
		}
		if err := CheckClosedPeriod(tx, schedule.ScheduledDate); err != nil {
			return err
		}

		var prepaid models.PrepaidExpense
		if err := tx.Preload("Purchase.Vendor").First(&prepaid, prepaidID).Error; err != nil {
			return fmt.Errorf("prepaid expense not found: %v", err)
		}

		if prepaid.Status == "TERMINATED" {
			return errors.New("cannot amortize a terminated prepaid expense")
		}

		// 2. Validasi Akun
		if prepaid.ExpenseAccountID == 0 {
			return errors.New("expense_account_id (akun beban penyusutan/amortisasi) belum disetting")
		}
		if prepaid.PrepaidAccountID == 0 {
			return errors.New("prepaid_account_id (akun aset lancar prepaid) tidak valid")
		}

		// 3. Buat Jurnal Amortisasi di SSOT Journal
		if s.journalSvc == nil {
			return errors.New("Unified Journal Service is not initialized")
		}

		desc := fmt.Sprintf("Amortisasi Bulan ke-%d: %s", schedule.MonthNumber, prepaid.Name)
		if prepaid.Purchase != nil && prepaid.Purchase.Vendor.Name != "" {
			desc += fmt.Sprintf(" (Vendor: %s)", prepaid.Purchase.Vendor.Name)
		}

		journalReq := &JournalEntryRequest{
			EntryDate:   schedule.ScheduledDate,
			Description: desc,
			SourceType:  "PREPAID_AMORTIZATION",
			SourceID:    uint64(schedule.ID),
			CreatedBy:   uint64(userID),
			AutoPost:    true,
			Lines: []JournalLineRequest{
				{
					AccountID:    uint64(prepaid.ExpenseAccountID),
					Description:  "Beban Amortisasi Prepaid",
					DebitAmount:  decimal.NewFromFloat(schedule.Amount),
					CreditAmount: decimal.NewFromFloat(0),
				},
				{
					AccountID:    uint64(prepaid.PrepaidAccountID),
					Description:  "Pengurangan Aset Prepaid",
					DebitAmount:  decimal.NewFromFloat(0),
					CreditAmount: decimal.NewFromFloat(schedule.Amount),
				},
			},
		}

		journalEntry, err := s.journalSvc.CreateJournalEntryWithTx(tx, journalReq)
		if err != nil {
			return fmt.Errorf("failed to create amortization journal: %v", err)
		}

		// 4. Update Schedule Status
		schedule.Status = "POSTED"
		journalID := uint(journalEntry.ID)
		schedule.JournalEntryID = &journalID
		if err := tx.Save(&schedule).Error; err != nil {
			return err
		}

		// 5. Cek apakah ini bulan terakhir? Jika ya, set Header menjadi COMPLETED
		var pendingCount int64
		tx.Model(&models.PrepaidSchedule{}).Where("prepaid_id = ? AND status = 'PENDING'", prepaidID).Count(&pendingCount)

		if pendingCount == 0 {
			tx.Model(&models.PrepaidExpense{}).Where("id = ?", prepaidID).Update("status", "COMPLETED")
		}

		return nil
	})
}

// TerminatePrepaid handles early termination (cancels all pending schedules and writes-off/refunds remaining balance)
func (s *PrepaidService) TerminatePrepaid(prepaidID uint, terminateType string, destinationAccountID uint, effectiveDate time.Time, userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var prepaid models.PrepaidExpense
		if err := tx.Preload("Purchase.Vendor").First(&prepaid, prepaidID).Error; err != nil {
			return fmt.Errorf("prepaid expense not found: %v", err)
		}

		if prepaid.Status != "ACTIVE" {
			return fmt.Errorf("hanya prepaid berstatus ACTIVE yang bisa di-terminate (current status: %s)", prepaid.Status)
		}
		if effectiveDate.IsZero() {
			effectiveDate = time.Now()
		}
		if err := CheckClosedPeriod(tx, effectiveDate); err != nil {
			return err
		}

		// 1. Hitung Sisa Saldo yang PENDING
		var remainingBalance float64
		// Tandai semua PENDING jadi CANCELLED
		var pendingSchedules []models.PrepaidSchedule
		tx.Where("prepaid_id = ? AND status = 'PENDING'", prepaidID).Find(&pendingSchedules)

		if len(pendingSchedules) == 0 {
			return errors.New("tidak ada sisa saldo/jadwal pending yang bisa di-terminate")
		}

		for _, sched := range pendingSchedules {
			remainingBalance += sched.Amount
		}

		// 2. Buat Jurnal SSOT Terminasi
		if s.journalSvc == nil {
			return errors.New("Unified Journal Service is not initialized")
		}

		desc := fmt.Sprintf("Early Termination: %s (Sisa %d bln)", prepaid.Name, len(pendingSchedules))

		journalReq := &JournalEntryRequest{
			EntryDate:   effectiveDate,
			Description: desc,
			SourceType:  "PREPAID_TERMINATION",
			SourceID:    uint64(prepaid.ID),
			CreatedBy:   uint64(userID),
			AutoPost:    true,
			Lines: []JournalLineRequest{
				{
					// Kredit selalu Aset Prepaid-nya untuk menghabiskan sisa saldonya
					AccountID:    uint64(prepaid.PrepaidAccountID),
					Description:  "Penghapusan Sisa Aset Prepaid",
					DebitAmount:  decimal.NewFromFloat(0),
					CreditAmount: decimal.NewFromFloat(remainingBalance),
				},
			},
		}

		// Debit bergantung tipe terminasi:
		// "WRITEOFF" (Hangus) -> Debit Akun Kerugian/Beban Terminasi
		// "REFUND" (Di-refund Vendor) -> Debit Kas / Bank
		debitDesc := "Beban/Kerugian Terminasi Kontrak"
		if terminateType == "REFUND" {
			debitDesc = "Refund Kas/Bank Terminasi Kontrak"
		}

		journalReq.Lines = append(journalReq.Lines, JournalLineRequest{
			AccountID:    uint64(destinationAccountID),
			Description:  debitDesc,
			DebitAmount:  decimal.NewFromFloat(remainingBalance),
			CreditAmount: decimal.NewFromFloat(0),
		})

		journalEntry, err := s.journalSvc.CreateJournalEntryWithTx(tx, journalReq)
		if err != nil {
			return fmt.Errorf("failed to create termination journal: %v", err)
		}

		// 3. Batalkan jadwal yang tersisa
		if err := tx.Model(&models.PrepaidSchedule{}).
			Where("prepaid_id = ? AND status = 'PENDING'", prepaidID).
			Updates(map[string]interface{}{
				"status":           "CANCELLED",
				"journal_entry_id": uint(journalEntry.ID),
			}).Error; err != nil {
			return err
		}

		// 4. Update Header Prepaid jadi TERMINATED
		return tx.Model(&models.PrepaidExpense{}).Where("id = ?", prepaidID).
			Update("status", "TERMINATED").Error
	})
}

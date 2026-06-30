package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	PayrollStatusPendingApproval = "PENDING_APPROVAL"
	PayrollStatusApproved        = "APPROVED"
	PayrollStatusRejected        = "REJECTED"
)

const (
	PayrollPPhPaymentStatusUnpaid      = "UNPAID"
	PayrollPPhPaymentStatusPartialPaid = "PARTIAL_PAID"
	PayrollPPhPaymentStatusPaid        = "PAID"
)

const (
	PayrollSalaryPaymentStatusUnpaid      = "UNPAID"
	PayrollSalaryPaymentStatusPartialPaid = "PARTIAL_PAID"
	PayrollSalaryPaymentStatusPaid        = "PAID"
)

const (
	PayrollSettlementTypeSalary = "SALARY"
	PayrollSettlementTypePPh21  = "PPH21"
	PayrollSettlementTypeBPJS   = "BPJS"
)

type PayrollRun struct {
	ID                         uint           `json:"id" gorm:"primaryKey"`
	PayrollNumber              string         `json:"payroll_number" gorm:"uniqueIndex;not null;size:50"`
	PeriodStart                time.Time      `json:"period_start" gorm:"not null"`
	PeriodEnd                  time.Time      `json:"period_end" gorm:"not null"`
	PaymentDate                *time.Time     `json:"payment_date"`
	Notes                      string         `json:"notes" gorm:"type:text"`
	Status                     string         `json:"status" gorm:"not null;default:'PENDING_APPROVAL';size:30"`
	TotalSalary                float64        `json:"total_salary" gorm:"type:decimal(15,2);default:0"`
	TotalBonus                 float64        `json:"total_bonus" gorm:"type:decimal(15,2);default:0"`
	TotalPPh                   float64        `json:"total_pph" gorm:"column:total_pph;type:decimal(15,2);default:0"`
	TotalBPJS                  float64        `json:"total_bpjs" gorm:"type:decimal(15,2);default:0"`
	TotalEmployeeLoanDeduction float64        `json:"total_employee_loan_deduction" gorm:"column:total_employee_loan_deduction;type:decimal(15,2);default:0"`
	TotalOverall               float64        `json:"total_overall" gorm:"type:decimal(15,2);default:0"`
	EmployeeLoanAccountID      *uint          `json:"employee_loan_account_id" gorm:"column:employee_loan_account_id;index"`
	SalaryPaymentStatus        string         `json:"salary_payment_status" gorm:"column:salary_payment_status;not null;default:'UNPAID';size:20"`
	SalaryPaidAmount           float64        `json:"salary_paid_amount" gorm:"column:salary_paid_amount;type:decimal(15,2);default:0"`
	SalaryPaymentDate          *time.Time     `json:"salary_payment_date" gorm:"column:salary_payment_date"`
	SalaryPaymentReference     string         `json:"salary_payment_reference" gorm:"column:salary_payment_reference;size:100"`
	SalaryPaymentNotes         string         `json:"salary_payment_notes" gorm:"column:salary_payment_notes;type:text"`
	SalaryPaymentJournalID     *uint64        `json:"salary_payment_journal_entry_id" gorm:"column:salary_payment_journal_entry_id;index"`
	BPJSPaymentStatus          string         `json:"bpjs_payment_status" gorm:"column:bpjs_payment_status;not null;default:'UNPAID';size:20"`
	BPJSPaidAmount             float64        `json:"bpjs_paid_amount" gorm:"column:bpjs_paid_amount;type:decimal(15,2);default:0"`
	BPJSPaymentDate            *time.Time     `json:"bpjs_payment_date" gorm:"column:bpjs_payment_date"`
	BPJSPaymentReference       string         `json:"bpjs_payment_reference" gorm:"column:bpjs_payment_reference;size:100"`
	BPJSPaymentNotes           string         `json:"bpjs_payment_notes" gorm:"column:bpjs_payment_notes;type:text"`
	BPJSPaymentJournalID       *uint64        `json:"bpjs_payment_journal_entry_id" gorm:"column:bpjs_payment_journal_entry_id;index"`
	PPhPaymentStatus           string         `json:"pph_payment_status" gorm:"column:pph_payment_status;not null;default:'UNPAID';size:20"`
	PPhPaidAmount              float64        `json:"pph_paid_amount" gorm:"column:pph_paid_amount;type:decimal(15,2);default:0"`
	PPhRecognitionDate         *time.Time     `json:"pph_recognition_date" gorm:"column:pph_recognition_date"`
	PPhPaymentDate             *time.Time     `json:"pph_payment_date" gorm:"column:pph_payment_date"`
	PPhPaymentReference        string         `json:"pph_payment_reference" gorm:"column:pph_payment_reference;size:100"`
	PPhPaymentNotes            string         `json:"pph_payment_notes" gorm:"column:pph_payment_notes;type:text"`
	PPhPayableJournalEntryID   *uint64        `json:"pph_payable_journal_entry_id" gorm:"column:pph_payable_journal_entry_id;index"`
	PPhPaymentJournalEntryID   *uint64        `json:"pph_payment_journal_entry_id" gorm:"column:pph_payment_journal_entry_id;index"`
	CreatedByID                uint           `json:"created_by_id" gorm:"not null;index"`
	ApprovedByID               *uint          `json:"approved_by_id"`
	ApprovedAt                 *time.Time     `json:"approved_at"`
	RejectedByID               *uint          `json:"rejected_by_id"`
	RejectedAt                 *time.Time     `json:"rejected_at"`
	RejectionReason            string         `json:"rejection_reason" gorm:"type:text"`
	CreatedAt                  time.Time      `json:"created_at"`
	UpdatedAt                  time.Time      `json:"updated_at"`
	DeletedAt                  gorm.DeletedAt `json:"-" gorm:"index"`

	CreatedBy           User                `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	ApprovedBy          *User               `json:"approved_by,omitempty" gorm:"foreignKey:ApprovedByID"`
	RejectedBy          *User               `json:"rejected_by,omitempty" gorm:"foreignKey:RejectedByID"`
	Items               []PayrollItem       `json:"items,omitempty" gorm:"foreignKey:PayrollRunID"`
	Settlements         []PayrollSettlement `json:"settlements,omitempty" gorm:"foreignKey:PayrollRunID"`
	EmployeeLoanAccount *Account            `json:"employee_loan_account,omitempty" gorm:"foreignKey:EmployeeLoanAccountID"`
}

// PayrollSettlement is the immutable audit trail for every salary or PPh 21
// settlement. PayrollRun keeps denormalized totals for fast list rendering,
// while this table preserves partial-payment history and journal links.
type PayrollSettlement struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	PayrollRunID   uint           `json:"payroll_run_id" gorm:"not null;index;index:idx_payroll_settlement_unique,unique"`
	SettlementType string         `json:"settlement_type" gorm:"not null;size:20;index;index:idx_payroll_settlement_unique,unique"`
	Amount         float64        `json:"amount" gorm:"type:decimal(15,2);not null"`
	PaymentDate    time.Time      `json:"payment_date" gorm:"not null;index"`
	CashBankID     uint           `json:"cash_bank_id" gorm:"not null;index"`
	JournalEntryID uint64         `json:"journal_entry_id" gorm:"not null;index;index:idx_payroll_settlement_unique,unique"`
	Reference      string         `json:"reference" gorm:"size:100"`
	Notes          string         `json:"notes" gorm:"type:text"`
	Source         string         `json:"source" gorm:"not null;default:'PAYROLL';size:30"`
	CreatedByID    uint           `json:"created_by_id" gorm:"not null;index"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	CashBank  CashBank `json:"cash_bank,omitempty" gorm:"foreignKey:CashBankID"`
	CreatedBy User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
}

type PayrollItem struct {
	ID                    uint           `json:"id" gorm:"primaryKey"`
	PayrollRunID          uint           `json:"payroll_run_id" gorm:"not null;index"`
	EmployeeName          string         `json:"employee_name" gorm:"not null;size:150"`
	EmployeeCode          string         `json:"employee_code" gorm:"size:50"`
	WorkingDay            float64        `json:"working_day" gorm:"type:decimal(10,2);default:0"`
	WorkUnit              string         `json:"work_unit" gorm:"size:100"`
	PaymentVia            string         `json:"payment_via" gorm:"size:100"`
	AccountNo             string         `json:"account_no" gorm:"size:100"`
	Division              string         `json:"division" gorm:"size:100"`
	Department            string         `json:"department" gorm:"size:100"`
	Position              string         `json:"position" gorm:"size:100"`
	BaseSalary            float64        `json:"base_salary" gorm:"type:decimal(15,2);default:0"`
	Bonus                 float64        `json:"bonus" gorm:"type:decimal(15,2);default:0"`
	AttendanceAllowance   float64        `json:"attendance_allowance" gorm:"type:decimal(15,2);default:0"`
	CompensationPKWT      float64        `json:"compensation_pkwt" gorm:"type:decimal(15,2);default:0"`
	Overtime              float64        `json:"overtime" gorm:"type:decimal(15,2);default:0"`
	PPh                   float64        `json:"pph" gorm:"type:decimal(15,2);default:0"`
	BPJS                  float64        `json:"bpjs" gorm:"type:decimal(15,2);default:0"`
	BPJSTKJHT             float64        `json:"bpjs_tk_jht" gorm:"type:decimal(15,2);default:0"`
	BPJSTKJP              float64        `json:"bpjs_tk_jp" gorm:"type:decimal(15,2);default:0"`
	BPJSKesehatan         float64        `json:"bpjs_kesehatan" gorm:"type:decimal(15,2);default:0"`
	Proportional          float64        `json:"proportional" gorm:"type:decimal(15,2);default:0"`
	OtherDeduction        float64        `json:"other_deduction" gorm:"type:decimal(15,2);default:0"`
	EmployeeLoanDeduction float64        `json:"employee_loan_deduction" gorm:"column:employee_loan_deduction;type:decimal(15,2);default:0"`
	NetPay                float64        `json:"net_pay" gorm:"type:decimal(15,2);default:0"`
	Notes                 string         `json:"notes" gorm:"type:text"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`
}

type PayrollRunCreateRequest struct {
	PeriodStart string                     `json:"period_start" binding:"required"`
	PeriodEnd   string                     `json:"period_end" binding:"required"`
	PaymentDate string                     `json:"payment_date"`
	Notes       string                     `json:"notes"`
	Items       []PayrollItemCreateRequest `json:"items" binding:"required"`
}

type PayrollItemCreateRequest struct {
	EmployeeName          string  `json:"employee_name" binding:"required"`
	EmployeeCode          string  `json:"employee_code"`
	WorkingDay            float64 `json:"working_day"`
	WorkUnit              string  `json:"work_unit"`
	PaymentVia            string  `json:"payment_via"`
	AccountNo             string  `json:"account_no"`
	Division              string  `json:"division"`
	Department            string  `json:"department"`
	Position              string  `json:"position"`
	BaseSalary            float64 `json:"base_salary"`
	Bonus                 float64 `json:"bonus"`
	AttendanceAllowance   float64 `json:"attendance_allowance"`
	CompensationPKWT      float64 `json:"compensation_pkwt"`
	Overtime              float64 `json:"overtime"`
	PPh                   float64 `json:"pph"`
	BPJS                  float64 `json:"bpjs"`
	BPJSTKJHT             float64 `json:"bpjs_tk_jht"`
	BPJSTKJP              float64 `json:"bpjs_tk_jp"`
	BPJSKesehatan         float64 `json:"bpjs_kesehatan"`
	Proportional          float64 `json:"proportional"`
	OtherDeduction        float64 `json:"other_deduction"`
	EmployeeLoanDeduction float64 `json:"employee_loan_deduction"`
	NetPay                float64 `json:"net_pay"`
	Notes                 string  `json:"notes"`
}

type PayrollRejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type PayrollPPhPaymentRequest struct {
	PaymentDate string  `json:"payment_date" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	CashBankID  uint    `json:"cash_bank_id" binding:"required"`
	Reference   string  `json:"reference"`
	Notes       string  `json:"notes"`
}

type PayrollSalaryPaymentRequest struct {
	PaymentDate string  `json:"payment_date" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	CashBankID  uint    `json:"cash_bank_id" binding:"required"`
	Reference   string  `json:"reference"`
	Notes       string  `json:"notes"`
}

type PayrollBPJSPaymentRequest = PayrollSalaryPaymentRequest

type PayrollRunResponse struct {
	PayrollRun
	DetailAllowed bool `json:"detail_allowed"`
}

type PayrollImportPreviewResponse struct {
	Items                []PayrollItemCreateRequest `json:"items"`
	Summary              PayrollImportSummary       `json:"summary"`
	Warnings             []string                   `json:"warnings"`
	Headers              []string                   `json:"headers"`
	SuggestedPeriodStart string                     `json:"suggested_period_start,omitempty"`
	SuggestedPeriodEnd   string                     `json:"suggested_period_end,omitempty"`
	SuggestedPaymentDate string                     `json:"suggested_payment_date,omitempty"`
}

type PayrollImportSummary struct {
	TotalRows                  int     `json:"total_rows"`
	ImportedRows               int     `json:"imported_rows"`
	TotalSalary                float64 `json:"total_salary"`
	TotalBonus                 float64 `json:"total_bonus"`
	TotalPPh                   float64 `json:"total_pph"`
	TotalBPJS                  float64 `json:"total_bpjs"`
	TotalEmployeeLoanDeduction float64 `json:"total_employee_loan_deduction"`
	TotalOverall               float64 `json:"total_overall"`
}

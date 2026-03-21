package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// COAPostedBalance represents a single account balance derived from SSOT (posted-only)
type COAPostedBalance struct {
	AccountID      uint    `json:"account_id"`
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	AccountType    string  `json:"account_type"`
	RawBalance     float64 `json:"raw_balance"`
	DisplayBalance float64 `json:"display_balance"`
	IsPositive     bool    `json:"is_positive"`
}

// COAPostedController serves COA balances based on SSOT posted-only data
type COAPostedController struct {
	db *gorm.DB
}

func NewCOAPostedController(db *gorm.DB) *COAPostedController {
	return &COAPostedController{db: db}
}

// GetPostedBalances returns posted-only balances for all non-header accounts using SSOT
// GET /api/v1/coa/posted-balances
func (ctl *COAPostedController) GetPostedBalances(c *gin.Context) {
	// Aggregate SSOT sums per account with posted-only rules.
	// Prefer unified_journal_*. Removed fallback sums from simple_ssot_journals.
	query := `
		WITH uj AS (
			SELECT 
				ujl.account_id,
				COALESCE(SUM(ujl.debit_amount) FILTER (WHERE ujd.status = 'POSTED' AND ujd.deleted_at IS NULL), 0) AS debit_sum,
				COALESCE(SUM(ujl.credit_amount) FILTER (WHERE ujd.status = 'POSTED' AND ujd.deleted_at IS NULL), 0) AS credit_sum
			FROM unified_journal_lines ujl
			JOIN unified_journal_ledger ujd ON ujd.id = ujl.journal_id
			GROUP BY ujl.account_id
		)
		SELECT
		  a.id   AS account_id,
		  a.code AS account_code,
		  a.name AS account_name,
		  a.type AS account_type,
		  COALESCE(uj.debit_sum, 0) AS debit_total,
		  COALESCE(uj.credit_sum, 0) AS credit_total
		FROM accounts a
		LEFT JOIN uj ON uj.account_id = a.id
		WHERE COALESCE(a.is_header, false) = false
		  AND a.deleted_at IS NULL
		ORDER BY a.code
	`

	type row struct {
		AccountID   uint
		AccountCode string
		AccountName string
		AccountType string
		DebitTotal  float64
		CreditTotal float64
	}
	var rows []row
	if err := ctl.db.Raw(query).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute posted balances", "message": err.Error()})
		return
	}

	balances := make([]COAPostedBalance, 0, len(rows))
	for _, r := range rows {
		var raw float64
		if r.AccountType == "ASSET" || r.AccountType == "EXPENSE" {
			raw = r.DebitTotal - r.CreditTotal
		} else {
			raw = r.CreditTotal - r.DebitTotal
		}

		// Display rule: REVENUE/LIABILITY/EQUITY show as positive (flip if negative)
		display := raw
		isPositive := raw >= 0
		switch r.AccountType {
		case "REVENUE", "LIABILITY", "EQUITY":
			if display < 0 {
				display = -display
			}
			isPositive = true
		}

		balances = append(balances, COAPostedBalance{
			AccountID:      r.AccountID,
			AccountCode:    r.AccountCode,
			AccountName:    r.AccountName,
			AccountType:    r.AccountType,
			RawBalance:     raw,
			DisplayBalance: display,
			IsPositive:     isPositive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   balances,
		"source": "SSOT_POSTED_ONLY",
	})
}

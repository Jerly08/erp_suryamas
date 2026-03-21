package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTAccountBalanceRow is a lightweight DTO for SSOT-derived balances
type SSOTAccountBalanceRow struct {
	AccountID   uint    `json:"account_id"`
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	DebitTotal  float64 `json:"debit_total"`
	CreditTotal float64 `json:"credit_total"`
	NetBalance  float64 `json:"net_balance"`
}

// SSOTAccountBalanceController exposes SSOT account balances for COA sync
type SSOTAccountBalanceController struct {
	db *gorm.DB
}

func NewSSOTAccountBalanceController(db *gorm.DB) *SSOTAccountBalanceController {
	return &SSOTAccountBalanceController{db: db}
}

// GetSSOTAccountBalances returns net balances per account from posted SSOT journals.
// GET /api/v1/ssot-reports/account-balances?as_of_date=YYYY-MM-DD&status_filter=POSTED&source_filter=ALL
func (ctl *SSOTAccountBalanceController) GetSSOTAccountBalances(c *gin.Context) {
	asOf := c.DefaultQuery("as_of_date", time.Now().Format("2006-01-02"))
	statusFilter := c.DefaultQuery("status_filter", "POSTED")
	sourceFilter := c.DefaultQuery("source_filter", "ALL")

	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid as_of_date, expected YYYY-MM-DD"})
		return
	}

	// Reserved for future filtering hooks. Financial balances currently trust
	// posted SSOT journals only and do not re-filter by source document status.
	_ = statusFilter
	_ = sourceFilter

	query := `
		SELECT
			a.id as account_id,
			a.code as account_code,
			a.name as account_name,
			a.type as account_type,
			COALESCE(SUM(ujl.debit_amount), 0) as debit_total,
			COALESCE(SUM(ujl.credit_amount), 0) as credit_total,
			CASE
				WHEN a.type IN ('ASSET', 'EXPENSE') THEN
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
				ELSE
					COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
			END as net_balance
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger ujd ON ujd.id = ujl.journal_id
		WHERE ujd.status = 'POSTED'
		  AND ujd.entry_date <= ?
		  AND ujd.deleted_at IS NULL
		  AND COALESCE(a.is_header, false) = false
		GROUP BY a.id, a.code, a.name, a.type
		ORDER BY a.code
	`

	rows := []SSOTAccountBalanceRow{}
	if err := ctl.db.Raw(query, asOf).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query SSOT balances", "message": err.Error()})
		return
	}

	hasSSotData := false
	for _, row := range rows {
		if row.DebitTotal != 0 || row.CreditTotal != 0 {
			hasSSotData = true
			break
		}
	}

	if !hasSSotData {
		fallbackQuery := `
			SELECT
				a.id as account_id,
				a.code as account_code,
				a.name as account_name,
				a.type as account_type,
				0 as debit_total,
				0 as credit_total,
				a.balance as net_balance
			FROM accounts a
			WHERE COALESCE(a.is_header, false) = false
			ORDER BY a.code
		`

		if err := ctl.db.Raw(fallbackQuery).Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query fallback balances", "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"data":    rows,
			"as_of":   asOf,
			"source":  "ACCOUNTS_TABLE",
			"message": "Using accounts.balance as SSOT data not available",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    rows,
		"as_of":   asOf,
		"source":  "SSOT_POSTED_ONLY",
		"message": "Using posted SSOT journals only",
	})
}

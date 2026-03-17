package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EnhancedInvoiceController handles enhanced invoice operations with clean PDF generation
type EnhancedInvoiceController struct {
	db         *gorm.DB
	pdfService services.PDFServiceInterface
}

// NewEnhancedInvoiceController creates a new enhanced invoice controller
func NewEnhancedInvoiceController(db *gorm.DB, pdfService services.PDFServiceInterface) *EnhancedInvoiceController {
	return &EnhancedInvoiceController{
		db:         db,
		pdfService: pdfService,
	}
}

// GenerateCleanInvoicePDF generates a clean PDF for an invoice
func (c *EnhancedInvoiceController) GenerateCleanInvoicePDF(ctx *gin.Context) {
	// Get sale ID from URL parameter
	saleIDStr := ctx.Param("id")
	saleID, err := strconv.ParseUint(saleIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid sale ID",
		})
		return
	}

	// Fetch sale with all related data
	var sale models.Sale
	if err := c.db.Preload("Customer").
		Preload("User").
		Preload("SalesPerson").
		Preload("SaleItems.Product").
		Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		First(&sale, uint(saleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "Sale not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch sale data",
			})
		}
		return
	}

	// Check if sale can have invoice generated
	if sale.Status == "DRAFT" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Cannot generate invoice for draft sale",
			"message": "Please confirm the sale first before generating invoice",
		})
		return
	}

	// Generate PDF using the enhanced service
	// Get language from query parameter (defaults to user's language from settings)
	lang := ctx.Query("lang")
	var pdfBytes []byte

	if lang != "" {
		// Use specified language
		pdfBytes, err = c.pdfService.GenerateInvoicePDFWithLanguage(&sale, lang)
	} else {
		// Use default language from settings
		pdfBytes, err = c.pdfService.GenerateInvoicePDF(&sale)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate invoice PDF",
			"message": err.Error(),
		})
		return
	}

	// Determine filename
	filename := fmt.Sprintf("invoice_%s_%s.pdf",
		sale.Code,
		time.Now().Format("20060102"))

	if sale.InvoiceNumber != "" {
		filename = fmt.Sprintf("invoice_%s_%s.pdf",
			sale.InvoiceNumber,
			time.Now().Format("20060102"))
	}

	// Set response headers for PDF download
	ctx.Header("Content-Type", "application/pdf")
	ctx.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")
	ctx.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	// Write PDF content to response
	ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// PreviewInvoice provides a preview of invoice data before PDF generation
func (c *EnhancedInvoiceController) PreviewInvoice(ctx *gin.Context) {
	// Get sale ID from URL parameter
	saleIDStr := ctx.Param("id")
	saleID, err := strconv.ParseUint(saleIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid sale ID",
		})
		return
	}

	// Fetch sale with all related data
	var sale models.Sale
	if err := c.db.Preload("Customer").
		Preload("User").
		Preload("SalesPerson").
		Preload("SaleItems.Product").
		First(&sale, uint(saleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "Sale not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch sale data",
			})
		}
		return
	}

	// Calculate invoice summary
	summary := calculateInvoiceSummary(&sale)

	// Prepare response data
	response := gin.H{
		"success": true,
		"data": gin.H{
			"sale":    sale,
			"summary": summary,
			"invoice_info": gin.H{
				"can_generate_pdf": sale.Status != "DRAFT",
				"filename": func() string {
					if sale.InvoiceNumber != "" {
						return fmt.Sprintf("invoice_%s_%s.pdf",
							sale.InvoiceNumber,
							time.Now().Format("20060102"))
					}
					return fmt.Sprintf("invoice_%s_%s.pdf",
						sale.Code,
						time.Now().Format("20060102"))
				}(),
				"status_info": gin.H{
					"current_status": sale.Status,
					"can_invoice":    sale.Status == "CONFIRMED" || sale.Status == "DRAFT",
					"is_invoiced":    sale.Status == "INVOICED" || sale.Status == "PAID",
				},
			},
		},
	}

	ctx.JSON(http.StatusOK, response)
}

// GetInvoiceTemplateData returns template data for frontend invoice preview
func (c *EnhancedInvoiceController) GetInvoiceTemplateData(ctx *gin.Context) {
	// Get sale ID from URL parameter
	saleIDStr := ctx.Param("id")
	saleID, err := strconv.ParseUint(saleIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid sale ID",
		})
		return
	}

	// Fetch sale with all related data
	var sale models.Sale
	if err := c.db.Preload("Customer").
		Preload("User").
		Preload("SalesPerson").
		Preload("SaleItems.Product").
		First(&sale, uint(saleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "Sale not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch sale data",
			})
		}
		return
	}

	// Get company settings for template
	var settings models.Settings
	c.db.First(&settings)

	// Prepare template data
	templateData := gin.H{
		"company": gin.H{
			"name":    settings.CompanyName,
			"address": settings.CompanyAddress,
			"phone":   settings.CompanyPhone,
			"email":   settings.CompanyEmail,
			"logo":    settings.CompanyLogo,
		},
		"invoice": gin.H{
			"number":    sale.InvoiceNumber,
			"sale_code": sale.Code,
			"date":      sale.Date.Format("02/01/2006"),
			"due_date":  formatDueDate(sale.DueDate),
			"status":    sale.Status,
		},
		"customer": gin.H{
			"name":    sale.Customer.Name,
			"address": sale.Customer.Address,
			"phone":   sale.Customer.Phone,
			"email":   sale.Customer.Email,
		},
		"items":         prepareItemsForTemplate(sale.SaleItems),
		"summary":       calculateInvoiceSummary(&sale),
		"payment_terms": sale.PaymentTerms,
		"notes":         sale.Notes,
		"generated_at":  time.Now().Format("02/01/2006 15:04"),
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templateData,
	})
}

// Helper functions

func calculateInvoiceSummary(sale *models.Sale) gin.H {
	subtotal := 0.0
	for _, item := range sale.SaleItems {
		if item.LineTotal > 0 {
			subtotal += item.LineTotal
		} else {
			// Fallback calculation if LineTotal is not set
			lineTotal := float64(item.Quantity)*item.UnitPrice - item.DiscountAmount
			subtotal += lineTotal
		}
	}

	discountAmount := subtotal * (sale.DiscountPercent / 100)
	taxableAmount := subtotal - discountAmount
	ppnAmount := taxableAmount * (sale.PPNPercent / 100)

	return gin.H{
		"subtotal":         subtotal,
		"discount_percent": sale.DiscountPercent,
		"discount_amount":  discountAmount,
		"taxable_amount":   taxableAmount,
		"ppn_percent":      sale.PPNPercent,
		"ppn_amount":       ppnAmount,
		"total_amount":     sale.TotalAmount,
		"currency":         sale.Currency,
	}
}

func prepareItemsForTemplate(saleItems []models.SaleItem) []gin.H {
	items := make([]gin.H, 0, len(saleItems))

	for i, item := range saleItems {
		lineTotal := item.LineTotal
		if lineTotal == 0 {
			// Fallback calculation
			lineTotal = float64(item.Quantity)*item.UnitPrice - item.DiscountAmount
		}

		items = append(items, gin.H{
			"number":      i + 1,
			"description": getItemDescription(item),
			"quantity":    item.Quantity,
			"unit_price":  item.UnitPrice,
			"discount":    item.DiscountAmount,
			"line_total":  lineTotal,
		})
	}

	return items
}

func getItemDescription(item models.SaleItem) string {
	if item.ProductName != "" {
		return item.ProductName
	}
	if item.Product.ID != 0 && item.Product.Name != "" {
		return item.Product.Name
	}
	if item.Description != "" {
		return item.Description
	}
	return "Product"
}

func formatDueDate(dueDate time.Time) string {
	if dueDate.IsZero() {
		return ""
	}
	return dueDate.Format("02/01/2006")
}

// ValidateInvoiceGeneration checks if invoice can be generated for a sale
func (c *EnhancedInvoiceController) ValidateInvoiceGeneration(ctx *gin.Context) {
	// Get sale ID from URL parameter
	saleIDStr := ctx.Param("id")
	saleID, err := strconv.ParseUint(saleIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid sale ID",
		})
		return
	}

	// Fetch sale
	var sale models.Sale
	if err := c.db.First(&sale, uint(saleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "Sale not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch sale data",
			})
		}
		return
	}

	// Validation logic
	validation := gin.H{
		"can_generate_pdf": true,
		"issues":           []string{},
		"warnings":         []string{},
	}

	// Check status
	if sale.Status == "DRAFT" {
		validation["can_generate_pdf"] = false
		validation["issues"] = append(validation["issues"].([]string), "Sale is in DRAFT status. Please confirm the sale first.")
	}

	if sale.Status == "CANCELLED" {
		validation["can_generate_pdf"] = false
		validation["issues"] = append(validation["issues"].([]string), "Cannot generate invoice for cancelled sale.")
	}

	// Check if has items
	var itemCount int64
	c.db.Model(&models.SaleItem{}).Where("sale_id = ?", sale.ID).Count(&itemCount)
	if itemCount == 0 {
		validation["can_generate_pdf"] = false
		validation["issues"] = append(validation["issues"].([]string), "Sale has no items.")
	}

	// Warnings
	if sale.Customer.Name == "" {
		validation["warnings"] = append(validation["warnings"].([]string), "Customer information is incomplete.")
	}

	if sale.TotalAmount == 0 {
		validation["warnings"] = append(validation["warnings"].([]string), "Sale total amount is zero.")
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success":    true,
		"validation": validation,
		"sale_info": gin.H{
			"id":     sale.ID,
			"code":   sale.Code,
			"status": sale.Status,
			"total":  sale.TotalAmount,
		},
	})
}

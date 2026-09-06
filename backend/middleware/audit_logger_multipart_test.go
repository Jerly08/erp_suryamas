package middleware

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPaymentAuditMiddlewareLeavesMultipartBodyForUploadHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	auditLogger := &AuditLogger{logger: log.New(io.Discard, "", 0)}
	router := gin.New()
	router.POST("/api/v1/payments/ssot/42/documents",
		auditLogger.AuditMiddleware(),
		auditLogger.PaymentAuditMiddleware(),
		func(c *gin.Context) {
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			files := form.File["documents"]
			if len(files) != 1 || files[0].Filename != "invoice.pdf" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "document missing"})
				return
			}
			c.Status(http.StatusCreated)
		},
	)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("documents", "invoice.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4\n%%EOF"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payments/ssot/42/documents", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
}

func TestCaptureAuditRequestDataUsesMetadataForMultipartWithoutReadingBody(t *testing.T) {
	body := bytes.NewBufferString("unread-multipart-content")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payments/ssot/42/documents", body)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test-boundary")

	data := captureAuditRequestData(request, sanitizePaymentData)
	require.Contains(t, data, "file_upload")
	require.Contains(t, data, "multipart/form-data")

	remaining, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	require.Equal(t, "unread-multipart-content", string(remaining))
}

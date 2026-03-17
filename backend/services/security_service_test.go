package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectSuspiciousPattern_AllowsExecuteRoute(t *testing.T) {
	service := &SecurityService{}

	suspicious, reason := service.DetectSuspiciousPattern(
		"POST",
		"/api/v1/period-closing/execute",
		"Mozilla/5.0",
		"127.0.0.1",
		map[string]string{
			"Content-Type": "application/json",
			"Origin":       "http://localhost:3003",
			"Referer":      "http://localhost:3003/settings/period-closing",
		},
	)

	require.False(t, suspicious)
	require.Empty(t, reason)
}

func TestDetectSuspiciousPattern_BlocksTraversalPath(t *testing.T) {
	service := &SecurityService{}

	suspicious, reason := service.DetectSuspiciousPattern(
		"GET",
		"/api/v1/files/../../etc/passwd",
		"Mozilla/5.0",
		"127.0.0.1",
		map[string]string{},
	)

	require.True(t, suspicious)
	require.Contains(t, reason, "SUSPICIOUS_PATH_PATTERN")
}

func TestDetectSuspiciousPattern_BlocksSuspiciousHeaderValue(t *testing.T) {
	service := &SecurityService{}

	suspicious, reason := service.DetectSuspiciousPattern(
		"GET",
		"/api/v1/reports",
		"Mozilla/5.0",
		"127.0.0.1",
		map[string]string{
			"Referer": "http://localhost:3003/report?next=javascript:alert(1)",
		},
	)

	require.True(t, suspicious)
	require.Contains(t, reason, "SUSPICIOUS_HEADER_REFERER")
}

func TestDetectSuspiciousPattern_BlocksSuspiciousUserAgent(t *testing.T) {
	service := &SecurityService{}

	suspicious, reason := service.DetectSuspiciousPattern(
		"GET",
		"/api/v1/health",
		"sqlmap/1.8",
		"127.0.0.1",
		map[string]string{},
	)

	require.True(t, suspicious)
	require.Contains(t, reason, "SUSPICIOUS_USER_AGENT")
}

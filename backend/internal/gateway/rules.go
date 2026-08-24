package gateway

import (
	"net/http"
	"strings"
)

func CanCancel(status string) bool {
	switch status {
	case "ACKNOWLEDGED", "PARTIALLY_FILLED":
		return true
	default:
		return false
	}
}

func PublicStatus(internal string) string {
	switch internal {
	case "NEW", "VALIDATING", "VALIDATED", "PENDING":
		return "QUEUED"
	default:
		return internal
	}
}

func ScopeFor(r *http.Request) string {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/cancel"):
		return ScopeOrdersCancel
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/orders"):
		return ScopeOrdersCreate
	case strings.Contains(path, "/trades"):
		return ScopeTradesRead
	case strings.Contains(path, "/positions"):
		return ScopePositionsRead
	case strings.Contains(path, "/firm"):
		return ""
	default:
		return ScopeOrdersRead
	}
}

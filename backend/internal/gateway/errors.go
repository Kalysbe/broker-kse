package gateway

import (
	"encoding/json"
	"errors"
	"net/http"

	"statebroker/sbis/internal/platform/httpx"
)

const (
	ScopeOrdersCreate  = "orders:create"
	ScopeOrdersRead    = "orders:read"
	ScopeOrdersCancel  = "orders:cancel"
	ScopeTradesRead    = "trades:read"
	ScopePositionsRead = "positions:read"
)

var DefaultScopes = []string{
	ScopeOrdersCreate, ScopeOrdersRead, ScopeOrdersCancel, ScopeTradesRead, ScopePositionsRead,
}

type ErrorBody struct {
	Error   string         `json:"error"`
	Details map[string]any `json:"details"`
}

func Write(w http.ResponseWriter, status int, errText string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: errText, Details: details})
}

func WriteErr(w http.ResponseWriter, r *http.Request, err error) {
	var api httpx.APIError
	if !errors.As(err, &api) {
		Write(w, http.StatusInternalServerError, "internal server error", map[string]any{
			"request_id": httpx.RequestIDFrom(r.Context()),
		})
		return
	}
	Write(w, api.Status, api.Message, map[string]any{
		"code":       api.Code,
		"request_id": httpx.RequestIDFrom(r.Context()),
	})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

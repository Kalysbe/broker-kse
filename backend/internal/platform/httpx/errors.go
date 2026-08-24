package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e APIError) Error() string { return e.Code + ": " + e.Message }

func E(status int, code, msg string) APIError {
	return APIError{Status: status, Code: code, Message: msg}
}

func EFields(status int, code, msg string, fields map[string]string) APIError {
	return APIError{Status: status, Code: code, Message: msg, Fields: fields}
}

var (
	ErrUnauthorized       = E(http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	ErrForbidden          = E(http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	ErrNotFound           = E(http.StatusNotFound, "NOT_FOUND", "Resource not found")
	ErrConflict           = E(http.StatusConflict, "CONFLICT", "Conflict")
	ErrValidation         = E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Validation failed")
	ErrInsufficientFunds  = E(http.StatusUnprocessableEntity, "INSUFFICIENT_FUNDS", "Insufficient available funds")
	ErrInsufficientPos    = E(http.StatusUnprocessableEntity, "INSUFFICIENT_POSITION", "Insufficient available quantity")
	ErrInvalidTransition  = E(http.StatusConflict, "ORDER_INVALID_TRANSITION", "Order status transition is not allowed")
	ErrSelfApproval       = E(http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", "Maker-checker policy forbids self-approval")
	ErrStepUpRequired     = E(http.StatusForbidden, "STEP_UP_REQUIRED", "Re-authentication is required")
	ErrAccountBlocked     = E(http.StatusUnprocessableEntity, "ACCOUNT_BLOCKED", "Account is not active")
	ErrInstrumentHalted   = E(http.StatusUnprocessableEntity, "INSTRUMENT_HALTED", "Instrument is not tradable")
	ErrUnsupportedType    = E(http.StatusUnprocessableEntity, "UNSUPPORTED_ORDER_TYPE", "Venue does not support this order type")
	ErrAdapterUnavailable = E(http.StatusServiceUnavailable, "ADAPTER_UNAVAILABLE", "No healthy adapter available")
	ErrIdempotency        = E(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency key reused with a different payload")
	ErrRateLimited        = E(http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded")
)

func DecodeJSONError(err error) APIError {
	if err == nil {
		return ErrValidation
	}
	if errors.Is(err, io.EOF) {
		return E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Тело запроса пустое")
	}
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) && ute != nil {
		name := ute.Field
		if name == "" {
			name = ute.Struct
		}
		if name == "" {
			return E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Неверный тип значения в JSON")
		}
		return EFields(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			fmt.Sprintf("Поле «%s» имеет неверный тип данных", name),
			map[string]string{name: "invalid_type"})
	}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Тело запроса не является корректным JSON")
	}
	msg := err.Error()
	const unknown = `json: unknown field "`
	if strings.HasPrefix(msg, unknown) {
		field := strings.TrimSuffix(strings.TrimPrefix(msg, unknown), `"`)
		return EFields(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			fmt.Sprintf("Неизвестное поле «%s». Проверьте, что сервер обновлён, или уберите лишнее поле.", field),
			map[string]string{field: "unknown"})
	}
	return E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Некорректное тело запроса: "+msg)
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := RequestIDFrom(r.Context())
	var api APIError
	if !errors.As(err, &api) {
		api = E(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(api.Status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: ErrorDetail{
		Code:      api.Code,
		Message:   api.Message,
		RequestID: reqID,
		Fields:    api.Fields,
	}})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

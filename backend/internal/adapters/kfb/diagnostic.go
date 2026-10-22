package kfb

import (
	"encoding/json"
	"time"
)

// RequestError preserves the evidence without authentication headers.
// A gateway response alone does not prove the exchange received the order.
type RequestError struct {
	Code       string    `json:"code"`
	Source     string    `json:"source"`
	Stage      string    `json:"stage"`
	Message    string    `json:"message"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	RequestID  string    `json:"request_id"`
	Request    any       `json:"request,omitempty"`
	HTTPStatus int       `json:"http_status,omitempty"`
	Response   string    `json:"response,omitempty"`
	At         time.Time `json:"at"`
	ElapsedMS  int64     `json:"elapsed_ms"`
}

func (e *RequestError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

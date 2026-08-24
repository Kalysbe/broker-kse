package httpx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeJSONErrorUnknownField(t *testing.T) {
	unknown := DecodeJSONError(errString(`json: unknown field "client_code"`))
	if unknown.Code != "VALIDATION_ERROR" || !strings.Contains(unknown.Message, "client_code") {
		t.Fatal(unknown.Message)
	}
	if unknown.Fields["client_code"] != "unknown" {
		t.Fatal(unknown.Fields)
	}
	typed := DecodeJSONError(&json.UnmarshalTypeError{Field: "tax_number", Value: "number", Type: reflect.TypeOf("")})
	if !strings.Contains(typed.Message, "tax_number") {
		t.Fatal(typed.Message)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

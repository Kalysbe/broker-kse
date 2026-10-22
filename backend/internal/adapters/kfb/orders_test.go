package kfb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchOrdersUsesSelectedConnectionAndPagination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/api/v1/orders" || r.Header.Get("Authorization") != "Bearer selected-token" {
			t.Errorf("unexpected order request: %s %s", r.Method, r.URL)
		}
		switch r.URL.Query().Get("offset") {
		case "":
			fmt.Fprint(w, `{"source":"quik_db","orders":[{"id":"quik-1","quantity":"1"}],"has_more":true,"next_offset":1}`)
		case "1":
			fmt.Fprint(w, `{"source":"quik_db","orders":[{"id":"quik-2"},{"id":"quik-3"}],"has_more":false}`)
		default:
			t.Error("unexpected offset")
		}
	}))
	defer server.Close()
	client := New("http://unused.invalid", "old-token")
	client.ApplyConnection(ConnectionSettings{RESTURL: server.URL, APIToken: "selected-token"})
	result, err := client.FetchOrders(context.Background())
	if err != nil || len(result.Orders) != 3 || result.Source != "quik_db" || requests != 2 {
		t.Fatalf("result=%+v requests=%d err=%v", result, requests, err)
	}
}

func TestFetchOrdersRejectsPartialOrInvalidLists(t *testing.T) {
	for _, body := range []string{`{}`, `{"orders":{}}`, `{"orders":[{"id":"one"}],"has_more":true,"next_offset":0}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if _, err := New(server.URL, "test").FetchOrders(context.Background()); err == nil {
				t.Fatal("invalid list accepted")
			}
		})
	}
}

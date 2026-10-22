package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"statebroker/sbis/internal/platform/config"
)

func TestWebSocketRejectsInvalidTokenThroughMiddleware(t *testing.T) {
	a := &App{Cfg: config.Config{JWTSecret: "test-secret", CORSOrigins: []string{"http://localhost:15173"}}, Log: zap.NewNop()}
	server := httptest.NewServer(a.Router())
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime/ws"
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Origin": []string{"http://localhost:15173"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(wsCommand{Type: "auth", Token: "invalid"}); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&reply); err != nil {
		t.Fatal(err)
	}
	if reply.Type != "unauthorized" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("unauthenticated socket remained open")
	}
}

func TestWebSocketRejectsForeignOrigin(t *testing.T) {
	a := &App{Log: zap.NewNop()}
	server := httptest.NewServer(a.Router())
	defer server.Close()
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime/ws", http.Header{"Origin": []string{"https://foreign.example"}})
	if conn != nil {
		conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatal("foreign origin accepted")
	}
}

func TestWebSocketRequestPaths(t *testing.T) {
	for _, path := range []string{"/v1/positions", "/v1/orders?status=ACTIVE", "/v1/auth/me"} {
		if !wsPathAllowed(path) {
			t.Errorf("valid path rejected: %s", path)
		}
	}
	for _, path := range []string{"https://example.com/v1/orders", "/health", "/v1/realtime/ws", "/v1/realtime%2fws", "/v1/auth/login", "/v1/orders/../auth/login"} {
		if wsPathAllowed(path) {
			t.Errorf("unsafe path accepted: %s", path)
		}
	}
}

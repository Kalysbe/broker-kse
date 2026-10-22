package app

import (
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http/httptest"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/ratelimit"
	"testing"
)

func TestRealtimeAdmissionUsesBrokerAcrossUsers(t *testing.T) {
	a := &App{RealtimeAdmission: ratelimit.NewAdmission()}
	a.RealtimeAdmission.Replace([]ratelimit.Policy{{Scope: "broker", Subject: "*", Operation: "Ping", Rate: 1, Burst: 1, Concurrent: 1, Enabled: true}})
	broker := uuid.New()
	first := httpx.Principal{BrokerID: broker, UserID: uuid.New()}
	second := httpx.Principal{BrokerID: broker, UserID: uuid.New()}
	release, err := a.admitRealtime(first, "Ping", "one")
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err = a.admitRealtime(second, "Ping", "two"); status.Code(err) != codes.ResourceExhausted {
		t.Fatal("broker budget not shared", err)
	}
	if details := status.Convert(err).Details(); len(details) != 1 {
		t.Fatal("retry missing")
	} else if _, ok := details[0].(*errdetails.RetryInfo); !ok {
		t.Fatal(details)
	}
	second.BrokerID = uuid.New()
	release, err = a.admitRealtime(second, "Ping", "other")
	if err != nil {
		t.Fatal("other broker blocked", err)
	}
	release()
}
func TestRealtimeAdmissionConnectConcurrency(t *testing.T) {
	a := &App{RealtimeAdmission: ratelimit.NewAdmission()}
	p := ratelimit.Policy{Scope: "broker", Subject: "*", Operation: "Connect", Rate: 100, Burst: 100, Concurrent: 1, Enabled: true}
	a.RealtimeAdmission.Replace([]ratelimit.Policy{p})
	principal := httpx.Principal{BrokerID: uuid.New()}
	release, err := a.admitRealtime(principal, "Connect", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.admitRealtime(principal, "Connect", ""); status.Code(err) != codes.ResourceExhausted {
		t.Fatal(err)
	}
	p.Concurrent = 2
	a.RealtimeAdmission.Replace([]ratelimit.Policy{p})
	other, err := a.admitRealtime(principal, "Connect", "")
	if err != nil {
		t.Fatal("dynamic concurrency", err)
	}
	other()
	release()
	release()
	if _, err = a.admitRealtime(httpx.Principal{}, "Connect", ""); status.Code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
}
func TestRealtimeLimitsAdministrationDenied(t *testing.T) {
	a := &App{}
	for _, role := range []string{"CLIENT", "BROKER", "ADMIN", "OPERATOR"} {
		if realtimeLimitsAdministrator(httpx.Principal{Roles: []string{role}}) {
			t.Fatal("unauthorized role", role)
		}
	}
	if !realtimeLimitsAdministrator(httpx.Principal{Roles: []string{"SUPER_ADMIN"}}) {
		t.Fatal("admin denied")
	}
	w := httptest.NewRecorder()
	a.handleRealtimeLimits(w, httptest.NewRequest("PUT", "/v1/realtime/rate-limits", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}

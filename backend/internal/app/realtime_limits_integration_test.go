package app

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"os"
	pb "statebroker/sbis/internal/gen/realtime/v1"
	"statebroker/sbis/internal/outbox"
	"statebroker/sbis/internal/platform/authn"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/ratelimit"
	"strings"
	"testing"
	"time"
)

func TestRealtimeBrokerStreamsIntegration(t *testing.T) {
	dsn, redisURL := os.Getenv("REALTIME_LIMIT_TEST_DATABASE_URL"), os.Getenv("REALTIME_LIMIT_TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Skip("isolated DB and Redis required")
	}
	if !strings.Contains(dsn, "/realtime_limit_test?") || !strings.Contains(redisURL, "broker-app-realtime-test-redis") {
		t.Fatal("isolated fixture endpoints required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	fixture := `CREATE TABLE brokers(id uuid PRIMARY KEY);
 CREATE TABLE users(id uuid PRIMARY KEY,status text,client_id uuid);
 CREATE TABLE sessions(id uuid PRIMARY KEY,user_id uuid,revoked_at timestamptz);
 CREATE TABLE roles(id uuid PRIMARY KEY,code text);
 CREATE TABLE user_roles(user_id uuid,role_id uuid);
 CREATE TABLE role_permissions(role_id uuid,permission text);
 CREATE TABLE orders(id uuid PRIMARY KEY,broker_id uuid,client_id uuid);`
	if _, err = pool.Exec(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/000017_realtime_rate_limits.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE realtime_rate_limits SET rate=1,burst=1 WHERE operation='Ping'; UPDATE realtime_rate_limits SET concurrent=2 WHERE operation='Connect'`); err != nil {
		t.Fatal(err)
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err = rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	a := &App{DB: pool, Redis: rdb, Cfg: config.Config{JWTSecret: "realtime-test"}, RealtimeAdmission: ratelimit.NewAdmission()}
	if err = a.refreshRealtimeLimits(ctx); err != nil {
		t.Fatal(err)
	}
	role := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO roles VALUES($1,'BROKER');`, role); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO role_permissions VALUES($1,'orders.read')`, role); err != nil {
		t.Fatal(err)
	}
	broker, otherBroker := uuid.New(), uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO brokers VALUES($1),($2)`, broker, otherBroker); err != nil {
		t.Fatal(err)
	}
	token := func(bid uuid.UUID) string {
		user, session := uuid.New(), uuid.New()
		for _, query := range []struct {
			sql  string
			args []any
		}{{`INSERT INTO users VALUES($1,'ACTIVE',NULL)`, []any{user}}, {`INSERT INTO sessions VALUES($1,$2,NULL)`, []any{session, user}}, {`INSERT INTO user_roles VALUES($1,$2)`, []any{user, role}}} {
			if _, err = pool.Exec(ctx, query.sql, query.args...); err != nil {
				t.Fatal(err)
			}
		}
		tok, err := authn.IssueAccess(a.Cfg.JWTSecret, user, session, bid, []string{"BROKER"}, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.MaxRecvMsgSize(64 << 10))
	a.RegisterRealtime(server)
	go server.Serve(listener)
	defer server.Stop()
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	open := func(tok string) grpc.BidiStreamingClient[pb.ClientMessage, pb.ServerMessage] {
		stream, err := pb.NewRealtimeServiceClient(conn).Connect(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok))
		if err != nil {
			t.Fatal(err)
		}
		_ = stream.Send(&pb.ClientMessage{Command: &pb.ClientMessage_Subscribe{Subscribe: &pb.Subscribe{Topics: []pb.Topic{pb.Topic_TOPIC_ORDERS}}}})
		return stream
	}
	ready := func(stream grpc.BidiStreamingClient[pb.ClientMessage, pb.ServerMessage]) {
		t.Helper()
		m, err := stream.Recv()
		if err != nil || m.GetSubscription() == nil {
			t.Fatal("subscription", err)
		}
	}
	first, second := open(token(broker)), open(token(broker))
	ready(first)
	ready(second)
	denied := open(token(broker))
	if _, err = denied.Recv(); status.Code(err) != codes.ResourceExhausted {
		t.Fatal("connection concurrency", err)
	}
	control := open(token(otherBroker))
	ready(control)
	ping := &pb.ClientMessage{RequestId: "ping", Command: &pb.ClientMessage_Ping{Ping: &pb.Ping{Nonce: "test"}}}
	if err = first.Send(ping); err != nil {
		t.Fatal(err)
	}
	if _, err = first.Recv(); err != nil {
		t.Fatal(err)
	}
	if err = second.Send(ping); err != nil {
		t.Fatal(err)
	}
	if _, err = second.Recv(); status.Code(err) != codes.ResourceExhausted {
		t.Fatal("cross-user shared broker rate", err)
	}
	if err = control.Send(ping); err != nil {
		t.Fatal(err)
	}
	if _, err = control.Recv(); err != nil {
		t.Fatal("other broker", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE realtime_rate_limits SET enabled=false WHERE operation='Ping'`); err != nil {
		t.Fatal(err)
	}
	if err = a.refreshRealtimeLimits(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = first.Send(ping); err != nil {
			t.Fatal(err)
		}
		if _, err = first.Recv(); err != nil {
			t.Fatal("active stream reconfiguration", err)
		}
	}
	// Outbound order events do not consume the exhausted inbound Ping budget.
	if _, err = pool.Exec(ctx, `UPDATE realtime_rate_limits SET enabled=true WHERE operation='Ping'`); err != nil {
		t.Fatal(err)
	}
	if err = a.refreshRealtimeLimits(ctx); err != nil {
		t.Fatal(err)
	}
	orderID := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO orders VALUES($1,$2,NULL)`, orderID, broker); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err = rdb.XAdd(ctx, &redis.XAddArgs{Stream: outbox.DefaultStream, Values: map[string]any{"outbox_id": fmt.Sprint(i), "aggregate_type": "order", "aggregate_id": orderID.String(), "event_type": "order.accepted", "payload": "{}", "created_at": time.Now().Format(time.RFC3339Nano)}}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 10; i++ {
		m, err := first.Recv()
		if err != nil || m.GetEvent() == nil {
			t.Fatal("outbound event throttled", err)
		}
	}
	restarted := &App{DB: pool, RealtimeAdmission: ratelimit.NewAdmission()}
	if err = restarted.refreshRealtimeLimits(ctx); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range restarted.RealtimeAdmission.Snapshot() {
		if u.Scope == "broker" && u.Operation == "Ping" && u.Rate == 1 && u.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatal("policy not persistent")
	}
}

package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	realtimev1 "statebroker/sbis/internal/gen/realtime/v1"
	"statebroker/sbis/internal/outbox"
	"statebroker/sbis/internal/platform/authn"
	"statebroker/sbis/internal/platform/httpx"
)

type realtimeServer struct {
	realtimev1.UnimplementedRealtimeServiceServer
	app *App
}

func (a *App) RegisterRealtime(server *grpc.Server) {
	realtimev1.RegisterRealtimeServiceServer(server, &realtimeServer{app: a})
}

func (s *realtimeServer) Connect(stream grpc.BidiStreamingServer[realtimev1.ClientMessage, realtimev1.ServerMessage]) error {
	p, err := s.authenticate(stream.Context())
	if err != nil {
		return err
	}
	md, _ := metadata.FromIncomingContext(stream.Context())
	requestID := ""
	if values := md.Get("x-request-id"); len(values) > 0 {
		requestID = values[0]
	}
	releaseConnect, err := s.app.admitRealtime(p, "Connect", requestID)
	if err != nil {
		return err
	}
	defer releaseConnect()
	if s.app.Redis == nil {
		return status.Error(codes.Unavailable, "realtime event stream is unavailable")
	}

	commands := make(chan *realtimev1.ClientMessage, 8)
	recvErr := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			select {
			case commands <- msg:
			case <-stream.Context().Done():
				return
			}
		}
	}()

	events := make(chan redis.XMessage, 32)
	readErr := make(chan error, 1)
	lastID := "0-0"
	tail, err := s.app.Redis.XRevRangeN(stream.Context(), outbox.DefaultStream, "+", "-", 1).Result()
	if err != nil {
		return status.Error(codes.Unavailable, "event stream unavailable")
	}
	if len(tail) > 0 {
		lastID = tail[0].ID
	}
	go s.readEvents(stream.Context(), events, readErr, lastID)
	subscribed := map[realtimev1.Topic]bool{}
	authTicker := time.NewTicker(20 * time.Second)
	defer authTicker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-authTicker.C:
			if _, err := s.authenticate(stream.Context()); err != nil {
				return err
			}
		case err := <-recvErr:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case err := <-readErr:
			return status.Error(codes.Unavailable, err.Error())
		case msg := <-commands:
			commandErr := func() error {
				method := "UnknownCommand"
				switch msg.Command.(type) {
				case *realtimev1.ClientMessage_Subscribe:
					method = "Subscribe"
				case *realtimev1.ClientMessage_Unsubscribe:
					method = "Unsubscribe"
				case *realtimev1.ClientMessage_Ping:
					method = "Ping"
				}
				release, err := s.app.admitRealtime(p, method, msg.RequestId)
				if err != nil {
					return err
				}
				defer release()
				now := time.Now().UnixMilli()
				switch cmd := msg.Command.(type) {
				case *realtimev1.ClientMessage_Subscribe:
					for _, topic := range cmd.Subscribe.Topics {
						if topicAllowed(p, topic) {
							subscribed[topic] = true
						}
					}
					if err := stream.Send(&realtimev1.ServerMessage{RequestId: msg.RequestId, SentAtUnixMs: now, Message: &realtimev1.ServerMessage_Subscription{Subscription: &realtimev1.SubscriptionAck{Topics: topicList(subscribed)}}}); err != nil {
						return err
					}
				case *realtimev1.ClientMessage_Unsubscribe:
					for _, topic := range cmd.Unsubscribe.Topics {
						delete(subscribed, topic)
					}
					if err := stream.Send(&realtimev1.ServerMessage{RequestId: msg.RequestId, SentAtUnixMs: now, Message: &realtimev1.ServerMessage_Subscription{Subscription: &realtimev1.SubscriptionAck{Topics: topicList(subscribed)}}}); err != nil {
						return err
					}
				case *realtimev1.ClientMessage_Ping:
					if err := stream.Send(&realtimev1.ServerMessage{RequestId: msg.RequestId, SentAtUnixMs: now, Message: &realtimev1.ServerMessage_Pong{Pong: &realtimev1.Pong{Nonce: cmd.Ping.Nonce}}}); err != nil {
						return err
					}
				default:
					if err := stream.Send(&realtimev1.ServerMessage{RequestId: msg.RequestId, SentAtUnixMs: now, Message: &realtimev1.ServerMessage_Error{Error: &realtimev1.StreamError{Code: "INVALID_COMMAND", Message: "command is required"}}}); err != nil {
						return err
					}
				}
				return nil
			}()
			if commandErr != nil {
				return commandErr
			}
		case raw := <-events:
			event := redisEvent(raw)
			if event.Topic == realtimev1.Topic_TOPIC_UNSPECIFIED || !subscribed[event.Topic] {
				continue
			}
			if !s.eventAllowed(stream.Context(), p, raw) {
				continue
			}
			if err := stream.Send(&realtimev1.ServerMessage{SentAtUnixMs: time.Now().UnixMilli(), Message: &realtimev1.ServerMessage_Event{Event: event}}); err != nil {
				return err
			}
		}
	}
}

func (s *realtimeServer) authenticate(ctx context.Context) (httpx.Principal, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) == 0 || !strings.HasPrefix(strings.ToLower(values[0]), "bearer ") {
		return httpx.Principal{}, status.Error(codes.Unauthenticated, "Bearer token required")
	}
	claims, err := authn.ParseAccess(s.app.Cfg.JWTSecret, strings.TrimSpace(values[0][7:]))
	if err != nil {
		return httpx.Principal{}, status.Error(codes.Unauthenticated, "invalid access token")
	}
	uid, err1 := uuid.Parse(claims.Subject)
	sid, err2 := uuid.Parse(claims.SessionID)
	bid, err3 := uuid.Parse(claims.BrokerID)
	if err1 != nil || err2 != nil || err3 != nil || uid == uuid.Nil || sid == uuid.Nil || bid == uuid.Nil {
		return httpx.Principal{}, status.Error(codes.Unauthenticated, "invalid token claims")
	}
	var clientID *uuid.UUID
	var active bool
	if err := s.app.DB.QueryRow(ctx, `SELECT s.revoked_at IS NULL AND u.status='ACTIVE', u.client_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=$1 AND s.user_id=$2`, sid, uid).Scan(&active, &clientID); err != nil || !active {
		return httpx.Principal{}, status.Error(codes.Unauthenticated, "session is inactive")
	}
	roles, perms, err := s.app.permissionsForUser(ctx, uid)
	if err != nil {
		return httpx.Principal{}, status.Error(codes.Internal, "authorization failed")
	}
	return httpx.Principal{UserID: uid, SessionID: sid, BrokerID: bid, ClientID: clientID, Roles: roles, Permissions: perms}, nil
}

func (s *realtimeServer) readEvents(ctx context.Context, out chan<- redis.XMessage, errs chan<- error, lastID string) {
	for {
		streams, err := s.app.Redis.XRead(ctx, &redis.XReadArgs{Streams: []string{outbox.DefaultStream, lastID}, Count: 50, Block: time.Second}).Result()
		if err != nil && err != redis.Nil {
			errs <- err
			return
		}
		for _, result := range streams {
			for _, msg := range result.Messages {
				lastID = msg.ID
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func redisEvent(raw redis.XMessage) *realtimev1.Event {
	eventType := valueString(raw.Values["event_type"])
	created, _ := time.Parse(time.RFC3339Nano, valueString(raw.Values["created_at"]))
	return &realtimev1.Event{Id: valueString(raw.Values["outbox_id"]), Topic: topicForEvent(eventType, valueString(raw.Values["aggregate_type"])), Type: eventType, AggregateId: valueString(raw.Values["aggregate_id"]), PayloadJson: []byte(valueString(raw.Values["payload"])), CreatedAtUnixMs: created.UnixMilli()}
}

func topicForEvent(eventType, aggregate string) realtimev1.Topic {
	v := strings.ToLower(eventType + " " + aggregate)
	switch {
	case strings.Contains(v, "order"):
		return realtimev1.Topic_TOPIC_ORDERS
	case strings.Contains(v, "execution"):
		return realtimev1.Topic_TOPIC_EXECUTIONS
	case strings.Contains(v, "trade"):
		return realtimev1.Topic_TOPIC_TRADES
	case strings.Contains(v, "market"):
		return realtimev1.Topic_TOPIC_MARKET_DATA
	case strings.Contains(v, "position"):
		return realtimev1.Topic_TOPIC_POSITIONS
	case strings.Contains(v, "notification"):
		return realtimev1.Topic_TOPIC_NOTIFICATIONS
	case strings.Contains(v, "adapter"):
		return realtimev1.Topic_TOPIC_ADAPTERS
	case strings.Contains(v, "alert"):
		return realtimev1.Topic_TOPIC_ALERTS
	}
	return realtimev1.Topic_TOPIC_UNSPECIFIED
}

func topicAllowed(p httpx.Principal, topic realtimev1.Topic) bool {
	switch topic {
	case realtimev1.Topic_TOPIC_ORDERS:
		return p.Has("orders.read")
	case realtimev1.Topic_TOPIC_EXECUTIONS, realtimev1.Topic_TOPIC_TRADES:
		return p.Has("trades.read")
	case realtimev1.Topic_TOPIC_MARKET_DATA:
		return p.Has("marketdata.read")
	case realtimev1.Topic_TOPIC_POSITIONS:
		return p.Has("portfolio.read")
	case realtimev1.Topic_TOPIC_NOTIFICATIONS:
		return true
	case realtimev1.Topic_TOPIC_ADAPTERS:
		return p.Has("adapters.read")
	case realtimev1.Topic_TOPIC_ALERTS:
		return p.Has("notifications.manage")
	}
	return false
}

func (s *realtimeServer) eventAllowed(ctx context.Context, p httpx.Principal, raw redis.XMessage) bool {
	if strings.ToLower(valueString(raw.Values["aggregate_type"])) == "order" {
		id, err := uuid.Parse(valueString(raw.Values["aggregate_id"]))
		if err != nil {
			return false
		}
		var allowed bool
		err = s.app.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1 AND broker_id=$2 AND ($3::uuid IS NULL OR client_id=$3))`, id, p.BrokerID, p.ClientID).Scan(&allowed)
		return err == nil && allowed
	}
	if p.ClientID == nil {
		return true
	}
	aggregate, idText := strings.ToLower(valueString(raw.Values["aggregate_type"])), valueString(raw.Values["aggregate_id"])
	if strings.Contains(aggregate, "market") {
		return true
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return false
	}
	var ok bool
	switch aggregate {
	case "order":
		_ = s.app.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1 AND client_id=$2)`, id, *p.ClientID).Scan(&ok)
	case "trade":
		_ = s.app.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trades WHERE id=$1 AND client_id=$2)`, id, *p.ClientID).Scan(&ok)
	case "execution":
		_ = s.app.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM executions WHERE id=$1 AND client_id=$2)`, id, *p.ClientID).Scan(&ok)
	default:
		return false
	}
	return ok
}

func topicList(set map[realtimev1.Topic]bool) []realtimev1.Topic {
	out := make([]realtimev1.Topic, 0, len(set))
	for topic := range set {
		out = append(out, topic)
	}
	return out
}
func valueString(v any) string {
	if v == nil {
		return ""
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	raw, _ := json.Marshal(v)
	if s, ok := v.(string); ok {
		return s
	}
	return strings.Trim(string(raw), "\"")
}

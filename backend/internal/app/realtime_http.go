package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	pb "statebroker/sbis/internal/gen/realtime/v1"
	"statebroker/sbis/internal/platform/httpx"
)

// Browser transport for our existing gRPC service; authorization and filtering stay in Connect.
func (a *App) handleOrderStream(w http.ResponseWriter, r *http.Request) {
	_, port, err := net.SplitHostPort(a.Cfg.GRPCAddr)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	conn, err := grpc.NewClient("localhost:"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(metadata.AppendToOutgoingContext(r.Context(), "authorization", r.Header.Get("Authorization"), "x-request-id", httpx.RequestIDFrom(r.Context())))
	defer cancel()
	stream, err := pb.NewRealtimeServiceClient(conn).Connect(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer stream.CloseSend()
	topics := []pb.Topic{pb.Topic_TOPIC_ORDERS}
	if r.URL.Query().Get("market") == "true" {
		topics = append(topics, pb.Topic_TOPIC_MARKET_DATA)
	}
	if err = stream.Send(&pb.ClientMessage{Command: &pb.ClientMessage_Subscribe{Subscribe: &pb.Subscribe{Topics: topics}}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	timer := time.AfterFunc(10*time.Second, cancel)
	first, err := stream.Recv()
	timer.Stop()
	if err != nil || first.GetSubscription() == nil {
		if status.Code(err) == codes.ResourceExhausted {
			retry := 1
			for _, detail := range status.Convert(err).Details() {
				if info, ok := detail.(*errdetails.RetryInfo); ok {
					retry = int(math.Ceil(info.RetryDelay.AsDuration().Seconds()))
					if retry < 1 {
						retry = 1
					}
				}
			}
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			httpx.WriteError(w, r, httpx.E(429, "RESOURCE_EXHAUSTED", "Broker realtime rate limit exceeded"))
			return
		}
		httpx.WriteError(w, r, httpx.E(503, "STREAM_UNAVAILABLE", "Поток событий недоступен"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	write := func(data string) error {
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		defer controller.SetWriteDeadline(time.Time{})
		if _, err := fmt.Fprint(w, data); err != nil {
			return err
		}
		return controller.Flush()
	}
	connectionFrame := func(kind string) string {
		return fmt.Sprintf("event: %s\ndata: {\"upstream_connected\":%t}\n\n", kind, a.KFBStreamConnected.Load())
	}
	if write(connectionFrame("ready")) != nil {
		return
	}
	// Only Send here; the receive loop is the sole Recv caller.
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if stream.Send(&pb.ClientMessage{Command: &pb.ClientMessage_Ping{Ping: &pb.Ping{Nonce: "heartbeat"}}}) != nil {
					cancel()
					return
				}
			}
		}
	}()
	for {
		message, err := stream.Recv()
		if err != nil {
			return
		}
		if event := message.GetEvent(); event != nil {
			payload, _ := json.Marshal(map[string]any{"order_id": event.AggregateId, "type": event.Type})
			if write("data: "+string(payload)+"\n\n") != nil {
				return
			}
		} else if write(connectionFrame("connection")) != nil {
			return
		}
	}
}

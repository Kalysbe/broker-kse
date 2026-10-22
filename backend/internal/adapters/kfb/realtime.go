package kfb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"

	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	pb "statebroker/sbis/internal/gen/kfbrealtime/v1"
)

// StreamOrders opens the gateway's native TLS stream. Credentials never reach browsers.
func (c *Client) StreamOrders(ctx context.Context, ready func() error, receive func(json.RawMessage) error) error {
	return c.StreamEvents(ctx, ready, func(kind string, raw json.RawMessage) error {
		if kind == "order.updated" {
			return receive(raw)
		}
		return nil
	})
}
func (c *Client) StreamEvents(ctx context.Context, ready func() error, receive func(string, json.RawMessage) error) error {
	if c.gatewayToken() == "" {
		return fmt.Errorf("KFB_GATEWAY_TOKEN is not configured")
	}

	settings := c.ConnectionSettings()
	if settings.GRPCTarget == "" {
		return fmt.Errorf("gRPC address is not configured")
	}
	roots, _ := x509.SystemCertPool()
	if roots == nil {
		roots = x509.NewCertPool()
	}
	if settings.CAPEM != "" && !roots.AppendCertsFromPEM([]byte(settings.CAPEM)) {
		return fmt.Errorf("invalid KFB realtime CA")
	}
	conn, err := grpc.NewClient(settings.GRPCTarget, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: settings.ServerName})), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(8<<20)))
	if err != nil {
		return err
	}
	defer conn.Close()
	streamCtx, stop := context.WithCancel(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+settings.APIToken))
	defer stop()
	stream, err := pb.NewRealtimeServiceClient(conn).Stream(streamCtx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	if err := stream.Send(&pb.ClientMessage{Command: &pb.ClientMessage_Subscribe{Subscribe: &pb.Subscribe{EventTypes: []string{"order.updated", "marketdata.tick", "positions.updated"}}}}); err != nil {
		return err
	}
	// Fail a stalled handshake without imposing a deadline on the long-lived subscription.
	timer := time.AfterFunc(10*time.Second, stop)
	defer timer.Stop()
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.GetReady() == nil {
		return fmt.Errorf("KFB realtime subscription not acknowledged")
	}
	timer.Stop()
	if err := ready(); err != nil {
		return err
	}
	for {
		message, err := stream.Recv()
		if err != nil {
			return err
		}
		event := message.GetEvent()
		if event == nil {
			continue
		}
		raw, err := json.Marshal(event.Data.AsMap())
		if err != nil {
			return err
		}
		if err := receive(event.Type, raw); err != nil {
			return err
		}
	}
}

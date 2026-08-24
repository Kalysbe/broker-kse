package app

import (
	"context"

	"go.uber.org/zap"
	"statebroker/sbis/internal/gateway"
)

func (a *App) Bootstrap(ctx context.Context) error {
	if err := a.loadHouse(ctx); err != nil {
		return err
	}
	if a.Limiter == nil {
		a.Limiter = gateway.NewLimiter()
	}
	if a.KFB != nil && a.KFB.Configured() {
		snap := a.KFB.Refresh(ctx)
		a.syncAdapterRow(ctx, "KFB", "EXCHANGE", "Кыргызская фондовая биржа", snap.Health, 20)
		if a.Log != nil {
			if snap.Connected {
				a.Log.Info("kfb_connected", zap.String("token_prefix", snap.TokenPrefix), zap.String("base_url", snap.BaseURL))
			} else {
				a.Log.Warn("kfb_disconnected", zap.String("token_prefix", snap.TokenPrefix), zap.String("error", snap.Error))
			}
		}
	}
	a.heartbeat(ctx, "api", "UP")
	return nil
}

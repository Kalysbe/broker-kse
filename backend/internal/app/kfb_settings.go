package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/platform/httpx"
)

const kfbSettingsKey = "kfb.connection.v1"

type kfbSettingsState struct {
	mu        sync.Mutex
	encrypted string
}

func sealKFBSettings(secret string, data []byte) (string, error) {
	key := sha256.Sum256([]byte("sbis:kfb:connection:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, data, []byte(kfbSettingsKey))), nil
}
func openKFBSettings(secret, encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte("sbis:kfb:connection:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted connection settings")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(kfbSettingsKey))
}

func (a *App) reloadKFBSettings(ctx context.Context) error {
	if a.KFB == nil || a.DB == nil {
		return nil
	}
	a.KFBSettings.mu.Lock()
	defer a.KFBSettings.mu.Unlock()
	var encrypted string
	err := a.DB.QueryRow(ctx, `SELECT value_json->>'encrypted' FROM system_settings WHERE key=$1`, kfbSettingsKey).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if encrypted == a.KFBSettings.encrypted {
		return nil
	}
	data, err := openKFBSettings(a.Cfg.JWTSecret, encrypted)
	if err != nil {
		return fmt.Errorf("cannot decrypt KFB settings")
	}
	var settings kfb.ConnectionSettings
	if err = json.Unmarshal(data, &settings); err != nil {
		return err
	}
	a.KFB.ApplyConnection(settings)
	a.KFBStreamConnected.Store(false)
	a.KFBStreamError.Store(nil)
	a.KFBSettings.encrypted = encrypted
	if a.Redis != nil {
		_ = a.Redis.Del(ctx, kfbPositionsKey, kfbQuotesKey).Err()
	}
	return nil
}

func (a *App) InitKFBSettings(ctx context.Context) error {
	if err := a.reloadKFBSettings(ctx); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := a.reloadKFBSettings(check)
				cancel()
				if err != nil && a.Log != nil {
					a.Log.Warn("KFB connection settings could not be loaded")
				}
			}
		}
	}()
	return nil
}

func certificateInfo(raw string) any {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	fingerprint := sha256.Sum256(cert.Raw)
	return map[string]any{"name": cert.Subject.CommonName, "expires_at": cert.NotAfter, "fingerprint": hex.EncodeToString(fingerprint[:])}
}

func (a *App) handleKFBSettings(w http.ResponseWriter, r *http.Request) {
	p, ok := httpx.PrincipalFrom(r.Context())
	if !ok || !p.HasRole("SUPER_ADMIN") {
		httpx.WriteError(w, r, httpx.ErrForbidden)
		return
	}
	if a.KFB == nil {
		httpx.WriteError(w, r, httpx.E(503, "KFB_UNAVAILABLE", "Подключение к бирже недоступно"))
		return
	}
	if err := a.reloadKFBSettings(r.Context()); err != nil {
		httpx.WriteError(w, r, httpx.E(500, "SETTINGS_UNAVAILABLE", "Не удалось загрузить настройки подключения"))
		return
	}
	settings := a.KFB.ConnectionSettings()
	profiles, err := a.loadKFBProfiles(r.Context(), settings)
	if err != nil {
		httpx.WriteError(w, r, httpx.E(500, "SETTINGS_UNAVAILABLE", "Не удалось загрузить профили серверов"))
		return
	}
	if r.Method == http.MethodPut {
		var input kfb.ConnectionSettings
		r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
		if err := decode(r, &input); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		input, err = updateKFBProfile(input, settings, profiles)
		if err != nil {
			httpx.WriteError(w, r, httpx.E(422, "INVALID_CONNECTION", err.Error()))
			return
		}
		previous := profiles[input.Environment]
		profiles[input.Environment] = input
		data, err := json.Marshal(kfbSettingsDocument{ConnectionSettings: input, Profiles: profiles})
		if err != nil {
			httpx.WriteError(w, r, httpx.E(500, "INTERNAL_ERROR", "Не удалось обработать настройки"))
			return
		}
		encrypted, err := sealKFBSettings(a.Cfg.JWTSecret, data)
		if err != nil {
			httpx.WriteError(w, r, httpx.E(500, "INTERNAL_ERROR", "Не удалось обработать настройки"))
			return
		}
		payload, _ := json.Marshal(map[string]string{"encrypted": encrypted})
		_, err = a.DB.Exec(r.Context(), `INSERT INTO system_settings(key,value_json,updated_by) VALUES($1,$2,$3) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json,updated_by=EXCLUDED.updated_by,updated_at=now()`, kfbSettingsKey, payload, p.UserID)
		if err != nil {
			httpx.WriteError(w, r, httpx.E(500, "SAVE_FAILED", "Не удалось сохранить настройки"))
			return
		}
		if err = a.reloadKFBSettings(r.Context()); err != nil {
			httpx.WriteError(w, r, httpx.E(500, "APPLY_FAILED", "Настройки сохранены, но не удалось их применить"))
			return
		}
		a.Audit.Write(r.Context(), "KFB_CONNECTION_CHANGED", "integration", nil, nil, map[string]any{"environment": input.Environment, "previous_environment": connectionEnvironment(settings), "rest_url": input.RESTURL, "grpc_target": input.GRPCTarget, "certificates_uploaded": input.CAPEM != previous.CAPEM})
		settings = input
	}
	streamError := ""
	if value := a.KFBStreamError.Load(); value != nil {
		streamError = *value
	}
	response := publicKFBProfile(settings)
	publicProfiles := map[string]any{}
	for name, profile := range profiles {
		publicProfiles[name] = publicKFBProfile(profile)
	}
	response["profiles"] = publicProfiles
	response["grpc_connected"] = a.KFBStreamConnected.Load()
	response["grpc_error"] = streamError
	httpx.WriteJSON(w, 200, response)
}

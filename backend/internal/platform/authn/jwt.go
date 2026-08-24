package authn

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	SessionID string   `json:"sid"`
	BrokerID  string   `json:"broker_id"`
	Roles     []string `json:"roles"`
	jwt.RegisteredClaims
}

func IssueAccess(secret string, userID, sessionID, brokerID uuid.UUID, roles []string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		SessionID: sessionID.String(),
		BrokerID:  brokerID.String(),
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
	return t.SignedString([]byte(secret))
}

func ParseAccess(secret, token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return c, nil
}

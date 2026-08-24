package id

import "github.com/google/uuid"

func New() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return u
}

func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

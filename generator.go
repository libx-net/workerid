package workerid

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

type Generator interface {
	// GetID 获取worker ID,返回 worker ID 和 token
	GetID() (int64, string, error)
	// Renew 续期 worker ID
	Renew(workerID int64, token string) error
	// Release 主动释放 worker ID
	Release(workerID int64, token string) error
}

var (
	ErrNoAvailableID   = errors.New("no available worker IDs")
	ErrInvalidWorkerID = errors.New("invalid worker ID")
	ErrTokenMismatch   = errors.New("token mismatch")
	ErrTokenExpired    = errors.New("token expired")
	ErrNotAssigned     = errors.New("worker ID not assigned")
	ErrInvalidToken    = errors.New("invalid token format")
)

func generateToken() string {
	tokenBytes := make([]byte, 16)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(tokenBytes)
}

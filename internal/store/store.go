package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Common errors
var (
	ErrNotFound = errors.New("record not found")
)

// ServiceRecord represents a service health record
type ServiceRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler
func (r *ServiceRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}{
		ID:        r.ID,
		Name:      r.Name,
		Status:    r.Status,
		Timestamp: r.Timestamp,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ServiceRecord) UnmarshalJSON(data []byte) error {
	var v struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	r.ID = v.ID
	r.Name = v.Name
	r.Status = v.Status
	r.Timestamp = v.Timestamp
	return nil
}

// Store defines the interface for service record storage
type Store interface {
	TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool
	ReleaseLock(ctx context.Context, service, domain, clientId string)
	KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration
	Close()
	// GetConfig returns the current store configuration
	GetConfig() StoreConfig
}

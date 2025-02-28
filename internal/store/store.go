package store

import (
	"context"
	"encoding/json"
	"errors"
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
	// Get retrieves a service record by ID
	Get(ctx context.Context, id string) (*ServiceRecord, error)

	// Put stores a service record
	Put(ctx context.Context, record *ServiceRecord) error

	// Delete removes a service record by ID
	Delete(ctx context.Context, id string) error

	// Close closes the store connection
	Close() error
}

// internal/store/store_test.go

package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestServiceRecordMarshalJSON(t *testing.T) {
	record := &ServiceRecord{
		ID:        "test-id",
		Name:      "test-service",
		Status:    "active",
		Timestamp: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Failed to marshal ServiceRecord: %v", err)
	}

	var unmarshaled ServiceRecord
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServiceRecord: %v", err)
	}

	if unmarshaled.ID != record.ID ||
		unmarshaled.Name != record.Name ||
		unmarshaled.Status != record.Status ||
		unmarshaled.Timestamp != record.Timestamp {
		t.Errorf("Unmarshaled record doesn't match original: %+v vs %+v", unmarshaled, record)
	}
}

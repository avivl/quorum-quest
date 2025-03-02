// internal/observability/metrics_test.go
package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestAttributesFromTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected int // Number of expected attributes
	}{
		{
			name:     "Empty tags",
			tags:     []string{},
			expected: 0,
		},
		{
			name:     "Even number of tags",
			tags:     []string{"key1", "value1", "key2", "value2"},
			expected: 2,
		},
		{
			name:     "Odd number of tags",
			tags:     []string{"key1", "value1", "key2", "value2", "orphan"},
			expected: 2, // The odd one should be ignored
		},
		{
			name:     "Single pair",
			tags:     []string{"key", "value"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := attributesFromTags(tt.tags)
			assert.Equal(t, tt.expected, len(attributes), "Number of attributes should match expected")

			// Verify key-value pairs
			for i := 0; i < len(tt.tags)/2 && i*2+1 < len(tt.tags); i++ {
				assert.Equal(t, tt.tags[i*2], string(attributes[i].Key))
				assert.Equal(t, tt.tags[i*2+1], attributes[i].Value.AsString())
			}
		})
	}
}

// TestMetricsInterface checks that our struct implements the interface
func TestMetricsInterface(t *testing.T) {
	// Compile-time check that OTelMetrics implements MetricsClient
	var _ MetricsClient = (*OTelMetrics)(nil)
}

// Custom mock for the MetricsClient interface for testing
type MockMetricsClient struct {
	countCalls     int
	latencyCalls   int
	lastCountName  string
	lastCountValue int64
	lastTags       []string
}

func (m *MockMetricsClient) Increment(ctx context.Context, name string, value int64, attributes ...string) {
	m.countCalls++
	m.lastCountName = name
	m.lastCountValue = value
	m.lastTags = attributes
}

func (m *MockMetricsClient) RecordLatency(ctx context.Context, duration time.Duration, tags ...string) error {
	m.latencyCalls++
	m.lastTags = tags
	return nil
}

func TestAttributeCreation(t *testing.T) {
	// Test creating attributes directly
	attrs := []attribute.KeyValue{
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
		attribute.Bool("key3", true),
	}

	assert.Equal(t, 3, len(attrs))
	assert.Equal(t, "key1", string(attrs[0].Key))
	assert.Equal(t, "value1", attrs[0].Value.AsString())
	assert.Equal(t, "key2", string(attrs[1].Key))
	assert.Equal(t, int64(42), attrs[1].Value.AsInt64())
	assert.Equal(t, "key3", string(attrs[2].Key))
	assert.Equal(t, true, attrs[2].Value.AsBool())
}

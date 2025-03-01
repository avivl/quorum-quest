// internal/store/scylladb/session_test.go
package scylladb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInitSessionQueries tests that the correct query strings are initialized
func TestInitSessionQueries(t *testing.T) {
	store, _ := SetupStoreWithMocks()

	// Reset query strings
	store.TryAcquireLockQuery = ""
	store.ValidateLockQuery = ""
	store.ReleaseLockQuery = ""

	// Call initSession (mock version doesn't actually execute queries)
	store.initSession()

	// Verify the query strings are properly formatted
	expectedTableName := "\"test_keyspace\".\"test_table\""
	assert.Contains(t, store.TryAcquireLockQuery, "INSERT INTO "+expectedTableName)
	assert.Contains(t, store.TryAcquireLockQuery, "VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?")
	assert.Contains(t, store.ValidateLockQuery, "SELECT client_id FROM "+expectedTableName)
	assert.Contains(t, store.ValidateLockQuery, "WHERE service =? and domain = ? and client_id =?")
	assert.Contains(t, store.ReleaseLockQuery, "DELETE FROM "+expectedTableName)
	assert.Contains(t, store.ReleaseLockQuery, "WHERE service =? and domain =?")
}

// TestSessionImplementation ensures that all required session methods are implemented
func TestSessionImplementation(t *testing.T) {
	store, _ := SetupStoreWithMocks()

	// Ensure mock has the methods we need
	assert.NotPanics(t, func() {
		store.initSession()
		store.validateKeyspace()
		err := store.validateTable()
		assert.Nil(t, err)
	})
}

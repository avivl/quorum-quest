// internal/store/dynamodb/table_test.go
package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEnsureTableExists(t *testing.T) {
	t.Run("table_already_exists", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()

		// Mock DescribeTable to return success (table exists)
		mockClient.On("DescribeTable", ctx, mock.Anything).Return(&dynamodb.DescribeTableOutput{}, nil)

		// Call the method under test
		err := mockStore.ensureTableExists(ctx)

		// Verify expectations
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
		// Verify that CreateTable was not called
		mockClient.AssertNotCalled(t, "CreateTable")
	})

	t.Run("table_does_not_exist_created_successfully", func(t *testing.T) {
		mockStore, mockClient, mockWaiter := SetupMockStore()
		ctx := context.Background()

		// Mock DescribeTable to return error (table doesn't exist)
		mockClient.On("DescribeTable", ctx, mock.Anything).Return(nil, errors.New("table not found"))

		// Mock CreateTable to return success
		mockClient.On("CreateTable", ctx, mock.Anything).Return(&dynamodb.CreateTableOutput{}, nil)

		// Mock Wait to return success
		mockWaiter.On("Wait", ctx, mock.Anything, mock.Anything).Return(nil)

		// Call the method under test
		err := mockStore.ensureTableExists(ctx)

		// Verify expectations
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
		mockWaiter.AssertExpectations(t)
	})

	t.Run("create_table_fails", func(t *testing.T) {
		mockStore, mockClient, _ := SetupMockStore()
		ctx := context.Background()

		// Mock DescribeTable to return error (table doesn't exist)
		mockClient.On("DescribeTable", ctx, mock.Anything).Return(nil, errors.New("table not found"))

		// Mock CreateTable to return error
		mockClient.On("CreateTable", ctx, mock.Anything).Return(nil, errors.New("unable to create table"))

		// Call the method under test
		err := mockStore.ensureTableExists(ctx)

		// Verify expectations
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create table")
		mockClient.AssertExpectations(t)
	})

	t.Run("table_create_wait_fails", func(t *testing.T) {
		mockStore, mockClient, mockWaiter := SetupMockStore()
		ctx := context.Background()

		// Mock DescribeTable to return error (table doesn't exist)
		mockClient.On("DescribeTable", ctx, mock.Anything).Return(nil, errors.New("table not found"))

		// Mock CreateTable to return success
		mockClient.On("CreateTable", ctx, mock.Anything).Return(&dynamodb.CreateTableOutput{}, nil)

		// Mock Wait to return error
		mockWaiter.On("Wait", ctx, mock.Anything, mock.Anything).Return(errors.New("timed out waiting for table"))

		// Call the method under test
		err := mockStore.ensureTableExists(ctx)

		// Verify expectations
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to wait for table creation")
		mockClient.AssertExpectations(t)
		mockWaiter.AssertExpectations(t)
	})
}

// internal/store/redis/redis_store_test.go
package redis

import (
	"fmt"
)

// Override the client creation in New() to use our replaceable function
func createRedisClient(host string, port int, password string, db int) redisClientMock {
	addr := fmt.Sprintf("%s:%d", host, port)
	return newRedisClientFn(addr, password, db)
}

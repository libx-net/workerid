module libx.net/workerid/adapter/goredis-v8

go 1.22

require (
	github.com/alicebob/miniredis/v2 v2.33.0
	github.com/go-redis/redis/v8 v8.11.5
	libx.net/workerid v0.3.0
)

require (
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)

replace libx.net/workerid => ../../

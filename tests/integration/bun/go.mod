module libx.net/workerid/tests/integration/bun

go 1.22.0

require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/uptrace/bun v1.2.11
	github.com/uptrace/bun/dialect/pgdialect v1.2.11
	libx.net/workerid v0.0.0
	libx.net/workerid/adapter/bun v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace libx.net/workerid => ../../..

replace libx.net/workerid/adapter/bun => ../../../adapter/bun

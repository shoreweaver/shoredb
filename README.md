# ShoreDB

A minimal, dependency-free Redis-protocol server written in Go — usable as a **standalone binary**, a **Docker container**, or an **embedded library** inside another Go program.

It speaks real RESP2 over TCP, so any standard Redis client (`redis-cli`, `go-redis`, `ioredis`, `redis-py`, ...) can talk to it. It also supports optional AOF and RDB-style persistence, so data can survive a restart, and 16 SELECT-able logical databases just like stock Redis.

This is a learning/side-project implementation, not a drop-in Redis replacement — see [Known limitations](#known-limitations) before relying on it for anything production-critical.

## Features

- Real RESP2 wire protocol (works with off-the-shelf Redis clients)
- Strings, Lists, Sets, Hashes, key expiration, Pub/Sub
- 16 logical databases (`SELECT 0`-`15`), each with its own lock so a snapshot or heavy workload on one database doesn't stall the others
- Optional AOF (append-only file) and RDB-style snapshotting, independently toggleable, both correctly scoped per logical database
- Event-driven `BLPOP`/`BRPOP`: blocked clients wake the instant a matching `LPUSH`/`RPUSH` happens, not on a polling timer
- `AUTH` / `requirepass` support
- Usable as a Go library two ways: the zero-network-layer data structures directly (`pkg/commands`), or the higher-level embeddable client (`pkg/shoredis`) with typed methods, `Select`, and optional RDB persistence
- No third-party dependencies — standard library only

## Supported commands

| Category | Commands |
|---|---|
| Strings | `GET` `SET` `DEL` `EXISTS` `MSET` `MGET` `TYPE` `INCR` `DECR` `INCRBY` `DECRBY` |
| Lists | `LPUSH` `RPUSH` `LPOP` `RPOP` `BLPOP` `BRPOP` `LRANGE` `LLEN` `LINDEX` `LSET` `LREM` |
| Sets | `SADD` `SREM` `SISMEMBER` `SMEMBERS` `SCARD` `SPOP` `SINTER` `SUNION` `SDIFF` |
| Hashes | `HSET` `HGET` `HDEL` `HEXISTS` `HGETALL` `HKEYS` `HVALS` `HLEN` `HMSET` `HMGET` `HINCRBY` |
| Expiration | `EXPIRE` `PEXPIRE` `TTL` `PTTL` `PERSIST` |
| Pub/Sub | `PUBLISH` `SUBSCRIBE` `UNSUBSCRIBE` |
| Databases | `SELECT` (0-15) |
| Admin / introspection | `KEYS` `SCAN` `DBSIZE` `HELLO` `COMMAND` (+ `COUNT`/`LIST`/`INFO`/`DOCS`/`GETKEYS`) `AUTH` |
| Persistence | `SAVE` `BGSAVE` `LASTSAVE` |

## Quick start

### Run as a binary

```bash
git clone https://github.com/shoreweaver/shoredb.git
cd shoredb
go build -o shoredb-server ./cmd/shoredb-server
./shoredb-server -port :6379 -config redis.config
```

Or via environment variables instead of flags:

```bash
SHOREDB_PORT=:6379 SHOREDB_CONFIG=redis.config ./shoredb-server
```

Then, from another terminal:

```bash
redis-cli -p 6379 SET hello world
redis-cli -p 6379 GET hello
redis-cli -p 6379 -n 1 SET hello elsewhere   # a different logical database
redis-cli -p 6379 -n 1 GET hello
```

### Run with Docker

```bash
docker compose up --build
```

This builds the image and exposes port `6379`, persisting data to a named Docker volume so it survives container restarts. See [`docker-compose.yml`](./docker-compose.yml) for the exact mount and env var setup, and make sure a `redis.config` is available at the path `SHOREDB_CONFIG` points to inside the container (bake it into the image, or copy it into the volume once).

### Use as an embedded Go library

```go
import "github.com/shoreweaver/shoredb/pkg/shoredis"

client := shoredis.New() // in-memory only, no persistence

client.Set("hello", "world")
value, ok, _ := client.Get("hello")

client.Select(1)            // switch to logical database 1
client.LPush("queue", "a", "b")
client.HGetAll("some-hash")

// Or, with RDB persistence backed by a redis.config file:
client, err := shoredis.Open("redis.config")
defer client.Close() // writes a final snapshot
```

Anything not covered by a typed method is reachable via `client.Do("COMMANDNAME", "arg1", "arg2", ...)`.

## Configuration

`redis.config` is a flat `key value` file, parsed line by line (`#` starts a comment):

| Key | Meaning | Default |
|---|---|---|
| `dir <path>` | Where the RDB/AOF files live | `.` |
| `dbfilename <name>` | RDB snapshot filename | `dump.rdb` |
| `save <secs> <changes>` | Snapshot trigger; repeatable, e.g. `save 900 1` | none |
| `appendonly yes\|no` | Enable the append-only file | `no` |
| `appendfilename <name>` | AOF filename | `appendonly.aof` |
| `appendfsync always\|everysec\|no` | How often the AOF is `fsync`'d | `everysec` |
| `requirepass <password>` | Require `AUTH` before any other command | unset |

If `appendonly yes`, the AOF is the source of truth on startup and takes precedence over any RDB snapshot; if `appendonly no`, the RDB snapshot is loaded instead. Both persist and replay all 16 logical databases, tagging entries with `SELECT` markers the same way real Redis's AOF does, so data ends up back in the database it came from.

## Architecture

All packages live under `pkg/` and can be imported independently by any Go module:

```
pkg/resp          RESP2 parser + writer
pkg/datastruct     List / Set / Hash implementations (thread-safe)
pkg/pubsub         Channel-based pub/sub broker
pkg/persistence    AOF + RDB-style snapshotting, multi-database aware
pkg/commands       Command handlers + the Database/MultiDB types (16 logical databases)
pkg/server         TCP server that wires the above together, with per-connection SELECT
pkg/shoredis        Embeddable in-process client with typed methods and no TCP/RESP overhead
```

`cmd/shoredb-server` is a thin binary wrapper around `pkg/server`.

## Testing

```bash
./shoredb-server -port :6399 -config /tmp/test-redis.config &
redis-cli -p 6399 SET foo bar
redis-cli -p 6399 GET foo
redis-cli -p 6399 SHUTDOWN NOSAVE 2>/dev/null; kill %1
```

Worth testing explicitly if you change persistence code:
- **RDB round-trip**: with `appendonly no`, push data into a list/set/hash on a couple of different `SELECT`ed databases, `SAVE`, restart, confirm each database's data is still there under the right index.
- **AOF durability**: with `appendonly yes` (the default config), `INCR` a counter a few times (optionally after a `SELECT`), restart with a graceful shutdown (`SIGTERM`, not `-9`), confirm the value survived in the correct database.
- **BLPOP/BRPOP latency**: a blocked `BLPOP` should return within roughly a millisecond of a matching `LPUSH`/`RPUSH`, not up to 50ms later.

## Known limitations

- `requirepass` gates commands per-connection via `AUTH`; there's no ACL/user model, and it isn't per-database.
- `SCAN` is a single-pass implementation (cursor is always `0`) — fine for small/medium datasets, not cursor-safe for huge ones.
- RESP3 (`HELLO 3`) isn't implemented; the server returns `NOPROTO`, so RESP3-aware clients fall back to RESP2.
- TTLs aren't persisted in the RDB snapshot and `EXPIRE`/`PEXPIRE`/`PERSIST` aren't written to the AOF, to avoid replaying a stale relative expiry after restart.
- `COMMAND`/`COMMAND DOCS` return hand-maintained metadata for the commands above, not the full introspection detail real Redis provides.
- `pkg/shoredis` wires up RDB persistence but not AOF: an embedded client has no long-running server loop to drive an AOF fsync ticker, so call `Save()` (or `Close()` on the way out) when you want a snapshot.

### License

- [AGPLv3](LICENSE)
---
title: Quick Start
section: Getting Started
order: 2
---

# Quick Start

ShoreDB can be run three ways. Pick whichever fits your workflow — the command set and behavior are identical across all three.

## Run as a binary

```bash
git clone https://github.com/shoreweaver/shoredb.git
cd shoredb
go build -o shoredb-server ./cmd/shoredb-server
./shoredb-server -port :6379 -config redis.config
```

Or configure it with environment variables instead of flags:

```bash
SHOREDB_PORT=:6379 SHOREDB_CONFIG=redis.config ./shoredb-server
```

Then, from another terminal, talk to it with any Redis client:

```bash
redis-cli -p 6379 SET hello world
redis-cli -p 6379 GET hello
redis-cli -p 6379 -n 1 SET hello elsewhere   # a different logical database
redis-cli -p 6379 -n 1 GET hello
```

### Flags and environment variables

| Flag | Env var | Meaning | Default |
|---|---|---|---|
| `-port` | `SHOREDB_PORT` | Address to listen on | `:6379` |
| `-config` | `SHOREDB_CONFIG` | Path to a `redis.config` file | `redis.config` |

Flags take priority over environment variables, which take priority over the built-in defaults.

## Run with Docker

```bash
docker compose up --build
```

This builds the image and exposes port `6379`, persisting data to a named Docker volume so it survives container restarts. See `docker-compose.yml` for the exact mount and env var setup, and make sure a `redis.config` is available at the path `SHOREDB_CONFIG` points to inside the container — bake it into the image, or copy it into the volume once.

```yaml
services:
  shoredb:
    build: .
    ports:
      - "6379:6379"
    volumes:
      - shoredb-data:/data
    environment:
      - SHOREDB_PORT=:6379
      - SHOREDB_CONFIG=/data/redis.config

volumes:
  shoredb-data:
```

## Use as an embedded Go library

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

See [Embedding ShoreDB](embedding.html) for the full typed API and the `Do` escape hatch for less-common commands.

## Graceful shutdown

Sending `SIGINT` or `SIGTERM` (e.g. `Ctrl-C`, or `kill` without `-9`) triggers a clean shutdown: the AOF is closed and a final RDB snapshot is written before the process exits. Killing the process with `-9` skips this, so prefer a graceful stop whenever you need the on-disk state to be current.

## Next steps

- [Configuration](configuration.html) to set up persistence and `requirepass`
- [Commands](commands-strings.html) for the full reference

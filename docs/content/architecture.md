---
title: Architecture
section: Guides
order: 1
---

# Architecture

All packages live under `pkg/` and can be imported independently by any Go module — nothing outside `cmd/shoredb-server` depends on the TCP layer.

```
pkg/resp          RESP2 parser + writer
pkg/datastruct    List / Set / Hash implementations (thread-safe)
pkg/pubsub        Channel-based pub/sub broker
pkg/persistence   AOF + RDB-style snapshotting, multi-database aware
pkg/commands      Command handlers + the Database/MultiDB types (16 logical databases)
pkg/server        TCP server that wires the above together, with per-connection SELECT
pkg/shoredis       Embeddable in-process client with typed methods and no TCP/RESP overhead
```

`cmd/shoredb-server` is a thin binary wrapper around `pkg/server`.

## Request flow (TCP server)

1. `pkg/server.Start` accepts a connection and hands it to `handleConnection` on its own goroutine.
2. Each connection parses RESP values off the wire with `pkg/resp.Parser` and tracks its **own** `dbIndex` — `SELECT` only ever affects that one connection.
3. `AUTH` and `SELECT` are handled inline; everything else is dispatched through `processCommand`, which looks the command up in `commands.CommandHandlers` (or `commands.RDBCommandHandlers` for `SAVE`/`BGSAVE`/`LASTSAVE`, which additionally need the `*persistence.RDB` handle).
4. The handler runs **before** anything is persisted: a command that fails argument validation or otherwise errors is never written to the AOF, so replay never sees a bad entry.
5. On a successful, mutating command, the original RESP value is appended to the AOF (tagged with the connection's `dbIndex`) and the RDB change counter is incremented.

## Logical databases

`commands.MultiDB` holds `NumDatabases` (16) `*Database` values, each with its **own mutex**, plus one `*pubsub.PubSub` broker shared across all of them. This split matters for two reasons:

- A snapshot walk or heavy workload on one logical database never blocks operations on another — earlier revisions used a single lock across every database, which meant an RDB save on database 3 stalled traffic on database 0 too.
- Pub/Sub is deliberately **not** duplicated per database: `PUBLISH`/`SUBSCRIBE` are global in real Redis, and ShoreDB matches that by sharing one broker across the whole `MultiDB`.

## Blocking list pops

`BLPOP`/`BRPOP` used to busy-wait with a fixed `time.Sleep` loop, so a pop sitting ready could go unnoticed for up to the sleep interval. Each `Database` now owns a `sync.Cond` (`listCond`) that every `LPUSH`/`RPUSH` broadcasts on. A blocked popper parks on that condition variable and wakes immediately when data arrives — or, if a timeout was given, a `time.AfterFunc` broadcasts once the deadline passes so the popper can give up cleanly.

## Persistence: AOF vs RDB

`pkg/persistence` avoids depending on `pkg/commands` directly (which would create an import cycle), and instead defines small structural interfaces — `Database` and `MultiDatabase` — that `commands.Database`/`commands.MultiDB` satisfy without ever importing this package. The same interface-boxing pattern is used for the `GetOrCreate{List,Set,Hash}Iface` methods, so RDB load/save code can get-or-create containers without a concrete-type dependency.

- **AOF** (`pkg/persistence/aof.go`) appends every successful mutating command to a log file, prefixing a `SELECT` pseudo-entry whenever the target database changes from the previous write. Replaying the log re-applies each command against the right database.
- **RDB** (`pkg/persistence/rdb.go`) walks all 16 databases and writes a compact binary snapshot — one `SELECTDB` marker and a resize hint per non-empty database, followed by its keys. Loading reverses this exactly.

If `appendonly yes`, the AOF is authoritative on startup and takes precedence over any RDB file; otherwise the RDB snapshot is loaded. Both cover all 16 databases.

## Where to go next

- [Configuration](configuration.html) for every `redis.config` key
- [Embedding ShoreDB](embedding.html) to use these packages directly from another Go program
- [Testing](testing.html) for what to exercise manually when touching persistence code

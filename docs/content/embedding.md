---
title: Embedding ShoreDB
section: Guides
order: 2
---

# Embedding ShoreDB

`pkg/shoredis` lets another Go program embed ShoreDB directly, in-process, with no TCP listener and no RESP encoding/decoding overhead.

## Creating a client

```go
import "github.com/shoreweaver/shoredb/pkg/shoredis"

client := shoredis.New() // purely in-memory, no persistence
```

`New` creates a client with `commands.NumDatabases` (16) logical databases and no persistence at all. For RDB-backed persistence, use `Open` with a `redis.config` path instead:

```go
client, err := shoredis.Open("redis.config")
defer client.Close() // writes a final snapshot
```

`Open` loads any existing snapshot at the configured path immediately, and wires up `client.Save()` / `client.Close()` to persist to it. AOF is intentionally **not** available for embedded clients — there's no long-running server loop to drive an fsync ticker — so call `Save()` explicitly (or rely on `Close()` on the way out) whenever you want a snapshot.

## Concurrent goroutines and `Select`

A `Client` is safe for concurrent use by multiple goroutines, with one caveat: `Select` changes which logical database *that* `Client`'s subsequent calls apply to, and that state is shared by whoever holds the same `Client`. If different goroutines need to work against different databases at the same time, give each its own client with `NewFrom`, rather than sharing one and calling `Select` from multiple places:

```go
client := shoredis.New()
worker := client.NewFrom() // shares the same underlying databases + persistence
worker.Select(1)           // only affects worker's own dbIndex
```

## Typed methods

Most commands have a typed convenience method that maps directly onto the RESP command of the same name — for example:

```go
client.Set("hello", "world")
value, ok, err := client.Get("hello")

client.LPush("queue", "a", "b")
items, err := client.LRange("queue", 0, -1)

client.HSet("profile", "name", "ada")
fields, err := client.HGetAll("profile")

client.Expire("session", 3600)
ttl, err := client.TTL("session")
```

Each typed method returns plain Go types (`string`, `int`, `bool`, `[]string`, `map[string]string`) rather than a raw RESP value, and surfaces command errors (like `ERR value is not an integer or out of range`) through the returned `error`.

## The `Do` escape hatch

Anything not covered by a typed method is reachable through `Do`, which accepts any supported command name (case-insensitive) with string arguments:

```go
res, err := client.Do("SISMEMBER", "tags", "go")
if err == nil && res.Err() == nil {
    isMember := res.Int() == 1
}
```

`Do` returns a `Result` wrapping the raw RESP reply, with accessors:

| Method | Use for |
|---|---|
| `Err()` | non-nil if the reply was a RESP error |
| `IsNil()` | whether the reply was a RESP null |
| `String()` | a bulk or simple string reply |
| `Int()` | an integer reply |
| `Strings()` | an array-of-bulk-strings reply (`LRANGE`, `SMEMBERS`, `KEYS`, ...) |

## Persistence from an embedded client

```go
client, err := shoredis.Open("redis.config")
if err != nil {
    // handle: e.g. bad config path
}
defer client.Close()

// ... use client ...

if err := client.Save(); err != nil {
    // only returns an error if the client wasn't created with Open
}
```

`Save()` and `Close()` both return an error if called on a client created with `New()`, since there's no persistence configured to write to.

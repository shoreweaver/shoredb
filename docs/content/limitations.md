---
title: Known Limitations
section: Reference
order: 1
---

# Known Limitations

ShoreDB is a learning / side-project implementation, not a drop-in Redis replacement. Read this before relying on it for anything production-critical.

## Access control

`requirepass` gates commands per-connection via `AUTH`. There is no ACL or per-user model, and the password is not scoped per logical database — one password guards the entire server.

## `SCAN` is single-pass

`SCAN` always returns cursor `0` — a real cursor-based incremental scan isn't implemented. This is fine for small or medium datasets, but it is **not** cursor-safe for huge ones: a single `SCAN` call walks (and returns) the entire matching key set at once. See [`SCAN`](commands-admin.html).

## No RESP3

`HELLO 3` isn't implemented — the server replies `NOPROTO unsupported protocol version`, so RESP3-aware clients fall back to RESP2 automatically. Everything else works over RESP2 as normal.

## TTLs don't survive a restart

TTLs are **not** persisted in the RDB snapshot, and `EXPIRE`/`PEXPIRE`/`PERSIST` are **not** written to the AOF. This is deliberate: replaying an absolute wall-clock deadline from a relative TTL recorded before a restart would either expire keys immediately or keep them alive far longer than intended, depending on how long the server was down. If you depend on TTLs surviving a restart, re-apply them explicitly after loading.

## `COMMAND` metadata is hand-maintained

`COMMAND` / `COMMAND DOCS` return a hand-maintained table of arity, flags, and key positions for the commands ShoreDB actually supports — not the full introspection detail real Redis provides (argument-level docs, since-version, grouping, etc.). Clients that only need arity/flags to validate their own calls work fine; clients expecting rich `COMMAND DOCS` output will see an empty per-command doc map.

## No AOF for embedded clients

`pkg/shoredis` wires up RDB persistence but not AOF. An embedded client has no long-running server loop to drive an AOF fsync ticker, so call `client.Save()` (or `client.Close()` on the way out) explicitly when you want a snapshot. See [Embedding ShoreDB](embedding.html).

## Not covered at all

- Replication, clustering, or any multi-node topology
- Redis Streams, Sorted Sets, HyperLogLog, Bitmaps, or Geo commands
- Lua scripting (`EVAL`/`EVALSHA`), transactions (`MULTI`/`EXEC`), or keyspace notifications
- TLS on the TCP listener

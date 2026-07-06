---
title: Introduction
section: Getting Started
order: 1
tagline: A minimal, dependency-free Redis-protocol server written in Go.
---

# ShoreDB

ShoreDB is a minimal, dependency-free **Redis-protocol** server written in Go. Run it as a standalone binary, a Docker container, or an embedded library inside another Go program — the same command set works everywhere.

It speaks real RESP2 over TCP, so any standard Redis client (`redis-cli`, `go-redis`, `ioredis`, `redis-py`, ...) can talk to it without modification. Optional AOF and RDB-style persistence let data survive a restart, and 16 SELECT-able logical databases behave just like stock Redis.

> This is a learning / side-project implementation, not a drop-in Redis replacement. See [Known Limitations](limitations.html) before relying on it for anything production-critical.

## Features

- Real RESP2 wire protocol — works with off-the-shelf Redis clients
- Strings, Lists, Sets, Hashes, key expiration, Pub/Sub
- 16 logical databases (`SELECT 0`–`15`), each with its own lock so a snapshot or heavy workload on one database doesn't stall the others
- Optional AOF (append-only file) and RDB-style snapshotting, independently toggleable, both correctly scoped per logical database
- Event-driven `BLPOP` / `BRPOP` — blocked clients wake the instant a matching `LPUSH` / `RPUSH` happens, not on a polling timer
- `AUTH` / `requirepass` support
- Usable as a Go library two ways: the zero-network-layer data structures directly (`pkg/commands`), or the higher-level embeddable client (`pkg/shoredis`) with typed methods, `Select`, and optional RDB persistence
- No third-party dependencies — standard library only

## Where to go next

- [Quick Start](quickstart.html) — install, run, and issue your first commands
- [Configuration](configuration.html) — every `redis.config` key explained
- [Commands](commands-strings.html) — the full reference, grouped by data type
- [Architecture](architecture.html) — how the packages fit together
- [Embedding ShoreDB](embedding.html) — use it as an in-process Go library
- [Known Limitations](limitations.html) — what ShoreDB intentionally doesn't do

## Supported commands at a glance

| Category | Commands |
|---|---|
| Strings | `GET` `SET` `DEL` `EXISTS` `MSET` `MGET` `TYPE` `INCR` `DECR` `INCRBY` `DECRBY` |
| Lists | `LPUSH` `RPUSH` `LPOP` `RPOP` `BLPOP` `BRPOP` `LRANGE` `LLEN` `LINDEX` `LSET` `LREM` |
| Sets | `SADD` `SREM` `SISMEMBER` `SMEMBERS` `SCARD` `SPOP` `SINTER` `SUNION` `SDIFF` |
| Hashes | `HSET` `HGET` `HDEL` `HEXISTS` `HGETALL` `HKEYS` `HVALS` `HLEN` `HMSET` `HMGET` `HINCRBY` |
| Expiration | `EXPIRE` `PEXPIRE` `TTL` `PTTL` `PERSIST` |
| Pub/Sub | `PUBLISH` `SUBSCRIBE` `UNSUBSCRIBE` |
| Databases | `SELECT` (0–15) |
| Admin | `KEYS` `SCAN` `DBSIZE` `HELLO` `COMMAND` `AUTH` |
| Persistence | `SAVE` `BGSAVE` `LASTSAVE` |

---
title: Testing
section: Guides
order: 3
---

# Testing

## Manual smoke test

```bash
./shoredb-server -port :6399 -config /tmp/test-redis.config &
redis-cli -p 6399 SET foo bar
redis-cli -p 6399 GET foo
redis-cli -p 6399 SHUTDOWN NOSAVE 2>/dev/null; kill %1
```

## What to test explicitly when touching persistence code

**RDB round-trip.** With `appendonly no`, push data into a list, a set, and a hash on a couple of different `SELECT`ed databases, then `SAVE`, restart the server, and confirm each database's data is still there under the right index.

```bash
redis-cli -p 6399 -n 0 RPUSH queue a b c
redis-cli -p 6399 -n 2 SADD tags go redis
redis-cli -p 6399 SAVE
# restart the server, then:
redis-cli -p 6399 -n 0 LRANGE queue 0 -1
redis-cli -p 6399 -n 2 SMEMBERS tags
```

**AOF durability.** With `appendonly yes` (the default `redis.config`), `INCR` a counter a few times — optionally after a `SELECT` — then restart with a **graceful** shutdown (`SIGTERM`, not `-9`), and confirm the value survived in the correct database.

```bash
redis-cli -p 6399 -n 1 INCR visits
redis-cli -p 6399 -n 1 INCR visits
kill -TERM %1   # graceful; a -9 kill skips the final flush
# restart the server, then:
redis-cli -p 6399 -n 1 GET visits
```

**`BLPOP`/`BRPOP` latency.** A blocked `BLPOP` should return within roughly a millisecond of a matching `LPUSH`/`RPUSH`, not up to 50ms later (the old busy-wait interval). Time it from two terminals:

```bash
# terminal 1
time redis-cli -p 6399 BLPOP jobs 5

# terminal 2, started right after
redis-cli -p 6399 LPUSH jobs "a job"
```

## Why these three

These three areas are exactly where the surrounding logic is easiest to get subtly wrong: per-database locking in the RDB writer, `SELECT`-tagging in the AOF, and the switch from polling to a condition variable for blocking pops. See [Architecture](architecture.html) for how each of those actually works.

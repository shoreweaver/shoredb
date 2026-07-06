---
title: Configuration
section: Getting Started
order: 3
---

# Configuration

`redis.config` is a flat `key value` file, parsed line by line. Lines starting with `#` are comments and blank lines are ignored.

```
# dir <path>              - where dump.rdb / appendonly.aof are stored
# dbfilename <name>        - RDB snapshot filename
# save <secs> <changes>    - snapshot trigger, repeatable
# appendonly yes|no        - enable AOF
# appendfilename <name>
# appendfsync always|everysec|no
# requirepass <password>   - require AUTH before other commands

dir data
dbfilename dump.rdb
save 900 1
save 300 10
appendonly yes
appendfilename appendonly.aof
appendfsync everysec
```

## Keys

| Key | Meaning | Default |
|---|---|---|
| `dir <path>` | Where the RDB/AOF files live | `.` |
| `dbfilename <name>` | RDB snapshot filename | `dump.rdb` |
| `save <secs> <changes>` | Snapshot trigger; repeatable, e.g. `save 900 1` | none |
| `appendonly yes\|no` | Enable the append-only file | `no` |
| `appendfilename <name>` | AOF filename | `appendonly.aof` |
| `appendfsync always\|everysec\|no` | How often the AOF is `fsync`'d | `everysec` |
| `requirepass <password>` | Require `AUTH` before any other command | unset |

If the file passed to `-config` / `SHOREDB_CONFIG` doesn't exist, ShoreDB logs a warning and falls back to the defaults above rather than failing to start.

## Which persistence mode loads on startup

- If `appendonly yes`: the **AOF is the source of truth**. It takes precedence over any RDB snapshot on disk.
- If `appendonly no`: the **RDB snapshot is loaded** instead.

Both modes persist and replay all 16 logical databases, tagging entries with `SELECT` markers the same way real Redis's AOF does, so data ends up back in the database it came from.

## `save` — RDB snapshot triggers

Each `save <secs> <changes>` line defines one trigger: if at least `<changes>` keys have changed and at least `<secs>` seconds have elapsed since the last snapshot, ShoreDB saves automatically. Multiple `save` lines are checked independently — any one of them firing triggers a save. With the example config above:

- `save 900 1` — save if at least 1 key changed in the last 900 seconds
- `save 300 10` — save if at least 10 keys changed in the last 300 seconds

An explicit `SAVE` or `BGSAVE` command always saves immediately regardless of these triggers. See [Persistence commands](commands-persistence.html).

## `appendfsync` modes

| Mode | Behavior |
|---|---|
| `always` | fsync roughly every 100ms |
| `everysec` | fsync roughly once a second (default) |
| `no` | never fsync from a background timer — relies on the OS to flush eventually |

## `requirepass`

When set, every connection must send `AUTH <password>` before any other command is accepted (the server always allows `AUTH` itself through). This is a single shared password per server — there is no per-user ACL model, and it is not scoped per logical database. See [`AUTH`](commands-admin.html#auth) in the admin command reference.

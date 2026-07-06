---
title: Admin & Introspection
section: Commands
order: 7
---

# Admin & Introspection Commands

Housekeeping, discovery, and connection-level commands. Backed by `pkg/commands/admin_commands.go` and `pkg/server/tcp.go`.

## `KEYS`

```
KEYS pattern
```

Returns every key (across all types) matching a glob-style `pattern`, e.g. `KEYS user:*`. Sweeps expired keys first, so results never include stale entries.

## `SCAN`

```
SCAN cursor [MATCH pattern]
```

A simplified, single-pass cursor: the cursor argument is validated but otherwise ignored, and ShoreDB always returns cursor `0` with the full matching result set in one reply. This is fine for small or medium datasets but is **not** cursor-safe for large ones — see [Known Limitations](limitations.html).

## `DBSIZE`

```
DBSIZE
```

Returns the total number of keys in the currently-selected logical database, across all types, after sweeping expired keys.

## `HELLO`

```
HELLO [protover]
```

Negotiates the protocol version. ShoreDB only implements RESP2: calling `HELLO 3` returns `NOPROTO unsupported protocol version` rather than switching protocols, so RESP3-aware clients fall back to RESP2. Called with no argument (or `2`), it returns server info (name, version, mode, role) in the standard array-of-pairs shape.

## `COMMAND`

```
COMMAND [COUNT | LIST | INFO [name ...] | DOCS [name ...] | GETKEYS cmd [arg ...]]
```

Introspection for clients that query command metadata on connect (arity, flags, key positions) to validate their own calls before sending them.

- **`COMMAND`** (no subcommand) or **`COMMAND INFO`** with no names — full metadata for every supported command
- **`COMMAND COUNT`** — the number of supported commands
- **`COMMAND LIST`** — just the command names
- **`COMMAND INFO name [name ...]`** — metadata for specific commands (nil for unknown ones)
- **`COMMAND DOCS [name ...]`** — acknowledges known commands with an empty per-command doc map; use `COMMAND INFO` for arity/flags, since full docs (summary, since, argument descriptions) aren't modeled
- **`COMMAND GETKEYS cmd [arg ...]`** — returns which of the given arguments are key names, based on the command's known key-position spec

## `AUTH` {#auth}

```
AUTH password
```

Authenticates the connection against `requirepass` (see [Configuration](configuration.html#requirepass)). Every other command is rejected with `NOAUTH Authentication required` until this succeeds, if `requirepass` is set. If no `requirepass` is configured, `AUTH` itself replies with an error, since there's nothing to authenticate against.

## `SELECT`

```
SELECT index
```

Switches which of the 16 logical databases (`0`–`15`) the **current connection** operates against. This is per-connection state — one client's `SELECT` never affects any other client. Returns `ERR DB index is out of range` for anything outside `0`–`15`.

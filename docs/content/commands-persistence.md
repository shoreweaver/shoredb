---
title: Persistence
section: Commands
order: 8
---

# Persistence Commands

Manual snapshot control, on top of whatever automatic `save` triggers are configured. Backed by `pkg/commands/string_commands.go` (handlers) and `pkg/persistence` (the RDB engine). See [Configuration](configuration.html) for the config keys that control persistence, and [Architecture](architecture.html) for how snapshotting is scoped per logical database.

These commands require an RDB handler to be configured; if none is available, each replies `ERR RDB not configured`.

## `SAVE`

```
SAVE
```

Performs an RDB snapshot **synchronously**, blocking until the write completes. Replies `OK` on success.

## `BGSAVE`

```
BGSAVE
```

Kicks off the same snapshot in a background goroutine and returns immediately with `Background saving started`, without waiting for it to finish.

## `LASTSAVE`

```
LASTSAVE
```

Returns the Unix timestamp of the most recent successful snapshot.

## How a snapshot is taken

Each of the 16 logical databases is walked and written independently, holding only that database's own read lock for the duration of its own walk. A snapshot of database 3 does not stall reads or writes against database 0. The snapshot is written to a temporary file first and only `rename`d into place after a successful flush and `fsync`, so a crash mid-save can't corrupt the existing on-disk snapshot.

Note that **TTLs are not included** in the snapshot — see [Known Limitations](limitations.html).

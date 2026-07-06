---
title: Expiration
section: Commands
order: 5
---

# Expiration Commands

Commands for setting and inspecting key TTLs. Backed by `pkg/commands/expire_commands.go`. Expiration applies to keys of any type — string, list, set, or hash.

Keys are evicted lazily (checked whenever they're accessed) **and** actively: a background sweep runs on a 200ms tick, so an expired key disappears close to on time even if nothing touches it in the meantime.

## `EXPIRE`

```
EXPIRE key seconds
```

Sets `key` to expire after `seconds`. Returns `1` if the key exists and the TTL was set, `0` if the key doesn't exist.

## `PEXPIRE`

```
PEXPIRE key milliseconds
```

The same as `EXPIRE`, but with millisecond precision.

## `TTL`

```
TTL key
```

Returns the remaining time to live in seconds:

- a positive number — seconds remaining
- `-1` — the key exists but has no TTL set
- `-2` — the key doesn't exist

## `PTTL`

```
PTTL key
```

The same as `TTL`, but in milliseconds.

## `PERSIST`

```
PERSIST key
```

Removes an existing TTL from `key`, making it persistent again. Returns `1` if a TTL was removed, `0` if the key had none (or doesn't exist).

## Notes

- `SET` and `MSET` clear any existing TTL on the keys they write.
- TTLs are **not** written to the RDB snapshot, and `EXPIRE`/`PEXPIRE`/`PERSIST` are **not** logged to the AOF — see [Known Limitations](limitations.html) for why.

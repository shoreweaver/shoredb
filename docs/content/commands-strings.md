---
title: Strings
section: Commands
order: 1
---

# String Commands

Commands for plain string values and integer counters. Backed by `pkg/commands/string_commands.go`.

## `GET`

```
GET key
```

Returns the value of `key`, or a nil reply if it doesn't exist (or has expired).

## `SET`

```
SET key value
```

Sets `key` to `value`, clearing any existing TTL on that key. Always replies `OK`.

## `DEL`

```
DEL key [key ...]
```

Deletes one or more keys, of any type (string, list, set, or hash). Returns the number of keys actually removed.

## `EXISTS`

```
EXISTS key [key ...]
```

Returns how many of the given keys currently exist, counting duplicates if the same key is passed more than once.

## `MSET`

```
MSET key value [key value ...]
```

Sets multiple keys to multiple values in one call, clearing any existing TTL on each key touched. Always replies `OK`.

## `MGET`

```
MGET key [key ...]
```

Returns the values for the given keys in the same order they were requested. Missing keys come back as nil in their position.

## `TYPE`

```
TYPE key
```

Returns the type of the value stored at `key`: `string`, `list`, `set`, `hash`, or `none` if the key doesn't exist.

## `INCR`

```
INCR key
```

Increments the integer value stored at `key` by 1. If the key doesn't exist it's treated as `0` first. Errors if the existing value isn't a valid integer.

## `DECR`

```
DECR key
```

Decrements the integer value stored at `key` by 1, with the same missing-key and non-integer behavior as `INCR`.

## `INCRBY`

```
INCRBY key increment
```

Increments the integer value at `key` by `increment` (which may be negative).

## `DECRBY`

```
DECRBY key decrement
```

Decrements the integer value at `key` by `decrement` (which may be negative).

---

All six counter commands (`INCR`, `DECR`, `INCRBY`, `DECRBY`) share one code path, so their error message and missing-key behavior are identical: a missing key starts from `0`, and a non-numeric existing value returns `ERR value is not an integer or out of range`.

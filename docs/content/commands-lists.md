---
title: Lists
section: Commands
order: 2
---

# List Commands

Commands for ordered, string-valued lists. Backed by `pkg/commands/list_commands.go` and `pkg/datastruct`.

## `LPUSH`

```
LPUSH key value [value ...]
```

Pushes one or more values onto the head (left side) of the list at `key`, creating the list if needed. Returns the resulting length. Wakes any client blocked in `BLPOP`/`BRPOP` on this key.

## `RPUSH`

```
RPUSH key value [value ...]
```

Pushes one or more values onto the tail (right side) of the list at `key`, creating the list if needed. Returns the resulting length. Wakes any client blocked in `BLPOP`/`BRPOP` on this key.

## `LPOP`

```
LPOP key
```

Removes and returns the first element of the list, or nil if the list doesn't exist or is empty. The key is deleted entirely once its last element is popped.

## `RPOP`

```
RPOP key
```

Removes and returns the last element of the list, with the same empty/missing behavior as `LPOP`.

## `BLPOP`

```
BLPOP key [key ...] timeout
```

Blocking left-pop across one or more keys. Checks each key in order and pops from the first one that has data; if none do, it waits until a push arrives on any of them or `timeout` seconds elapse (`0` waits indefinitely). Returns a two-element array of `[key, value]`, or nil on timeout.

> Blocked clients are woken by a condition variable that every `LPUSH`/`RPUSH` broadcasts on, so a pop is noticed within about a millisecond of the corresponding push — not on a polling interval.

## `BRPOP`

```
BRPOP key [key ...] timeout
```

The same as `BLPOP`, but pops from the tail of whichever key has data first.

## `LRANGE`

```
LRANGE key start stop
```

Returns the elements between `start` and `stop`, inclusive. Negative indices count from the end of the list (`-1` is the last element). Returns an empty array if the key doesn't exist.

## `LLEN`

```
LLEN key
```

Returns the length of the list, or `0` if it doesn't exist.

## `LINDEX`

```
LINDEX key index
```

Returns the element at `index` (negative indices count from the end), or nil if the index is out of range or the key doesn't exist.

## `LSET`

```
LSET key index value
```

Overwrites the element at `index` with `value`. Replies `ERR no such key` if the list doesn't exist, or `ERR index out of range` if the index is invalid.

## `LREM`

```
LREM key count value
```

Removes occurrences of `value` from the list, direction and quantity controlled by `count`:

- `count > 0` — remove up to `count` occurrences, searching head to tail
- `count < 0` — remove up to `abs(count)` occurrences, searching tail to head
- `count = 0` — remove all occurrences

Returns the number of elements removed. The key is deleted entirely if the list becomes empty.

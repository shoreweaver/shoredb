---
title: Sets
section: Commands
order: 3
---

# Set Commands

Commands for unordered collections of unique strings. Backed by `pkg/commands/set_commands.go` and `pkg/datastruct`.

## `SADD`

```
SADD key member [member ...]
```

Adds one or more members to the set at `key`, creating it if needed. Returns the number of members actually added (duplicates already present don't count).

## `SREM`

```
SREM key member [member ...]
```

Removes one or more members from the set. Returns the number actually removed. The key is deleted entirely once the set becomes empty.

## `SISMEMBER`

```
SISMEMBER key member
```

Returns `1` if `member` is in the set, `0` otherwise (including when the key doesn't exist).

## `SMEMBERS`

```
SMEMBERS key
```

Returns all members of the set in unspecified order, or an empty array if the key doesn't exist.

## `SCARD`

```
SCARD key
```

Returns the number of members in the set, or `0` if it doesn't exist.

## `SPOP`

```
SPOP key
```

Removes and returns one random member from the set, or nil if it's empty or missing. The key is deleted entirely once the set becomes empty.

## `SINTER`

```
SINTER key [key ...]
```

Returns the intersection of all given sets — members present in every one. Returns an empty array immediately if any key is missing.

## `SUNION`

```
SUNION key [key ...]
```

Returns the union of all given sets. Missing keys are simply treated as empty sets rather than short-circuiting the result.

## `SDIFF`

```
SDIFF key [key ...]
```

Returns the members of the first set that are **not** present in any of the subsequent sets. Returns an empty array immediately if the first key is missing; later missing keys are treated as empty.

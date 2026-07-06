---
title: Hashes
section: Commands
order: 4
---

# Hash Commands

Commands for field/value maps stored under a single key. Backed by `pkg/commands/hash_commands.go` and `pkg/datastruct`.

## `HSET`

```
HSET key field value
```

Sets `field` in the hash at `key` to `value`, creating the hash if needed. Returns `1` if the field is new, `0` if it already existed and was overwritten.

## `HGET`

```
HGET key field
```

Returns the value of `field`, or nil if the field or the key doesn't exist.

## `HDEL`

```
HDEL key field [field ...]
```

Removes one or more fields from the hash. Returns the number actually removed. The key is deleted entirely once the hash becomes empty.

## `HEXISTS`

```
HEXISTS key field
```

Returns `1` if `field` exists in the hash, `0` otherwise.

## `HGETALL`

```
HGETALL key
```

Returns all fields and values as a flat array (`field1 value1 field2 value2 ...`), or an empty array if the key doesn't exist.

## `HKEYS`

```
HKEYS key
```

Returns all field names in the hash, or an empty array if the key doesn't exist.

## `HVALS`

```
HVALS key
```

Returns all values in the hash, or an empty array if the key doesn't exist.

## `HLEN`

```
HLEN key
```

Returns the number of fields in the hash, or `0` if it doesn't exist.

## `HMSET`

```
HMSET key field value [field value ...]
```

Sets multiple field/value pairs in one call, creating the hash if needed. Always replies `OK`.

## `HMGET`

```
HMGET key field [field ...]
```

Returns the values for the given fields in the order requested. Fields that don't exist (or a wholly missing key) come back as nil in their position.

## `HINCRBY`

```
HINCRBY key field increment
```

Increments the integer value of `field` by `increment`, creating the hash and/or field (starting from `0`) if needed. Errors if the existing field value isn't a valid integer.

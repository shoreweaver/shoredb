package commands

import (
	"strconv"

	"github.com/shoreweaver/shoredb/pkg/resp"
)

// HSET key field value
func hset(args []resp.Value, db *Database) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hset' command"}
	}

	key := args[0].Str
	field := args[1].Str
	value := args[2].Str

	hash := db.GetOrCreateHash(key)
	isNew := hash.HSet(field, value)

	if isNew {
		return resp.Value{Type: resp.Integer, Num: 1}
	}
	return resp.Value{Type: resp.Integer, Num: 0}
}

// HGET key field
func hget(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hget' command"}
	}

	key := args[0].Str
	field := args[1].Str

	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Null}
	}

	value, ok := hash.HGet(field)
	if !ok {
		return resp.Value{Type: resp.Null}
	}

	return resp.Value{Type: resp.BulkString, Str: value}
}

// HDEL key field [field ...]
func hdel(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hdel' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	fields := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		fields[i-1] = args[i].Str
	}

	deleted := hash.HDel(fields...)

	if hash.HLen() == 0 {
		db.Mu.Lock()
		delete(db.Hashes, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.Integer, Num: deleted}
}

// HEXISTS key field
func hexists(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hexists' command"}
	}

	key := args[0].Str
	field := args[1].Str

	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	if hash.HExists(field) {
		return resp.Value{Type: resp.Integer, Num: 1}
	}
	return resp.Value{Type: resp.Integer, Num: 0}
}

// HGETALL key
func hgetall(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hgetall' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	fields := hash.HGetAll()
	result := make([]resp.Value, 0, len(fields)*2)
	for field, value := range fields {
		result = append(result, resp.Value{Type: resp.BulkString, Str: field})
		result = append(result, resp.Value{Type: resp.BulkString, Str: value})
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// HKEYS key
func hkeys(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hkeys' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	keys := hash.HKeys()
	result := make([]resp.Value, len(keys))
	for i, k := range keys {
		result[i] = resp.Value{Type: resp.BulkString, Str: k}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// HVALS key
func hvals(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hvals' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	values := hash.HVals()
	result := make([]resp.Value, len(values))
	for i, v := range values {
		result[i] = resp.Value{Type: resp.BulkString, Str: v}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// HLEN key
func hlen(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hlen' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	return resp.Value{Type: resp.Integer, Num: hash.HLen()}
}

// HMSET key field value [field value ...]
func hmset(args []resp.Value, db *Database) resp.Value {
	if len(args) < 3 || len(args)%2 == 0 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hmset' command"}
	}

	key := args[0].Str
	hash := db.GetOrCreateHash(key)

	pairs := make(map[string]string)
	for i := 1; i < len(args); i += 2 {
		field := args[i].Str
		value := args[i+1].Str
		pairs[field] = value
	}

	hash.HMSet(pairs)

	return resp.Value{Type: resp.SimpleString, Str: "OK"}
}

// HMGET key field [field ...]
func hmget(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hmget' command"}
	}

	key := args[0].Str
	hash, exists := db.GetHash(key)

	fields := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		fields[i-1] = args[i].Str
	}

	var values []string
	if exists {
		values = hash.HMGet(fields...)
	} else {
		values = make([]string, len(fields))
	}

	result := make([]resp.Value, len(values))
	for i, v := range values {
		if v == "" && (!exists || !hash.HExists(fields[i])) {
			result[i] = resp.Value{Type: resp.Null}
		} else {
			result[i] = resp.Value{Type: resp.BulkString, Str: v}
		}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// HINCRBY key field increment
func hincrby(args []resp.Value, db *Database) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'hincrby' command"}
	}

	key := args[0].Str
	field := args[1].Str
	increment, err := strconv.Atoi(args[2].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}

	hash := db.GetOrCreateHash(key)
	newValue, err := hash.HIncrBy(field, increment)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR " + err.Error()}
	}

	return resp.Value{Type: resp.Integer, Num: newValue}
}

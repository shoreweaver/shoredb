package commands

import (
	"strconv"

	"github.com/shoreweaver/shoredb/pkg/persistence"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

func ping(args []resp.Value, db *Database) resp.Value {
	if len(args) == 0 {
		return resp.Value{Type: resp.SimpleString, Str: "PONG"}
	}
	return resp.Value{Type: resp.BulkString, Str: args[0].Str}
}

func set(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'set' command"}
	}

	key := args[0].Str
	value := args[1].Str

	db.Mu.Lock()
	db.Store[key] = value
	delete(db.Expires, key)
	db.Mu.Unlock()

	return resp.Value{Type: resp.SimpleString, Str: "OK"}
}

func get(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'get' command"}
	}

	key := args[0].Str
	db.expireIfNeeded(key)

	db.Mu.RLock()
	value, ok := db.Store[key]
	db.Mu.RUnlock()

	if !ok {
		return resp.Value{Type: resp.Null}
	}

	return resp.Value{Type: resp.BulkString, Str: value}
}

func del(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'del' command"}
	}

	deleted := 0
	for _, arg := range args {
		key := arg.Str
		deleted += db.DeleteKey(key)
	}

	return resp.Value{Type: resp.Integer, Num: deleted}
}

func exists(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'exists' command"}
	}

	count := 0
	for _, arg := range args {
		key := arg.Str
		if db.KeyExists(key) {
			count++
		}
	}

	return resp.Value{Type: resp.Integer, Num: count}
}

func mset(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 || len(args)%2 != 0 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'mset' command"}
	}

	db.Mu.Lock()
	for i := 0; i < len(args); i += 2 {
		key := args[i].Str
		value := args[i+1].Str
		db.Store[key] = value
		delete(db.Expires, key)
	}
	db.Mu.Unlock()

	return resp.Value{Type: resp.SimpleString, Str: "OK"}
}

func mget(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'mget' command"}
	}

	for _, arg := range args {
		db.expireIfNeeded(arg.Str)
	}

	result := make([]resp.Value, len(args))

	db.Mu.RLock()
	for i, arg := range args {
		key := arg.Str
		if value, ok := db.Store[key]; ok {
			result[i] = resp.Value{Type: resp.BulkString, Str: value}
		} else {
			result[i] = resp.Value{Type: resp.Null}
		}
	}
	db.Mu.RUnlock()

	return resp.Value{Type: resp.Array, Array: result}
}

func typeCmd(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'type' command"}
	}

	key := args[0].Str
	keyType := db.GetKeyType(key)

	return resp.Value{Type: resp.SimpleString, Str: keyType}
}

func save(args []resp.Value, db *Database, rdb *persistence.RDB) resp.Value {
	if rdb == nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR RDB not configured"}
	}

	if err := rdb.Save(); err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR " + err.Error()}
	}

	return resp.Value{Type: resp.SimpleString, Str: "OK"}
}

func bgsave(args []resp.Value, db *Database, rdb *persistence.RDB) resp.Value {
	if rdb == nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR RDB not configured"}
	}

	go rdb.Save()

	return resp.Value{Type: resp.SimpleString, Str: "Background saving started"}
}

func lastsave(args []resp.Value, db *Database, rdb *persistence.RDB) resp.Value {
	if rdb == nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR RDB not configured"}
	}

	timestamp := int(rdb.LastSave().Unix())
	return resp.Value{Type: resp.Integer, Num: timestamp}
}

func incr(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'incr' command"}
	}
	return increment(db, args[0].Str, 1)
}

func decr(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'decr' command"}
	}
	return increment(db, args[0].Str, -1)
}

func incrby(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'incrby' command"}
	}
	n, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}
	return increment(db, args[0].Str, n)
}

func decrby(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'decrby' command"}
	}
	n, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}
	return increment(db, args[0].Str, -n)
}

func increment(db *Database, key string, amount int) resp.Value {
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	val, ok := db.Store[key]
	if !ok {
		val = "0"
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}

	newVal := n + amount
	db.Store[key] = strconv.Itoa(newVal)
	return resp.Value{Type: resp.Integer, Num: newVal}
}

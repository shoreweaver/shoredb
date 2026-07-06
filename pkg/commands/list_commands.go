package commands

import (
	"strconv"
	"time"

	"github.com/shoreweaver/shoredb/pkg/resp"
)

// LPUSH key value [value ...]
func lpush(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lpush' command"}
	}

	key := args[0].Str
	values := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		values[i-1] = args[i].Str
	}

	list := db.GetOrCreateList(key)
	length := list.LPush(values...)
	db.signalListPush()

	return resp.Value{Type: resp.Integer, Num: length}
}

// RPUSH key value [value ...]
func rpush(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'rpush' command"}
	}

	key := args[0].Str
	values := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		values[i-1] = args[i].Str
	}

	list := db.GetOrCreateList(key)
	length := list.RPush(values...)
	db.signalListPush()

	return resp.Value{Type: resp.Integer, Num: length}
}

// LPOP key
func lpop(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lpop' command"}
	}

	key := args[0].Str
	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Null}
	}

	value, ok := list.LPop()
	if !ok {
		return resp.Value{Type: resp.Null}
	}

	if list.LLen() == 0 {
		db.Mu.Lock()
		delete(db.Lists, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.BulkString, Str: value}
}

// RPOP key
func rpop(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'rpop' command"}
	}

	key := args[0].Str
	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Null}
	}

	value, ok := list.RPop()
	if !ok {
		return resp.Value{Type: resp.Null}
	}

	if list.LLen() == 0 {
		db.Mu.Lock()
		delete(db.Lists, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.BulkString, Str: value}
}

// BLPOP key [key ...] timeout
func blpop(args []resp.Value, db *Database) resp.Value {
	return blockingPop(args, db, true)
}

// BRPOP key [key ...] timeout
func brpop(args []resp.Value, db *Database) resp.Value {
	return blockingPop(args, db, false)
}

// blockingPop used to busy-wait: time.Sleep(50ms) in a loop, which meant a
// pop could sit ready for up to 50ms before a blocked BLPOP/BRPOP noticed
// it. It now waits on db's list condition variable, which LPUSH/RPUSH
// broadcast on directly, so a blocked popper wakes as soon as data arrives
// (or exactly at the deadline, via a timer-driven broadcast).
func blockingPop(args []resp.Value, db *Database, left bool) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'blpop' command"}
	}

	keys := make([]string, len(args)-1)
	for i := 0; i < len(args)-1; i++ {
		keys[i] = args[i].Str
	}

	timeoutSec, err := strconv.ParseFloat(args[len(args)-1].Str, 64)
	if err != nil || timeoutSec < 0 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR timeout is not a float or out of range"}
	}

	hasDeadline := timeoutSec > 0
	var deadline time.Time
	if hasDeadline {
		deadline = time.Now().Add(time.Duration(timeoutSec * float64(time.Second)))
	}

	for {
		for _, key := range keys {
			list, exists := db.GetList(key)
			if !exists {
				continue
			}

			var value string
			var ok bool
			if left {
				value, ok = list.LPop()
			} else {
				value, ok = list.RPop()
			}
			if !ok {
				continue
			}

			if list.LLen() == 0 {
				db.Mu.Lock()
				delete(db.Lists, key)
				db.Mu.Unlock()
			}

			return resp.Value{Type: resp.Array, Array: []resp.Value{
				{Type: resp.BulkString, Str: key},
				{Type: resp.BulkString, Str: value},
			}}
		}

		if hasDeadline && time.Now().After(deadline) {
			return resp.Value{Type: resp.Null}
		}

		db.waitForListSignal(deadline, hasDeadline)
	}
}

// LRANGE key start stop
func lrange(args []resp.Value, db *Database) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lrange' command"}
	}

	key := args[0].Str
	start, err1 := strconv.Atoi(args[1].Str)
	stop, err2 := strconv.Atoi(args[2].Str)

	if err1 != nil || err2 != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}

	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	elements := list.LRange(start, stop)
	result := make([]resp.Value, len(elements))
	for i, elem := range elements {
		result[i] = resp.Value{Type: resp.BulkString, Str: elem}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// LLEN key
func llen(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'llen' command"}
	}

	key := args[0].Str
	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	return resp.Value{Type: resp.Integer, Num: list.LLen()}
}

// LINDEX key index
func lindex(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lindex' command"}
	}

	key := args[0].Str
	index, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}

	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Null}
	}

	value, ok := list.LIndex(index)
	if !ok {
		return resp.Value{Type: resp.Null}
	}

	return resp.Value{Type: resp.BulkString, Str: value}
}

// LSET key index value
func lset(args []resp.Value, db *Database) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lset' command"}
	}

	key := args[0].Str
	index, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}
	value := args[2].Str

	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.SimpleError, Str: "ERR no such key"}
	}

	if !list.LSet(index, value) {
		return resp.Value{Type: resp.SimpleError, Str: "ERR index out of range"}
	}

	return resp.Value{Type: resp.SimpleString, Str: "OK"}
}

// LREM key count value
func lrem(args []resp.Value, db *Database) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'lrem' command"}
	}

	key := args[0].Str
	count, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}
	value := args[2].Str

	list, exists := db.GetList(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	removed := list.LRem(count, value)

	if list.LLen() == 0 {
		db.Mu.Lock()
		delete(db.Lists, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.Integer, Num: removed}
}

package commands

import (
	"strconv"
	"time"

	"github.com/shoreweaver/shoredb/pkg/resp"
)

func keyExistsLocked(db *Database, key string) bool {
	if _, ok := db.Store[key]; ok {
		return true
	}
	if _, ok := db.Lists[key]; ok {
		return true
	}
	if _, ok := db.Sets[key]; ok {
		return true
	}
	if _, ok := db.Hashes[key]; ok {
		return true
	}
	return false
}

// EXPIRE key seconds
func expire(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'expire' command"}
	}
	return setExpire(db, args[0].Str, args[1].Str, time.Second)
}

// PEXPIRE key milliseconds
func pexpire(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'pexpire' command"}
	}
	return setExpire(db, args[0].Str, args[1].Str, time.Millisecond)
}

func setExpire(db *Database, key, amountStr string, unit time.Duration) resp.Value {
	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"}
	}

	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	if !keyExistsLocked(db, key) {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	db.Expires[key] = time.Now().Add(time.Duration(amount) * unit)
	return resp.Value{Type: resp.Integer, Num: 1}
}

// TTL key
func ttl(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'ttl' command"}
	}
	return ttlValue(db, args[0].Str, time.Second)
}

// PTTL key
func pttl(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'pttl' command"}
	}
	return ttlValue(db, args[0].Str, time.Millisecond)
}

func ttlValue(db *Database, key string, unit time.Duration) resp.Value {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	if !keyExistsLocked(db, key) {
		return resp.Value{Type: resp.Integer, Num: -2}
	}

	exp, ok := db.Expires[key]
	if !ok {
		return resp.Value{Type: resp.Integer, Num: -1}
	}

	remaining := time.Until(exp)
	if remaining < 0 {
		remaining = 0
	}
	return resp.Value{Type: resp.Integer, Num: int(remaining / unit)}
}

// PERSIST key
func persist(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'persist' command"}
	}

	key := args[0].Str
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	if _, ok := db.Expires[key]; !ok {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	delete(db.Expires, key)
	return resp.Value{Type: resp.Integer, Num: 1}
}

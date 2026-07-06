package commands

import (
	"github.com/shoreweaver/shoredb/pkg/datastruct"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

// SADD key member [member ...]
func sadd(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'sadd' command"}
	}

	key := args[0].Str
	members := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		members[i-1] = args[i].Str
	}

	set := db.GetOrCreateSet(key)
	added := set.SAdd(members...)

	return resp.Value{Type: resp.Integer, Num: added}
}

// SREM key member [member ...]
func srem(args []resp.Value, db *Database) resp.Value {
	if len(args) < 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'srem' command"}
	}

	key := args[0].Str
	set, exists := db.GetSet(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	members := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		members[i-1] = args[i].Str
	}

	removed := set.SRem(members...)

	if set.SCard() == 0 {
		db.Mu.Lock()
		delete(db.Sets, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.Integer, Num: removed}
}

// SISMEMBER key member
func sismember(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'sismember' command"}
	}

	key := args[0].Str
	member := args[1].Str

	set, exists := db.GetSet(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	if set.SIsMember(member) {
		return resp.Value{Type: resp.Integer, Num: 1}
	}
	return resp.Value{Type: resp.Integer, Num: 0}
}

// SMEMBERS key
func smembers(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'smembers' command"}
	}

	key := args[0].Str
	set, exists := db.GetSet(key)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	members := set.SMembers()
	result := make([]resp.Value, len(members))
	for i, member := range members {
		result[i] = resp.Value{Type: resp.BulkString, Str: member}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// SCARD key
func scard(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'scard' command"}
	}

	key := args[0].Str
	set, exists := db.GetSet(key)
	if !exists {
		return resp.Value{Type: resp.Integer, Num: 0}
	}

	return resp.Value{Type: resp.Integer, Num: set.SCard()}
}

// SPOP key
func spop(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'spop' command"}
	}

	key := args[0].Str
	set, exists := db.GetSet(key)
	if !exists {
		return resp.Value{Type: resp.Null}
	}

	member, ok := set.SPop()
	if !ok {
		return resp.Value{Type: resp.Null}
	}

	if set.SCard() == 0 {
		db.Mu.Lock()
		delete(db.Sets, key)
		db.Mu.Unlock()
	}

	return resp.Value{Type: resp.BulkString, Str: member}
}

// SINTER key [key ...]
func sinter(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'sinter' command"}
	}

	firstKey := args[0].Str
	firstSet, exists := db.GetSet(firstKey)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	otherSets := make([]*datastruct.Set, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		set, exists := db.GetSet(args[i].Str)
		if !exists {
			return resp.Value{Type: resp.Array, Array: []resp.Value{}}
		}
		otherSets = append(otherSets, set)
	}

	members := firstSet.SInter(otherSets...)
	result := make([]resp.Value, len(members))
	for i, member := range members {
		result[i] = resp.Value{Type: resp.BulkString, Str: member}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// SUNION key [key ...]
func sunion(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'sunion' command"}
	}

	firstKey := args[0].Str
	firstSet, exists := db.GetSet(firstKey)
	if !exists {
		firstSet = datastruct.NewSet()
	}

	otherSets := make([]*datastruct.Set, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		set, exists := db.GetSet(args[i].Str)
		if exists {
			otherSets = append(otherSets, set)
		}
	}

	members := firstSet.SUnion(otherSets...)
	result := make([]resp.Value, len(members))
	for i, member := range members {
		result[i] = resp.Value{Type: resp.BulkString, Str: member}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// SDIFF key [key ...]
func sdiff(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'sdiff' command"}
	}

	firstKey := args[0].Str
	firstSet, exists := db.GetSet(firstKey)
	if !exists {
		return resp.Value{Type: resp.Array, Array: []resp.Value{}}
	}

	otherSets := make([]*datastruct.Set, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		set, exists := db.GetSet(args[i].Str)
		if exists {
			otherSets = append(otherSets, set)
		}
	}

	members := firstSet.SDiff(otherSets...)
	result := make([]resp.Value, len(members))
	for i, member := range members {
		result[i] = resp.Value{Type: resp.BulkString, Str: member}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

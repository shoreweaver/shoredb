package commands

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shoreweaver/shoredb/pkg/resp"
)

// KEYS pattern
func keysCmd(args []resp.Value, db *Database) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'keys' command"}
	}

	db.SweepExpired()
	pattern := args[0].Str

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	result := make([]resp.Value, 0)
	for k := range db.Store {
		if matched, _ := filepath.Match(pattern, k); matched {
			result = append(result, resp.Value{Type: resp.BulkString, Str: k})
		}
	}
	for k := range db.Lists {
		if matched, _ := filepath.Match(pattern, k); matched {
			result = append(result, resp.Value{Type: resp.BulkString, Str: k})
		}
	}
	for k := range db.Sets {
		if matched, _ := filepath.Match(pattern, k); matched {
			result = append(result, resp.Value{Type: resp.BulkString, Str: k})
		}
	}
	for k := range db.Hashes {
		if matched, _ := filepath.Match(pattern, k); matched {
			result = append(result, resp.Value{Type: resp.BulkString, Str: k})
		}
	}

	return resp.Value{Type: resp.Array, Array: result}
}

// DBSIZE
func dbsize(args []resp.Value, db *Database) resp.Value {
	db.SweepExpired()

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	total := len(db.Store) + len(db.Lists) + len(db.Sets) + len(db.Hashes)
	return resp.Value{Type: resp.Integer, Num: total}
}

// SCAN cursor [MATCH pattern]
// Simplified single-pass scan: always returns cursor 0 (full result set).
func scan(args []resp.Value, db *Database) resp.Value {
	if len(args) < 1 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'scan' command"}
	}
	if _, err := strconv.Atoi(args[0].Str); err != nil {
		return resp.Value{Type: resp.SimpleError, Str: "ERR invalid cursor"}
	}

	pattern := "*"
	for i := 1; i < len(args)-1; i++ {
		if args[i].Str == "MATCH" || args[i].Str == "match" {
			pattern = args[i+1].Str
		}
	}

	keysResp := keysCmd([]resp.Value{{Type: resp.BulkString, Str: pattern}}, db)

	return resp.Value{Type: resp.Array, Array: []resp.Value{
		{Type: resp.BulkString, Str: "0"},
		keysResp,
	}}
}

// HELLO [protover]
func hello(args []resp.Value, db *Database) resp.Value {
	if len(args) >= 1 && args[0].Str == "3" {
		return resp.Value{Type: resp.SimpleError, Str: "NOPROTO unsupported protocol version"}
	}

	info := []resp.Value{
		{Type: resp.BulkString, Str: "server"},
		{Type: resp.BulkString, Str: "shoredis"},
		{Type: resp.BulkString, Str: "version"},
		{Type: resp.BulkString, Str: "1.0"},
		{Type: resp.BulkString, Str: "proto"},
		{Type: resp.Integer, Num: 2},
		{Type: resp.BulkString, Str: "mode"},
		{Type: resp.BulkString, Str: "standalone"},
		{Type: resp.BulkString, Str: "role"},
		{Type: resp.BulkString, Str: "master"},
		{Type: resp.BulkString, Str: "modules"},
		{Type: resp.Array, Array: []resp.Value{}},
	}
	return resp.Value{Type: resp.Array, Array: info}
}

// cmdSpec is a minimal, hand-maintained description of a supported command,
// modeled loosely on real Redis's COMMAND INFO tuple:
// [name, arity, flags, first_key, last_key, step].
// arity: positive = exact number of tokens including the command name;
// negative = "at least abs(n)", for variadic commands.
type cmdSpec struct {
	name     string
	arity    int
	flags    []string
	firstKey int
	lastKey  int
	step     int
}

var commandSpecs = []cmdSpec{
	{"PING", -1, []string{"fast"}, 0, 0, 0},
	{"SET", 3, []string{"write", "denyoom"}, 1, 1, 1},
	{"GET", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"DEL", -2, []string{"write"}, 1, -1, 1},
	{"EXISTS", -2, []string{"readonly", "fast"}, 1, -1, 1},
	{"MSET", -3, []string{"write", "denyoom"}, 1, -1, 2},
	{"MGET", -2, []string{"readonly", "fast"}, 1, -1, 1},
	{"TYPE", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"INCR", 2, []string{"write", "fast"}, 1, 1, 1},
	{"DECR", 2, []string{"write", "fast"}, 1, 1, 1},
	{"INCRBY", 3, []string{"write", "fast"}, 1, 1, 1},
	{"DECRBY", 3, []string{"write", "fast"}, 1, 1, 1},
	{"LPUSH", -3, []string{"write", "fast"}, 1, 1, 1},
	{"RPUSH", -3, []string{"write", "fast"}, 1, 1, 1},
	{"LPOP", 2, []string{"write", "fast"}, 1, 1, 1},
	{"RPOP", 2, []string{"write", "fast"}, 1, 1, 1},
	{"LRANGE", 4, []string{"readonly"}, 1, 1, 1},
	{"LLEN", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"LINDEX", 3, []string{"readonly"}, 1, 1, 1},
	{"LSET", 4, []string{"write"}, 1, 1, 1},
	{"LREM", 4, []string{"write"}, 1, 1, 1},
	{"BLPOP", -3, []string{"write", "blocking"}, 1, -2, 1},
	{"BRPOP", -3, []string{"write", "blocking"}, 1, -2, 1},
	{"SADD", -3, []string{"write", "fast"}, 1, 1, 1},
	{"SREM", -3, []string{"write", "fast"}, 1, 1, 1},
	{"SISMEMBER", 3, []string{"readonly", "fast"}, 1, 1, 1},
	{"SMEMBERS", 2, []string{"readonly"}, 1, 1, 1},
	{"SCARD", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"SPOP", 2, []string{"write", "fast"}, 1, 1, 1},
	{"SINTER", -2, []string{"readonly"}, 1, -1, 1},
	{"SUNION", -2, []string{"readonly"}, 1, -1, 1},
	{"SDIFF", -2, []string{"readonly"}, 1, -1, 1},
	{"HSET", 4, []string{"write", "fast"}, 1, 1, 1},
	{"HGET", 3, []string{"readonly", "fast"}, 1, 1, 1},
	{"HDEL", -3, []string{"write", "fast"}, 1, 1, 1},
	{"HEXISTS", 3, []string{"readonly", "fast"}, 1, 1, 1},
	{"HGETALL", 2, []string{"readonly"}, 1, 1, 1},
	{"HKEYS", 2, []string{"readonly"}, 1, 1, 1},
	{"HVALS", 2, []string{"readonly"}, 1, 1, 1},
	{"HLEN", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"HMSET", -4, []string{"write"}, 1, 1, 1},
	{"HMGET", -3, []string{"readonly"}, 1, 1, 1},
	{"HINCRBY", 4, []string{"write", "fast"}, 1, 1, 1},
	{"EXPIRE", 3, []string{"write", "fast"}, 1, 1, 1},
	{"PEXPIRE", 3, []string{"write", "fast"}, 1, 1, 1},
	{"TTL", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"PTTL", 2, []string{"readonly", "fast"}, 1, 1, 1},
	{"PERSIST", 2, []string{"write", "fast"}, 1, 1, 1},
	{"KEYS", 2, []string{"readonly"}, 0, 0, 0},
	{"SCAN", -2, []string{"readonly"}, 0, 0, 0},
	{"DBSIZE", 1, []string{"readonly", "fast"}, 0, 0, 0},
	{"HELLO", -1, []string{"fast"}, 0, 0, 0},
	{"PUBLISH", 3, []string{"pubsub", "fast"}, 0, 0, 0},
	{"SUBSCRIBE", -2, []string{"pubsub"}, 0, 0, 0},
	{"UNSUBSCRIBE", -1, []string{"pubsub"}, 0, 0, 0},
	{"AUTH", -2, []string{"fast", "loading", "stale"}, 0, 0, 0},
	{"SELECT", 2, []string{"fast", "loading", "stale"}, 0, 0, 0},
	{"SAVE", 1, []string{"admin"}, 0, 0, 0},
	{"BGSAVE", 1, []string{"admin"}, 0, 0, 0},
	{"LASTSAVE", 1, []string{"readonly", "fast"}, 0, 0, 0},
	{"COMMAND", -1, []string{"loading", "stale"}, 0, 0, 0},
}

func commandInfoValue(spec cmdSpec) resp.Value {
	flags := make([]resp.Value, len(spec.flags))
	for i, f := range spec.flags {
		flags[i] = resp.Value{Type: resp.SimpleString, Str: f}
	}
	return resp.Value{Type: resp.Array, Array: []resp.Value{
		{Type: resp.BulkString, Str: strings.ToLower(spec.name)},
		{Type: resp.Integer, Num: spec.arity},
		{Type: resp.Array, Array: flags},
		{Type: resp.Integer, Num: spec.firstKey},
		{Type: resp.Integer, Num: spec.lastKey},
		{Type: resp.Integer, Num: spec.step},
	}}
}

func findCommandSpec(name string) (cmdSpec, bool) {
	upper := strings.ToUpper(name)
	for _, spec := range commandSpecs {
		if spec.name == upper {
			return spec, true
		}
	}
	return cmdSpec{}, false
}

func allCommandInfo() []resp.Value {
	result := make([]resp.Value, len(commandSpecs))
	for i, spec := range commandSpecs {
		result[i] = commandInfoValue(spec)
	}
	return result
}

// COMMAND [COUNT | LIST | INFO [name ...] | DOCS [name ...] | GETKEYS ...]
//
// Previously this was a stub that always returned an empty array regardless
// of subcommand, which meant any client that calls COMMAND/COMMAND DOCS on
// connect (to learn arity/flags for its own argument validation) saw zero
// known commands - and then treated every subsequent command as unsupported.
func command(args []resp.Value, db *Database) resp.Value {
	if len(args) == 0 {
		return resp.Value{Type: resp.Array, Array: allCommandInfo()}
	}

	switch strings.ToUpper(args[0].Str) {
	case "COUNT":
		return resp.Value{Type: resp.Integer, Num: len(commandSpecs)}

	case "LIST":
		result := make([]resp.Value, len(commandSpecs))
		for i, spec := range commandSpecs {
			result[i] = resp.Value{Type: resp.BulkString, Str: strings.ToLower(spec.name)}
		}
		return resp.Value{Type: resp.Array, Array: result}

	case "INFO":
		names := args[1:]
		if len(names) == 0 {
			return resp.Value{Type: resp.Array, Array: allCommandInfo()}
		}
		result := make([]resp.Value, len(names))
		for i, n := range names {
			if spec, ok := findCommandSpec(n.Str); ok {
				result[i] = commandInfoValue(spec)
			} else {
				result[i] = resp.Value{Type: resp.Null}
			}
		}
		return resp.Value{Type: resp.Array, Array: result}

	case "DOCS":
		// Minimal support: acknowledge known commands with an (empty)
		// per-command doc map rather than erroring. Real Redis returns
		// rich docs (summary, since, group, arguments...); modeling that
		// fully isn't done here, but clients that treat missing/partial
		// DOCS output as informational-only will still work fine using
		// COMMAND / COMMAND INFO for arity and flags.
		names := args[1:]
		if len(names) == 0 {
			names = make([]resp.Value, len(commandSpecs))
			for i, spec := range commandSpecs {
				names[i] = resp.Value{Str: strings.ToLower(spec.name)}
			}
		}
		result := make([]resp.Value, 0, len(names)*2)
		for _, n := range names {
			if spec, ok := findCommandSpec(n.Str); ok {
				result = append(result,
					resp.Value{Type: resp.BulkString, Str: strings.ToLower(spec.name)},
					resp.Value{Type: resp.Array, Array: []resp.Value{}},
				)
			}
		}
		return resp.Value{Type: resp.Array, Array: result}

	case "GETKEYS":
		if len(args) < 2 {
			return resp.Value{Type: resp.SimpleError, Str: "ERR Unknown command name"}
		}
		spec, ok := findCommandSpec(args[1].Str)
		if !ok || spec.firstKey == 0 {
			return resp.Value{Type: resp.SimpleError, Str: "ERR The command has no key arguments"}
		}
		cmdArgs := args[2:]
		keyIdx := spec.firstKey - 1
		if keyIdx < 0 || keyIdx >= len(cmdArgs) {
			return resp.Value{Type: resp.SimpleError, Str: "ERR Invalid arguments specified for command"}
		}
		last := spec.lastKey
		if last < 0 {
			last = len(cmdArgs) + last + 1
		}
		if last <= spec.firstKey {
			last = spec.firstKey
		}
		result := make([]resp.Value, 0)
		for i := spec.firstKey - 1; i < last && i < len(cmdArgs); i += spec.step {
			result = append(result, resp.Value{Type: resp.BulkString, Str: cmdArgs[i].Str})
		}
		if len(result) == 0 {
			return resp.Value{Type: resp.SimpleError, Str: "ERR The command has no key arguments"}
		}
		return resp.Value{Type: resp.Array, Array: result}

	default:
		return resp.Value{Type: resp.SimpleError, Str: "ERR Unknown subcommand or wrong number of arguments for '" + args[0].Str + "'"}
	}
}

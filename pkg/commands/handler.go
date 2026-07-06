package commands

import (
	"github.com/shoreweaver/shoredb/pkg/persistence"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

type CommandHandler func([]resp.Value, *Database) resp.Value
type RDBCommandHandler func([]resp.Value, *Database, *persistence.RDB) resp.Value

var CommandHandlers = map[string]CommandHandler{
	"COMMAND": command,
	// String commands
	"PING":   ping,
	"SET":    set,
	"GET":    get,
	"DEL":    del,
	"EXISTS": exists,
	"MSET":   mset,
	"MGET":   mget,
	"TYPE":   typeCmd,

	// Counters
	"INCR":   incr,
	"DECR":   decr,
	"DECRBY": decrby,
	"INCRBY": incrby,

	// List commands
	"LPUSH":  lpush,
	"RPUSH":  rpush,
	"LPOP":   lpop,
	"RPOP":   rpop,
	"LRANGE": lrange,
	"LLEN":   llen,
	"LINDEX": lindex,
	"LSET":   lset,
	"LREM":   lrem,
	"BLPOP":  blpop,
	"BRPOP":  brpop,

	// Set commands
	"SADD":      sadd,
	"SREM":      srem,
	"SISMEMBER": sismember,
	"SMEMBERS":  smembers,
	"SCARD":     scard,
	"SPOP":      spop,
	"SINTER":    sinter,
	"SUNION":    sunion,
	"SDIFF":     sdiff,

	// Hash commands
	"HSET":    hset,
	"HGET":    hget,
	"HDEL":    hdel,
	"HEXISTS": hexists,
	"HGETALL": hgetall,
	"HKEYS":   hkeys,
	"HVALS":   hvals,
	"HLEN":    hlen,
	"HMSET":   hmset,
	"HMGET":   hmget,
	"HINCRBY": hincrby,

	// Key expiration
	"EXPIRE":  expire,
	"PEXPIRE": pexpire,
	"TTL":     ttl,
	"PTTL":    pttl,
	"PERSIST": persist,

	// Admin / introspection
	"KEYS":   keysCmd,
	"SCAN":   scan,
	"DBSIZE": dbsize,
	"HELLO":  hello,

	// Pub/Sub
	"PUBLISH": publish,
}

var RDBCommandHandlers = map[string]RDBCommandHandler{
	"SAVE":     save,
	"BGSAVE":   bgsave,
	"LASTSAVE": lastsave,
}

type RDBPersister interface {
	Save() error
	StartBackgroundSave() bool
	FinishBackgroundSave(err error)
}

package server

import (
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shoreweaver/shoredb/pkg/commands"
	"github.com/shoreweaver/shoredb/pkg/pubsub"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Server listening on %s", s.listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// safeWriter serializes writes to a connection since pub/sub delivery and
// command replies can happen from different goroutines concurrently.
type safeWriter struct {
	mu sync.Mutex
	w  *resp.Writer
}

func (sw *safeWriter) Write(v resp.Value) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(v)
}

func (s *Server) handleConnection(conn net.Conn) {
	connID := atomic.AddInt64(&s.connCounter, 1)
	remoteAddr := conn.RemoteAddr().String()
	startTime := time.Now()

	log.Printf("Client #%d connected from %s", connID, remoteAddr)

	subs := make(map[string]chan pubsub.Message)
	defer func() {
		for channel, ch := range subs {
			s.db.PubSub.Unsubscribe(channel, ch)
		}
		conn.Close()
		log.Printf("Client #%d disconnected (duration: %v)", connID, time.Since(startTime).Round(time.Second))
	}()

	parser := resp.NewParser(conn)
	writer := &safeWriter{w: resp.NewWriter(conn)}

	authenticated := s.requirePass == ""

	// dbIndex is this connection's currently SELECTed logical database
	// (0-15). Each connection has its own; SELECT never affects other
	// connections.
	dbIndex := 0

	for {
		value, err := parser.Parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("Error parsing RESP: %v", err)
			return
		}

		if value.Type != resp.Array || len(value.Array) == 0 {
			writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR invalid command format"})
			continue
		}

		command := strings.ToUpper(value.Array[0].Str)
		args := value.Array[1:]

		if command == "AUTH" {
			authenticated = s.handleAuth(args, writer)
			continue
		}

		if !authenticated {
			writer.Write(resp.Value{Type: resp.SimpleError, Str: "NOAUTH Authentication required"})
			continue
		}

		if command == "SELECT" {
			dbIndex = s.handleSelect(args, writer, dbIndex)
			continue
		}

		switch command {
		case "SUBSCRIBE":
			s.handleSubscribe(args, subs, writer)
			continue
		case "UNSUBSCRIBE":
			s.handleUnsubscribe(args, subs, writer)
			continue
		}

		response := s.processCommand(command, args, value, dbIndex)
		if err := writer.Write(response); err != nil {
			log.Printf("Error writing response: %v", err)
			return
		}
	}
}

func (s *Server) handleAuth(args []resp.Value, writer *safeWriter) bool {
	if len(args) != 1 {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'auth' command"})
		return s.requirePass == ""
	}

	if s.requirePass == "" {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR Client sent AUTH, but no password is set"})
		return true
	}

	if args[0].Str != s.requirePass {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "WRONGPASS invalid username-password pair"})
		return false
	}

	writer.Write(resp.Value{Type: resp.SimpleString, Str: "OK"})
	return true
}

// handleSelect validates and applies a SELECT index. It returns the
// connection's (possibly unchanged) current database index.
func (s *Server) handleSelect(args []resp.Value, writer *safeWriter, currentIndex int) int {
	if len(args) != 1 {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'select' command"})
		return currentIndex
	}

	index, err := strconv.Atoi(args[0].Str)
	if err != nil {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR value is not an integer or out of range"})
		return currentIndex
	}

	if index < 0 || index >= commands.NumDatabases {
		writer.Write(resp.Value{Type: resp.SimpleError, Str: "ERR DB index is out of range"})
		return currentIndex
	}

	writer.Write(resp.Value{Type: resp.SimpleString, Str: "OK"})
	return index
}

func (s *Server) handleSubscribe(args []resp.Value, subs map[string]chan pubsub.Message, writer *safeWriter) {
	for _, arg := range args {
		channel := arg.Str
		if _, exists := subs[channel]; exists {
			continue
		}

		ch := s.db.PubSub.Subscribe(channel)
		subs[channel] = ch

		go func(ch chan pubsub.Message) {
			for msg := range ch {
				writer.Write(resp.Value{Type: resp.Array, Array: []resp.Value{
					{Type: resp.BulkString, Str: "message"},
					{Type: resp.BulkString, Str: msg.Channel},
					{Type: resp.BulkString, Str: msg.Payload},
				}})
			}
		}(ch)

		writer.Write(resp.Value{Type: resp.Array, Array: []resp.Value{
			{Type: resp.BulkString, Str: "subscribe"},
			{Type: resp.BulkString, Str: channel},
			{Type: resp.Integer, Num: len(subs)},
		}})
	}
}

func (s *Server) handleUnsubscribe(args []resp.Value, subs map[string]chan pubsub.Message, writer *safeWriter) {
	channels := make([]string, 0, len(args))
	if len(args) == 0 {
		for channel := range subs {
			channels = append(channels, channel)
		}
	} else {
		for _, arg := range args {
			channels = append(channels, arg.Str)
		}
	}

	for _, channel := range channels {
		if ch, exists := subs[channel]; exists {
			s.db.PubSub.Unsubscribe(channel, ch)
			delete(subs, channel)
		}

		writer.Write(resp.Value{Type: resp.Array, Array: []resp.Value{
			{Type: resp.BulkString, Str: "unsubscribe"},
			{Type: resp.BulkString, Str: channel},
			{Type: resp.Integer, Num: len(subs)},
		}})
	}
}

func (s *Server) processCommand(command string, args []resp.Value, original resp.Value, dbIndex int) resp.Value {
	db := s.db.DB(dbIndex)

	if rdbHandler, ok := commands.RDBCommandHandlers[command]; ok {
		return rdbHandler(args, db, s.rdb)
	}

	handler, ok := commands.CommandHandlers[command]
	if !ok {
		return resp.Value{
			Type: resp.SimpleError,
			Str:  "ERR unknown command '" + command + "'",
		}
	}

	// Execute first, persist second. Persisting before execution meant a
	// command that failed argument validation (or errored for any other
	// reason) still got written to the AOF, corrupting replay. Only a
	// successful (non-error) mutating command should ever be logged.
	response := handler(args, db)

	if response.Type != resp.SimpleError && shouldPersist(command) {
		if s.aof != nil {
			if err := s.aof.Write(dbIndex, original); err != nil {
				log.Printf("Error writing to AOF: %v", err)
			}
		}

		if s.rdb != nil {
			s.rdb.IncrementChanges()
		}
	}

	return response
}

func shouldPersist(command string) bool {
	switch command {
	case "SET", "DEL", "MSET":
		return true
	case "INCR", "DECR", "INCRBY", "DECRBY":
		return true
	case "LPUSH", "RPUSH", "LPOP", "RPOP", "LSET", "LREM", "BLPOP", "BRPOP":
		return true
	case "SADD", "SREM", "SPOP":
		return true
	case "HSET", "HDEL", "HMSET", "HINCRBY":
		return true
	default:
		return false
	}
}

// replayCommand re-applies a single logged command against the logical
// database it was recorded for (already resolved by the caller from the
// AOF's SELECT markers - see persistence.Aof.Read).
func replayCommand(value resp.Value, db *commands.Database) {
	if value.Type != resp.Array || len(value.Array) == 0 {
		return
	}

	command := strings.ToUpper(value.Array[0].Str)
	args := value.Array[1:]

	switch command {
	case "SET":
		if len(args) >= 2 {
			db.Mu.Lock()
			db.Store[args[0].Str] = args[1].Str
			delete(db.Expires, args[0].Str)
			db.Mu.Unlock()
		}
	case "DEL":
		if len(args) >= 1 {
			for _, arg := range args {
				db.DeleteKey(arg.Str)
			}
		}
	case "MSET":
		if len(args) >= 2 && len(args)%2 == 0 {
			db.Mu.Lock()
			for i := 0; i < len(args); i += 2 {
				db.Store[args[i].Str] = args[i+1].Str
				delete(db.Expires, args[i].Str)
			}
			db.Mu.Unlock()
		}

	case "INCR":
		if len(args) >= 1 {
			replayIncrement(db, args[0].Str, 1)
		}
	case "DECR":
		if len(args) >= 1 {
			replayIncrement(db, args[0].Str, -1)
		}
	case "INCRBY":
		if len(args) >= 2 {
			if n, err := parseInt(args[1].Str); err == nil {
				replayIncrement(db, args[0].Str, n)
			}
		}
	case "DECRBY":
		if len(args) >= 2 {
			if n, err := parseInt(args[1].Str); err == nil {
				replayIncrement(db, args[0].Str, -n)
			}
		}

	case "LPUSH":
		if len(args) >= 2 {
			key := args[0].Str
			values := make([]string, len(args)-1)
			for i := 1; i < len(args); i++ {
				values[i-1] = args[i].Str
			}
			db.GetOrCreateList(key).LPush(values...)
		}
	case "RPUSH":
		if len(args) >= 2 {
			key := args[0].Str
			values := make([]string, len(args)-1)
			for i := 1; i < len(args); i++ {
				values[i-1] = args[i].Str
			}
			db.GetOrCreateList(key).RPush(values...)
		}
	case "LPOP":
		replayListPop(args, db, true)
	case "RPOP":
		replayListPop(args, db, false)
	case "BLPOP":
		if len(args) >= 2 {
			replayListPop(args[:len(args)-1], db, true)
		}
	case "BRPOP":
		if len(args) >= 2 {
			replayListPop(args[:len(args)-1], db, false)
		}
	case "LSET":
		if len(args) >= 3 {
			key := args[0].Str
			if list, exists := db.GetList(key); exists {
				if index, err := parseInt(args[1].Str); err == nil {
					list.LSet(index, args[2].Str)
				}
			}
		}
	case "LREM":
		if len(args) >= 3 {
			key := args[0].Str
			if list, exists := db.GetList(key); exists {
				if count, err := parseInt(args[1].Str); err == nil {
					list.LRem(count, args[2].Str)
					if list.LLen() == 0 {
						db.Mu.Lock()
						delete(db.Lists, key)
						db.Mu.Unlock()
					}
				}
			}
		}

	case "SADD":
		if len(args) >= 2 {
			key := args[0].Str
			members := make([]string, len(args)-1)
			for i := 1; i < len(args); i++ {
				members[i-1] = args[i].Str
			}
			db.GetOrCreateSet(key).SAdd(members...)
		}
	case "SREM":
		if len(args) >= 2 {
			key := args[0].Str
			if set, exists := db.GetSet(key); exists {
				members := make([]string, len(args)-1)
				for i := 1; i < len(args); i++ {
					members[i-1] = args[i].Str
				}
				set.SRem(members...)
				if set.SCard() == 0 {
					db.Mu.Lock()
					delete(db.Sets, key)
					db.Mu.Unlock()
				}
			}
		}
	case "SPOP":
		if len(args) >= 1 {
			key := args[0].Str
			if set, exists := db.GetSet(key); exists {
				set.SPop()
				if set.SCard() == 0 {
					db.Mu.Lock()
					delete(db.Sets, key)
					db.Mu.Unlock()
				}
			}
		}

	case "HSET":
		if len(args) >= 3 {
			db.GetOrCreateHash(args[0].Str).HSet(args[1].Str, args[2].Str)
		}
	case "HDEL":
		if len(args) >= 2 {
			key := args[0].Str
			if hash, exists := db.GetHash(key); exists {
				fields := make([]string, len(args)-1)
				for i := 1; i < len(args); i++ {
					fields[i-1] = args[i].Str
				}
				hash.HDel(fields...)
				if hash.HLen() == 0 {
					db.Mu.Lock()
					delete(db.Hashes, key)
					db.Mu.Unlock()
				}
			}
		}
	case "HMSET":
		if len(args) >= 3 && len(args)%2 == 1 {
			key := args[0].Str
			pairs := make(map[string]string)
			for i := 1; i < len(args); i += 2 {
				pairs[args[i].Str] = args[i+1].Str
			}
			db.GetOrCreateHash(key).HMSet(pairs)
		}
	case "HINCRBY":
		if len(args) >= 3 {
			if increment, err := parseInt(args[2].Str); err == nil {
				db.GetOrCreateHash(args[0].Str).HIncrBy(args[1].Str, increment)
			}
		}
	}
}

// replayIncrement re-applies an INCR/DECR/INCRBY/DECRBY from the AOF. It
// mirrors commands.increment's logic directly since that function is
// unexported in the commands package.
func replayIncrement(db *commands.Database, key string, amount int) {
	db.Mu.Lock()
	defer db.Mu.Unlock()

	val, ok := db.Store[key]
	if !ok {
		val = "0"
	}

	n, err := parseInt(val)
	if err != nil {
		return
	}

	db.Store[key] = strconv.Itoa(n + amount)
}

// replayListPop re-applies a pop that already happened when the AOF entry
// was written. The specific value popped isn't stored in the log, so on
// replay we simply pop from the first key with data - mirroring the same
// approximation already used for SPOP above.
func replayListPop(args []resp.Value, db *commands.Database, left bool) {
	for i := 0; i < len(args); i++ {
		key := args[i].Str
		list, exists := db.GetList(key)
		if !exists {
			continue
		}

		var ok bool
		if left {
			_, ok = list.LPop()
		} else {
			_, ok = list.RPop()
		}
		if !ok {
			continue
		}

		if list.LLen() == 0 {
			db.Mu.Lock()
			delete(db.Lists, key)
			db.Mu.Unlock()
		}
		return
	}
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	result := 0
	negative := false
	start := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		start = 1
	} else if len(s) > 0 && s[0] == '+' {
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, &parseIntError{s}
		}
		result = result*10 + int(s[i]-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}

type parseIntError struct{ value string }

func (e *parseIntError) Error() string { return "invalid integer: " + e.value }

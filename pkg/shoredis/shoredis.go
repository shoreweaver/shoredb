// Package shoredis lets another Go program embed shoredb directly, in
// process, with no TCP listener and no RESP encoding/decoding overhead.
//
//	client := shoredis.New()
//	client.Set("hello", "world")
//	value, ok, _ := client.Get("hello")
//
// Or, to also get RDB persistence backed by a redis.config file:
//
//	client, err := shoredis.Open("redis.config")
package shoredis

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/shoreweaver/shoredb/pkg/commands"
	"github.com/shoreweaver/shoredb/pkg/persistence"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

// Client is an in-process shoredb client. It is safe for concurrent use by
// multiple goroutines, except that Select changes which logical database
// *this* Client's subsequent calls apply to - if you need different
// goroutines to work against different databases concurrently, give each
// goroutine its own Client (see NewFrom) rather than sharing one and
// calling Select from multiple places.
type Client struct {
	mdb *commands.MultiDB
	rdb *persistence.RDB

	mu      sync.Mutex
	dbIndex int
}

// New creates a purely in-memory Client with commands.NumDatabases logical
// databases and no persistence.
func New() *Client {
	return &Client{mdb: commands.NewMultiDB()}
}

// Open creates a Client backed by the given redis.config file. If the
// config enables RDB snapshotting (the default, absent "appendonly yes"),
// any existing snapshot at the configured path is loaded immediately, and
// Client.Save/Client.Close will persist to it.
//
// AOF is intentionally not wired up here: an embedded, in-process client
// has no long-running server loop to drive it, so RDB (explicit Save, or
// Close on the way out) is the persistence model for embedding.
func Open(configPath string) (*Client, error) {
	config := persistence.ReadConfig(configPath)
	mdb := commands.NewMultiDB()

	rdb, err := persistence.NewRDB(config, mdb)
	if err != nil {
		return nil, fmt.Errorf("shoredis: opening %s: %w", configPath, err)
	}

	return &Client{mdb: mdb, rdb: rdb}, nil
}

// NewFrom returns a new Client sharing the same underlying databases (and
// persistence, if any) as c, but with its own independently-selectable
// current database. Use this to give concurrent goroutines their own
// SELECT state without them stepping on each other.
func (c *Client) NewFrom() *Client {
	return &Client{mdb: c.mdb, rdb: c.rdb}
}

// Select switches which of the commands.NumDatabases (0-15) logical
// databases this Client's subsequent calls operate against.
func (c *Client) Select(index int) error {
	if index < 0 || index >= commands.NumDatabases {
		return fmt.Errorf("shoredis: DB index %d out of range (0-%d)", index, commands.NumDatabases-1)
	}
	c.mu.Lock()
	c.dbIndex = index
	c.mu.Unlock()
	return nil
}

func (c *Client) currentDB() *commands.Database {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mdb.DB(c.dbIndex)
}

// Save writes an RDB snapshot immediately. It errors if this Client wasn't
// created with Open (i.e. has no persistence configured).
func (c *Client) Save() error {
	if c.rdb == nil {
		return errors.New("shoredis: no persistence configured (create the client with Open, not New)")
	}
	return c.rdb.Save()
}

// Close saves a final snapshot (if persistence is configured) and releases
// background resources. A Client created with New has nothing to release
// and Close is a no-op.
func (c *Client) Close() error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// Result wraps a raw RESP reply from a command handler, with typed
// accessors. All of Do's callers, and every typed convenience method below,
// go through this.
type Result struct {
	raw resp.Value
}

// Err returns non-nil if the command replied with a RESP error.
func (r Result) Err() error {
	if r.raw.Type == resp.SimpleError {
		return errors.New(r.raw.Str)
	}
	return nil
}

// IsNil reports whether the reply was a RESP null (e.g. GET on a missing
// key).
func (r Result) IsNil() bool {
	return r.raw.Type == resp.Null
}

// String returns the reply's string value (bulk or simple string).
func (r Result) String() string {
	return r.raw.Str
}

// Int returns the reply's integer value.
func (r Result) Int() int {
	return r.raw.Num
}

// Strings returns the reply's array elements as strings, for commands that
// reply with an array of bulk strings (LRANGE, SMEMBERS, KEYS, ...).
func (r Result) Strings() []string {
	out := make([]string, len(r.raw.Array))
	for i, v := range r.raw.Array {
		out[i] = v.Str
	}
	return out
}

// Do executes any supported command by name (case-insensitive) with string
// arguments, and is how new/less-common commands can be reached without a
// dedicated typed method below.
func (c *Client) Do(cmd string, args ...string) (Result, error) {
	handlerArgs := make([]resp.Value, len(args))
	for i, a := range args {
		handlerArgs[i] = resp.Value{Type: resp.BulkString, Str: a}
	}

	db := c.currentDB()

	if rdbHandler, ok := commands.RDBCommandHandlers[normalizeCmd(cmd)]; ok {
		return Result{raw: rdbHandler(handlerArgs, db, c.rdb)}, nil
	}

	handler, ok := commands.CommandHandlers[normalizeCmd(cmd)]
	if !ok {
		return Result{}, fmt.Errorf("shoredis: unknown command %q", cmd)
	}

	return Result{raw: handler(handlerArgs, db)}, nil
}

func normalizeCmd(cmd string) string {
	upper := make([]byte, len(cmd))
	for i := 0; i < len(cmd); i++ {
		b := cmd[i]
		if b >= 'a' && b <= 'z' {
			b -= 'a' - 'A'
		}
		upper[i] = b
	}
	return string(upper)
}

// --- Typed convenience methods -------------------------------------------------

func (c *Client) Set(key, value string) error {
	res, err := c.Do("SET", key, value)
	if err != nil {
		return err
	}
	return res.Err()
}

func (c *Client) Get(key string) (value string, ok bool, err error) {
	res, err := c.Do("GET", key)
	if err != nil {
		return "", false, err
	}
	if e := res.Err(); e != nil {
		return "", false, e
	}
	if res.IsNil() {
		return "", false, nil
	}
	return res.String(), true, nil
}

func (c *Client) Del(keys ...string) (int, error) {
	res, err := c.Do("DEL", keys...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) Exists(keys ...string) (int, error) {
	res, err := c.Do("EXISTS", keys...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) Incr(key string) (int, error) {
	res, err := c.Do("INCR", key)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) IncrBy(key string, delta int) (int, error) {
	res, err := c.Do("INCRBY", key, strconv.Itoa(delta))
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) Decr(key string) (int, error) {
	res, err := c.Do("DECR", key)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) DecrBy(key string, delta int) (int, error) {
	res, err := c.Do("DECRBY", key, strconv.Itoa(delta))
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) LPush(key string, values ...string) (int, error) {
	res, err := c.Do("LPUSH", append([]string{key}, values...)...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) RPush(key string, values ...string) (int, error) {
	res, err := c.Do("RPUSH", append([]string{key}, values...)...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) LPop(key string) (value string, ok bool, err error) {
	res, err := c.Do("LPOP", key)
	if err != nil {
		return "", false, err
	}
	if e := res.Err(); e != nil {
		return "", false, e
	}
	if res.IsNil() {
		return "", false, nil
	}
	return res.String(), true, nil
}

func (c *Client) RPop(key string) (value string, ok bool, err error) {
	res, err := c.Do("RPOP", key)
	if err != nil {
		return "", false, err
	}
	if e := res.Err(); e != nil {
		return "", false, e
	}
	if res.IsNil() {
		return "", false, nil
	}
	return res.String(), true, nil
}

func (c *Client) LRange(key string, start, stop int) ([]string, error) {
	res, err := c.Do("LRANGE", key, strconv.Itoa(start), strconv.Itoa(stop))
	if err != nil {
		return nil, err
	}
	if e := res.Err(); e != nil {
		return nil, e
	}
	return res.Strings(), nil
}

func (c *Client) LLen(key string) (int, error) {
	res, err := c.Do("LLEN", key)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) SAdd(key string, members ...string) (int, error) {
	res, err := c.Do("SADD", append([]string{key}, members...)...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) SRem(key string, members ...string) (int, error) {
	res, err := c.Do("SREM", append([]string{key}, members...)...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) SMembers(key string) ([]string, error) {
	res, err := c.Do("SMEMBERS", key)
	if err != nil {
		return nil, err
	}
	if e := res.Err(); e != nil {
		return nil, e
	}
	return res.Strings(), nil
}

func (c *Client) SIsMember(key, member string) (bool, error) {
	res, err := c.Do("SISMEMBER", key, member)
	if err != nil {
		return false, err
	}
	return res.Int() == 1, res.Err()
}

func (c *Client) HSet(key, field, value string) (isNew bool, err error) {
	res, err := c.Do("HSET", key, field, value)
	if err != nil {
		return false, err
	}
	return res.Int() == 1, res.Err()
}

func (c *Client) HGet(key, field string) (value string, ok bool, err error) {
	res, err := c.Do("HGET", key, field)
	if err != nil {
		return "", false, err
	}
	if e := res.Err(); e != nil {
		return "", false, e
	}
	if res.IsNil() {
		return "", false, nil
	}
	return res.String(), true, nil
}

func (c *Client) HGetAll(key string) (map[string]string, error) {
	res, err := c.Do("HGETALL", key)
	if err != nil {
		return nil, err
	}
	if e := res.Err(); e != nil {
		return nil, e
	}
	flat := res.raw.Array
	out := make(map[string]string, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		out[flat[i].Str] = flat[i+1].Str
	}
	return out, nil
}

func (c *Client) HDel(key string, fields ...string) (int, error) {
	res, err := c.Do("HDEL", append([]string{key}, fields...)...)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

// Expire sets a TTL of seconds on key. It returns false if key doesn't
// exist.
func (c *Client) Expire(key string, seconds int) (bool, error) {
	res, err := c.Do("EXPIRE", key, strconv.Itoa(seconds))
	if err != nil {
		return false, err
	}
	return res.Int() == 1, res.Err()
}

// TTL returns key's remaining time-to-live in seconds, -1 if it has no
// expiry, or -2 if it doesn't exist.
func (c *Client) TTL(key string) (int, error) {
	res, err := c.Do("TTL", key)
	if err != nil {
		return 0, err
	}
	return res.Int(), res.Err()
}

func (c *Client) Persist(key string) (bool, error) {
	res, err := c.Do("PERSIST", key)
	if err != nil {
		return false, err
	}
	return res.Int() == 1, res.Err()
}

func (c *Client) Type(key string) (string, error) {
	res, err := c.Do("TYPE", key)
	if err != nil {
		return "", err
	}
	return res.String(), res.Err()
}

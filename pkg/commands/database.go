package commands

import (
	"sync"
	"time"

	"github.com/shoreweaver/shoredb/pkg/datastruct"
	"github.com/shoreweaver/shoredb/pkg/pubsub"
)

// NumDatabases is the number of logical databases a server exposes, matching
// stock Redis's default of 16 (SELECT 0-15).
const NumDatabases = 16

// Database represents a single logical database (one SELECT-able slot).
// Each Database owns its own mutex so that operations against one logical
// database (including an RDB snapshot walk) never contend with operations
// against another.
type Database struct {
	Store   map[string]string
	Lists   map[string]*datastruct.List
	Sets    map[string]*datastruct.Set
	Hashes  map[string]*datastruct.Hash
	Expires map[string]time.Time
	Mu      *sync.RWMutex

	// PubSub is intentionally the *same* instance across every logical
	// database within a MultiDB: real Redis PUBLISH/SUBSCRIBE is global and
	// is not scoped to whichever database happens to be SELECTed.
	PubSub *pubsub.PubSub

	// listMu/listCond back the blocking list pops (BLPOP/BRPOP). Any
	// LPUSH/RPUSH broadcasts on listCond so a blocked popper wakes up
	// immediately instead of polling on a timer.
	listMu   sync.Mutex
	listCond *sync.Cond
}

func newDatabase(ps *pubsub.PubSub) *Database {
	db := &Database{
		Store:   make(map[string]string),
		Lists:   make(map[string]*datastruct.List),
		Sets:    make(map[string]*datastruct.Set),
		Hashes:  make(map[string]*datastruct.Hash),
		Expires: make(map[string]time.Time),
		Mu:      &sync.RWMutex{},
		PubSub:  ps,
	}
	db.listCond = sync.NewCond(&db.listMu)
	return db
}

// NewDatabase creates a single, standalone logical database with its own
// PubSub instance. Most callers building a full server should use
// NewMultiDB instead; this is kept for callers (and tests) that only need
// one logical database.
func NewDatabase() *Database {
	return newDatabase(pubsub.New())
}

// signalListPush wakes any goroutines parked in a blocking pop (BLPOP/BRPOP)
// on this database. Called after any LPUSH/RPUSH.
func (db *Database) signalListPush() {
	db.listMu.Lock()
	db.listCond.Broadcast()
	db.listMu.Unlock()
}

// waitForListSignal parks the calling goroutine until either a push is
// signaled on this database or, if hasDeadline is set, the deadline elapses.
// It replaces a fixed-interval sleep-and-poll loop with an event-driven
// wait so BLPOP/BRPOP wake up as soon as data is available.
func (db *Database) waitForListSignal(deadline time.Time, hasDeadline bool) {
	db.listMu.Lock()
	defer db.listMu.Unlock()

	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		timer := time.AfterFunc(remaining, func() {
			db.listMu.Lock()
			db.listCond.Broadcast()
			db.listMu.Unlock()
		})
		db.listCond.Wait()
		timer.Stop()
		return
	}

	db.listCond.Wait()
}

// MultiDB holds all NumDatabases logical databases plus the single PubSub
// broker shared across all of them, mirroring Redis's SELECT-able database
// model.
type MultiDB struct {
	DBs    [NumDatabases]*Database
	PubSub *pubsub.PubSub
}

// NewMultiDB builds a MultiDB with NumDatabases logical databases (indices
// 0..NumDatabases-1), all sharing one PubSub broker.
func NewMultiDB() *MultiDB {
	ps := pubsub.New()
	m := &MultiDB{PubSub: ps}
	for i := range m.DBs {
		m.DBs[i] = newDatabase(ps)
	}
	return m
}

// DB returns the logical database at the given index. The index is not
// bounds-checked; callers (SELECT validation in pkg/server) are expected to
// have already validated 0 <= index < NumDatabases.
func (m *MultiDB) DB(index int) *Database {
	return m.DBs[index]
}

// SweepExpired evicts expired keys across every logical database.
func (m *MultiDB) SweepExpired() {
	for _, db := range m.DBs {
		db.SweepExpired()
	}
}

// NumDatabases and DatabaseAt satisfy persistence.MultiDatabase without
// pkg/commands importing pkg/persistence (which would create an import
// cycle, since pkg/persistence intentionally has no dependency on
// pkg/commands). DatabaseAt returns interface{} so the persistence package
// can type-assert it against its own (structurally identical) Database
// interface - the same boxing pattern already used by
// GetOrCreate{List,Set,Hash}Iface below.
func (m *MultiDB) NumDatabases() int {
	return len(m.DBs)
}

func (m *MultiDB) DatabaseAt(index int) interface{} {
	return m.DBs[index]
}

func (db *Database) GetStringStore() *map[string]string {
	return &db.Store
}

func (db *Database) GetListStore() interface{} {
	result := make(map[string]interface{}, len(db.Lists))
	for k, v := range db.Lists {
		result[k] = v
	}
	return result
}

func (db *Database) GetSetStore() interface{} {
	result := make(map[string]interface{}, len(db.Sets))
	for k, v := range db.Sets {
		result[k] = v
	}
	return result
}

func (db *Database) GetHashStore() interface{} {
	result := make(map[string]interface{}, len(db.Hashes))
	for k, v := range db.Hashes {
		result[k] = v
	}
	return result
}

func (db *Database) GetMutex() *sync.RWMutex {
	return db.Mu
}

// GetOrCreateListIface/SetIface/HashIface exist purely so the persistence
// package (which only depends on interface{}-shaped methods to avoid an
// import of pkg/datastruct) can get-or-create a container during RDB load.
// A method declared to return interface{} can box a concrete *datastruct.X
// value; a type assertion against a method returning the concrete type
// cannot be satisfied by these methods, which is why those existed as a
// bug previously - see GetOrCreateList/GetOrCreateSet/GetOrCreateHash below
// for the concrete-typed versions used everywhere else in this package.
func (db *Database) GetOrCreateListIface(key string) interface{} {
	return db.GetOrCreateList(key)
}

func (db *Database) GetOrCreateSetIface(key string) interface{} {
	return db.GetOrCreateSet(key)
}

func (db *Database) GetOrCreateHashIface(key string) interface{} {
	return db.GetOrCreateHash(key)
}

// expireIfNeeded lazily evicts key if its TTL has passed.
func (db *Database) expireIfNeeded(key string) {
	db.Mu.RLock()
	exp, ok := db.Expires[key]
	db.Mu.RUnlock()

	if !ok || time.Now().Before(exp) {
		return
	}

	db.Mu.Lock()
	delete(db.Store, key)
	delete(db.Lists, key)
	delete(db.Sets, key)
	delete(db.Hashes, key)
	delete(db.Expires, key)
	db.Mu.Unlock()
}

// SweepExpired actively evicts all keys whose TTL has passed.
func (db *Database) SweepExpired() {
	db.Mu.RLock()
	now := time.Now()
	expired := make([]string, 0)
	for k, exp := range db.Expires {
		if now.After(exp) {
			expired = append(expired, k)
		}
	}
	db.Mu.RUnlock()

	for _, k := range expired {
		db.expireIfNeeded(k)
	}
}

func (db *Database) GetOrCreateList(key string) *datastruct.List {
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	if list, exists := db.Lists[key]; exists {
		return list
	}

	list := datastruct.NewList()
	db.Lists[key] = list
	return list
}

func (db *Database) GetList(key string) (*datastruct.List, bool) {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	list, exists := db.Lists[key]
	return list, exists
}

func (db *Database) GetOrCreateSet(key string) *datastruct.Set {
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	if set, exists := db.Sets[key]; exists {
		return set
	}

	set := datastruct.NewSet()
	db.Sets[key] = set
	return set
}

func (db *Database) GetSet(key string) (*datastruct.Set, bool) {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	set, exists := db.Sets[key]
	return set, exists
}

func (db *Database) GetOrCreateHash(key string) *datastruct.Hash {
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	if hash, exists := db.Hashes[key]; exists {
		return hash
	}

	hash := datastruct.NewHash()
	db.Hashes[key] = hash
	return hash
}

func (db *Database) GetHash(key string) (*datastruct.Hash, bool) {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	hash, exists := db.Hashes[key]
	return hash, exists
}

func (db *Database) DeleteKey(key string) int {
	db.expireIfNeeded(key)

	db.Mu.Lock()
	defer db.Mu.Unlock()

	deleted := 0

	if _, exists := db.Store[key]; exists {
		delete(db.Store, key)
		deleted++
	}

	if _, exists := db.Lists[key]; exists {
		delete(db.Lists, key)
		deleted++
	}

	if _, exists := db.Sets[key]; exists {
		delete(db.Sets, key)
		deleted++
	}

	if _, exists := db.Hashes[key]; exists {
		delete(db.Hashes, key)
		deleted++
	}

	delete(db.Expires, key)

	return deleted
}

func (db *Database) KeyExists(key string) bool {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	if _, exists := db.Store[key]; exists {
		return true
	}
	if _, exists := db.Lists[key]; exists {
		return true
	}
	if _, exists := db.Sets[key]; exists {
		return true
	}
	if _, exists := db.Hashes[key]; exists {
		return true
	}

	return false
}

func (db *Database) GetKeyType(key string) string {
	db.expireIfNeeded(key)

	db.Mu.RLock()
	defer db.Mu.RUnlock()

	if _, exists := db.Store[key]; exists {
		return "string"
	}
	if _, exists := db.Lists[key]; exists {
		return "list"
	}
	if _, exists := db.Sets[key]; exists {
		return "set"
	}
	if _, exists := db.Hashes[key]; exists {
		return "hash"
	}

	return "none"
}

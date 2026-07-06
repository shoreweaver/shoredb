package persistence

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

const (
	// rdbHeader is 12 bytes: "DATABASE" (8 bytes) + a 4-digit version
	// ("0009"). Load() previously only read the first 9 bytes of this
	// header, leaving 3 stray version-string bytes sitting in the stream
	// in front of the first real opcode. Every real save/load round-trip
	// then failed immediately with "unknown opcode: 0x30" (0x30 is ASCII
	// '0', the first of those 3 leftover bytes). Load() now reads the
	// full len(rdbHeader) bytes so the stream is correctly aligned before
	// opcode parsing begins.
	rdbHeader = "DATABASE0009"

	opSelectDB = 0xFE
	opEOF      = 0xFF
	opResizeDB = 0xFB

	typeString = 0
	typeList   = 1
	typeSet    = 2
	typeHash   = 4
)

// Database is the minimal surface persistence needs from a single logical
// database in the commands package. It's expressed as an interface (rather
// than importing pkg/commands directly) to avoid a persistence <-> commands
// import cycle.
//
// NOTE: GetOrCreate*Iface methods must be declared to return interface{}.
// commands.Database.GetOrCreateList/Set/Hash return concrete
// *datastruct.List/Set/Hash types, and Go's interface satisfaction is not
// covariant - a method returning a concrete type can NEVER satisfy an
// interface method declared to return interface{}, even though the value
// itself could be boxed into one. That mismatch is what silently broke RDB
// loading previously: the type assertion against such an interface always
// failed, so lists/sets/hashes were never recreated on Load().
type Database interface {
	GetStringStore() *map[string]string
	GetListStore() interface{}
	GetSetStore() interface{}
	GetHashStore() interface{}
	GetMutex() *sync.RWMutex
	GetOrCreateListIface(key string) interface{}
	GetOrCreateSetIface(key string) interface{}
	GetOrCreateHashIface(key string) interface{}
}

// MultiDatabase is the multi-database surface persistence needs from
// commands.MultiDB. DatabaseAt returns interface{} (boxing a concrete
// *commands.Database) rather than Database directly, again to avoid
// persistence depending on commands: commands.MultiDB satisfies this
// interface structurally without ever importing this package.
type MultiDatabase interface {
	NumDatabases() int
	DatabaseAt(index int) interface{}
}

type RDB struct {
	path     string
	mu       sync.Mutex
	changes  int
	lastSave time.Time
	stopCh   chan struct{}
	db       MultiDatabase
}

func NewRDB(config *Config, db MultiDatabase) (*RDB, error) {
	path := config.GetFullPath(config.RDBFilename())

	r := &RDB{
		path:     path,
		lastSave: time.Now(),
		stopCh:   make(chan struct{}),
		db:       db,
	}

	if err := r.Load(); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("RDB: Load warning: %v", err)
		}
	}

	go r.snapshotLoop(config)

	return r, nil
}

func NewRDBWithoutLoad(config *Config, db MultiDatabase) (*RDB, error) {
	path := config.GetFullPath(config.RDBFilename())

	r := &RDB{
		path:     path,
		lastSave: time.Now(),
		stopCh:   make(chan struct{}),
		db:       db,
	}

	log.Printf("RDB: Skipping load (AOF enabled)")

	go r.snapshotLoop(config)

	return r, nil
}

func (r *RDB) IncrementChanges() {
	r.mu.Lock()
	r.changes++
	r.mu.Unlock()
}

func (r *RDB) snapshotLoop(config *Config) {
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			elapsed := time.Since(r.lastSave).Seconds()
			changes := r.changes
			r.mu.Unlock()

			for _, s := range config.RDBSnapshots() {
				if elapsed >= float64(s.Secs) && changes >= s.KeysChanged {
					log.Printf("RDB: Snapshot condition met. Saving...")
					if err := r.Save(); err == nil {
						r.mu.Lock()
						r.lastSave = time.Now()
						r.changes = 0
						r.mu.Unlock()
					} else {
						log.Printf("RDB: Save error: %v", err)
					}
					break
				}
			}
		case <-r.stopCh:
			return
		}
	}
}

// dbAt type-asserts db.DatabaseAt(index) against Database, boxing/unboxing
// through interface{} to keep this package independent of pkg/commands.
func (r *RDB) dbAt(index int) (Database, bool) {
	d, ok := r.db.DatabaseAt(index).(Database)
	return d, ok
}

func (r *RDB) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("RDB: Starting snapshot save to %s", r.path)

	tmpPath := r.path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	bw := bufio.NewWriter(f)
	ew := &errWriter{w: bw}

	ew.write(rdbHeader)

	totalKeysAll := 0

	// Each logical database is snapshotted independently, locking only
	// that database's own mutex for the duration of its walk. Previously
	// a single lock spanned the entire dataset (all databases sat behind
	// one mutex), so a snapshot on any one logical database stalled
	// traffic on every other one too. Locking per-database means a
	// snapshot of db 3 no longer blocks reads/writes against db 0.
	for i := 0; i < r.db.NumDatabases(); i++ {
		db, ok := r.dbAt(i)
		if !ok {
			continue
		}

		keys := r.saveDatabase(ew, i, db)
		totalKeysAll += keys
	}

	ew.write(byte(opEOF))
	ew.write(uint64(0))

	if ew.err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write error: %w", ew.err)
	}

	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("flush error: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync error: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close error: %w", err)
	}

	if err := os.Rename(tmpPath, r.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename error: %w", err)
	}

	log.Printf("RDB: Snapshot saved successfully (%d keys across %d databases)", totalKeysAll, r.db.NumDatabases())
	return nil
}

// saveDatabase writes one logical database's SELECT marker, resize hint,
// and keys, holding only that database's own mutex.
func (r *RDB) saveDatabase(ew *errWriter, index int, db Database) int {
	mu := db.GetMutex()
	mu.RLock()
	defer mu.RUnlock()

	stringStore := db.GetStringStore()
	listStore, _ := db.GetListStore().(map[string]interface{})
	setStore, _ := db.GetSetStore().(map[string]interface{})
	hashStore, _ := db.GetHashStore().(map[string]interface{})

	totalKeys := 0
	if stringStore != nil {
		totalKeys += len(*stringStore)
	}
	totalKeys += len(listStore) + len(setStore) + len(hashStore)

	if totalKeys == 0 {
		return 0
	}

	ew.write(byte(opSelectDB))
	r.writeLength(ew, index)

	ew.write(byte(opResizeDB))
	r.writeLength(ew, totalKeys)
	r.writeLength(ew, 0)

	if stringStore != nil {
		for key, value := range *stringStore {
			ew.write(byte(typeString))
			r.writeString(ew, key)
			r.writeString(ew, value)
		}
	}

	for key, listObj := range listStore {
		var items []string
		if list, ok := listObj.(interface{ LRange(int, int) []string }); ok {
			items = list.LRange(0, -1)
		}

		if len(items) > 0 {
			ew.write(byte(typeList))
			r.writeString(ew, key)
			r.writeLength(ew, len(items))
			for _, item := range items {
				r.writeString(ew, item)
			}
		}
	}

	for key, setObj := range setStore {
		var members []string
		if set, ok := setObj.(interface{ SMembers() []string }); ok {
			members = set.SMembers()
		}

		if len(members) > 0 {
			ew.write(byte(typeSet))
			r.writeString(ew, key)
			r.writeLength(ew, len(members))
			for _, member := range members {
				r.writeString(ew, member)
			}
		}
	}

	for key, hashObj := range hashStore {
		var fields map[string]string
		if hash, ok := hashObj.(interface{ HGetAll() map[string]string }); ok {
			fields = hash.HGetAll()
		}

		if len(fields) > 0 {
			ew.write(byte(typeHash))
			r.writeString(ew, key)
			r.writeLength(ew, len(fields))
			for field, value := range fields {
				r.writeString(ew, field)
				r.writeString(ew, value)
			}
		}
	}

	return totalKeys
}

func (r *RDB) Load() error {
	f, err := os.Open(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf("RDB: Loading snapshot from %s", r.path)

	rd := bufio.NewReader(f)

	header := make([]byte, len(rdbHeader))
	if _, err := io.ReadFull(rd, header); err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	if string(header[:8]) != "DATABASE" {
		return fmt.Errorf("invalid RDB header: expected DATABASE, got %s", string(header[:8]))
	}

	keysLoaded := 0
	currentDB := 0

	for {
		op, err := rd.ReadByte()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("unexpected EOF")
			}
			return err
		}

		switch op {
		case opEOF:
			var checksum uint64
			if err := binary.Read(rd, binary.BigEndian, &checksum); err != nil {
				return fmt.Errorf("read EOF checksum: %w", err)
			}
			log.Printf("RDB: Loaded %d keys successfully", keysLoaded)
			return nil

		case opSelectDB:
			dbNum, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read SELECTDB index: %w", err)
			}
			if dbNum < 0 || dbNum >= r.db.NumDatabases() {
				return fmt.Errorf("database index %d out of range (0-%d)", dbNum, r.db.NumDatabases()-1)
			}
			currentDB = dbNum

		case opResizeDB:
			dbSize, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read resize db_size: %w", err)
			}
			expiresSize, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read resize expires_size: %w", err)
			}
			log.Printf("RDB: Resize hint (db %d) - db_size: %d, expires: %d", currentDB, dbSize, expiresSize)

		case typeString:
			key, err := r.readString(rd)
			if err != nil {
				return fmt.Errorf("read string key: %w", err)
			}
			value, err := r.readString(rd)
			if err != nil {
				return fmt.Errorf("read string value: %w", err)
			}

			db, ok := r.dbAt(currentDB)
			if !ok {
				return fmt.Errorf("no database at index %d", currentDB)
			}

			mu := db.GetMutex()
			mu.Lock()
			stringStore := db.GetStringStore()
			if stringStore != nil {
				(*stringStore)[key] = value
			}
			mu.Unlock()
			keysLoaded++

		case typeList:
			key, err := r.readString(rd)
			if err != nil {
				return fmt.Errorf("read list key: %w", err)
			}

			length, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read list length: %w", err)
			}

			items := make([]string, length)
			for i := 0; i < length; i++ {
				item, err := r.readString(rd)
				if err != nil {
					return fmt.Errorf("read list item: %w", err)
				}
				items[i] = item
			}

			db, ok := r.dbAt(currentDB)
			if !ok {
				return fmt.Errorf("no database at index %d", currentDB)
			}

			// GetOrCreateListIface manages the database's own mutex
			// internally (via GetOrCreateList), so it must NOT be called
			// while that mutex is already held - doing so would deadlock
			// on a non-reentrant sync.RWMutex.
			if list, ok := db.GetOrCreateListIface(key).(interface{ RPush(...string) int }); ok {
				list.RPush(items...)
			}
			keysLoaded++

		case typeSet:
			key, err := r.readString(rd)
			if err != nil {
				return fmt.Errorf("read set key: %w", err)
			}

			length, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read set length: %w", err)
			}

			members := make([]string, length)
			for i := 0; i < length; i++ {
				member, err := r.readString(rd)
				if err != nil {
					return fmt.Errorf("read set member: %w", err)
				}
				members[i] = member
			}

			db, ok := r.dbAt(currentDB)
			if !ok {
				return fmt.Errorf("no database at index %d", currentDB)
			}

			if set, ok := db.GetOrCreateSetIface(key).(interface{ SAdd(...string) int }); ok {
				set.SAdd(members...)
			}
			keysLoaded++

		case typeHash:
			key, err := r.readString(rd)
			if err != nil {
				return fmt.Errorf("read hash key: %w", err)
			}

			length, err := r.readLength(rd)
			if err != nil {
				return fmt.Errorf("read hash length: %w", err)
			}

			fields := make(map[string]string, length)
			for i := 0; i < length; i++ {
				field, err := r.readString(rd)
				if err != nil {
					return fmt.Errorf("read hash field: %w", err)
				}
				value, err := r.readString(rd)
				if err != nil {
					return fmt.Errorf("read hash value: %w", err)
				}
				fields[field] = value
			}

			db, ok := r.dbAt(currentDB)
			if !ok {
				return fmt.Errorf("no database at index %d", currentDB)
			}

			if hash, ok := db.GetOrCreateHashIface(key).(interface{ HMSet(map[string]string) }); ok {
				hash.HMSet(fields)
			}
			keysLoaded++

		default:
			return fmt.Errorf("unknown opcode: 0x%02X", op)
		}
	}
}

func (r *RDB) Close() error {
	close(r.stopCh)
	return r.Save()
}

func (r *RDB) LastSave() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSave
}

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) write(data interface{}) {
	if ew.err != nil {
		return
	}
	switch v := data.(type) {
	case byte:
		ew.err = binary.Write(ew.w, binary.LittleEndian, v)
	case string:
		_, ew.err = io.WriteString(ew.w, v)
	default:
		ew.err = binary.Write(ew.w, binary.BigEndian, v)
	}
}

func (r *RDB) writeLength(ew *errWriter, n int) {
	if n < 64 {
		ew.write(byte(n))
	} else if n < 16384 {
		ew.write(byte((n >> 8) | 0x40))
		ew.write(byte(n & 0xFF))
	} else {
		ew.write(byte(0x80))
		ew.write(uint32(n))
	}
}

func (r *RDB) writeString(ew *errWriter, s string) {
	r.writeLength(ew, len(s))
	ew.write(s)
}

// readLength previously discarded every error from ReadByte/binary.Read,
// silently returning zero-value lengths on a truncated/corrupt stream
// instead of surfacing the failure. All three branches now propagate their
// read error to the caller.
func (r *RDB) readLength(rd *bufio.Reader) (int, error) {
	b, err := rd.ReadByte()
	if err != nil {
		return 0, err
	}
	switch (b & 0xC0) >> 6 {
	case 0:
		return int(b & 0x3F), nil
	case 1:
		b2, err := rd.ReadByte()
		if err != nil {
			return 0, err
		}
		return (int(b&0x3F) << 8) | int(b2), nil
	default:
		var n uint32
		if err := binary.Read(rd, binary.BigEndian, &n); err != nil {
			return 0, err
		}
		return int(n), nil
	}
}

// readString previously ignored the length-read error and the
// io.ReadFull error, so a truncated/corrupt stream produced a
// zero-length or partially-zeroed string instead of an error. Both are
// now propagated.
func (r *RDB) readString(rd *bufio.Reader) (string, error) {
	n, err := r.readLength(rd)
	if err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(rd, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

package persistence

import (
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shoreweaver/shoredb/pkg/resp"
)

type Aof struct {
	file   *os.File
	mutex  sync.Mutex
	fsync  FSyncMode
	stopCh chan struct{}

	// lastDB tracks which logical database the most recently written
	// command applies to, mirroring how real Redis's AOF works: a SELECT
	// pseudo-command is written ahead of a data command only when the
	// target database actually changes, rather than on every write.
	lastDB int
}

func NewAof(config *Config) (*Aof, error) {
	path := config.GetFullPath(config.AOFFilename())

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	aof := &Aof{
		file:   file,
		fsync:  config.AOFFsync(),
		stopCh: make(chan struct{}),
		lastDB: -1, // forces a SELECT ahead of the very first write
	}

	if config.AOFFsync() != No {
		go aof.syncLoop()
	}

	return aof, nil
}

func (aof *Aof) syncLoop() {
	var syncInterval time.Duration

	switch aof.fsync {
	case Always:
		syncInterval = 100 * time.Millisecond
	case EverySec:
		syncInterval = time.Second
	default:
		return
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			aof.mutex.Lock()
			if err := aof.file.Sync(); err != nil {
				log.Printf("Error syncing AOF file: %v", err)
			}
			aof.mutex.Unlock()
		case <-aof.stopCh:
			return
		}
	}
}

func (aof *Aof) Close() error {
	close(aof.stopCh)

	aof.mutex.Lock()
	defer aof.mutex.Unlock()

	return aof.file.Close()
}

// Write appends value to the log, tagged for logical database dbIndex. A
// SELECT pseudo-command is written first whenever dbIndex differs from the
// database the previous entry was written for.
func (aof *Aof) Write(dbIndex int, value resp.Value) error {
	aof.mutex.Lock()
	defer aof.mutex.Unlock()

	writer := resp.NewWriter(aof.file)

	if dbIndex != aof.lastDB {
		selectEntry := resp.Value{Type: resp.Array, Array: []resp.Value{
			{Type: resp.BulkString, Str: "SELECT"},
			{Type: resp.BulkString, Str: strconv.Itoa(dbIndex)},
		}}
		if err := writer.Write(selectEntry); err != nil {
			return err
		}
		aof.lastDB = dbIndex
	}

	return writer.Write(value)
}

// Read replays every logged command, invoking fn with the logical database
// index the command was recorded against. SELECT pseudo-entries update that
// index rather than being handed to fn.
func (aof *Aof) Read(fn func(dbIndex int, value resp.Value)) error {
	aof.mutex.Lock()
	defer aof.mutex.Unlock()

	if _, err := aof.file.Seek(0, 0); err != nil {
		return err
	}

	parser := resp.NewParser(aof.file)
	currentDB := 0

	for {
		value, err := parser.Parse()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if isSelectEntry(value) {
			if idx, err := strconv.Atoi(value.Array[1].Str); err == nil {
				currentDB = idx
			}
			continue
		}

		fn(currentDB, value)
	}
	return nil
}

func isSelectEntry(value resp.Value) bool {
	return value.Type == resp.Array &&
		len(value.Array) == 2 &&
		strings.EqualFold(value.Array[0].Str, "SELECT")
}

package server

import (
	"log"
	"time"

	"github.com/shoreweaver/shoredb/pkg/commands"
	"github.com/shoreweaver/shoredb/pkg/persistence"
	"github.com/shoreweaver/shoredb/pkg/resp"
)

// Options configures a Server so it can be embedded in other Go projects
// without relying on a fixed port or config file location.
type Options struct {
	ListenAddr string // e.g. ":6379". Defaults to ":6379".
	ConfigPath string // path to a redis.config file. Defaults to "redis.config".
}

type Server struct {
	listenAddr  string
	db          *commands.MultiDB
	aof         *persistence.Aof
	rdb         *persistence.RDB
	requirePass string
	connCounter int64
	expiryStop  chan struct{}
}

// New builds a Server from the given Options, loading any existing
// RDB/AOF persistence found at the configured path.
func New(opts Options) *Server {
	if opts.ListenAddr == "" {
		opts.ListenAddr = ":6379"
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = "redis.config"
	}

	config := persistence.ReadConfig(opts.ConfigPath)
	db := commands.NewMultiDB()

	var rdb *persistence.RDB
	var aof *persistence.Aof

	if config.AOFEnabled() {
		rdbHandler, err := persistence.NewRDBWithoutLoad(config, db)
		if err != nil {
			log.Printf("Error initializing RDB: %v", err)
		} else {
			rdb = rdbHandler
			log.Printf("RDB initialized (AOF takes precedence for loading)")
		}

		aofHandler, err := persistence.NewAof(config)
		if err != nil {
			log.Printf("Error initializing AOF: %v", err)
		} else {
			aof = aofHandler
			if err := aof.Read(func(dbIndex int, value resp.Value) {
				replayCommand(value, db.DB(dbIndex))
			}); err != nil {
				log.Printf("Error reading AOF: %v", err)
			} else {
				log.Printf("AOF loaded and replayed successfully")
			}
		}
	} else {
		rdbHandler, err := persistence.NewRDB(config, db)
		if err != nil {
			log.Printf("Error initializing RDB: %v", err)
		} else {
			rdb = rdbHandler
			log.Printf("RDB initialized and loaded successfully")
		}
	}

	s := &Server{
		listenAddr:  opts.ListenAddr,
		db:          db,
		aof:         aof,
		rdb:         rdb,
		requirePass: config.RequirePass(),
		expiryStop:  make(chan struct{}),
	}

	go s.expiryLoop()
	return s
}

func (s *Server) expiryLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.db.SweepExpired()
		case <-s.expiryStop:
			return
		}
	}
}

func (s *Server) Shutdown() error {
	log.Println("Shutting down server...")

	close(s.expiryStop)

	if s.aof != nil {
		if err := s.aof.Close(); err != nil {
			log.Printf("Error closing AOF: %v", err)
		}
	}

	if s.rdb != nil {
		if err := s.rdb.Close(); err != nil {
			log.Printf("Error saving final RDB snapshot: %v", err)
			return err
		}
	}

	return nil
}

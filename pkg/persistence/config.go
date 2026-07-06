package persistence

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FSyncMode string

const (
	Always   FSyncMode = "always"
	EverySec FSyncMode = "everysec"
	No       FSyncMode = "no"
)

type RDBSnapshot struct {
	Secs        int
	KeysChanged int
}

type Config struct {
	dir         string
	rdb         []RDBSnapshot
	rdbFn       string
	aofEnabled  bool
	aofFn       string
	aofFsync    FSyncMode
	requirePass string
}

func NewConfig() *Config {
	return &Config{
		dir:      ".",
		aofFsync: EverySec,
		rdbFn:    "dump.rdb",
		aofFn:    "appendonly.aof",
	}
}

func ReadConfig(filename string) *Config {
	config := NewConfig()
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Config: %s not found, using defaults", filename)
		return config
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parseLine(scanner.Text(), config)
	}

	if config.dir != "." && config.dir != "" {
		os.MkdirAll(config.dir, 0755)
	}
	return config
}

func (c *Config) GetFullPath(filename string) string {
	return filepath.Join(c.dir, filename)
}

// Getters
func (c *Config) Dir() string                 { return c.dir }
func (c *Config) RDBFilename() string         { return c.rdbFn }
func (c *Config) AOFFilename() string         { return c.aofFn }
func (c *Config) AOFEnabled() bool            { return c.aofEnabled }
func (c *Config) AOFFsync() FSyncMode         { return c.aofFsync }
func (c *Config) RDBSnapshots() []RDBSnapshot { return c.rdb }
func (c *Config) RequirePass() string         { return c.requirePass }

func parseLine(line string, config *Config) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case "save":
		if len(parts) < 3 {
			return
		}
		s, _ := strconv.Atoi(parts[1])
		c, _ := strconv.Atoi(parts[2])
		config.rdb = append(config.rdb, RDBSnapshot{Secs: s, KeysChanged: c})
	case "dbfilename":
		config.rdbFn = parts[1]
	case "appendfilename":
		config.aofFn = parts[1]
	case "appendfsync":
		config.aofFsync = FSyncMode(parts[1])
	case "dir":
		config.dir = parts[1]
	case "appendonly":
		config.aofEnabled = (parts[1] == "yes")
	case "requirepass":
		config.requirePass = parts[1]
	}
}

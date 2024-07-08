package multibase

import (
	"time"

	"gorm.io/gorm/logger"
)

// NodeConf: Node config
type NodeConf struct {
	Host              string
	Port              int
	User              string
	Password          string
	Db                string
	Sslmode           string
	TimeZone          string // e.g. Asia/Shanghai
	DbMaxOpenConns    int
	DbMaxIdleConns    int
	DbConnMaxLifetime time.Duration
	DbLogger          logger.Interface
}

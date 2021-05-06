package config

import (
	"time"

	"gorm.io/gorm/logger"
)

type NodeConf struct {
	Host              string
	Port              int
	User              string
	Password          string
	Db                string
	Sslmode           string
	DbMaxOpenConns    int
	DbMaxIdleConns    int
	DbConnMaxLifetime time.Duration
	DbLogger          logger.Interface
}

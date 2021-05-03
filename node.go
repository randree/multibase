package database

import (
	"database/sql"

	"gorm.io/gorm"
)

type Node struct {
	name        string
	db          *gorm.DB
	sql         *sql.DB
	connpool    gorm.ConnPool
	queryCount  int64
	online      bool
	pingTries   int
	errorsCount int
}

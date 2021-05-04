package multibase

import (
	"database/sql"

	"gorm.io/gorm"
)

type Node struct {
	name        string
	host        string
	port        int
	db          *gorm.DB
	sql         *sql.DB
	connpool    gorm.ConnPool
	queryCount  int64
	online      bool
	pingTries   int
	errorsCount int
}

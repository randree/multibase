package multibase

// This is a multi database replication connector
// You can have multiple data sources and for each source there is a write/read configuration
// If a read database connection is lost other "reads" or the write take over
// When the connection is "repaired" it will reconnect
// I tried the following configuration W/R/R with 800 concurrent read requests.
// Then I switched off the first R and the second R so that the W took the full load.
// Except 40 errors there was no interruption. While on the request side we had 290618.
// That is a ratio of 0.014%!

// ToDo: Multiple write nodes. At this time only one write node is used

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	// c "app/database/structure"

	"github.com/randree/multibase/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB Database
)

func isTransaction(connPool gorm.ConnPool) bool {
	_, ok := connPool.(gorm.TxCommitter)
	return ok
}

// InitDb initializes data bases
func InitDb(databaseConfs config.DatabaseConf, debug bool) {

	// Initializing sources
	DB = NewDatabase()

	//Connect all databases according to config
	for databaseName, databaseConf := range databaseConfs {

		replication := NewReplication()
		// WRITE

		if len(databaseConf.Write) == 0 {
			log.Panic("MULTIBASE | No WRITE defined on " + databaseName)
		}

		for _, replicationConf := range databaseConf.Write {
			db, err := openNode(replicationConf)
			sql, sqlErr := db.DB()

			// No master node should be down, panic
			if err != nil || sqlErr != nil {
				log.Error("MULTIBASE | WRITE Connection error on " + databaseName + " (" + replicationConf.Host + ":" + strconv.Itoa(replicationConf.Port) + ")")
				log.Panic(err)
			}
			node := &Node{
				name:        fmt.Sprintf("%s:%d", replicationConf.Host, replicationConf.Port),
				host:        replicationConf.Host,
				port:        replicationConf.Port,
				db:          db,
				sql:         sql,
				connpool:    db.Statement.ConnPool,
				online:      SetStatus(err),
				pingTries:   0,
				errorsCount: 0,
			}

			// writer = append(writer, dbDatabaseObj)
			replication.AppendWriteNode(node)

			log.Info("MULTIBASE | Connected WRITE " + databaseName + " (" + node.name + ")")
		}

		// READ
		for _, replicationConf := range databaseConf.Read {
			db, err := openNode(replicationConf)
			sql, sqlErr := db.DB()

			// No panic if connection fails, but set offline
			if err != nil || sqlErr != nil {
				log.Error("MULTIBASE | READ Connection error on " + databaseName + " (" + replicationConf.Host + ":" + strconv.Itoa(replicationConf.Port) + ")")
				log.Error(err)
			}
			node := &Node{
				name:        fmt.Sprintf("%s:%d", replicationConf.Host, replicationConf.Port),
				host:        replicationConf.Host,
				port:        replicationConf.Port,
				db:          db,
				sql:         sql,
				connpool:    db.Statement.ConnPool,
				online:      SetStatus(err),
				pingTries:   0,
				errorsCount: 0,
			}

			replication.AppendReadNode(node)

			log.Info("MULTIBASE | Connected READ " + databaseName + " (" + node.name + ")")
			if db == nil {
				node.errorsCount++
				log.Error("MULTIBASE | ERROR READ " + databaseName + " (" + node.name + ")")
			}
		}

		// Append to Database list
		DB.AppendReplication(databaseName, replication)

		// Choose first master as gateway
		replication.gateway = replication.Write[0].db

		// Create a singleton for timer to prevent concurrency multiple checkings
		replication.readTickerActive = true
		// same for checking if a connection is still offline
		replication.readCheckerActive = true

		// If read replicas are available we divide all database interactions into read and write by using GORM callbacks
		if len(replication.Read) != 0 {

			replication.gateway.Callback().Query().Before("*").Register("distributor", func(db *gorm.DB) {
				if !isTransaction(db.Statement.ConnPool) {
					nextSource := getNextDbDatabase(replication.Read)
					if nextSource != nil {
						db.Statement.ConnPool = nextSource.connpool
						nextSource.queryCount++
						return
					}
					// If no READ found the default is write. So we do the count here
					replication.Write[0].queryCount++
				}
			})

			replication.gateway.Callback().Query().After("*").Register("errorhandler", func(db *gorm.DB) {
				// If an error occurs it could mean the loss of the connection
				// So we ping all connections to make sure there is no error
				// If a ping fails the connection will be marked as offline
				if !isTransaction(db.Statement.ConnPool) {
					if db.Error != nil {
						log.Error("MULTIBASE | error occurred ", db.Error)
						pingAllReadSetOfflineOnConnError(replication.Read)
						tickerToReconnect(&replication.readTickerActive, replication.Read)
					}
				}
			})

		} else {
			// If no READ is set
			replication.gateway.Callback().Query().Before("*").Register("distributor", func(db *gorm.DB) {
				if !isTransaction(db.Statement.ConnPool) {
					replication.Write[0].queryCount++
				}
			})
		}

	}
	if debug {
		// The logTicker shows the status of all nodes and refreshes it all x seconds
		logrus.SetOutput(ioutil.Discard)
		logTicker()
	}

}

func tickerToReconnect(active *bool, db []*Node) {
	if *active {
		*active = false
		go func() {
			for range time.Tick(4 * time.Second) {
				// Status()
				allOnline := true
				for _, replication := range db {
					if !replication.online {
						sql, _ := replication.db.DB()
						if err := sql.Ping(); err != nil {
							log.Error("is still offline: " + replication.name)
							replication.online = false
							allOnline = false
						} else {
							log.Warn("MULTIBASE | is reconnected and online now: " + replication.name)
							replication.online = true
						}
					}
				}
				if allOnline {
					// If all are online we can stop checking and reset the ticker
					*active = true
					return
				}
			}
		}()
	}
}

func pingAllReadSetOfflineOnConnError(db []*Node) {
	// If an error occurs ping all read databases if they are still alive
	for _, replication := range db {
		if replication.online {
			sql, _ := replication.db.DB()
			if err := sql.Ping(); err != nil {
				replication.errorsCount++
				log.Error("MULTIBASE | ping failed, switch to offline: " + replication.name)
				replication.online = false
			}
		}
	}
}

func logTicker() {
	go func() {
		for range time.Tick(4 * time.Second) {
			fmt.Println(Status())
		}
	}()
}

func Status() string {
	output := ""
	for databaseName, sourceData := range DB {
		for _, writeData := range sourceData.Write {
			output += fmt.Sprintln("MULTIBASE | STATUS ", databaseName, " \t WRITE \t database ", writeData.name, " : \t ", writeData.online, " \treads: ", writeData.queryCount, " \terrors: ", writeData.errorsCount)
		}
		for _, readData := range sourceData.Read {
			output += fmt.Sprintln("MULTIBASE | STATUS ", databaseName, " \t READ \t database ", readData.name, " : \t ", readData.online, " \treads: ", readData.queryCount, " \terrors: ", readData.errorsCount)
		}
	}
	return output
}

// If no error then status is online (true)
func SetStatus(err error) bool {
	return err == nil
}

// No need to build a round robin because of concurrency
// Returns a nil if all read databases are offline
// Then the fallback is using the write database
func getNextDbDatabase(db []*Node) *Node {
	// Collect all online
	tempDb := make([]*Node, 0)
	for _, replication := range db {
		if replication.online {
			tempDb = append(tempDb, replication)
		}
	}

	// If no read Database available give back a nil
	if len(tempDb) == 0 {
		return nil
	}
	randomIndex := rand.Intn(len(tempDb))
	return tempDb[randomIndex]
}

func Use(databaseName string) (*gorm.DB, error) {
	// Get db from the Database list
	db := DB.GetReplicationset(databaseName)
	if db == nil {
		return nil, errors.New("MULTIBASE | no database \"" + databaseName + "\" known")
	}

	// No gateway found
	if db.gateway == nil {
		return nil, errors.New("MULTIBASE | no gateway found at \"" + databaseName + "\"")
	}

	return db.gateway, nil
}

func UseNode(databaseName string, host string, port int) *Node {
	db := DB.GetReplicationset(databaseName)
	if db == nil {
		log.Error("MULTIBASE | no database \"" + databaseName + "\" known")
		return nil
	}
	for _, write := range db.Write {
		if write.host == host && write.port == port {
			return write
		}
	}

	for _, read := range db.Read {
		if read.host == host && read.port == port {
			return read
		}
	}
	log.Error("MULTIBASE | no node found at ", host, ":", port)
	return nil
}

func openNode(nodeConf *config.NodeConf) (*gorm.DB, error) {

	psqlDSN := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=%s",
		nodeConf.Host,
		nodeConf.Port,
		nodeConf.User,
		nodeConf.Password,
		nodeConf.Db,
		nodeConf.Sslmode)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: psqlDSN, // data Database name, refer https://github.com/jackc/pgx
		//PreferSimpleProtocol: true, // disables implicit prepared statement usage. By default pgx automatically uses the extended protocol
	}), &gorm.Config{
		Logger: nodeConf.DbLogger,
	})

	sql, _ := db.DB()

	//For more information http://go-database-sql.org/connection-pool.html
	sql.SetMaxOpenConns(nodeConf.DbMaxOpenConns)
	sql.SetMaxIdleConns(nodeConf.DbMaxIdleConns)
	sql.SetConnMaxLifetime(nodeConf.DbConnMaxLifetime)

	return db, err
}

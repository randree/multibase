package database

// This is a multi database replication connector
// You can have multible data sources and for each source there is a write/read configuration
// If a read database connection is lost other "reads" or the write take over
// When the connection is "repaired" it will reconnect
// I tried the following configuration W/R/R with 800 concurrent read requests.
// Then I switched off the first R and the second R so that the W took the full load.
// Execpt 40 errors there was no interruption. While on the request side we had 290618.
// That is a ratio of 0.014%!

// ToDo: Multible write nodes. At this time only one write node is used

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	// c "app/database/structure"

	"github.com/randree/multibase/config"
	"github.com/randree/multibase/logger"

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

// InitDb initalizes data bases
func InitDb(databaseConfs config.DatabaseConf, debug bool) {

	// Initializing sources
	DB = NewDatabase()

	//Connect all databases according to config
	for databaseName, databaseConf := range databaseConfs {

		replication := NewReplication()
		// WRITE

		if len(databaseConf.Write) == 0 {
			log.Panic("DATABASE | No WRITE defined on " + databaseName)
		}

		for _, replicationConf := range databaseConf.Write {
			db, err := openDB(replicationConf)
			sql, sqlErr := db.DB()

			// No master node should be down, panic
			if err != nil || sqlErr != nil {
				log.Error("DATABASE | WRITE Connection error on " + databaseName + " (" + replicationConf.Host + ":" + strconv.Itoa(replicationConf.Port) + ")")
				log.Panic(err)
			}
			node := &Node{
				name:        fmt.Sprintf("%s:%d", replicationConf.Host, replicationConf.Port),
				db:          db,
				sql:         sql,
				connpool:    db.Statement.ConnPool,
				online:      true,
				pingTries:   0,
				errorsCount: 0,
			}

			// writer = append(writer, dbDatabaseObj)
			replication.AppendWriteNode(node)

			log.Info("DATABASE | Connected WRITE " + databaseName + " (" + node.name + ")")
		}

		// READ
		for _, replicationConf := range databaseConf.Read {
			db, err := openDB(replicationConf)
			sql, sqlErr := db.DB()

			// No panic if connection fails, but set offline
			if err != nil || sqlErr != nil {
				log.Error("DATABASE | READ Connection error on " + databaseName + " (" + replicationConf.Host + ":" + strconv.Itoa(replicationConf.Port) + ")")
				log.Error(err)
			}
			node := &Node{
				name:        fmt.Sprintf("%s:%d", replicationConf.Host, replicationConf.Port),
				db:          db,
				sql:         sql,
				connpool:    db.Statement.ConnPool,
				online:      RandBool(),
				pingTries:   0,
				errorsCount: 0,
			}

			replication.AppendReadNode(node)

			log.Info("DATABASE | Connected READ " + databaseName + " (" + node.name + ")")
			if db == nil {
				node.errorsCount++
				log.Error("DATABASE | ERROR READ " + databaseName + " (" + node.name + ")")
			}
		}

		// Append to Database list
		DB.AppendReplication(databaseName, replication)

		// Choose first master as gateway
		replication.gateway = replication.Write[0].db

		// Create a singelton for timer to prevent concurency multible checkings
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
				// If an error occures it could mean the loss of the connection
				// So we ping all connectinos to make sure there is no error
				// If a ping fails the connection will be marked as offline
				if db.Error != nil {
					log.Error("DATABASE | error occured ", db.Error)
					pingAllReadSetOfflineOnConnError(replication.Read)
					tickerToReconnect(&replication.readTickerActive, replication.Read)
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
				// logStatus()
				allOnline := true
				for _, replication := range db {
					if !replication.online {
						sql, _ := replication.db.DB()
						if err := sql.Ping(); err != nil {
							log.Error("is still offline: " + replication.name)
							replication.online = false
							allOnline = false
						} else {
							log.Warn("DATABASE | is reconnected and online now: " + replication.name)
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
	// If an error occures ping all read databases if they are still alive
	for _, replication := range db {
		if replication.online {
			sql, _ := replication.db.DB()
			if err := sql.Ping(); err != nil {
				replication.errorsCount++
				log.Error("DATABASE | ping failed, swich to offline: " + replication.name)
				replication.online = false
			}
		}
	}
}

func logTicker() {
	go func() {
		for range time.Tick(4 * time.Second) {
			logStatus()
		}
	}()
}

func logStatus() {
	for databaseName, sourceData := range DB {
		for _, writeData := range sourceData.Write {
			fmt.Println("DATABASE | STATUS ", databaseName, " \t WRITE \t database ", writeData.name, " : \t ", writeData.online, " \tqueries: ", writeData.queryCount, " \terrors: ", writeData.errorsCount)
		}
		for _, readData := range sourceData.Read {
			fmt.Println("DATABASE | STATUS ", databaseName, " \t READ \t database ", readData.name, " : \t ", readData.online, " \tqueries: ", readData.queryCount, " \terrors: ", readData.errorsCount)
		}
	}
	fmt.Println()
}

func RandBool() bool {
	// rand.Seed(1)
	return true
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

func Use(databaseName string) *gorm.DB {
	// Get db from the Database list
	db := DB.GetReplicationset(databaseName)
	if db == nil {
		log.Error("DATABASE | no datasource \"" + databaseName + "\" known")
		return nil
	}

	// No gateway found
	if db.gateway == nil {
		log.Error("DATABASE | no gateway found at \"" + databaseName + "\"")
		return nil
	}

	return db.gateway
}

func openDB(nodeConf *config.NodeConf) (*gorm.DB, error) {

	config := logger.LoggerConfig{
		SlowThreshold:         200 * time.Millisecond,
		SkipErrRecordNotFound: false,
		LogQuery:              nodeConf.LogQuery,
	}
	newLogger := logger.New(config)
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=%s",
		nodeConf.Host,
		nodeConf.Port,
		nodeConf.User,
		nodeConf.Password,
		nodeConf.Db,
		nodeConf.Sslmode)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: psqlInfo, // data Database name, refer https://github.com/jackc/pgx
		//PreferSimpleProtocol: true, // disables implicit prepared statement usage. By default pgx automatically uses the extended protocol
	}), &gorm.Config{
		Logger: newLogger,
	})

	sql, _ := db.DB()

	//http://go-database-sql.org/connection-pool.html
	sql.SetMaxOpenConns(40)
	sql.SetMaxIdleConns(8)
	// sqlDB.SetConnMaxLifetime(0.01)
	//sqlDB.SetConnMaxLifetime(time.Hour)

	return db, err
}

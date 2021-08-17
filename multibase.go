package multibase

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type multibase struct {
	writeNode node
	readNodes []node
}

type node struct {
	DB                   *gorm.DB
	online               bool
	queryCount           int64
	errorCount           int64
	errorConnectionCount int64
}

func New(writeDB *gorm.DB, readDBs ...*gorm.DB) *multibase {
	//setup node struct
	writeNode := node{
		DB:                   writeDB,
		online:               false,
		queryCount:           0,
		errorCount:           0,
		errorConnectionCount: 0,
	}

	// Setup read nodes list
	readNodes := []node{}
	for _, readDB := range readDBs {
		node := node{
			DB:                   readDB,
			online:               false,
			queryCount:           0,
			errorCount:           0,
			errorConnectionCount: 0,
		}
		readNodes = append(readNodes, node)
	}

	return &multibase{
		writeNode: writeNode,
		readNodes: readNodes,
	}
}

func (m *multibase) ConnectWriteNode() error {
	if m.writeNode.DB == nil {
		return fmt.Errorf("write DB is nil")
	}
	// Check connection and connect write node with ping
	if err := CheckConnection(m.writeNode.DB); err != nil {
		return fmt.Errorf("can't connect to write DB")
	} else {
		m.writeNode.online = true
	}
	return nil
}

func (m *multibase) ConnectReadNodes() error {

	if len(m.readNodes) == 0 {
		return fmt.Errorf("no read DB set")
	}
	// Check connection and connect read nodes with ping
	var err error
	for i, readDB := range m.readNodes {

		errConn := CheckConnection(readDB.DB)
		if errConn != nil {
			if err == nil {
				err = errConn
			} else {
				err = fmt.Errorf("%v | %v", err, errConn)
			}
		} else {
			m.readNodes[i].online = true
		}
	}
	return err
}

func (m *multibase) GetDatabaseReplicaSet() *gorm.DB {

	m.writeNode.DB.Callback().Query().Before("*").Register("distributor", func(db *gorm.DB) {
		if !isTransaction(db.Statement.ConnPool) {
			nextNode := getNextDbDatabase(m.readNodes)
			if nextNode != nil {
				db.Statement.ConnPool = nextNode.DB.ConnPool

				nextNode.queryCount++
				return
			}
			// If no READ found the default is write. So we do the count here
			m.writeNode.queryCount++
		}
	})

	m.writeNode.DB.Callback().Query().After("*").Register("errorhandler", func(db *gorm.DB) {

		if !isTransaction(db.Statement.ConnPool) {
			if db.Error != nil {
				for i, node := range m.readNodes {
					if db.Statement.ConnPool == node.DB.ConnPool {
						m.readNodes[i].errorCount++

						sql, _ := node.DB.DB()
						if err := sql.Ping(); err != nil {
							m.readNodes[i].errorConnectionCount++
							m.readNodes[i].online = false
						}
					}
				}
				// If all read nodes are offline the error must be count on the write node
				if IsAllReadNodesOffline(m.readNodes) {
					m.writeNode.errorCount++

					sql, _ := m.writeNode.DB.DB()
					if err := sql.Ping(); err != nil {
						m.writeNode.errorConnectionCount++
						m.writeNode.online = false
					}
				}
			}
		}
	})
	return m.writeNode.DB
}

// StartReconnector is used to check and reconnect offline databases
func (m *multibase) StartReconnector(d time.Duration) error {
	go func() {
		for {
			// The write node is checked always
			sql, _ := m.writeNode.DB.DB()
			if err := sql.Ping(); err == nil {
				m.writeNode.online = true
			} else {
				m.writeNode.online = false
			}

			for i, readDB := range m.readNodes {
				sql, _ := readDB.DB.DB()
				if err := sql.Ping(); err == nil {
					m.readNodes[i].online = true
				} else {
					m.readNodes[i].online = false
				}
			}

			time.Sleep(d)
		}
	}()
	return nil
}

type Statistic struct {
	online               bool
	queryCount           int64
	errorCount           int64
	errorConnectionCount int64
}

func (m *multibase) GetStatistics() map[string]Statistic {

	statistics := map[string]Statistic{
		"write": {
			online:               m.writeNode.online,
			queryCount:           m.writeNode.queryCount,
			errorCount:           m.writeNode.errorCount,
			errorConnectionCount: m.writeNode.errorConnectionCount,
		},
	}
	for i, node := range m.readNodes {
		statistics["read"+strconv.Itoa(i)] = Statistic{
			online:               node.online,
			queryCount:           node.queryCount,
			errorCount:           node.errorCount,
			errorConnectionCount: node.errorConnectionCount,
		}
	}
	return statistics
}

func IsAllReadNodesOffline(nodes []node) bool {
	allOffline := true
	for _, node := range nodes {
		if node.online {
			allOffline = false
		}
	}
	return allOffline
}

func getNextDbDatabase(nodes []node) *node {
	// Collect all online
	indices := []int{}
	for i, node := range nodes {
		if node.online {
			indices = append(indices, i)
		}
	}

	// If no read Database available give back a nil
	if len(nodes) == 0 {
		return nil
	}
	if len(indices) == 0 {
		return nil
	}
	if len(indices) == 1 {
		return &nodes[0]
	}
	randomIndex := indices[rand.Intn(len(indices))]
	return &nodes[randomIndex]
}

func isTransaction(connPool gorm.ConnPool) bool {
	_, ok := connPool.(gorm.TxCommitter)
	return ok
}

func CheckConnection(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Ping()
	if err != nil {
		return err
	}
	return nil
}

func OpenNode(nodeConf *NodeConf) (*gorm.DB, error) {

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
		Logger:               nodeConf.DbLogger,
		DisableAutomaticPing: true, // Important, to prevent for pinging after initializing
	})

	if err != nil {
		return nil, err
	}

	sql, err := db.DB()

	if err != nil {
		return nil, err
	}

	//For more information http://go-database-sql.org/connection-pool.html
	sql.SetMaxOpenConns(nodeConf.DbMaxOpenConns)
	sql.SetMaxIdleConns(nodeConf.DbMaxIdleConns)
	sql.SetConnMaxLifetime(nodeConf.DbConnMaxLifetime)

	return db, err
}

package multibase

import (
	"fmt"
	"sync"
	"testing"

	"github.com/randree/multibase/config"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var (
	wg sync.WaitGroup
)

func NewDatabaseConfig() config.DatabaseConf {

	// First database with one write node and two read nodes
	nodeWrite := &config.NodeConf{
		// WRITE NODE
		Host:                 "mycomputer",
		Port:                 9000,
		User:                 "database_user",
		Password:             "database_password",
		Sslmode:              "disable",
		Db:                   "testdb",
		DbMaxConnections:     40,
		DbMaxOpenConnections: 8,
		LogQuery:             false,
	}

	nodeRead1 := &config.NodeConf{
		// READ NODE 1
		Host:                 "mycomputer",
		Port:                 9001,
		User:                 "database_user", // User must be the master.
		Password:             "database_password",
		Sslmode:              "disable",
		Db:                   "testdb",
		DbMaxConnections:     40,
		DbMaxOpenConnections: 8,
		LogQuery:             false,
	}

	nodeRead2 := &config.NodeConf{
		// READ NODE 2
		Host:                 "mycomputer",
		Port:                 9002,
		User:                 "database_user",
		Password:             "database_password",
		Sslmode:              "disable",
		Db:                   "testdb",
		DbMaxConnections:     40,
		DbMaxOpenConnections: 8,
		LogQuery:             false,
	}

	replica := config.NewReplicationConf()
	replica.AppendWriteNodeConf(nodeWrite)
	replica.AppendReadNodeConf(nodeRead1)
	replica.AppendReadNodeConf(nodeRead2)

	database := config.NewDatabaseConf()
	// Name for connection can be different from the database name, here first_db != testdb
	database.AppendReplicationConf("first_db", replica)

	// Second database with only one write node
	nodeCustomerWrite := &config.NodeConf{
		Host:                 "mycomputer",
		Port:                 9003,
		User:                 "user_second_write",
		Password:             "second_writepw",
		Sslmode:              "disable",
		Db:                   "second_write",
		DbMaxConnections:     40,
		DbMaxOpenConnections: 8,
		LogQuery:             false,
	}

	replicaCustomer := config.NewReplicationConf()
	replicaCustomer.AppendWriteNodeConf(nodeCustomerWrite)

	database.AppendReplicationConf("second_db", replicaCustomer)

	return database

}

type Firsttable struct {
	gorm.Model
	Name string `gorm:"size:255"`
}

type Secondtable struct {
	gorm.Model
	Name string `gorm:"size:255"`
}

func Test_multibase(t *testing.T) {

	// logrus.SetOutput(ioutil.Discard)

	dbConfig := NewDatabaseConfig()

	//With debug mode all 4 seconds a status of all databases is shown
	debugmode := false

	// Use InitDb as database initializer, e.q. in your main
	InitDb(dbConfig, debugmode)

	// Later get your db with Use
	firstdb, _ := Use("first_db")
	seconddb, _ := Use("second_db")

	thirddb, _ := Use("third_db") //does not exists
	t.Run("There is no database third_db", func(t *testing.T) {
		assert.Nil(t, thirddb, "Should be nil")
	})

	t.Run("DB connection initialized", func(t *testing.T) {
		assert.NotNil(t, firstdb, "Should be nil")
		assert.NotNil(t, seconddb, "Should be nil")
	})

	// Crate tables if not exist -----------------
	err := firstdb.AutoMigrate(&Firsttable{})

	t.Run("Automigrate first db", func(t *testing.T) {
		assert.NoError(t, err)
	})

	err = seconddb.AutoMigrate(&Secondtable{})

	t.Run("Automigrate second db", func(t *testing.T) {
		assert.NoError(t, err)
	})

	// Insert some entries -----------------
	err = firstdb.Create(&Firsttable{
		Name: "Hettie Martins",
	}).Error

	t.Run("Insert into first db", func(t *testing.T) {
		assert.NoError(t, err)
	})

	err = firstdb.Create(&Firsttable{
		Name: "Fintan Wynn",
	}).Error

	t.Run("Insert into first db", func(t *testing.T) {
		assert.NoError(t, err)
	})

	err = seconddb.Create(&Secondtable{
		Name: "Anayah Maguire",
	}).Error

	t.Run("Insert into second db", func(t *testing.T) {
		assert.NoError(t, err)
	})

	// Til here we just used write commands
	// The status will show no reads
	fmt.Println("READ TEST 1")
	fmt.Println(Status())

	// Now, do a bunch of reads -----------------
	test1 := &Firsttable{}
	test2 := &Secondtable{}

	wg.Add(200)
	for i := 0; i < 100; i++ {
		go func() {
			firstdb.Take(test1)
			wg.Done()
		}()
		go func() {
			seconddb.Take(test2)
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Println("READ TEST 2")
	fmt.Println(Status())
	// This should be the result
	// MULTIBASE | STATUS  first_db      WRITE   database  mycomputer:9000  :     true          reads:  0       errors:  0
	// MULTIBASE | STATUS  first_db      READ    database  mycomputer:9001  :     true          reads:  43      errors:  0
	// MULTIBASE | STATUS  first_db      READ    database  mycomputer:9002  :     true          reads:  57      errors:  0
	// MULTIBASE | STATUS  second_db     WRITE   database  mycomputer:9003  :     true          reads:  100     errors:  0

	//Get one of the read nodes of first_db
	firstWriteNode := UseNode("first_db", "mycomputer", 9000)
	firstReadNode1 := UseNode("first_db", "mycomputer", 9001)
	firstReadNode2 := UseNode("first_db", "mycomputer", 9002)
	secondWriteNode := UseNode("second_db", "mycomputer", 9003)

	t.Run("Verify that READ is distributed", func(t *testing.T) {
		assert.Less(t, int(firstReadNode1.queryCount), 70)
		assert.Less(t, int(firstReadNode2.queryCount), 70)
		assert.Equal(t, 0, int(firstWriteNode.queryCount))
	})

	t.Run("Verify that Second WRITE got all 100 queries", func(t *testing.T) {
		assert.Equal(t, 100, int(secondWriteNode.queryCount))
	})

	// Now we close one connection to read node 1 -----------------
	// The second read node takes over
	sql, _ := firstReadNode1.db.DB()
	sql.Close()
	wg.Add(200)
	for i := 0; i < 100; i++ {
		go func() {
			firstdb.Take(test1)
			wg.Done()
		}()
		go func() {
			seconddb.Take(test2)
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Println("READ TEST 3")
	fmt.Println(Status())
	t.Run("Verify that READ is distributed to last read node", func(t *testing.T) {
		assert.Less(t, int(firstReadNode1.queryCount), 70)  // Still about 50
		assert.Less(t, int(firstReadNode2.queryCount), 170) // Here plus 100 after it took over
		assert.Equal(t, 0, int(firstWriteNode.queryCount))  // Still no read access on master

		assert.Less(t, int(firstReadNode1.errorsCount), 2) // At least one error occurred on read node 1
	})

	// Now we close one connection to read node 2 -----------------
	// Master takes over
	sql, _ = firstReadNode2.db.DB()
	sql.Close()

	wg.Add(200)
	for i := 0; i < 100; i++ {
		go func() {
			firstdb.Take(test1)
			wg.Done()
		}()
		go func() {
			seconddb.Take(test2)
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Println(Status())
	t.Run("Verify all goes over master", func(t *testing.T) {
		assert.Less(t, int(firstReadNode1.queryCount), 70) // Both numbers stays the same
		assert.Less(t, int(firstReadNode2.queryCount), 170)
		assert.Less(t, 95, int(firstWriteNode.queryCount)) // With concurrency it can happen that more than two queries are lost

		assert.Less(t, int(firstReadNode1.errorsCount), 2) // At least one error occurred on read node 1
	})

	// finally, drop tables
	err = firstdb.Migrator().DropTable("firsttables")
	t.Run("Drop table of firstdb", func(t *testing.T) {
		assert.NoError(t, err)
	})

	err = seconddb.Migrator().DropTable("secondtables")
	t.Run("Drop table of firstdb", func(t *testing.T) {
		assert.NoError(t, err)
	})

}

# Postgres high-availability multiple databases connector for GROM

Simple module to access multiple (Postgres or other) database nodes. Databases are divided into ONE write (or master) and MANY read (or slave) nodes. Replication is handled by the database and is not part of this module. You can choose for example [bitnami/bitnami-docker-postgresql](https://github.com/bitnami/bitnami-docker-postgresql) docker containers. They take care of replication.

A loss of a connection to any (including the master) of the nodes does not end up in a panic. Instead that node is marked as offline. If all read nodes are offline the the load will be redirected to the master. If the master is down no query can be processed but still if the nodes reconnect everything gets back to normal.

The distribution to read nodes is done randomly. 

Under the hood this package uses [GROM hooks](https://gorm.io/docs/hooks.html). Similar to [go-gorm/dbresolver](https://github.com/go-gorm/dbresolver).

## Example

Lets start with one write and two read nodes.

```golang

import (
...
	"github.com/randree/multibase/v2"
...
)

func main() {

	logger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Silent, // Log level
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,         // Disable color
		},
	)

	// WRITE NODE
	nodeWrite := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9000,
		User:              "database_user",
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    20,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          logger,
	}
	// READ NODE 1
	nodeRead1 := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9001,
		User:              "database_user", // User must be the master.
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    20,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          logger,
	}

	// READ NODE 2
	nodeRead2 := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9002,
		User:              "database_user",
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    20,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          logger,
	}

	// OpenNode uses gorm.Open with DisableAutomaticPing: true
	// You can replace it by any other GORM opener
	// The result should be a *gorm.DB instance
	dbWrite, _ := multibase.OpenNode(nodeWrite) // Feel free to check err
	dbRead1, _ := multibase.OpenNode(nodeRead1)
	dbRead2, _ := multibase.OpenNode(nodeRead2)

	// Initiate multibase
	// At this stage NO actual connection is made
	mb := multibase.New(dbWrite, dbRead1, dbRead2)

	// The most important node is the write node.
	// We use the following lines of code to ping the write node and connect to it.
	// Even if no connection can be established, a panic does not occur.
	for {
		err := mb.ConnectWriteNode()
		if err != nil {
			fmt.Println(err)
		} else {
			break
		}
		time.Sleep(time.Millisecond * 1000) // You can choose the interval
	}

	// After the write node is set up, it is time to connect the read nodes
	err := mb.ConnectReadNodes()
	if err != nil {
		fmt.Println(err)
	}

	// After this is done, GetDatabaseReplicaSet binds all nodes to a GORM database
	// All read queries are forwarded to the read nodes.
	db := mb.GetDatabaseReplicaSet()

	// The StartReconnector is a go routine that checks the connection and 
	// reconnects to the nodes if necessary.
	mb.StartReconnector(time.Second * 1)

	// Now we can use db as usual
		type User struct {
		ID   int `gorm:"primarykey"`
		Name string
	}
	db.AutoMigrate(&User{})

	user := &User{}
	db.FirstOrInit(user, User{Name: "Jackx"})

	...

	// To get some statistics use GetStatistics
	statistics := mb.GetStatistics()
	fmt.Println(statistics)

}

```
Statistics is a struct of following shape:
```golang
type Statistic struct {
	online               bool
	queryCount           int64
	errorCount           int64
	errorConnectionCount int64
}
```
The output is similar to 
```
map[read0:{true 91214 3 0} read1:{true 98232 2 0} write:{true 234 0 0}]
```


# Try it out

Use the `docker-compose.yml` to test the example.

1. Open two consoles.  
2. In the first one spin up nodes with
```bash
$ docker-compose up -d
```
3. Write a `main.go` like one the above, add
```golang
	for {
		fmt.Print("\033[H\033[2J") // to clear the console after each cycle
		user := []User{}
		db.Find(&user)

		// Print out statistics
		fmt.Println(mb.GetStatistics())
		// map[read0:{true 1125 0 0} read1:{true 1087 0 0} write:{true 0 0 0}]

		time.Sleep(time.Millisecond * 100) // refresh each 100 ms
	}
``` 
4. In the second console run go
```bash
$ go run main.go
```
the result should be a similar to 
```
map[read0:{true 91214 3 0} read1:{true 98232 2 0} write:{true 234 0 0}]
```
5. While the go program is running stop a read node with
```bash
$ docker stop read2
```
and see that all queries are forwarded to `read0` (count starts at here at 0) and `read1` (read2) is marked as offline.

6. Turn read2 on.
```bash
$ docker start read2
```
and the count is distributed between both read nodes.

7. Play around. Stop the write node. Stop all nodes. Then start all with `docker start write read1 read2`.

The program should never stop or panic.

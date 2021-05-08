# Postgres high-availability multiple databases connector for GROM

Simple module to access multiple Postgres database clusters. Databases are divided into ONE write (or master) and MANY read (or slave) nodes. Replication is handled by the database and not part of this module. You can choose for example [bitnami/bitnami-docker-postgresql](https://github.com/bitnami/bitnami-docker-postgresql) docker containers. They take care of replication.

If an error occurs while process a read query the connector marks the node as offline and tries to ping until success. During offline time all queries are handled by the remaining read nodes. If all read nodes are offline the the load will be redirected to the master.

The distribution to read nodes is done randomly. 

Under the hood this package uses [GROM hooks](https://gorm.io/docs/hooks.html). Similar to [go-gorm/dbresolver](https://github.com/go-gorm/dbresolver).

## 1. Configuration

Lets start with one write and two read nodes.

```golang

// You can write your own logger or you can use the GORM logger
func NewLogger() logger.Interface {
	
	config := logrusLogger.LoggerConfig{
		SlowThreshold:         1000 * time.Millisecond, //show query if it takes to long to process
		SkipErrRecordNotFound: false,
		LogQuery:              false, // show all queries in logs
	}

	return logrusLogger.New(config)
}

func NewDatabaseConfig(logger *logger.Interface) multibase.DatabaseConf

	// WRITE NODE
	nodeWrite := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9000,
		User:              "database_user",
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    40,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          *logger, // If needed each node can get its own individual logger
	}

	// READ NODE 1
	nodeRead1 := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9001,
		User:              "database_user", // User must be the master.
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    40,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          *logger,
	}

	// READ NODE 2
	nodeRead2 := &multibase.NodeConf{
		Host:              "mycomputer",
		Port:              9002,
		User:              "database_user",
		Password:          "database_password",
		Sslmode:           "disable",
		Db:                "testdb",
		DbMaxOpenConns:    40,
		DbMaxIdleConns:    8,
		DbConnMaxLifetime: 1 * time.Hour,
		DbLogger:          *logger,
	}

	replica := multibase.NewReplicationConf()
	replica.AppendWriteNodeConf(nodeWrite)
	replica.AppendReadNodeConf(nodeRead1)
	replica.AppendReadNodeConf(nodeRead2)

	databaseSetConf := multibase.NewDatabaseConf()
	databaseSetConf.AppendReplicationConf("first_db", replica)

	return databaseSetConf
}
```
Database access is called `first_db`. Appending databases is done by `AppendReplicationConf("...", replica)`

## 2. Initialize databases
You can choose the [GORM logger]() or a [logrus logger](https://github.com/onrik/gorm-logrus) or write your own one.

```golang
logger := NewLogger()

dbConfig := NewDatabaseConfig(&logger)

//Shows all 4 seconds a status of all databases
debugmode := false

// Call InitDb to initialize databases, e.q. in your main
InitDb(databaseSetConf, debugmode)
```

## 3. Get instance of database

```golang
db, err := Use("first_db")
if err != nil {
    ...
}
```
`Use` returns a `*gorm.DB`. 

## 4. Using GROM as usual

Now we can use `db` as database in GORM.

```
db.First(&user) //read1 or read2
db.Where("name <> ?", "foo").Find(&users) //read1 or read2

db.Create(&users) //write

```

# Status of nodes
The status of a node can be verified by 
```golang
node := UseNode("mydatabase", "host", 9000)
```

| Field         |type    |                     |
| ------------- |--------|---------------------|
| Errors        | `int`  | `node.errorsCount`  |
| Queries       | `int64`| `node.queryCount`   |
| Online        | `bool` | `node.online`       |

Turning off a read node on the fly can be done with `node.online = false`. Then it can be safely turned down, replaced and restarted again. When it's ready with `node.online = true` it can be activated.

# Benchmark

After shutting down one read node after another under heavy concurrent load the rerouting was (almost) seamless. When turning them on again the system went back to a equal distribution between read nodes without any interruptions.

A quantitative analysis would be interesting.

# Testing


For testing this module go to directory root and execute
```bash
$ docker-compose up
```
and
```bash
$ go test -count=1 -v ./...
```


# ToDos

* Adding multiple write nodes. At this time only one write node does the work.
* If a connection on a read node is interrupted there is a loss of about one query depending on the concurrency. This can be fixed by catching these queries.
* Adding new nodes on the fly.
* Optimizing the administration.
* More testing.




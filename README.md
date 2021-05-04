# Postgres high-availability multiple databases connector for GROM

Simple package to access multiple Postgres database clusters. Databases are divided into ONE write (or master) and MANY read (or slave) nodes. Replication is handled by the database and not part of this package. You can choose for example [bitnami/bitnami-docker-postgresql](https://github.com/bitnami/bitnami-docker-postgresql) docker containers. They take care of replication.

If an error occurs while process a read query the connector marks the node as offline and tries to ping until success. During offline time all queries are handled by the remaining read nodes. If all read nodes are offline the the load will be redirected to the master.

The distribution to read nodes is done randomly. 

Under the hood this package uses [GROM hooks](https://gorm.io/docs/hooks.html). Similar to [go-gorm/dbresolver](https://github.com/go-gorm/dbresolver).

## 1. Configuration

Lets start with one write and two read nodes.

```golang
func NewDatabaseConfig() config.DatabaseConf {
	// WRITE NODE
	nodeWrite := &config.NodeConf{
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

	// READ NODE 1
	nodeRead1 := &config.NodeConf{
		Host:                 "mycomputer",
		Port:                 9001,
		User:                 "database_user",
		Password:             "database_password",
		Sslmode:              "disable",
		Db:                   "testdb",
		DbMaxConnections:     40,
		DbMaxOpenConnections: 8,
		LogQuery:             false,
	}

	// READ NODE 2
	nodeRead2 := &config.NodeConf{
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

	databaseSetConf := config.NewDatabaseConf()
	databaseSetConf.AppendReplicationConf("first_db", replica)

	return databaseSetConf
}
```
The database access is called `first_db`. Appending databases is done by `AppendReplicationConf("...", replica)`

## 2. Initialize databases

```golang
databaseSetConf := NewDatabaseConfig()

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

# Testing

Even under heavy concurrent load, the rerouting was (almost) seamless. 

# ToDos

* Adding multiple write nodes. At this time only one write node does the work
* If a connection on a read node is interrupted there is a loss of about one query depending on the concurrency. This can be fixed by catching these queries
* Add new nodes on the fly
* Optimizing the administration
* More testing 




package multibase

/*
Equivalent to the conf struct we set up the actual node structure:
├── Database (e.g. adminDb)
│   ├── Replication struct
│   │   ├── Write Node
│   │   ├── Read  []*Node
├── Database (e.g. customerDb)
│   ├── Replication struct
│   │   ├── Write Node
│   │   ├── Read  []*Node
*/
// type Database map[string]*Replication

// func NewDatabase() Database {
// 	return make(Database)
// }

// func (db Database) AppendReplication(name string, rep *Replication) {
// 	db[name] = rep
// }

// func (db Database) GetReplicationset(name string) *Replication {
// 	return db[name]
// }

// type Replication struct {
// 	Write             *Node
// 	Read              []*Node
// 	readTickerActive  bool
// 	readCheckerActive bool
// 	gateway           *gorm.DB
// }

// func NewReplication() *Replication {
// 	return new(Replication)
// }

// func (r *Replication) AppendWriteNode(write *Node) {
// 	r.Write = write
// }

// func (r *Replication) AppendReadNode(read *Node) {
// 	r.Read = append(r.Read, read)
// }

// type Node struct {
// 	name        string
// 	host        string
// 	port        int
// 	db          *gorm.DB
// 	sql         *sql.DB
// 	connpool    gorm.ConnPool
// 	queryCount  int64
// 	online      bool
// 	pingTries   int
// 	errorsCount int
// }

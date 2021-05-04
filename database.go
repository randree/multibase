package multibase

type Database map[string]*Replication

func NewDatabase() Database {
	return make(Database)
}

func (db Database) AppendReplication(name string, rep *Replication) {
	db[name] = rep
}

func (db Database) GetReplicationset(name string) *Replication {
	return db[name]
}

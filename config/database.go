package config

type DatabaseConf map[string]*ReplicationConf

func NewDatabaseConf() DatabaseConf {
	return make(DatabaseConf)
}

func (dbc DatabaseConf) AppendReplicationConf(name string, repConf *ReplicationConf) {
	dbc[name] = repConf
}

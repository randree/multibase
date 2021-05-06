package multibase

type ReplicationConf struct {
	Write []*NodeConf
	Read  []*NodeConf
}

func NewReplicationConf() *ReplicationConf {
	return new(ReplicationConf)
}

func (r *ReplicationConf) AppendWriteNodeConf(write *NodeConf) {
	r.Write = append(r.Write, write)
}

func (r *ReplicationConf) AppendReadNodeConf(read *NodeConf) {
	r.Read = append(r.Read, read)
}

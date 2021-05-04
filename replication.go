package multibase

import "gorm.io/gorm"

type Replication struct {
	Write             []*Node
	Read              []*Node
	readTickerActive  bool
	readCheckerActive bool
	gateway           *gorm.DB
}

func NewReplication() *Replication {
	return new(Replication)
}

func (r *Replication) AppendWriteNode(write *Node) {
	r.Write = append(r.Write, write)
}

func (r *Replication) AppendReadNode(read *Node) {
	r.Read = append(r.Read, read)
}

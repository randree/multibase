package config

type NodeConf struct {
	Host                 string
	Port                 int
	User                 string
	Password             string
	Db                   string
	Sslmode              string
	DbMaxOpenConns       int
	DbMaxIdleConns       int
	DbMaxConnections     int
	DbMaxOpenConnections int
	LogQuery             bool
}

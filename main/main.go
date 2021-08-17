package main

/* This is a test environment */

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/randree/multibase/v2"
	"gorm.io/gorm/logger"
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

	dbWrite, _ := multibase.OpenNode(nodeWrite)
	dbRead1, _ := multibase.OpenNode(nodeRead1)
	dbRead2, _ := multibase.OpenNode(nodeRead2)

	fmt.Println("Start")

	type User struct {
		ID   int `gorm:"primarykey"`
		Name string
	}
	// fmt.Println(fmt.Sprintf("%+v", dbWrite), errWrite)
	// fmt.Println(fmt.Sprintf("%+v", dbRead1), errRead1)
	// fmt.Println(fmt.Sprintf("%+v", dbRead2), errRead2)

	// dbWrite.AutoMigrate(&User{})
	// user := &User{}
	// dbWrite.FirstOrInit(user, User{Name: "Jackx"})
	// dbWrite.Create(&User{
	// 	Name: "Jack"})
	// dbWrite.Create(&User{
	// 	Name: "Hans"})

	mb := multibase.New(dbWrite, dbRead1, dbRead2)

	for {
		err := mb.ConnectWriteNode()
		if err != nil {
			fmt.Println(err)
		} else {
			break
		}
		time.Sleep(time.Millisecond * 1000)
	}
	err := mb.ConnectReadNodes()
	if err != nil {
		fmt.Println(err)
	}

	db := mb.GetDatabaseReplicaSet()

	mb.StartReconnector(time.Second * 1)

	db.AutoMigrate(&User{})
	user := &User{}
	db.FirstOrInit(user, User{Name: "Jackx"})
	users := []User{}
	db.Find(&users)
	fmt.Println(users)

	// TEST START ============================
	steps := 1000
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(steps)
	for i := 0; i < steps; i++ {
		go func() {
			user := []User{}
			db.Find(&user)
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Println("---> TIME", elapsed)

	start = time.Now()
	for i := 0; i < steps; i++ {
		user := []User{}
		db.Find(&user)
	}
	elapsed = time.Since(start)
	fmt.Println("---> TIME", elapsed)
	// TEST END ============================

	fmt.Println(mb.GetStatistics())

	for {
		fmt.Print("\033[H\033[2J")
		// fmt.Println("W: ", multibase.CheckConnection(dbWrite))
		// fmt.Println("R1: ", multibase.CheckConnection(dbRead1))
		// fmt.Println("R2: ", multibase.CheckConnection(dbRead2))

		// fmt.Println(mb.GetStatistics())
		// db.Create(&User{Name: "Jackxw"})

		// db.Delete(User{}, "id <> 0")
		user := []User{}
		db.Find(&user)
		// fmt.Println(user)

		fmt.Println(mb.GetStatistics())

		time.Sleep(time.Millisecond * 10)
	}

}

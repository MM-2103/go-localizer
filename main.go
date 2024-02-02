package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	sqldblogger "github.com/simukti/sqldb-logger"
	"github.com/simukti/sqldb-logger/logadapter/zerologadapter"
	// "cloud.google.com/go/translate/apiv3"
)

// This function handles loading .env and preparing database connection
func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// This function will handle the database connection
func main() {
	debug := flag.Bool("debug", false, "Enable debug mode for SQL logging")
	flag.Parse()

	LoadEnv()

	dbUser := os.Getenv("DATABASE_USER")
	dbName := os.Getenv("DATABASE_NAME")
	dbPass := os.Getenv("DATABASE_PASS")
	dbHost := os.Getenv("DATABASE_HOST")
	dbPort := os.Getenv("DATABASE_PORT")

	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName)

	var db *sql.DB
	var err error

	if *debug {
		logger := zerolog.New(os.Stdout)
		loggerAdapter := zerologadapter.New(logger)
		db, err = sql.Open("mysql", dataSourceName)
		if err != nil {
			log.Fatalf("Could not open database: %v", err)
		}
		db = sqldblogger.OpenDriver(dataSourceName, db.Driver(), loggerAdapter)
	} else {
		db, err = sql.Open("mysql", dataSourceName)
		if err != nil {
			log.Fatalf("Could not connect to database: %v", err)
		}
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	} else {
		fmt.Println("Succesfully connected to database!")
	}

	err = sendQuery(db)
	if err != nil {
		log.Fatalf("Failed to send query: %v", err)
	} else {
		fmt.Println("Query executed succesfully!")
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
}

func sendQuery(db *sql.DB) error {
	stmt, err := db.Prepare("SELECT `name` FROM `trrc_product_flat` WHERE `locale` = 'nl';")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	if err != nil {
		return err
	}
	return nil
}

// This function handles translation of string through google cloud api
// func translateProducts() {
//
// }

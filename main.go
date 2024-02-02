package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"

	"cloud.google.com/go/translate/apiv3"
)

// This function will handle the database connection
func main() {
	db, err := sql.Open("mysql", "user:password@/dbname")
	if err != nil {
		panic(err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
}

// This function handles translation of string through google cloud api
func translateProducts() {

}

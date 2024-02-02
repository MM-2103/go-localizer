package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"cloud.google.com/go/translate/apiv3"
)

func main() {
	// This is function will initiate all other functions and will not have any complex logic

}

func connectToDatabase() {
	// This function handles the database conncetion
}

func translate() {
	// This function handles translation of string through google cloud api
}

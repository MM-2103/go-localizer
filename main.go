package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	translate "cloud.google.com/go/translate/apiv3"
	"cloud.google.com/go/translate/apiv3/translatepb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	sqldblogger "github.com/simukti/sqldb-logger"
	"github.com/simukti/sqldb-logger/logadapter/zerologadapter"
)

type QueryOutput struct {
	Name             string
	Description      string
	ShortDescription string
}

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

	err = SelectQuery(db)
	if err != nil {
		log.Fatalf("Failed to send query: %v", err)
	} else {
		fmt.Println("Query executed succesfully!")
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
}

func SelectQuery(db *sql.DB) error {
	query := "SELECT `name`, `description`, `short_description` FROM `trrc_product_flat` WHERE `locale` = 'nl';"

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var querySlice []QueryOutput
	for rows.Next() {
		var name string
		var description string
		var short_description string

		err := rows.Scan(&name, &description, &short_description)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(name, description, short_description)
		querySlice = append(querySlice, QueryOutput{Name: name, Description: description, ShortDescription: short_description})
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	return nil
}

/* This function handles translation of string through google cloud api */
func translateProducts(w io.Writer, projectID string, sourceLang string, targetLang string, text string) error {
	projectID = "my-project-id"
	sourceLang = "nl"
	targetLang = "fr"
	text = "Text you wish to translate"

	ctx := context.Background()
	client, err := translate.NewTranslationClient(ctx)
	if err != nil {
		return fmt.Errorf("NewTranslationClient: %w", err)
	}
	defer client.Close()

	req := &translatepb.TranslateTextRequest{
		Parent:             fmt.Sprintf("projects/%s/locations/global", projectID),
		SourceLanguageCode: sourceLang,
		TargetLanguageCode: targetLang,
		MimeType:           "text/plain", // Mime types: "text/plain", "text/html"
		Contents:           []string{text},
	}

	resp, err := client.TranslateText(ctx, req)
	if err != nil {
		return fmt.Errorf("TranslateText: %w", err)
	}

	for _, translation := range resp.GetTranslations() {
		fmt.Fprintf(w, "Translated text: %v\n", translation.GetTranslatedText())
	}
	return nil
}

func UpdateQuery(db *sql.DB) error {
	stmt, err := db.Prepare("SELECT `name`, `description`, `short_description` FROM `trrc_product_flat` WHERE `locale` = 'nl';")
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

// Fields to translate
// short_description, description,

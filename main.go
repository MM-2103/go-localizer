package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
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

var (
	name_attribute_id       = 1
	short_desc_attribute_id = 9
	desc_attribute_id       = 10
)

type (

	QueryOutput struct {
		Name             string
		Description      string
		ShortDescription string
		Sku              string
	}

	AttributeTable struct {
		id		int
		product_id	int
		locale		string
		channel		string
		attribute_id	int
	} 
)


func GoogleAuth() {
	ctx := context.Background()
	client, err := translate.NewTranslationClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create translation client: %v", err)
	}
	defer client.Close()
}

// LoadEnv This function handles loading .env and preparing database connection
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
		fmt.Println("Successfully connected to database!")
	}

	if err != nil {
		log.Fatalf("Failed to send query: %v", err)
	} else {
		fmt.Println("Query executed successfully!")
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	ctx := context.Background()

	// Initialize Google Cloud Translation client
	client, err := translate.NewTranslationClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create translation client: %v", err)
	}
	defer client.Close()

	// Retrieve querySlice from SelectQuery
	querySlice, err := SelectQuery(db)
	if err != nil {
		log.Fatalf("Failed to execute select query: %v", err)
	}

	// Translate the products
	projectID := os.Getenv("GOOGLE_PROJECTID") // Make sure this is set in your .env or environment

	// After setting up the client and retrieving querySlice
	if err := translateProducts(ctx, client, db, querySlice, projectID); err != nil {
		log.Fatalf("Failed to translate and update products: %v", err)
	}

}

func SelectQuery(db *sql.DB) ([]QueryOutput, error) {
	query := "SELECT `name`, `description`, `short_description`, `sku` FROM `trrc_product_flat` WHERE `locale` = 'nl';"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var querySlice []QueryOutput
	for rows.Next() {
		var product QueryOutput
		err := rows.Scan(&product.Name, &product.Description, &product.ShortDescription, &product.Sku)
		if err != nil {
			log.Fatal(err) // Consider returning error instead
		}
		querySlice = append(querySlice, product)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err) // Consider returning error instead
	}
	return querySlice, nil
}

// Assuming querySlice is accessible or passed to this function

func translateProducts(ctx context.Context, client *translate.TranslationClient, db *sql.DB, querySlice []QueryOutput, projectID string) error {
	sourceLang := "nl"
	targetLangs := []string{"en", "fr", "de"}

	for _, product := range querySlice {
		for _, targetLang := range targetLangs {

			productName := product.Name

			translatedDescription, err := translateText(ctx, client, projectID, sourceLang, targetLang, product.Description)
			if err != nil {
				log.Printf("Failed to translate description to %s: %v", targetLang, err)
				continue
			}

			translatedShortDescription, err := translateText(ctx, client, projectID, sourceLang, targetLang, product.ShortDescription)
			if err != nil {
				log.Printf("Failed to translate short description to %s: %v", targetLang, err)
				continue
			}

			// Instead of printing, update the database with the translated text
			if err := updateProductTranslations(db, product.Sku, targetLang, productName, translatedDescription, translatedShortDescription); err != nil {
				log.Printf("Failed to update translations and name for SKU %s to %s: %v", product.Sku, targetLang, err)
			}
		}
	}
	return nil
}

// Helper function to abstract the translation API call
func translateText(ctx context.Context, client *translate.TranslationClient, projectID, sourceLang, targetLang, text string) (string, error) {
	req := &translatepb.TranslateTextRequest{
		Parent:             fmt.Sprintf("projects/%s/locations/global", projectID),
		SourceLanguageCode: sourceLang,
		TargetLanguageCode: targetLang,
		MimeType:           "text/plain",
		Contents:           []string{text},
	}

	resp, err := client.TranslateText(ctx, req)
	if err != nil {
		return "", fmt.Errorf("TranslateText: %w", err)
	}

	if len(resp.GetTranslations()) > 0 {
		return resp.GetTranslations()[0].GetTranslatedText(), nil
	}
	return "", nil
}

func updateProductTranslations(db *sql.DB, sku, locale, name, description, shortDescription string) error {
	query := `UPDATE trrc_product_flat SET name = ?, description = ?, short_description = ? WHERE sku = ? AND locale = ?`
	_, err := db.Exec(query, name, description, shortDescription, sku, locale)
	return err
}

func addAttributeValue(db *sql.DB, name, ) {
	query := `INSERT INTO trrc_product_attributes_values(`id`, `text_value`, `product_id`, `attribute_id`) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(id, text_value, product_id, attribute_id)
}

// Update query function to use later

/*
func UpdateQuery(db *sql.DB) error {
	stmt, err := db.Prepare("UPDATE `trrc_product_flat` SET `name`='product-name', `short_description`='translated-short-description',`description`='translated-description' WHERE `sku` = 'product-sku' AND `locale` = 'product-locale'")
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
*/

// My old function for translation, keeping just in case
/* This function handles translation of string through Google cloud api */
// func translateProducts(w io.Writer, projectID string, sourceLang string, targetLang string, text string) error {
// 	projectID = os.Getenv("GOOGLE_PROJECTID")
// 	sourceLang = "nl"
// 	targetLang = "fr"
//
// 	ctx := context.Background()
// 	client, err := translate.NewTranslationClient(ctx)
// 	if err != nil {
// 		return fmt.Errorf("NewTranslationClient: %w", err)
// 	}
// 	defer client.Close()
//
// 	req := &translatepb.TranslateTextRequest{
// 		Parent:             fmt.Sprintf("projects/%s/locations/global", projectID),
// 		SourceLanguageCode: sourceLang,
// 		TargetLanguageCode: targetLang,
// 		MimeType:           "text/plain", // Mime types: "text/plain", "text/html"
// 		Contents:           []string{text},
// 	}
//
// 	resp, err := client.TranslateText(ctx, req)
// 	if err != nil {
// 		return fmt.Errorf("TranslateText: %w", err)
// 	}
//
// 	for _, translation := range resp.GetTranslations() {
// 		fmt.Fprintf(w, "Translated text: %v\n", translation.GetTranslatedText())
// 	}
// 	return nil
// }

// Fields to translate
// short_description, description,

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

type (
	QueryOutput struct {
		Name             string
		Description      string
		ShortDescription string
		Sku              string
		Channel          string
		ProductId        int
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

	projectID := os.Getenv("GOOGLE_PROJECTID")

	if err := translateProducts(ctx, client, db, querySlice, projectID); err != nil {
		log.Fatalf("Failed to translate and update products: %v", err)
	}

}

func SelectQuery(db *sql.DB) ([]QueryOutput, error) {
	query := "SELECT `name`, `description`, `short_description`, `sku`, `channel`, `product_id` FROM `trrc_product_flat` WHERE `locale` = 'nl';"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var querySlice []QueryOutput
	for rows.Next() {
		var product QueryOutput
		err := rows.Scan(&product.Name, &product.Description, &product.ShortDescription, &product.Sku, &product.Channel, &product.ProductId)
		if err != nil {
			log.Fatal(err)
		}
		querySlice = append(querySlice, product)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	return querySlice, nil
}

func translateProducts(ctx context.Context, client *translate.TranslationClient, db *sql.DB, querySlice []QueryOutput, projectID string) error {
	const (
		shortDescriptionAttributeId = 9
		descriptionAttributeId      = 10
	)

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

			// Update the database with the translated text for the description
			if err := updateProductTranslations(db, product.Sku, targetLang, productName, translatedDescription, translatedShortDescription); err != nil {
				log.Printf("Failed to update translations and name for SKU %s to %s: %v", product.Sku, targetLang, err)
			}

			// Update attribute translations
			if err := insertAttributeTranslation(db, targetLang, product.Channel, translatedDescription, product.ProductId, descriptionAttributeId); err != nil {
				log.Printf("Failed to update attribute translation for description: %v", err)
			}

			// Corrected to use translatedShortDescription
			if err := insertAttributeTranslation(db, targetLang, product.Channel, translatedShortDescription, product.ProductId, shortDescriptionAttributeId); err != nil {
				log.Printf("Failed to update attribute translation for short description: %v", err)
			}

		}
	}
	return nil
}

// SELECT a.`code`, pav.`id`, pav.`locale`, pav.`text_value`, pav.`product_id`, pro.`sku` FROM `trrc_attributes` a INNER JOIN `trrc_product_attribute_values` pav ON a.`id` = pav.`attribute_id` INNER JOIN `trrc_products` pro ON pav.`product_id` = pro.`id` WHERE pav.`locale` IS NOT NULL AND pav.`locale` = 'nl' AND a.`code` = 'description' OR a.`code` = 'short_description';

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

func insertAttributeTranslation(db *sql.DB, locale, channel, textValue string, productId, attributeId int) error {
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM trrc_product_attribute_values WHERE product_id = ? AND attribute_id = ? AND locale = ? LIMIT 1)`
	err := db.QueryRow(checkQuery, productId, attributeId, locale).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		updateQuery := `UPDATE trrc_product_attribute_values SET text_value = ?, channel = ? WHERE product_id = ? AND attribute_id = ? AND locale = ?`
		_, err := db.Exec(updateQuery, textValue, channel, productId, attributeId, locale)
		if err != nil {
			return err
		}
	} else {
		log.Printf("No existing row to update for Product ID: %d, Attribute ID: %d, Locale: %s. Consider inserting or handling this case differently.", productId, attributeId, locale)
	}

	return nil
}

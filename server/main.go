package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

const apiUrl string = "https://economia.awesomeapi.com.br/json/last/USD-BRL"
const requestTimeout int = 200 // tempo em milissegundos
const databaseTimeout int = 10 // tempo em milissegundos
const timeFormat = "2006-01-02 15:04"

type Application struct {
	db       *sql.DB
	Port     int
	APIUrl   string
	APIToken string
}

type Currency struct {
	CurrencyInfo struct {
		Code       string `json:"code"`
		Codein     string `json:"codein"`
		Name       string `json:"name"`
		High       string `json:"high"`
		Low        string `json:"low"`
		VarBid     string `json:"varBid"`
		PctChange  string `json:"pctChange"`
		Bid        string `json:"bid"`
		Ask        string `json:"ask"`
		Timestamp  string `json:"timestamp"`
		CreateDate string `json:"create_date"`
	} `json:"USDBRL"`
}

type CurrencyQuote struct {
	ID        int
	QuoteDate string
	Value     float64
}

func main() {
	app := Application{APIUrl: apiUrl, Port: 8080}
	err := app.Init()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/cotacao", app.QuotationHandler)
	log.Printf("Listening on port %d...\n", app.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", app.Port), mux))
}

func (app *Application) SaveIntoDatabase(value float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(databaseTimeout)*time.Millisecond)
	defer cancel()
	sql := `INSERT INTO currency (quote_date, value) VALUES (?, ?)`
	stmt, err := app.db.Prepare(sql)
	if err != nil {
		return err
	}

	t := time.Now()
	formattedDate := t.Format(timeFormat)

	_, err = stmt.ExecContext(ctx, formattedDate, value)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("Error: Timeout while saviung to database")
		}
		return err
	}

	return nil
}

func (app *Application) GetFromDatabase(quoteDate string) (*CurrencyQuote, error) {
	sql := `SELECT * FROM currency where quote_date = ?`
	stmt, err := app.db.Prepare(sql)
	if err != nil {
		return nil, err
	}

	row, err := stmt.Query(quoteDate)
	if err != nil {
		return nil, err
	}

	defer row.Close()

	if row.Next() {
		var currencyQuote CurrencyQuote
		row.Scan(&currencyQuote.ID, &currencyQuote.QuoteDate, &currencyQuote.Value)
		return &currencyQuote, nil
	}

	return nil, nil
}

func (app *Application) initDBConnection() error {
	db, err := sql.Open("sqlite3", "./app.db")
	if err != nil {
		return err
	}

	app.db = db
	return nil
}

func (app *Application) initializeTables() error {
	sql := `CREATE TABLE IF NOT EXISTS currency (id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, quote_date TEXT, value REAL)`
	_, err := app.db.Exec(sql)
	if err != nil {
		return err
	}

	return nil
}

func (app *Application) Init() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	app.APIToken = os.Getenv("API_TOKEN")

	log.Println("Opening SQLite file...")
	err = app.initDBConnection()
	if err != nil {
		return err
	}

	log.Println("Creating tables (if needed)...")
	err = app.initializeTables()
	if err != nil {
		return err
	}

	return nil
}

func (app *Application) QuotationHandler(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	formattedDate := t.Format(timeFormat)

	currencyQuote, err := app.GetFromDatabase(formattedDate)
	if err != nil {
		app.writeResponse(w, "", err)
	}

	if currencyQuote != nil {
		log.Println("Quotation retrieved from the database")
		bidResponse := fmt.Sprintf(`{"bidPrice": "%.4f"}`, currencyQuote.Value)
		app.writeResponse(w, bidResponse, err)
		return
	} else {
		log.Println("Getting quotation from the server")
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(requestTimeout)*time.Millisecond)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", app.APIUrl, nil)
		if err != nil {
			app.writeResponse(w, "", err)
			return
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", app.APIToken))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Println("Error: Timeout while getting quote")
			}
			app.writeResponse(w, "", err)
			return
		}

		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			app.writeResponse(w, "", err)
			return
		}

		var currency Currency
		err = json.Unmarshal(bodyBytes, &currency)
		if err != nil {
			app.writeResponse(w, "", err)
			return
		}

		bidResponse := fmt.Sprintf(`{"bidPrice": "%s"}`, currency.CurrencyInfo.Bid)
		bid, err := strconv.ParseFloat(currency.CurrencyInfo.Bid, 64)
		if err != nil {
			app.writeResponse(w, "", err)
			return
		}

		app.SaveIntoDatabase(bid)
		app.writeResponse(w, bidResponse, nil)
		return
	}
}

func (app *Application) writeResponse(w http.ResponseWriter, content string, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}
}

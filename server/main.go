package main

/*
	NOTA:

	Não consegui utilizar a API indicada no exercício em um tier gratuito. Procurei uma opção similar e acabei encontrando esta:
	https://brapi.dev/docs/moedas
	Este serviço também tem limit rate no tier gratuito. Para não gerar impactos nos testes enquanto fazia o desafio, escrevi um serviço de mock.
	Por este motivo a url é configurável na struct Application.
*/

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const apiToken string = ""
const apiUrl string = "https://brapi.dev/api/v2/currency?currency=USD-BRL"
const requestTimeout int = 200 // tempo em milissegundos
const databaseTimeout int = 10 // tempo em milissegundos

type Application struct {
	db       *sql.DB
	Port     int
	APIUrl   string
	APIToken string
}

type CurrencyInfo struct {
	ToCurrency         string `json:"toCurrency"`
	Name               string `json:"name"`
	High               string `json:"high"`
	Low                string `json:"low"`
	BidVariation       string `json:"bidVariation"`
	PercentageChange   string `json:"percentageChange"`
	BidPrice           string `json:"bidPrice"`
	AskPrice           string `json:"askPrice"`
	UpdatedAtTimestamp string `json:"updatedAtTimestamp"`
	UpdatedAtDate      string `json:"updatedAtDate"`
}

type Currency struct {
	Currency []CurrencyInfo `json:"currency"`
}

type CurrencyQuote struct {
	ID        int
	QuoteDate string
	Value     float64
}

func main() {
	app := Application{APIUrl: "http://localhost:8181/mock", APIToken: apiToken, Port: 8080}
	err := app.Init()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/cotacao", app.QuotationHandler)
	fmt.Printf("Listening on port %d...\n", app.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", app.Port), mux))
}

func (app *Application) SaveIntoDatabase(value float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(databaseTimeout)*time.Millisecond)
	defer cancel()
	sql := `INSERT INTO currency (quote_date, value) VALUES (date(), ?)`
	stmt, err := app.db.Prepare(sql)
	if err != nil {
		return err
	}

	_, err = stmt.ExecContext(ctx, value)
	if err != nil {
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
	fmt.Println("Opening SQLite file...")
	err := app.initDBConnection()
	if err != nil {
		return err
	}

	fmt.Println("Creating tables (if needed)...")
	err = app.initializeTables()
	if err != nil {
		return err
	}

	return nil
}

func (app *Application) QuotationHandler(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	formattedDate := t.Format("2006-01-02")

	currencyQuote, err := app.GetFromDatabase(formattedDate)
	if err != nil {
		app.writeResponse(w, "", err)
	}

	if currencyQuote != nil {
		bidResponse := fmt.Sprintf(`{"bidPrice": "%.4f"}`, currencyQuote.Value)
		app.writeResponse(w, bidResponse, err)
		return
	} else {
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

		if len(currency.Currency) > 0 {
			bidResponse := fmt.Sprintf(`{"bidPrice": "%s"}`, currency.Currency[0].BidPrice)
			bid, err := strconv.ParseFloat(currency.Currency[0].BidPrice, 64)
			if err != nil {
				app.writeResponse(w, "", err)
				return
			}
			fmt.Printf("Pegou o valor %s e converteu para %f\n", currency.Currency[0].BidPrice, bid)
			app.SaveIntoDatabase(bid)
			app.writeResponse(w, bidResponse, nil)
			return
		} else {
			errorResponse := `{"error": "Could not obatin currency quote"}`
			err := errors.New(errorResponse)
			app.writeResponse(w, "", err)
			return
		}
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

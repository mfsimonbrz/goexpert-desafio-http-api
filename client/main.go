package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type CurrencyQuote struct {
	BidPrice string `json:"bidPrice"`
}

const requestTimeout int = 300 // em milissegundos
const currencyServerAddr string = "http://localhost:8080/cotacao"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, currencyServerAddr, nil)
	if err != nil {
		log.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()

	quotation, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var currencyQuote CurrencyQuote
	err = json.Unmarshal(quotation, &currencyQuote)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(currencyQuote.BidPrice)

}

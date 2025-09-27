package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("Error: Timeout while getting quote...")
		} else {
			log.Fatal(err)
		}
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

	currencyFile, err := os.OpenFile("cotacao.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer currencyFile.Close()
	bidString := fmt.Sprintf("%s\t%s\n", time.Now().Format(time.ANSIC), currencyQuote.BidPrice)
	currencyFile.WriteString(bidString)

	log.Println(currencyQuote.BidPrice)

}

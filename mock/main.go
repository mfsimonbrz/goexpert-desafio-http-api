package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/mock", mockHandler)
	http.ListenAndServe(":8181", mux)
}

func mockHandler(w http.ResponseWriter, r *http.Request) {
	response := `{
		"currency": [
			{
			"fromCurrency": "USD",
			"toCurrency": "BRL",
			"name": "Dólar Americano/Real Brasileiro",
			"high": "5.22",
			"low": "5.162",
			"bidVariation": "0.0454",
			"percentageChange": "0.88",
			"bidPrice": "5.2097",
			"askPrice": "5.2127",
			"updatedAtTimestamp": "1696601423",
			"updatedAtDate": "2023-10-06 11:10:23"
			}
		]
	}`

	select {
	case <-time.After(500 * time.Millisecond):
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	case <-r.Context().Done():
		fmt.Println("Cancelled by user")
	}
}

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Index(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode("Hello World")
}

func main() {
	router := httprouter.New()

	router.GET("/", Index)

	log.Fatal(http.ListenAndServe(":8080", router))
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"salon/routers"
)

func main() {
	/// Kreirajte instancu routera
	r := routers.Router()

	// Definišite putanju do vašeg statičkog sadržaja (front-end resursi)
	staticDir := http.Dir("./static/")
	staticFileHandler := http.FileServer(staticDir)

	// Definišite putanju na kojoj će biti serviran statički sadržaj (npr. /static/)
	http.Handle("/static/", http.StripPrefix("/static/", staticFileHandler))

	// Postavite rutere koji će obrađivati back-end zahteve
	http.Handle("/api/", r)

	fmt.Println("Starting server on the port 8080..")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

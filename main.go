package main

import (
	"fmt"
	"log"
	"net/http"
	"salon/routers"
	//"github.com/rs/cors"
)

func main() {
	/// Kreirajte instancu routera
	r := routers.Router()

	/*c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})*/

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

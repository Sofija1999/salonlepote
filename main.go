package main

import (
	"fmt"
	"log"
	"net/http"
	"salon/routers"
	//"github.com/rs/cors"
)

func main() {
	//instanca routera
	r := routers.Router()

	//definisana putanja do statičkog sadržaja (front-end resursi)
	staticDir := http.Dir("./static/")
	staticFileHandler := http.FileServer(staticDir)

	//definisana putanja na kojoj će biti serviran statički sadržaj (npr. /static/)
	http.Handle("/static/", http.StripPrefix("/static/", staticFileHandler))

	//postavljanje rutera koji će obrađivati back-end zahteve
	http.Handle("/api/", r)

	fmt.Println("Starting server on the port 8080..")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

package routers

import (
	"salon/middleware"

	"github.com/gorilla/mux"
)

func Router() *mux.Router {
	//Init router
	router := mux.NewRouter()

	//Route Handlers
	router.HandleFunc("/api/newkupac", middleware.CreateCustomer).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/newreservation", middleware.CreateReservation).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/deletereservation", middleware.DeleteReservation).Methods("DELETE", "OPTIONS")
	router.HandleFunc("/api/reservation", middleware.GetReservation).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/deletestavka/{uslugaID}", middleware.DeleteStavka).Methods("DELETE", "OPTIONS")

	return router
}

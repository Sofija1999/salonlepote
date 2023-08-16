package models

import "time"

type Kupac struct {
	Ime           string `json:"ime"`
	Prezime       string `json:"prezime"`
	Kompanija     string `json:"kompanija,omitempty"`
	Adresa1       string `json:"adresa1"`
	Adresa2       string `json:"adresa2,omitempty"`
	PostanskiBroj string `json:"postanski_broj"`
	Mesto         string `json:"mesto"`
	Drzava        string `json:"drzava"`
	Email         string `json:"email"`
	EmailPotvrda  string `json:"potvrda_email"`
}

type KupacResponse struct {
	ID      int64  `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type StavkaRezervacije struct {
	//ID            int64  `json:"id"`
	//RezervacijaID int64  `json:"rezervacija_id"`
	UslugaNaziv string `json:"usluga_naziv"`
	Cena        int64  `json:"cena"`
}

type ReservationRequest struct {
	Kupac             Kupac               `json:"kupac"`
	Termin            string              `json:"termin"`
	Cena              int64               `json:"cena"`
	PromoKod          string              `json:"promo_kod,omitempty"`
	StavkeRezervacije []StavkaRezervacije `json:"stavke_rezervacije"`
}

type ReservationResponse struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

type DeleteReservationRequest struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type DeleteReservationResponse struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

type GetReservationRequest struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type GetReservationResponse struct {
	ID                int64                  `json:"id"`
	UkupnaCena        int64                  `json:"ukupna_cena"`
	StavkeRezervacije []StavkaRezervacijeGet `json:"stavke_rezervacije"`
}

type StavkaRezervacijeGet struct {
	ID          int64  `json:"id"`
	UslugaID    int    `json:"usluga_id"`
	UslugaNaziv string `json:"usluga_naziv"`
	Cena        int64  `json:"cena"`
}

type StavkaRezervacijeInsert struct {
	RezervacijaID int64  `json:"rezervacija_id"`
	UslugaNaziv   string `json:"usluga_naziv"`
	Cena          int64  `json:"cena"`
}

type StavkaRezervacijeInsertResponse struct {
	ID    int64     `json:"id"`
	UslugaNaziv string    `json:"usluga_naziv"`
	Cena        int64     `json:"cena"`
	UkupnaCena  int64     `json:"ukupna_cena"`
	Termin      time.Time `json:"vreme"`
}

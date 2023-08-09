package models

type Kupac struct {
	Ime           string `json:"ime"`
	Prezime       string `json:"prezime"`
	Kompanija     string `json:"kompanija"`
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

type ReservationRequest struct {
	Kupac    Kupac  `json:"kupac"`
	Termin   string `json:"termin"`
	UslugaID int    `json:"usluga_id"`
	Cena     int64  `json:"cena"`
	PromoKod string `json:"promo_kod,omitempty"`
}

type ReservationResponse struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

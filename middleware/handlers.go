package middleware

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"salon/models"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Handler za serviranje početne stranice
func ServeFrontEnd(w http.ResponseWriter, r *http.Request) {
	// Učitajte HTML fajl sa početnom stranicom i šaljite ga kao HTTP odgovor
	http.ServeFile(w, r, "static/html/index.html")
}

/*type response struct {
	Id      int64  `json:"id, omitempty"`
	Message string `json:"message,omitempty"`
}*/

func createConnection() *sql.DB {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		panic(err)
	}

	err = db.Ping()

	if err != nil {
		panic(err)
	}

	fmt.Println("Successfully connected to postgres..")
	return db
}

// Funkcija za generisanje slučajnog promo koda
func generatePromoKod() string {
	const promoKodLength = 8 // Dužina promo koda

	rand.Seed(time.Now().UnixNano())

	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Karakteri koji će biti uključeni u generisani kod
	result := make([]byte, promoKodLength)

	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}

// Funkcija za generisanje slučajnog tokena
func generateToken() string {
	const tokenLength = 16 // Dužina tokena

	rand.Seed(time.Now().UnixNano())

	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // Karakteri koji će biti uključeni u generisani token
	result := make([]byte, tokenLength)

	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}

func CreateReservation(w http.ResponseWriter, r *http.Request) {
	var reservation models.ReservationRequest

	err := json.NewDecoder(r.Body).Decode(&reservation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prvo kreiramo ili dohvatamo korisnika na osnovu email adrese
	customerID := getOrCreateCustomer(reservation.Kupac)

	// Zatim kreiramo rezervaciju sa dobijenim korisnikom
	insertID := insertReservation(customerID, reservation)

	res := models.ReservationResponse{
		ID:      insertID,
		Message: "Reservation created successfully",
	}
	json.NewEncoder(w).Encode(res)
}

func insertReservation(customerID int64, reservation models.ReservationRequest) int64 {
	db := createConnection()
	defer db.Close()

	///Check time for reservation////
	/*isFree, err := checkOverlap(db, reservation.UslugaID, reservation.Termin)
	if isFree == false {
		log.Fatalf("Vec postoji rezervacija u ovom terminu, probajte neki drugi termin!")
	}*/
	//////

	/////Check early bird
	layout := "2006-01-02 15:04:05"
	termin, err := time.Parse(layout, reservation.Termin)
	if err != nil {
		log.Fatalf("greska prilikom menjanja vremena")
	}
	isEarlyBird := earlyBird(termin)
	if isEarlyBird == true {
		reservation.Cena = int64(float64(reservation.Cena) * 0.95)
		fmt.Println("Dobili ste popust jer je vas termin pre 2.10.2023.")
	}
	////

	////Check promo kod
	if reservation.PromoKod != "" {
		validPromo, err := checkPromoCodeWithDB(db, customerID, reservation.PromoKod)
		if err != nil {
			log.Fatalf("error promo kod %v", err)
		}
		if validPromo == true {
			reservation.Cena = int64(float64(reservation.Cena) * 0.90)
			fmt.Println("Dobili ste popust zbog promo koda")
		} else {
			log.Fatalf("Iskoriscen promo kod! %v", reservation.PromoKod)
		}
	}
	/////

	promokod := generatePromoKod()
	token := generateToken()

	sqlStatement := `
		INSERT INTO rezervacija(
			vreme, promo_kod, token,ukupna_cena, kupac_id
		) VALUES ($1, $2, $3, $4, $5) RETURNING id`

	var id int64

	// Simulacija trajanja rezervacije u minutima
	//durationInMinutes := "2023-08-09 08:30:00"

	err = db.QueryRow(
		sqlStatement,
		reservation.Termin, promokod, token, reservation.Cena, customerID,
	).Scan(&id)

	if err != nil {
		log.Fatalf("Unable to execute the query %v", err)
	}

	// Dodajemo stavke rezervacije u bazu
	for _, stavka := range reservation.StavkeRezervacije {
		applyDiscountToStavka(db, stavka)
		insertStavkaRezervacije(id, stavka)
	}

	return id
}

func insertStavkaRezervacije(rezervacijaID int64, stavka models.StavkaRezervacije) {
	db := createConnection()
	defer db.Close()

	sqlStatement := `
		INSERT INTO stavka_rezervacije(
			rezervacija_id, usluga_id, usluga_naziv, cena
		) VALUES ($1, $2, $3, $4)`

	_, err := db.Exec(
		sqlStatement,
		rezervacijaID, stavka.UslugaID, stavka.UslugaNaziv, stavka.Cena,
	)

	if err != nil {
		log.Fatalf("Unable to execute the query %v", err)
	}
}

func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var kupac models.Kupac

	err := json.NewDecoder(r.Body).Decode(&kupac)
	if err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	customerID := getOrCreateCustomer(kupac)
	if err != nil {
		http.Error(w, "Error creating customer", http.StatusInternalServerError)
		return
	}

	response := models.KupacResponse{
		ID:      customerID,
		Message: "Customer created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getOrCreateCustomer(kupac models.Kupac) int64 {
	db := createConnection()
	defer db.Close()

	var customerID int64

	// Provera da li korisnik već postoji u bazi
	err := db.QueryRow(
		"SELECT id FROM kupac WHERE email = $1",
		kupac.Email,
	).Scan(&customerID)

	// Ako korisnik ne postoji, kreiraj novog korisnika
	if err != nil {
		err := db.QueryRow(
			"INSERT INTO kupac(ime, prezime, kompanija, adresa1, adresa2, postanski_broj, mesto, drzava, email, potvrda_email) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id",
			kupac.Ime, kupac.Prezime, kupac.Kompanija, kupac.Adresa1, kupac.Adresa2, kupac.PostanskiBroj, kupac.Mesto, kupac.Drzava, kupac.Email, kupac.EmailPotvrda,
		).Scan(&customerID)

		if err != nil {
			log.Fatalf("Unable to execute the query %v", err)
		}
	}

	return customerID
}

func applyDiscount(price int64) int64 {
	return int64(float64(price) * 0.9)
}

func applyDiscountToStavka(db *sql.DB, stavka models.StavkaRezervacije) error {
	// Upit za prebrojavanje prethodnih rezervacija sa istim rezervacija_id i istom kategorijom usluge
	query := `
		SELECT COUNT(*) FROM stavka_rezervacije sr
		JOIN rezervacija r ON sr.rezervacija_id = r.id
		JOIN usluga u ON sr.usluga_id = u.id
		WHERE r.rezervacija_id = $1 AND u.kategorija_id = (SELECT kategorija_id FROM usluga WHERE id = $2)`

	var brojRezervacija int
	err := db.QueryRow(query, stavka.RezervacijaID, stavka.UslugaID).Scan(&brojRezervacija)
	if err != nil {
		return err
	}
	fmt.Println(brojRezervacija)

	// Ako je broj rezervacija neparnih, primeni popust na cenu stavke
	if brojRezervacija%2 == 1 {
		fmt.Println("uslo je u paran!")
		stavka.Cena = applyDiscount(stavka.Cena)
	}

	return nil
}

func earlyBird(termin time.Time) bool {
	earlyBirdDatum := time.Date(2023, time.October, 2, 0, 0, 0, 0, time.UTC)
	return termin.Before(earlyBirdDatum)
}

func checkPromoCodeWithDB(db *sql.DB, kupacID int64, promoKod string) (bool, error) {
	var existingPromoCode string
	err := db.QueryRow("SELECT promo_kod FROM rezervacija WHERE promo_kod = $1 AND kupac_id != $2 AND koristio_promo_kod = false", promoKod, kupacID).Scan(&existingPromoCode)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("nema takav red u bazi")
			return false, nil // Promo kod ne postoji ili je već iskorišćen
		}
		return false, err // Greška pri izvršavanju upita
	}

	// Ako promo kod postoji, ažuriraj status i dodaj poruku korisniku
	_, err = db.Exec("UPDATE rezervacija SET koristio_promo_kod = true WHERE promo_kod = $1", promoKod)
	if err != nil {
		return false, err // Greška pri ažuriranju statusa
	}

	return true, nil // Promo kod je validan
}

func parseDurationString(durationStr string) (time.Duration, error) {
	parts := strings.Split(durationStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}

	duration := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	return duration, nil
}

func checkOverlap(db *sql.DB, uslugaID int64, termin string) (bool, error) {
	// Izvuci trajanje usluge iz baze
	var trajanjeStr string
	query := "SELECT trajanje FROM usluga WHERE id = $1"
	err := db.QueryRow(query, termin).Scan(&trajanjeStr)
	fmt.Println(trajanjeStr)
	if err != nil {
		return false, err
	}

	splitTrajanje := strings.Split(trajanjeStr, ":")
	sati, _ := strconv.Atoi(splitTrajanje[0])
	minuti, _ := strconv.Atoi(splitTrajanje[1])
	sekunde, _ := strconv.Atoi(splitTrajanje[2])

	trajanje := time.Duration(sati)*time.Hour + time.Duration(minuti)*time.Minute + time.Duration(sekunde)*time.Second
	fmt.Println(trajanje)

	// Parsiraj termin iz stringa u time.Time
	terminStr, err := time.Parse("2006-01-02 15:04:05", termin)
	fmt.Println(terminStr)
	if err != nil {
		return false, err
	}

	// Izračunaj vreme završetka rezervacije
	vremeZavrsetka := terminStr.Add(trajanje)
	fmt.Println(vremeZavrsetka)

	query = `
	SELECT COUNT(*)
	FROM rezervacija
	WHERE usluga_id = $1
	AND vreme BETWEEN $2 AND $3
    `
	var brojPreklapanja int
	err = db.QueryRow(query, uslugaID, terminStr, vremeZavrsetka).Scan(&brojPreklapanja)
	if err != nil {
		fmt.Println("greska prilikom izvršavanja upita:", err)
		return false, err
	}

	fmt.Println(brojPreklapanja)
	// Ako ima preklapanja, rezervacija se ne može napraviti
	return brojPreklapanja == 0, nil
}

// ///brisanje rezervacije i njenih stavki ///////////////
func DeleteReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	fmt.Println("usao u delete funkciju")

	var request models.DeleteReservationRequest

	fmt.Println("dekodiranje...")

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		fmt.Println("error pri dekodiranju", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("uspesno dekodiranje")

	// Provera da li rezervacija postoji sa datim tokenom i emailom
	reservationID, err := findReservationID(request.Token, request.Email)
	if err != nil {
		http.Error(w, "Reservation not found", http.StatusNotFound)
		return
	}

	// Brisanje svih stavki rezervacije
	err = deleteStavkeRezervacije(reservationID)
	if err != nil {
		http.Error(w, "Failed to delete reservation items", http.StatusInternalServerError)
		return
	}

	// Brisanje rezervacije
	deletedRows := deleteReservation(reservationID)
	if deletedRows == 0 {
		http.Error(w, "Failed to delete reservation", http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Reservation deleted successfully, rows affected: %v", deletedRows)
	res := models.DeleteReservationResponse{
		ID:      reservationID,
		Message: msg,
	}
	// Ako je sve u redu, postavite HTTP status 200 OK
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func findReservationID(token, email string) (int64, error) {
	db := createConnection()
	defer db.Close()

	// Provera da li postoji rezervacija sa datim tokenom i emailom kupca
	var reservationID int64
	sqlStatement := `SELECT r.id FROM rezervacija r
					 JOIN kupac k ON r.kupac_id = k.id
					 WHERE r.token = $1 AND k.email = $2`

	err := db.QueryRow(sqlStatement, token, email).Scan(&reservationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, errors.New("Reservation not found")
		}
		return 0, err
	}

	return reservationID, nil
}

func deleteReservation(id int64) int64 {
	db := createConnection()
	defer db.Close()

	sqlStatement := `DELETE FROM rezervacija WHERE id=$1`

	res, err := db.Exec(sqlStatement, id)
	if err != nil {
		log.Fatalf("Unable to execute the query %v", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("Error while checking the affected rows %v", err)
	}

	return rowsAffected
}

func deleteStavkeRezervacije(reservationID int64) error {
	db := createConnection()
	defer db.Close()

	sqlStatement := `DELETE FROM stavka_rezervacije WHERE rezervacija_id = $1`

	_, err := db.Exec(sqlStatement, reservationID)
	if err != nil {
		return err
	}

	return nil
}

////////////////////////
/////// Get Reservation////////

func GetReservation(w http.ResponseWriter, r *http.Request) {
	var request models.GetReservationRequest
	fmt.Println("usao je u GetRes")

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		fmt.Println("error pri dekodiranju", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Izvršite upit za dohvat rezervacije i njenih stavki
	reservation, err := getReservation(request.Token, request.Email)
	if err != nil {
		// Slanje odgovora u slučaju greške
		fmt.Println("ovde je greska!!!")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	// Postavite Content-Type zaglavlje na application/json
	w.Header().Set("Content-Type", "application/json")

	// Koristite json.NewEncoder za slanje odgovora u JSON formatu
	if err := json.NewEncoder(w).Encode(reservation); err != nil {
		// Slanje odgovora u slučaju greške
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
}

func getReservation(token, email string) (models.GetReservationResponse, error) {
	fmt.Println("usao je u getRes")
	db := createConnection()
	defer db.Close()

	var reservation models.GetReservationResponse

	// Prvo dobijemo ID kupca na osnovu email-a
	var kupacID int64
	err := db.QueryRow("SELECT id FROM kupac WHERE email = $1", email).Scan(&kupacID)
	if err != nil {
		return reservation, err
	}
	fmt.Println(kupacID)

	err = db.QueryRow(`
	SELECT r.id, r.ukupna_cena
	FROM rezervacija r
	WHERE r.token = $1 AND r.kupac_id = $2`,
		token, kupacID).Scan(&reservation.ID, &reservation.UkupnaCena)

	fmt.Println(token)
	fmt.Println(reservation.ID)
	fmt.Println(reservation.UkupnaCena)

	// Provera da li je rezervacija pronađena ili ne
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("greska!!!")
			return reservation, fmt.Errorf("Rezervacija nije pronađena")
		}
		fmt.Println("greska 2")
		return reservation, err
	}

	// Dohvat stavki rezervacije
	rows, err := db.Query(`
	 SELECT srg.id, srg.usluga_id, us.naziv AS usluga_naziv, us.cena
	 FROM stavka_rezervacije srg
	 JOIN usluga us ON srg.usluga_id = us.id
	 WHERE srg.rezervacija_id = $1`, reservation.ID)
	if err != nil {
		fmt.Println("greska 3")
		return reservation, err
	}
	defer rows.Close()

	var stavke []models.StavkaRezervacijeGet
	for rows.Next() {
		var stavka models.StavkaRezervacijeGet
		if err = rows.Scan(&stavka.ID, &stavka.UslugaID, &stavka.UslugaNaziv, &stavka.Cena); err != nil {
			fmt.Println("ovde je greska")
			return reservation, err
		}
		stavke = append(stavke, stavka)
	}
	reservation.StavkeRezervacije = stavke

	fmt.Println("kraj getRes")
	return reservation, nil
}

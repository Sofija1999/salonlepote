package middleware

import (
	"database/sql"
	"encoding/json"
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
		log.Fatalf("Unable to decode the request body, %v", err)
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
	isFree, err := checkOverlap(db, reservation.UslugaID, reservation.Termin)
	if isFree == false {
		log.Fatalf("Vec postoji rezervacija u ovom terminu, probajte neki drugi termin!")
	}
	//////

	////Check number of services
	paranBrojUsluga, err := checkNumberOfService(db, customerID, reservation.UslugaID)
	if paranBrojUsluga == false {
		reservation.Cena = int64(float64(reservation.Cena) * 0.9)
		fmt.Println("Dobili ste popust na svaku drugu uslugu iz iste kategorije")
	}
	/////

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
			vreme, promo_kod, token,ukupna_cena, kupac_id, usluga_id
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	var id int64

	// Simulacija trajanja rezervacije u minutima
	//durationInMinutes := "2023-08-09 08:30:00"

	err = db.QueryRow(
		sqlStatement,
		reservation.Termin, promokod, token, reservation.Cena, customerID, reservation.UslugaID,
	).Scan(&id)

	if err != nil {
		log.Fatalf("Unable to execute the query %v", err)
	}

	return id
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

func checkNumberOfService(db *sql.DB, kupacID int64, uslugaID int) (bool, error) {
	//brojac
	var brojRezervacija int

	//upit za prebrojavanje rezervacija
	query := "SELECT COUNT(*) FROM rezervacija r JOIN usluga u ON r.usluga_id = u.id WHERE r.kupac_id = $1 AND u.kategorija_id = (SELECT kategorija_id FROM usluga WHERE id = $2)"
	err := db.QueryRow(query, kupacID, uslugaID).Scan(&brojRezervacija)
	if err != nil {
		return false, err
	}

	// Provera da li je broj rezervacija paran ili neparan
	isParan := brojRezervacija%2 == 0

	return isParan, nil
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

func checkOverlap(db *sql.DB, uslugaID int, termin string) (bool, error) {
	// Izvuci trajanje usluge iz baze
	var trajanjeStr string
	query := "SELECT trajanje FROM usluga WHERE id = $1"
	err := db.QueryRow(query, uslugaID).Scan(&trajanjeStr)
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

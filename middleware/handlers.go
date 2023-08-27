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

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Handler za serviranje početne stranice
func ServeFrontEnd(w http.ResponseWriter, r *http.Request) {
	// Učitajte HTML fajl sa početnom stranicom i šaljite ga kao HTTP odgovor
	http.ServeFile(w, r, "static/html/index.html")
}

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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	var reservation models.ReservationRequest

	err := json.NewDecoder(r.Body).Decode(&reservation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//kreiramo kupca ukoliko vec ne postoji u bazi
	customerID := getOrCreateCustomer(reservation.Kupac)

	//kreiram rezervaciju sa korisnikom
	insertID, promoKod, token, err, ukupnaCena := insertReservation(customerID, reservation)
	if err != nil {
		if err.Error() == "Promo kod je nevažeći ili već iskorišćen." {
			res := models.ReservationResponse{
				ID:       0, // Neuspešno kreiranje rezervacije, postavite željenu vrednost
				Message:  "Reservation creation failed",
				PromoKod: "",
				Token:    "",
				Error:    err.Error(), // Postavljanje poruke o grešci
			}
			json.NewEncoder(w).Encode(res)
		}
		if err.Error() == "rezervacija već postoji za ovaj termin" {
			res := models.ReservationResponse{
				ID:       0, // Neuspešno kreiranje rezervacije, postavite željenu vrednost
				Message:  "Reservation creation failed",
				PromoKod: "",
				Token:    "",
				Error:    err.Error(), // Postavljanje poruke o grešci
			}
			json.NewEncoder(w).Encode(res)
		}

		return
	}

	fmt.Println(insertID)

	res := models.ReservationResponse{
		ID:         insertID,
		Message:    "Reservation created successfully",
		PromoKod:   promoKod,
		Token:      token,
		Error:      "", // Postavljanje inicijalne vrednosti za grešku
		UkupnaCena: ukupnaCena,
	}
	fmt.Println(res)
	json.NewEncoder(w).Encode(res)
}

func insertReservation(customerID int64, reservation models.ReservationRequest) (int64, string, string, error, int64) {
	fmt.Println("usao u insert reservation")
	db := createConnection()
	defer db.Close()

	///Check time for reservation////
	isFree, err := checkOverlapWithTotalDuration(db, reservation.StavkeRezervacije, reservation.Termin)
	if err != nil {
		return 0, "", "", fmt.Errorf("greška prilikom provere preklapanja rezervacija: %v", err), 0
	}
	if !isFree {
		return 0, "", "", fmt.Errorf("rezervacija već postoji za ovaj termin"), 0
	}
	//////

	/////Check early bird
	layout := "2006-01-02T15:04" // Format za datetime-local
	termin, err := time.Parse(layout, reservation.Termin)
	if err != nil {
		fmt.Println("greska u menjanju vremena")
		log.Fatalf("greska prilikom menjanja vremena")
	}
	isEarlyBird := earlyBird(termin)
	if isEarlyBird == true {
		//reservation.Cena = int64(float64(reservation.Cena) * 0.95)
		//fmt.Println("Dobili ste popust jer je vas termin pre 2.10.2023.")
	}
	////

	fmt.Println(reservation.PromoKod)
	validPromo := false

	////Check promo kod
	if reservation.PromoKod != "" {
		fmt.Println("usao u proveru promo koda")
		validPromo, err = checkPromoCodeWithDB(db, customerID, reservation.PromoKod)
		fmt.Println(validPromo)
		if err != nil {
			log.Fatalf("error promo kod %v", err)
		}
		if validPromo == true {
			fmt.Println("valid je true")
			validPromo = true
			//reservation.Cena = int64(float64(reservation.Cena) * 0.90)
			//fmt.Println("Dobili ste popust zbog promo koda")
		} else {
			fmt.Println("usao u else")
			return 0, "", "", fmt.Errorf("Promo kod je nevažeći ili već iskorišćen."), 0
		}
	}

	/////

	promokod := generatePromoKod()
	token := generateToken()

	sqlStatement := `
		INSERT INTO rezervacija(
			vreme, promo_kod, token,ukupna_cena, kupac_id
		) VALUES ($1, $2, $3, $4, $5) RETURNING id, promo_kod, token, ukupna_cena`

	var id int64
	var ukupnaCena int64

	err = db.QueryRow(
		sqlStatement,
		reservation.Termin, promokod, token, reservation.Cena, customerID,
	).Scan(&id, &promokod, &token, &ukupnaCena)

	if err != nil {
		log.Fatalf("Unable to execute the query %v", err)
	}
	fmt.Println("prosao upisivanje rezervacije")

	// Dodajemo stavke rezervacije u bazu
	for _, stavka := range reservation.StavkeRezervacije {
		var novaCena int64
		fmt.Println("usao u for petlju za stavke")
		novaCena, err = applyDiscountToStavka(db, stavka, id)
		insertStavkaRezervacije(id, stavka, novaCena)
	}

	fmt.Println(validPromo)
	ukCena, err := updateUkupnaCena2(id, validPromo, isEarlyBird)

	return id, promokod, token, nil, ukCena
}

func updateUkupnaCena2(rezervacijaID int64, validPromo bool, isEarlyBird bool) (int64, error) {
	db := createConnection()
	defer db.Close()

	query := `
        SELECT SUM(cena)
        FROM stavka_rezervacije
        WHERE rezervacija_id = $1
    `

	var ukupnaCena int64
	err := db.QueryRow(query, rezervacijaID).Scan(&ukupnaCena)
	if err != nil {
		return 0, err
	}

	query = `
        UPDATE rezervacija
        SET ukupna_cena = $1
        WHERE id = $2
    `

	_, err = db.Exec(query, ukupnaCena, rezervacijaID)
	if err != nil {
		return 0, err
	}

	novaCena := int64(float64(ukupnaCena) * 0.90)

	if validPromo == true {
		fmt.Println("promo kod iskoriscen")
		query = `
        UPDATE rezervacija
        SET ukupna_cena = $1
        WHERE id = $2
    `
		_, err = db.Exec(query, novaCena, rezervacijaID)
		if err != nil {
			return 0, err
		}
	}

	novaCena2 := novaCena - (int64(float64(ukupnaCena) * 0.05))

	if isEarlyBird == true {
		fmt.Println("early bird iskoriscen")
		query = `
        UPDATE rezervacija
        SET ukupna_cena = $1
        WHERE id = $2
    `
		_, err = db.Exec(query, novaCena2, rezervacijaID)
		if err != nil {
			return 0, err
		}
	}

	query = `
        SELECT ukupna_cena
        FROM rezervacija
        WHERE id = $1
    `

	err = db.QueryRow(query, rezervacijaID).Scan(&ukupnaCena)
	if err != nil {
		return 0, err
	}

	return ukupnaCena, nil
}

func getUslugaIDByNaziv(naziv string) (int64, error) {
	db := createConnection()
	defer db.Close()

	var uslugaID int64
	sqlStatement := `SELECT id FROM usluga WHERE naziv = $1`
	fmt.Println(naziv)

	row := db.QueryRow(sqlStatement, naziv)
	err := row.Scan(&uslugaID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("Usluga sa nazivom '%s' nije pronađena", naziv)
		}
		return 0, err
	}

	return uslugaID, nil
}

func insertStavkaRezervacije(rezervacijaID int64, stavka models.StavkaRezervacije, novaCena int64) {
	fmt.Println("usao u insert stavka rez")
	db := createConnection()
	defer db.Close()

	usluga_id, err := getUslugaIDByNaziv(stavka.UslugaNaziv)
	fmt.Println(usluga_id)
	fmt.Println(stavka.Cena)

	sqlStatement := `
		INSERT INTO stavka_rezervacije(
			rezervacija_id, usluga_id, usluga_naziv, cena
		) VALUES ($1, $2, $3, $4)`

	fmt.Println("prosao sqlStatement")
	_, err = db.Exec(
		sqlStatement,
		rezervacijaID, usluga_id, stavka.UslugaNaziv, novaCena,
	)

	if err != nil {
		fmt.Println("error u insert stavka")
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

	//provera da li kupac vec postoji u bazi
	err := db.QueryRow(
		"SELECT id FROM kupac WHERE email = $1",
		kupac.Email,
	).Scan(&customerID)

	//ako kupac ne postoji, ubacujemo ga u bazu
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

func applyDiscountToStavka(db *sql.DB, stavka models.StavkaRezervacije, id int64) (int64, error) {
	fmt.Println("usao u apply")
	//upit za prebrojavanje prethodnih rezervacija sa istim rezervacija_id i istom kategorijom usluge
	usluga_id, err := getUslugaIDByNaziv(stavka.UslugaNaziv)
	fmt.Println("prosao usluga id")
	query := `
		SELECT COUNT(*) FROM stavka_rezervacije sr
		JOIN rezervacija r ON sr.rezervacija_id = r.id
		JOIN usluga u ON sr.usluga_id = u.id
		WHERE sr.rezervacija_id = $1 AND u.kategorija_id = (SELECT kategorija_id FROM usluga WHERE id = $2)`

	var brojRezervacija int
	err = db.QueryRow(query, id, usluga_id).Scan(&brojRezervacija)
	if err != nil {
		return 0, err
	}
	fmt.Println(brojRezervacija)
	var novaCena int64

	// Ako je broj rezervacija neparnih, primeni popust na cenu stavke
	if brojRezervacija%2 == 1 {
		fmt.Println("uslo je u paran!")
		novaCena = int64(float64(stavka.Cena) * 0.9)
		fmt.Println(novaCena)
	} else {
		novaCena = stavka.Cena
	}

	return novaCena, nil
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

	//ako promo kod postoji, azurira se status da je iskorisce promo kod
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

func checkOverlapWithTotalDuration(db *sql.DB, stavkeRezervacije []models.StavkaRezervacije, termin string) (bool, error) {
	//ukupno trajanje svih usluga u rezervaciji
	totalDuration, err := calculateTotalDurationFromStavke(db, stavkeRezervacije)
	fmt.Println(totalDuration)

	fmt.Println(termin)
	//parsira se termin iz stringa u time.Time
	terminStr, err := time.Parse("2006-01-02T15:04", termin)
	fmt.Println(terminStr)
	if err != nil {
		fmt.Println("ovde je greska!!")
		return false, err
	}

	//vreme završetka nove rezervacije
	vremeZavrsetka := terminStr.Add(totalDuration)
	fmt.Println(vremeZavrsetka)

	//provera radno vreme
	workEnd, _ := time.Parse("15:04:05", "18:00:00")

	if vremeZavrsetka.Hour() > workEnd.Hour() {
		fmt.Println(vremeZavrsetka)
		fmt.Println(workEnd)
		return false, nil
	}

	query := `SELECT COUNT(*)
			FROM rezervacija
			WHERE vreme BETWEEN $1 AND $2`

	var brojPreklapanja int
	err = db.QueryRow(query, terminStr, vremeZavrsetka).Scan(&brojPreklapanja)
	fmt.Println(brojPreklapanja)
	if err != nil {
		return false, err
	}

	if brojPreklapanja != 0 {
		fmt.Println("broj preklapanja veci od nule")
		return false, nil
	}

	return true, nil
}

func calculateTotalDurationFromStavke(db *sql.DB, stavke []models.StavkaRezervacije) (time.Duration, error) {
	totalDuration := time.Duration(0)
	fmt.Println("usao u calculate total duration")

	for _, stavka := range stavke {
		// Dobiti trajanje usluge za datu stavku
		var trajanjeStr string
		query := "SELECT trajanje FROM usluga WHERE naziv = $1"
		err := db.QueryRow(query, stavka.UslugaNaziv).Scan(&trajanjeStr)
		if err != nil {
			return 0, err
		}

		// Pretvoriti trajanje iz stringa u vreme
		splitTrajanje := strings.Split(trajanjeStr, ":")
		sati, _ := strconv.Atoi(splitTrajanje[0])
		minuti, _ := strconv.Atoi(splitTrajanje[1])
		sekunde, _ := strconv.Atoi(splitTrajanje[2])

		trajanje := time.Duration(sati)*time.Hour + time.Duration(minuti)*time.Minute + time.Duration(sekunde)*time.Second
		totalDuration += trajanje
	}

	return totalDuration, nil
}

// ///brisanje rezervacije i njenih stavki ///////////////
func DeleteReservation(w http.ResponseWriter, r *http.Request) {
	/*w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")*/

	var request models.DeleteReservationRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		fmt.Println("error pri dekodiranju", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("uspesno dekodiranje")

	//provera da li rezervacija postoji sa datim tokenom i emailom
	reservationID, err := findReservationID(request.Token, request.Email)
	if err != nil {
		http.Error(w, "Reservation not found", http.StatusNotFound)
		return
	}

	//brisanje svih stavki rezervacije
	err = deleteStavkeRezervacije(reservationID)
	if err != nil {
		http.Error(w, "Failed to delete reservation items", http.StatusInternalServerError)
		return
	}

	//brisanje rezervacije
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
	//ako je sve u redu, postavlja se http na 200
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func findReservationID(token, email string) (int64, error) {
	db := createConnection()
	defer db.Close()

	//provera da li postoji rezervacija sa datim tokenom i emailom kupca
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
	fmt.Println("usao je u GetRes")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	token := r.URL.Query().Get("token")
	email := r.URL.Query().Get("email")

	fmt.Println("Token:", token)
	fmt.Println("Email:", email)

	//na osnovu tokena i emaila vracamo rezervaciju iz baze
	reservation, err := getReservation(token, email)
	if err != nil {
		//slanje odgovora u slučaju greške
		fmt.Println("ovde je greska!!!")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	//koristite json.NewEncoder za slanje odgovora u JSON formatu
	if err := json.NewEncoder(w).Encode(reservation); err != nil {
		//slanje odgovora u slučaju greške
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
}

func getReservation(token, email string) (models.GetReservationResponse, error) {
	fmt.Println("usao je u getRes")
	db := createConnection()
	defer db.Close()

	var reservation models.GetReservationResponse

	//uzimamo ID kupca na osnovu email-a
	var kupacID int64
	err := db.QueryRow("SELECT id FROM kupac WHERE email = $1", email).Scan(&kupacID)
	fmt.Println(kupacID)
	fmt.Println(email)
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

	//provera da li je rezervacija pronađena ili ne
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("greska!!!")
			return reservation, fmt.Errorf("Rezervacija nije pronađena")
		}
		fmt.Println("greska 2")
		return reservation, err
	}

	//dohvat stavki rezervacije
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

/////delete stavka rezervacije

// Funkcija za brisanje stavke iz rezervacije na osnovu uslugaID
func DeleteStavka(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uslugaID := vars["uslugaID"]
	fmt.Println("usao u delete stavka")

	// Dohvati ID rezervacije na osnovu uslugaID
	reservationID, err := getReservationIDByUslugaID(uslugaID)
	if err != nil {
		fmt.Println("eror2")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Pozovi funkciju za brisanje stavke
	err = deleteStavkaByID(uslugaID)
	if err != nil {
		fmt.Println("erorr!!!!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ažuriraj ukupnu cenu rezervacije
	ukupnaCena, err := updateUkupnaCena(reservationID)
	if err != nil {
		fmt.Println("eror3")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.StavkaRezervacijeDeleteResponse{
		Message:    "Stavka je uspešno obrisana iz rezervacije.",
		UkupnaCena: ukupnaCena,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(responseJSON)

}

// Funkcija za brisanje stavke po uslugaID
func deleteStavkaByID(uslugaID string) error {
	db := createConnection()
	defer db.Close()
	fmt.Println("usao u delete stavka by id")

	// SQL upit za brisanje stavke po uslugaID
	sqlStatement := `
        DELETE FROM stavka_rezervacije
        WHERE id = $1`

	// Izvrši SQL upit za brisanje stavke
	_, err := db.Exec(sqlStatement, uslugaID)
	if err != nil {
		fmt.Println("error 1")
		return err
	}

	return nil
}

// Funkcija za dohvatanje ID rezervacije na osnovu uslugaID
func getReservationIDByUslugaID(uslugaID string) (int64, error) {
	db := createConnection()
	defer db.Close()

	var rezervacijaID int64

	// SQL upit za dohvatanje ID rezervacije na osnovu uslugaID
	sqlStatement := `
        SELECT rezervacija_id FROM stavka_rezervacije
        WHERE id = $1`
	fmt.Println(uslugaID)

	// Izvrši SQL upit za dohvatanje ID rezervacije
	row := db.QueryRow(sqlStatement, uslugaID)
	err := row.Scan(&rezervacijaID)
	fmt.Println(rezervacijaID)
	if err != nil {
		fmt.Println("eror u get res id by usluga")
		return 0, err
	}

	return rezervacijaID, nil
}

// Ažuriraj ukupnu cenu rezervacije
func updateUkupnaCena(reservationID int64) (int64, error) {
	db := createConnection()
	defer db.Close()

	var novaUkupnaCena int64

	// SQL upit za dohvatanje nove ukupne cene rezervacije na osnovu ID rezervacije
	sqlStatement := `
        SELECT COALESCE(SUM(cena), 0) FROM stavka_rezervacije
        WHERE rezervacija_id = $1`

	// Izvrši SQL upit za dohvatanje nove ukupne cene
	row := db.QueryRow(sqlStatement, reservationID)
	err := row.Scan(&novaUkupnaCena)
	if err != nil {
		fmt.Println("eror u ukupna cena")
		return 0, err
	}

	// Ažuriraj ukupnu cenu rezervacije u bazi
	updateStatement := `
        UPDATE rezervacija SET ukupna_cena = $1 WHERE id = $2`

	_, err = db.Exec(updateStatement, novaUkupnaCena, reservationID)
	if err != nil {
		fmt.Println("eror ukupna cena 2")
		return 0, err
	}

	return novaUkupnaCena, nil
}

////Create stavka rezervacije

func CreateStavka(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	var novaStavka models.StavkaRezervacijeInsert
	fmt.Println("usao u create stavka")
	fmt.Println(r.Body)

	// Dekodiranje JSON zahteva
	err := json.NewDecoder(r.Body).Decode(&novaStavka)
	fmt.Println(r.Body)
	if err != nil {
		fmt.Println("greska u dekodiranju")
		http.Error(w, "Greška pri dekodiranju JSON zahteva", http.StatusBadRequest)
		return
	}

	fmt.Println("prosao dekodiranje")
	fmt.Println(novaStavka.RezervacijaID)
	fmt.Println(novaStavka.UslugaNaziv)
	fmt.Println(novaStavka.Cena)
	// Dohvatanje ID-a usluge na osnovu naziva usluge

	uslugaID, err := getUslugaIDByNaziv(novaStavka.UslugaNaziv)
	if err != nil {
		fmt.Println("greska u get usluga id by naziv")
		http.Error(w, "Greška pri dohvatanju ID-a usluge", http.StatusInternalServerError)
		return
	}

	termin, err := getTerminRezervacije(novaStavka.RezervacijaID)
	listaStavki, err := getStavkeByReservationID(novaStavka.RezervacijaID)

	// Upisivanje nove stavke rezervacije u bazu
	id, err := insertStavkaRezervacije2(novaStavka, uslugaID, termin, listaStavki)
	if err != nil {
		fmt.Println("greska pri insertu stavke")
		http.Error(w, "Greška pri upisivanju stavke rezervacije u bazu", http.StatusInternalServerError)
		return
	}

	// Ažuriranje ukupne cene rezervacije
	ukupnaCena, err := azurirajUkupnuCenuRezervacije(novaStavka.RezervacijaID, novaStavka.Cena)
	if err != nil {
		fmt.Println("greska pri azuriranju cene")
		http.Error(w, "Greška pri ažuriranju ukupne cene rezervacije", http.StatusInternalServerError)
		return
	}

	var odgovor models.StavkaRezervacijeInsertResponse

	odgovor.UkupnaCena = ukupnaCena
	odgovor.ID = id
	odgovor.UslugaNaziv = novaStavka.UslugaNaziv
	odgovor.Cena = novaStavka.Cena
	odgovor.Termin = termin

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(odgovor)

}

func insertStavkaRezervacije2(novaStavka models.StavkaRezervacijeInsert, uslugaID int64, termin time.Time, listaStavki []models.StavkaRezervacije) (int64, error) {
	db := createConnection()
	defer db.Close()
	fmt.Println("usao u insert stavka 2")

	// Definišite željeni format datuma i vremena
	format := "2006-01-02 15:04:05"

	// Koristite funkciju Format da pretvorite vreme u string
	terminString := termin.Format(format)
	fmt.Println(terminString)

	isFree, err := checkOverlapWithTotalDuration2(db, listaStavki, terminString)
	if err != nil {
		return 0, fmt.Errorf("greška prilikom provere preklapanja rezervacija: %v", err)
	}
	if !isFree {
		return 0, fmt.Errorf("rezervacija već postoji za ovaj termin")
	}

	// Priprema SQL upita za unos nove stavke rezervacije
	sqlStatement := `
		INSERT INTO stavka_rezervacije (rezervacija_id, usluga_id, usluga_naziv, cena)
		VALUES ($1, $2, $3, $4) RETURNING id
	`

	var id int64 // Promenljiva za čuvanje rezultata

	// Izvršavanje SQL upita i čuvanje rezultata
	err = db.QueryRow(sqlStatement, novaStavka.RezervacijaID, uslugaID, novaStavka.UslugaNaziv, novaStavka.Cena).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func azurirajUkupnuCenuRezervacije(rezervacijaID int64, novaCena int64) (int64, error) {
	db := createConnection()
	defer db.Close()

	//trenutna ukupna cena rezervacije
	trenutnaUkupnaCena, err := getUkupnaCenaRezervacije(rezervacijaID)
	if err != nil {
		return 0, err
	}

	//izračunavanje nove ukupne cene
	novaUkupnaCena := trenutnaUkupnaCena + novaCena

	// Priprema SQL upita za ažuriranje ukupne cene rezervacije
	sqlStatement := `
		UPDATE rezervacija
		SET ukupna_cena = $1
		WHERE id = $2
	`

	// Izvršavanje SQL upita za ažuriranje ukupne cene
	_, err = db.Exec(sqlStatement, novaUkupnaCena, rezervacijaID)
	if err != nil {
		return 0, err
	}

	return novaUkupnaCena, nil
}

func getUkupnaCenaRezervacije(rezervacijaID int64) (int64, error) {
	db := createConnection()
	defer db.Close()

	var ukupnaCena int64

	// Priprema SQL upita za dohvatanje ukupne cene rezervacije
	sqlStatement := `
		SELECT ukupna_cena
		FROM rezervacija
		WHERE id = $1
	`

	// Izvršavanje SQL upita za dohvatanje ukupne cene
	row := db.QueryRow(sqlStatement, rezervacijaID)
	err := row.Scan(&ukupnaCena)
	if err != nil {
		return 0, err
	}

	return ukupnaCena, nil
}

// Funkcija za dohvat termina rezervacije na osnovu rezervacija ID
func getTerminRezervacije(rezervacijaID int64) (time.Time, error) {
	db := createConnection()
	defer db.Close()

	var vreme time.Time

	query := "SELECT vreme FROM rezervacija WHERE id = $1"
	err := db.QueryRow(query, rezervacijaID).Scan(&vreme)
	fmt.Println(vreme)

	if err != nil {
		fmt.Println("Greška pri dohvatanju termina:", err)
		return time.Time{}, err
	}

	return vreme, nil
}

func getStavkeByReservationID(reservationID int64) ([]models.StavkaRezervacije, error) {
	db := createConnection()
	defer db.Close()

	var stavke []models.StavkaRezervacije

	query := "SELECT usluga_naziv, cena FROM stavka_rezervacije WHERE rezervacija_id = $1"
	rows, err := db.Query(query, reservationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stavka models.StavkaRezervacije
		err := rows.Scan(&stavka.UslugaNaziv, &stavka.Cena)
		if err != nil {
			return nil, err
		}
		stavke = append(stavke, stavka)
	}

	return stavke, nil
}

func checkOverlapWithTotalDuration2(db *sql.DB, stavkeRezervacije []models.StavkaRezervacije, termin string) (bool, error) {
	// Izračunaj ukupno trajanje svih usluga u rezervaciji
	totalDuration, err := calculateTotalDurationFromStavke2(db, stavkeRezervacije)
	fmt.Println(totalDuration)

	fmt.Println(termin)
	// Parsiraj termin iz stringa u time.Time
	terminStr, err := time.Parse("2006-01-02 15:04:05", termin)
	if err != nil {
		fmt.Println("ovde je greska!!")
		return false, err
	}

	// Izračunaj vreme završetka nove rezervacije
	vremeZavrsetka := terminStr.Add(totalDuration)
	fmt.Println(vremeZavrsetka)

	//provera radno vreme
	workEnd, _ := time.Parse("15:04:05", "18:00:00")

	if vremeZavrsetka.Hour() > workEnd.Hour() {
		fmt.Println(vremeZavrsetka)
		fmt.Println(workEnd)
		return false, nil
	}

	query := `SELECT COUNT(*)
			FROM rezervacija
			WHERE vreme BETWEEN $1 AND $2`

	var brojPreklapanja int
	err = db.QueryRow(query, terminStr, vremeZavrsetka).Scan(&brojPreklapanja)
	fmt.Println(brojPreklapanja)
	if err != nil {
		return false, err
	}

	if brojPreklapanja > 1 {
		fmt.Println("broj preklapanja veci od jedan")
		return false, nil
	}

	return true, nil
}

func calculateTotalDurationFromStavke2(db *sql.DB, stavke []models.StavkaRezervacije) (time.Duration, error) {
	totalDuration := time.Duration(0)
	fmt.Println("usao u calculate total duration")

	for _, stavka := range stavke {
		// Dobiti trajanje usluge za datu stavku
		var trajanjeStr string
		query := "SELECT trajanje FROM usluga WHERE naziv = $1"
		err := db.QueryRow(query, stavka.UslugaNaziv).Scan(&trajanjeStr)
		if err != nil {
			return 0, err
		}

		// Pretvoriti trajanje iz stringa u vreme
		splitTrajanje := strings.Split(trajanjeStr, ":")
		sati, _ := strconv.Atoi(splitTrajanje[0])
		minuti, _ := strconv.Atoi(splitTrajanje[1])
		sekunde, _ := strconv.Atoi(splitTrajanje[2])

		trajanje := time.Duration(sati)*time.Hour + time.Duration(minuti)*time.Minute + time.Duration(sekunde)*time.Second
		totalDuration += trajanje
	}

	return totalDuration, nil
}

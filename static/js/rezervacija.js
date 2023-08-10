

/*document.addEventListener("DOMContentLoaded", function() {
    const otkaziBtn = document.getElementById("otkaziBtn");
    const rezultatPoruka = document.getElementById("rezultatPoruka");

    otkaziBtn.addEventListener("click", function() {
        const token = document.getElementById("token").value;
        const emailOtkaz = document.getElementById("emailOtkaz").value;

        console.log(token)
        console.log(emailOtkaz)

        // Objekat sa podacima za slanje
        const data = {
            token: token,
            email: emailOtkaz,
        };


        console.log(data)
        console.log(JSON.stringify(data));


        // Poslati zahtev na backend za otkazivanje rezervacije
        fetch("http://localhost:8080/api/deletereservation", {
            method: "DELETE",
            headers: {
                "Content-Type": "application/json"
            },
            mode: "cors", // Dodajte ovu opciju
            credentials: "same-origin", // I ovu opciju ako je potrebno
            body: JSON.stringify(data),
        })
        .then(response => response.json())
        .then(data => {
            rezultatPoruka.textContent = data.message;
        })
        .catch(error => {
            console.error("Greška prilikom otkazivanja:", error);
            rezultatPoruka.textContent = "Došlo je do greške prilikom otkazivanja rezervacije.";
        });
    });
});*/
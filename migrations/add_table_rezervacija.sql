CREATE TABLE public.rezervacija (
	id bigserial NOT NULL,
	vreme timestamp NOT NULL,
	promo_kod varchar NOT NULL,
	"token" varchar NOT NULL,
	koristio_promo_kod boolean NOT NULL DEFAULT false,
	ukupna_cena numeric NOT NULL,
	kupac_id bigserial NOT NULL,
	usluga_id varchar NOT NULL,
	CONSTRAINT rezervacija_pk PRIMARY KEY (id),
	CONSTRAINT rezervacija_fk FOREIGN KEY (usluga_id) REFERENCES public.usluga(id),
	CONSTRAINT rezervacija_fk_1 FOREIGN KEY (kupac_id) REFERENCES public.kupac(id)
);
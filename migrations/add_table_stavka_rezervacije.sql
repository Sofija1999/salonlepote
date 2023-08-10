CREATE TABLE public.stavka_rezervacije (
	id bigserial NOT NULL,
	rezervacija_id bigserial NOT NULL,
	usluga_id varchar NOT NULL,
	usluga_naziv varchar NOT NULL,
	cena numeric NOT NULL,
	CONSTRAINT stavka_rezervacije_pk PRIMARY KEY (id),
	CONSTRAINT stavka_rezervacije_fk FOREIGN KEY (usluga_id) REFERENCES public.usluga(id),
	CONSTRAINT stavka_rezervacije_fk_1 FOREIGN KEY (rezervacija_id) REFERENCES public.rezervacija(id)
);
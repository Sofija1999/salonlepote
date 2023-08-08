CREATE TABLE public.usluga (
	id varchar NOT NULL,
	naziv varchar NOT NULL,
	trajanje interval NOT NULL,
	cena numeric NOT NULL,
	kategorija_id varchar NOT NULL,
	vreme_od timestamp NOT NULL,
	vreme_do timestamp NOT NULL,
	CONSTRAINT usluga_pk PRIMARY KEY (id)
);
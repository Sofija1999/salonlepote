CREATE TABLE public.kupac (
	id bigserial NOT NULL,
	ime varchar NOT NULL,
	prezime varchar NOT NULL,
	kompanija varchar NULL,
	adresa1 varchar NOT NULL,
	adresa2 varchar NULL,
	postanski_broj varchar NOT NULL,
	mesto varchar NOT NULL,
	drzava varchar NOT NULL,
	email varchar NOT NULL,
	potvrda_email varchar NOT NULL,
	CONSTRAINT kupac_pk PRIMARY KEY (id)
);
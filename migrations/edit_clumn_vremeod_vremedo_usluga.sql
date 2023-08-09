ALTER TABLE public.usluga ALTER COLUMN vreme_do TYPE time USING vreme_do::time;
ALTER TABLE public.usluga ALTER COLUMN vreme_od TYPE time USING vreme_od::time;

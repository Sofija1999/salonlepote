# Promenljive
APP_NAME=salon

# Komande
all: build

build:
	go build -o $(salon) .

run:
	go run .

clean:
	rm -f $(salon)
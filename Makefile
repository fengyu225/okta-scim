# Makefile for okta-scim project

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=okta-scim
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package path
MAIN_PACKAGE=.

# Database parameters
DB_USER=user
DB_PASSWORD=password
DB_NAME=scim

# SQLC
SQLC=sqlc

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

run: build
	./$(BINARY_NAME)

deps:
	$(GOGET) github.com/julienschmidt/httprouter
	$(GOGET) github.com/lib/pq
	$(GOGET) github.com/google/uuid

# Generate database code using sqlc
sqlc:
	$(SQLC) generate

# Start the PostgreSQL database using Docker
db-start:
	docker-compose -f docker/docker-compose.yml up -d

# Stop the PostgreSQL database
db-stop:
	docker-compose -f docker/docker-compose.yml down

# Create the database
db-create:
	docker exec -it docker_postgres_1 createdb -U $(DB_USER) $(DB_NAME)

# Run database migrations
migrate:
	@echo "Running database migrations..."
	@for file in db/migrations/*.up.sql; do \
		echo "Applying $$file"; \
		docker exec -i docker_postgres_1 psql -U $(DB_USER) -d $(DB_NAME) < $$file; \
	done

# Cross compilation for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

.PHONY: all build test clean run deps sqlc db-start db-stop db-create migrate build-linux
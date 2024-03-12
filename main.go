package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	"main/db"
)

func main() {
	connStr := "user=user password=password dbname=scim sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	user := os.Getenv("SCIM_USER")
	password := os.Getenv("SCIM_PASSWORD")

	if password == "" || user == "" {
		log.Fatal("Requires environment variables SCIM_USER and SCIM_PASSWORD")
	}

	oktaAPIToken := os.Getenv("OKTA_API_TOKEN")
	if oktaAPIToken == "" {
		log.Fatal("Requires environment variable OKTA_API_TOKEN")
	}

	oktaClient, err := newOktaClient(oktaAPIToken)

	queries := db.New(dbConn)

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	h := NewHandler(user, password, logger, queries, dbConn, oktaClient)

	router := httprouter.New()
	router.GET("/scim/v2/Users/:id", h.GetUser())
	router.GET("/scim/v2/Users", h.GetUsers())
	router.POST("/scim/v2/Users", h.CreateUser())
	router.PUT("/scim/v2/Users/:id", h.UpdateUser())
	router.DELETE("/scim/v2/Users/:id", h.DeactivateUser())

	router.POST("/scim/v2/Groups", h.CreateGroup())
	router.GET("/scim/v2/Groups/:id", h.GetGroup())
	router.GET("/scim/v2/Groups", h.ListGroups())
	router.PUT("/scim/v2/Groups/:id", h.UpdateGroup())
	router.DELETE("/scim/v2/Groups/:id", h.DeleteGroup())

	log.Fatal(http.ListenAndServe(":8080", router))
}

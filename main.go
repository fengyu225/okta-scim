package main

import (
	"database/sql"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	"log"
	"main/db"
	"net/http"
	"os"
)

func main() {
	connStr := "user=user password=password dbname=scim sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	queries := db.New(dbConn)

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	h := NewHandler("admin", "password", logger, queries, dbConn)

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
	router.PATCH("/scim/v2/Groups/:id", h.UpdateGroup())
	router.DELETE("/scim/v2/Groups/:id", h.DeleteGroup())

	log.Fatal(http.ListenAndServe(":8080", router))
}

package main

import (
	"log"
	"github.com/alderon07/log/internal/server"
)

func main(){
	server := server.NewHTTPServer(":8080")
	log.Fatal(server.ListenAndServe())
}

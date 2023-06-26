package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/open-sauced/pizza/oven/pkg/database"
	"github.com/open-sauced/pizza/oven/pkg/server"
)

func main() {
	// Load the environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Failed to load the dot env file. Continuing with existing environment: %v", err)
	}

	// Envs for the pizza oven database handler
	databaseHost := os.Getenv("DATABASE_HOST")
	databasePort := os.Getenv("DATABASE_PORT")
	databaseUser := os.Getenv("DATABASE_USER")
	databasePwd := os.Getenv("DATABASE_PASSWORD")
	databaseDbName := os.Getenv("DATABASE_DBNAME")

	// Env vars for the pizza oven server
	serverPort := os.Getenv("SERVER_PORT")

	pizzaOven := database.NewPizzaOvenDbHandler(databaseHost, databasePort, databaseUser, databasePwd, databaseDbName)
	pizzaOvenServer := server.NewPizzaOvenServer(pizzaOven)
	pizzaOvenServer.Run(serverPort)
}

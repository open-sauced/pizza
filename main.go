package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/open-sauced/pizza/oven/pkg/database"
	"github.com/open-sauced/pizza/oven/pkg/providers"
	"github.com/open-sauced/pizza/oven/pkg/server"
)

func main() {
	var logger *zap.Logger
	var err error

	// Initialize & parse flags
	debugMode := flag.Bool("debug", false, "run in debug mode")
	flag.Parse()

	if *debugMode {
		logger, err = zap.NewDevelopment()
		if err != nil {
			log.Fatalf("Could not initiate debug zap logger: %v", err)
		}
	} else {
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatalf("Could not initiate production zap logger: %v", err)
		}
	}

	sugarLogger := logger.Sugar()
	sugarLogger.Infof("initiated zap logger with level: %d", sugarLogger.Level())

	// Load the environment variables from the .env file
	err = godotenv.Load()
	if err != nil {
		sugarLogger.Warnf("Failed to load the dot env file. Continuing with existing environment: %v", err)
	}

	// Envs for the pizza oven database handler
	databaseHost := os.Getenv("DATABASE_HOST")
	databasePort := os.Getenv("DATABASE_PORT")
	databaseUser := os.Getenv("DATABASE_USER")
	databasePwd := os.Getenv("DATABASE_PASSWORD")
	databaseDbName := os.Getenv("DATABASE_DBNAME")

	// Env vars for the pizza oven server
	serverPort := os.Getenv("SERVER_PORT")

	// User specify which git provider to use
	gitProvider := os.Getenv("GIT_PROVIDER")

	// Initialize the database handler
	pizzaOven := database.NewPizzaOvenDbHandler(databaseHost, databasePort, databaseUser, databasePwd, databaseDbName)

	var pizzaGitProvider providers.GitRepoProvider
	switch gitProvider {
	case "cache":
		sugarLogger.Infof("Initiating cache git provider")

		// Env vars for the git provider
		cacheDir := os.Getenv("CACHE_DIR")
		minFreeDisk := os.Getenv("MIN_FREE_DISK_GB")

		// Validates the provided minimum free disk int is parsable as a uint64
		//
		// TODO - should dynamically check file system bit size after compilation.
		// 64 bit wide words should be fine for almost all use cases for now.
		minFreeDiskUint64, err := strconv.ParseUint(minFreeDisk, 10, 64)
		if err != nil {
			sugarLogger.Fatalf(": %s", err.Error())
		}

		pizzaGitProvider, err = providers.NewLRUCacheGitRepoProvider(cacheDir, minFreeDiskUint64, sugarLogger)
		if err != nil {
			sugarLogger.Fatalf("Could not create a cache git provider: %s", err.Error())
		}
	case "memory":
		sugarLogger.Infof("Initiating in-memory git provider")
		pizzaGitProvider = providers.NewInMemoryGitRepoProvider(sugarLogger)
	default:
		sugarLogger.Fatal("must specify the GIT_PROVIDER env variable (i.e. cache, memory)")
	}

	pizzaOvenServer := server.NewPizzaOvenServer(pizzaOven, pizzaGitProvider, sugarLogger)
	pizzaOvenServer.Run(serverPort)
}

package main

import (
	"context"
	"os"
	"recording-proxy/internal"
	"recording-proxy/internal/mongodb"
	"strconv"
)

func main() {
	// get port, targetSchema, targetHost from environment variables
	// or from command line arguments
	// ...
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		panic(err)
	}

	targetSchema := os.Getenv("TARGET_SCHEMA")
	targetHost := os.Getenv("TARGET_HOST")

	proxy := internal.NewRecordingProxy(
		port,
		targetSchema,
		targetHost,
	)

	config := mongodb.Config{
		Uri:            os.Getenv("MONGODB_URI"),
		DatabaseName:   os.Getenv("MONGODB_DATABASE_NAME"),
		Username:       os.Getenv("MONGODB_USERNAME"),
		Password:       os.Getenv("MONGODB_PASSWORD"),
		CollectionName: os.Getenv("MONGODB_COLLECTION_NAME"),
	}

	storer, err := mongodb.NewStorer(context.Background(), config)

	if err != nil {
		panic(err)
	}

	proxy.AddHandler(storer)

	proxy.Run()
}

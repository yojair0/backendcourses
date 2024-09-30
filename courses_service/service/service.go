package services

import (
	"context"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Conectar a userDB
func ConnectUserDB() (*mongo.Collection, error) {
	clientOptions := options.Client().ApplyURI(os.Getenv("USER_DB_URI"))
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	collection := client.Database("userDB").Collection("users")
	return collection, nil
}

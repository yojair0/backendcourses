package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"courses_service/graph"
	"courses_service/rabbitmq"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func testRabbitMQConnection() {
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		rabbitMQURL = "amqp://localhost:5672"
	}

	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Error al conectar a RabbitMQ: %v", err)
	}
	defer conn.Close()

	log.Println("Conexión a RabbitMQ exitosa")
}

func main() {
	// Test de conexión a RabbitMQ
	testRabbitMQConnection()

	// Cargar variables de entorno
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Conectar a MongoDB
	clientOptions := options.Client().ApplyURI(os.Getenv("MONGO_URI"))
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		log.Fatalf("Error creating MongoDB client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Error connecting to MongoDB: %v", err)
	}

	db := client.Database("coursesDB")
	courseCollection := db.Collection("courses")
	fmt.Println("Connected to MongoDB")

	// Crear la cola para notificaciones
	err = rabbitmq.CreateQueue("new_courses_notifications")
	if err != nil {
		log.Fatalf("Error creating queue: %v", err)
	}

	// Configurar el servidor GraphQL
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{DB: db, CourseCollection: courseCollection},
	}))

	// Ejecutar consumidor en un goroutine
	go func() {
		rabbitmq.ConsumeAndRespond()
	}()

	// Iniciar servidor HTTP
	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)
	log.Printf("connect to http://localhost:8080/ for GraphQL playground")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

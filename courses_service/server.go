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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Cargar las variables de entorno desde el archivo .env
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Verificar el valor de MONGO_URI
	log.Println("Mongo URI: ", os.Getenv("MONGO_URI"))

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

	// nuevoooooooo
	// Crear la cola para notificaciones de nuevos cursos
	err = rabbitmq.CreateQueue("new_courses_notifications")
	if err != nil {
		log.Fatalf("Error creating queue: %v", err)
	}

	// Configurar el servidor GraphQL
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{DB: db, CourseCollection: courseCollection},
	}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	// Prueba RabbitMQ
	// Llamar a la función de prueba
	testRabbitMQ()

	// Continuar con el resto del código
	log.Printf("connect to http://localhost:8080/ for GraphQL playground")
	log.Fatal(http.ListenAndServe(":8080", nil))

}

// Agregar esta función de prueba en `main.go` después de configurar el servidor
func testRabbitMQ() {
	// Reemplaza con un ID de curso válido para probar
	courseID := "curso123"
	err := rabbitmq.SendCourseDetails(courseID)
	if err != nil {
		log.Fatalf("Error sending course details: %v", err)
	} else {
		log.Println("Successfully sent course details to RabbitMQ")
	}
}

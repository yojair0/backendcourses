package rabbitmq

import (
	"context"
	"courses_service/graph/model"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectRabbitMQ() (*amqp.Channel, error) {
	conn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
		return nil, err
	}
	return ch, nil
}

// Publicar un mensaje en RabbitMQ
func PublishMessage(queueName string, body []byte) error {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName, // Name of the queue
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
		return err
	}

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		log.Fatalf("Failed to publish a message: %v", err)
		return err
	}

	log.Printf("Message published to queue %s: %s", queueName, body)
	return nil
}

// Enviar los detalles de un curso específico a través de RabbitMQ
func SendCourseDetails(courseID string) error {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		return err
	}
	defer ch.Close()

	// Initialize MongoDB client and collection
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		return fmt.Errorf("Error connecting to MongoDB: %v", err)
	}
	defer client.Disconnect(context.TODO())

	courseCollection := client.Database("coursesDB").Collection("courses")

	// Busca el curso en la base de datos
	var course model.Course
	err = courseCollection.FindOne(context.TODO(), bson.M{"_id": courseID}).Decode(&course)
	if err != nil {
		return fmt.Errorf("Error finding course: %v", err)
	}

	courseDetails, err := json.Marshal(course)
	if err != nil {
		return fmt.Errorf("Error marshaling course details: %v", err)
	}

	// Declarar la cola "get_course_details" antes de publicar
	_, err = ch.QueueDeclare(
		"get_course_details", // nombre de la cola
		false,                // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return fmt.Errorf("Failed to declare the queue: %v", err)
	}

	// Publicar los detalles del curso en la cola "get_course_details"
	err = ch.Publish(
		"",
		"get_course_details",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        courseDetails,
		},
	)
	if err != nil {
		return fmt.Errorf("Error publishing course details: %v", err)
	}

	log.Printf("Course details published to queue get_course_details: %s", courseDetails)
	return nil
}

// creando la cola necesaria en RabbitMQ para manejar las notificaciones sobre nuevos cursos
func CreateQueue(queueName string) error {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		queueName, // nombre de la cola
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("Failed to declare a queue: %v", err)
	}

	log.Printf("Queue %s created successfully.", queueName)
	return nil
}

// Cuando se cree un nuevo curso, enviar una notificación a RabbitMQ
func NotifyNewCourse(course model.Course) error {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		return err
	}
	defer ch.Close()

	// Serializa los detalles del curso a JSON
	courseDetails, err := json.Marshal(course)
	if err != nil {
		return fmt.Errorf("Error marshaling course details: %v", err)
	}

	// Publica el mensaje en la cola de notificaciones de nuevos cursos
	err = ch.Publish(
		"",                          // exchange
		"new_courses_notifications", // routing key (nombre de la cola)
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        courseDetails,
		},
	)
	if err != nil {
		return fmt.Errorf("Error publishing message: %v", err)
	}

	log.Printf("New course notification sent: %s", courseDetails)
	return nil
}

func ConsumeCourseDetails() {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		"get_course_details", // nombre de la cola
		"",                   // consumer
		true,                 // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			// Procesar los detalles del curso aquí
		}
	}()

	log.Printf("Waiting for messages. To exit press CTRL+C")
	<-forever
}

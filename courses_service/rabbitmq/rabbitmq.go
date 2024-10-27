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

func ConsumeAndRespond() {
	ch, err := ConnectRabbitMQ()
	if err != nil {
		log.Fatalf("Error connecting to RabbitMQ: %v", err)
	}
	defer ch.Close()

	queue := "get_course_details"

	// Declara la cola para asegurarte de que existe antes de consumir
	_, err = ch.QueueDeclare(
		queue,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		log.Fatalf("Error declaring queue: %v", err)
	}

	msgs, err := ch.Consume(
		queue,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error registering consumer: %v", err)
	}

	log.Println("Esperando mensajes en", queue)

	for d := range msgs {
		var payload map[string]interface{}
		err := json.Unmarshal(d.Body, &payload)
		if err != nil {
			log.Printf("Error al deserializar el mensaje: %v", err)
			continue
		}

		courseId, ok := payload["courseId"].(string)
		if !ok {
			log.Printf("courseId no encontrado o no es una cadena")
			continue
		}

		// Obtener los detalles del curso desde la base de datos
		courseDetails, err := getCourseDetailsFromDB(courseId)
		if err != nil {
			log.Printf("Error al obtener detalles del curso desde la base de datos: %v", err)
			continue
		}

		// Publicar la respuesta de vuelta a la cola especificada en ReplyTo
		if d.ReplyTo != "" {
			responseBody, _ := json.Marshal(courseDetails)
			err = ch.Publish(
				"",
				d.ReplyTo, // Cola de respuesta desde ReplyTo
				false,
				false,
				amqp.Publishing{
					ContentType:   "application/json",
					CorrelationId: d.CorrelationId,
					Body:          responseBody,
				})
			if err != nil {
				log.Printf("Error al enviar respuesta: %v", err)
			} else {
				log.Printf("Detalles del curso enviados de vuelta a la cola %s con CorrelationId %s", d.ReplyTo, d.CorrelationId)
			}
		}
	}
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
		"get_course_details",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var payload map[string]interface{}
			err := json.Unmarshal(d.Body, &payload)
			if err != nil {
				log.Printf("Error al deserializar el mensaje: %v", err)
				continue
			}

			courseId, ok := payload["courseId"].(string)
			if !ok {
				log.Printf("courseId no encontrado o no es una cadena")
				continue
			}

			courseDetails, err := getCourseDetailsFromDB(courseId)
			if err != nil {
				log.Printf("Error al obtener detalles del curso desde la base de datos: %v", err)
				continue
			}

			// Publicar respuesta de vuelta a la cola especificada en ReplyTo
			if d.ReplyTo != "" {
				responseBody, _ := json.Marshal(courseDetails)
				err = ch.Publish(
					"",
					d.ReplyTo, // Cola de respuesta desde ReplyTo
					false,
					false,
					amqp.Publishing{
						ContentType:   "application/json",
						CorrelationId: d.CorrelationId,
						Body:          responseBody,
					})
				if err != nil {
					log.Printf("Error al enviar respuesta: %v", err)
				}
			}
		}
	}()

	log.Printf("Esperando mensajes en get_course_details. Para salir presiona CTRL+C")
	<-forever
}

// Ejemplo de función para obtener los detalles del curso desde la base de datos
// Función para obtener los detalles del curso desde la base de datos
func getCourseDetailsFromDB(courseId string) (interface{}, error) {
	// Conexión a MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		log.Printf("Error connecting to MongoDB: %v", err)
		return nil, err
	}
	defer client.Disconnect(context.TODO())

	// Selecciona la colección de cursos
	courseCollection := client.Database("coursesDB").Collection("courses")
	var courseDetails map[string]interface{}

	// Busca el curso en la base de datos usando el campo `id` en lugar de `_id`
	err = courseCollection.FindOne(context.TODO(), bson.M{"id": courseId}).Decode(&courseDetails)
	if err != nil {
		log.Printf("Error finding course with ID %s: %v", courseId, err)
		return nil, err
	}

	// Log para verificar el contenido del curso encontrado
	log.Printf("Curso encontrado: %+v", courseDetails)

	return courseDetails, nil
}

package graph

import (
	"context"
	"courses_service/graph/model"
	"courses_service/rabbitmq"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Resolver para agregar un curso al carrito
func (r *mutationResolver) AddToCart(ctx context.Context, courseID string, userID string) (string, error) {
	// Buscar el curso por ID en MongoDB
	var course model.Course
	err := r.CourseCollection.FindOne(ctx, bson.M{"_id": courseID}).Decode(&course)
	if err != nil {
		log.Printf("Error finding course by ID: %v", err)
		return "", err
	}

	// Enviar los detalles del curso a través de RabbitMQ
	err = rabbitmq.SendCourseDetails(courseID)
	if err != nil {
		log.Printf("Failed to publish course details to RabbitMQ: %v", err)
		return "", err
	}

	response := "Course details sent to user service"
	return response, nil
}

// Resolver para crear un curso
func (r *mutationResolver) CreateCourse(ctx context.Context, input model.NewCourse) (*model.Course, error) {
	log.Println("Received request to create course")

	newCourse := model.Course{
		ID:          primitive.NewObjectID().Hex(),
		Title:       input.Title,
		Description: input.Description,
		Category:    input.Category,
		Price:       input.Price,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	_, err := r.CourseCollection.InsertOne(ctx, newCourse)
	if err != nil {
		log.Printf("Failed to insert new course: %v", err)
		return nil, err
	}

	// Publicar mensaje en RabbitMQ
	message := "New course created: " + newCourse.Title
	err = rabbitmq.PublishMessage("courses_queue", []byte(message))
	if err != nil {
		log.Printf("Failed to publish message to RabbitMQ: %v", err)
	}

	return &newCourse, nil
}

// Resolver para eliminar un curso
func (r *mutationResolver) DeleteCourse(ctx context.Context, id string) (*string, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Invalid ID format: %v", err)
		emptyString := ""
		return &emptyString, err
	}

	filter := bson.D{{Key: "_id", Value: objectID}}

	result, err := r.CourseCollection.DeleteOne(ctx, filter)
	if err != nil {
		log.Printf("Failed to delete course with ID %s: %v", id, err)
		emptyString := ""
		return &emptyString, err
	}

	if result.DeletedCount == 0 {
		log.Printf("No course found with ID %s", id)
		emptyString := ""
		return &emptyString, fmt.Errorf("no course found with ID %s", id)
	}

	response := "Course successfully deleted"
	return &response, nil
}

// Mutación para limpiar el carrito
func (r *mutationResolver) ClearCart(ctx context.Context) (string, error) {
	err := rabbitmq.PublishMessage("cart_queue", []byte(`{"action":"clear_cart"}`))
	if err != nil {
		log.Printf("Failed to publish message to RabbitMQ: %v", err)
		return "", err
	}
	response := "Cart cleared"
	return response, nil
}

// Resolver para obtener todos los cursos
func (r *queryResolver) Courses(ctx context.Context) ([]*model.Course, error) {
	var courses []*model.Course

	cursor, err := r.CourseCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Printf("Failed to find courses: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var course model.Course
		if err := cursor.Decode(&course); err != nil {
			log.Println("Error decoding course:", err)
			continue
		}
		courses = append(courses, &course)
	}

	return courses, nil
}

// Resolver para obtener un curso por ID
func (r *queryResolver) Course(ctx context.Context, id string) (*model.Course, error) {
	var course model.Course

	filter := bson.D{{Key: "_id", Value: id}}
	err := r.CourseCollection.FindOne(ctx, filter).Decode(&course)
	if err != nil {
		log.Printf("Failed to find course with ID %s: %v", id, err)
		return nil, err
	}

	return &course, nil
}

// Filtro para cursos por categoría y precio
func (r *queryResolver) FilterCourses(ctx context.Context, category *string, minPrice *float64, maxPrice *float64) ([]*model.Course, error) {
	var filter bson.D

	if category != nil {
		filter = append(filter, bson.E{Key: "category", Value: *category})
	}
	if minPrice != nil && maxPrice != nil {
		filter = append(filter, bson.E{Key: "price", Value: bson.D{
			{Key: "$gte", Value: *minPrice},
			{Key: "$lte", Value: *maxPrice},
		}})
	}

	var courses []*model.Course
	cursor, err := r.CourseCollection.Find(ctx, filter)
	if err != nil {
		log.Printf("Failed to filter courses: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var course model.Course
		if err := cursor.Decode(&course); err != nil {
			log.Println("Error decoding course:", err)
			continue
		}
		courses = append(courses, &course)
	}

	return courses, nil
}

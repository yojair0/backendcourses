package graph

import "go.mongodb.org/mongo-driver/mongo"

// Resolver es la estructura que contiene la base de datos y la colección de cursos.
type Resolver struct {
	DB               *mongo.Database
	CourseCollection *mongo.Collection
}

// Mutation devuelve el resolver para las mutaciones.
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}

// Query devuelve el resolver para las consultas.
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// mutationResolver es el tipo que implementa las mutaciones.
type mutationResolver struct{ *Resolver }

// queryResolver es el tipo que implementa las consultas.
type queryResolver struct{ *Resolver }

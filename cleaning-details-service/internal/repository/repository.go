package repository

import (
	"cleaning-app/cleaning-details-service/internal/models"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

const (
	collectionName = "cleaning_services"
)

type CleaningServiceRepository struct {
	db *mongo.Database
}

func NewCleaningServiceRepository(db *mongo.Database) *CleaningServiceRepository {
	return &CleaningServiceRepository{
		db: db,
	}
}

func (r *CleaningServiceRepository) GetAllServices(ctx context.Context) ([]models.CleaningService, error) {
	collection := r.db.Collection(collectionName)

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var services []models.CleaningService
	if err = cursor.All(ctx, &services); err != nil {
		return nil, err
	}

	if services == nil {
		services = []models.CleaningService{}
	}

	return services, nil
}

func (r *CleaningServiceRepository) GetActiveServices(ctx context.Context) ([]models.CleaningService, error) {
	collection := r.db.Collection(collectionName)

	cursor, err := collection.Find(ctx, bson.M{"isActive": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var services []models.CleaningService
	if err = cursor.All(ctx, &services); err != nil {
		return nil, err
	}

	if services == nil {
		services = []models.CleaningService{}
	}

	return services, nil
}

func (r *CleaningServiceRepository) CreateService(ctx context.Context, service *models.CleaningService) error {
	collection := r.db.Collection(collectionName)

	// Check for duplicate name
	count, err := collection.CountDocuments(ctx, bson.M{"name": service.Name})
	if err != nil {
		return err
	}
	if count > 0 {
		return models.ErrDuplicate
	}

	// Set timestamps
	now := primitive.NewDateTimeFromTime(time.Now())
	service.CreatedAt = now
	service.UpdatedAt = now

	// Insert document
	result, err := collection.InsertOne(ctx, service)
	if err != nil {
		return err
	}

	// Set the generated ID
	service.ID = result.InsertedID.(primitive.ObjectID)

	return nil
}

func (r *CleaningServiceRepository) UpdateService(ctx context.Context, service *models.CleaningService) error {
	collection := r.db.Collection(collectionName)

	// Check if ID is valid
	if service.ID.IsZero() {
		return models.ErrInvalidID
	}

	// Check for duplicate name (excluding current service)
	count, err := collection.CountDocuments(ctx, bson.M{
		"name": service.Name,
		"_id":  bson.M{"$ne": service.ID},
	})
	if err != nil {
		return err
	}
	if count > 0 {
		return models.ErrDuplicate
	}

	// Update timestamp
	service.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	// Update document
	filter := bson.M{"_id": service.ID}
	update := bson.M{"$set": bson.M{
		"name":      service.Name,
		"price":     service.Price,
		"isActive":  service.IsActive,
		"updatedAt": service.UpdatedAt,
	}}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return r.handleDatabaseError(err)
	}

	if result.MatchedCount == 0 {
		return models.ErrNotFound
	}

	return nil
}

func (r *CleaningServiceRepository) DeleteService(ctx context.Context, id primitive.ObjectID) error {
	collection := r.db.Collection(collectionName)

	// Delete document
	filter := bson.M{"_id": id}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return r.handleDatabaseError(err)
	}

	if result.DeletedCount == 0 {
		return models.ErrNotFound
	}

	return nil
}

func (r *CleaningServiceRepository) UpdateServiceStatus(ctx context.Context, id primitive.ObjectID, isActive bool) error {
	collection := r.db.Collection(collectionName)

	// Update document
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{
		"isActive":  isActive,
		"updatedAt": primitive.NewDateTimeFromTime(time.Now()),
	}}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return r.handleDatabaseError(err)
	}

	if result.MatchedCount == 0 {
		return models.ErrNotFound
	}

	return nil
}

func (r *CleaningServiceRepository) handleDatabaseError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.ErrNotFound
	}
	return err
}

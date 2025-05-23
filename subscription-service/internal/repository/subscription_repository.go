package repository

import (
	"cleaning-app/subscription-service/internal/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SubscriptionRepository struct {
	col *mongo.Collection
}

func NewSubscriptionRepository(db *mongo.Database) *SubscriptionRepository {
	return &SubscriptionRepository{col: db.Collection("subscriptions")}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	sub.ID = primitive.NewObjectID()
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, sub)
	return err
}

func (r *SubscriptionRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *SubscriptionRepository) GetActiveSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	filter := bson.M{"status": models.StatusActive, "remaining_cleanings": bson.M{"$gt": 0}}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var result []models.Subscription
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SubscriptionRepository) UpdateAfterOrder(ctx context.Context, id primitive.ObjectID, nextDate time.Time) error {
	return r.Update(ctx, id, bson.M{
		"remaining_cleanings": bson.M{"$inc": -1},
		"last_order_date":     time.Now(),
		"next_planned_date":   nextDate,
	})
}

func (r *SubscriptionRepository) SetExpired(ctx context.Context, id primitive.ObjectID) error {
	return r.Update(ctx, id, bson.M{"status": models.StatusExpired})
}

func (r *SubscriptionRepository) GetByClient(ctx context.Context, clientID string) ([]models.Subscription, error) {
	cursor, err := r.col.Find(ctx, bson.M{"client_id": clientID})
	if err != nil {
		return nil, err
	}
	var result []models.Subscription
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error) {
	var sub models.Subscription
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}
func (r *SubscriptionRepository) GetAll(ctx context.Context) ([]models.Subscription, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}
func (r *SubscriptionRepository) FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"next_planned_date": targetDate,
		"status":            models.StatusActive,
	})
	if err != nil {
		return nil, err
	}
	var subs []models.Subscription
	err = cursor.All(ctx, &subs)
	return subs, err
}

func (r *SubscriptionRepository) FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"next_planned_date":   bson.M{"$lte": before},
		"status":              models.StatusActive,
		"remaining_cleanings": bson.M{"$lte": 0},
	})
	if err != nil {
		return nil, err
	}
	var subs []models.Subscription
	err = cursor.All(ctx, &subs)
	return subs, err
}

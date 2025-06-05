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

func (r *SubscriptionRepository) Create(ctx context.Context, s *models.Subscription) error {
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now
	s.Status = models.StatusActive

	res, err := r.col.InsertOne(ctx, s)
	if err != nil {
		return err
	}

	// Присваиваем сгенерированный Mongo ObjectID обратно в структуру
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		s.ID = oid
	}

	return nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, id primitive.ObjectID, update primitive.M) error {
	// В любом случае обновляем UpdatedAt
	update["updated_at"] = time.Now().UTC()
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *SubscriptionRepository) GetActiveSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	now := time.Now().UTC()
	filter := bson.M{
		"status":   models.StatusActive,
		"end_date": bson.M{"$gte": now},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
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

func (r *SubscriptionRepository) GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error) {
	clientID, err := primitive.ObjectIDFromHex(clientIDHex)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"user_id": clientID}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error) {
	var sub models.Subscription
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetAll(ctx context.Context) ([]models.Subscription, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepository) FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error) {
	dayStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Nanosecond)
	filter := bson.M{
		"end_date": bson.M{
			"$gte": dayStart,
			"$lte": dayEnd,
		},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepository) FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	filter := bson.M{"end_date": bson.M{"$lt": before}}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepository) FindDue(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	filter := bson.M{
		"status":            models.StatusActive,
		"next_planned_date": bson.M{"$lte": before},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []models.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error {
	update := bson.M{"status": status, "updated_at": time.Now().UTC()}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

package repository

import (
	"cleaning-app/review-media-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// Добавляем структуру для статистики в пакет models
type ReviewStat struct {
	TargetID string  `json:"target_id"`
	Count    int     `json:"count"`
	Average  float64 `json:"average"`
}

type ReviewRepository struct {
	collection *mongo.Collection
}

func NewReviewRepository(db *mongo.Database) *ReviewRepository {
	return &ReviewRepository{collection: db.Collection("reviews")}
}

func (r *ReviewRepository) Create(ctx context.Context, review *models.Review) error {
	review.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, review)
	return err
}

func (r *ReviewRepository) GetByTargetID(ctx context.Context, targetID string) ([]models.Review, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"target_id": targetID})
	if err != nil {
		return nil, err
	}
	var results []models.Review
	err = cursor.All(ctx, &results)
	return results, err
}

func (r *ReviewRepository) ExistsByOrderAndReviewer(ctx context.Context, orderID, reviewerID string) (bool, error) {
	filter := bson.M{"order_id": orderID, "reviewer_id": reviewerID}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ReviewRepository) AggregateStatistics(ctx context.Context) ([]ReviewStat, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":     "$target_id",
			"average": bson.M{"$avg": "$rating"},
			"total":   bson.M{"$sum": 1},
		}}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var results []struct {
		TargetID string  `bson:"_id"`
		Average  float64 `bson:"average"`
		Total    int     `bson:"total"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	stats := make([]ReviewStat, len(results))
	for i, r := range results {
		stats[i] = ReviewStat{
			TargetID: r.TargetID,
			Count:    r.Total,
			Average:  r.Average,
		}
	}
	return stats, nil
}

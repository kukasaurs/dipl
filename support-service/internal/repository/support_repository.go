package repository

import (
	"context"
	"time"

	"cleaning-app/support-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ChatRepository struct {
	Collection *mongo.Collection
}

func NewChatRepository(db *mongo.Database) *ChatRepository {
	return &ChatRepository{
		Collection: db.Collection("messages"),
	}
}

func (r *ChatRepository) SaveMessage(ctx context.Context, msg *models.Message) error {
	msg.Timestamp = time.Now()
	_, err := r.Collection.InsertOne(ctx, msg)
	return err
}

func (r *ChatRepository) GetMessages(ctx context.Context, userID1, userID2 string) ([]*models.Message, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"sender_id": userID1, "receiver_id": userID2},
			{"sender_id": userID2, "receiver_id": userID1},
		},
	}
	cursor, err := r.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var messages []*models.Message
	if err = cursor.All(ctx, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

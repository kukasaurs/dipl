package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"cleaning-app/user-management-service/internal/models"
)

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{col: db.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	u.ID = primitive.NewObjectID()
	_, err := r.col.InsertOne(ctx, u)

	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)

	return &user, err
}

func (r *UserRepository) GetAll(ctx context.Context, role models.Role) ([]models.User, error) {
	filter := bson.M{}
	if role != models.RoleAll {
		filter["role"] = role
	}

	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) SetBanStatus(ctx context.Context, id primitive.ObjectID, banned bool) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{"banned": banned}})

	return err
}

func (r *UserRepository) UpdateRole(ctx context.Context, id primitive.ObjectID, role models.Role) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"role": role,
		},
	})

	return err
}

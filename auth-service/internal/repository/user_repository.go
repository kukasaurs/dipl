package repositories

import (
	"cleaning-app/auth-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

func (r *UserRepository) CreateUser(user *models.User) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	user.ID = result.InsertedID.(primitive.ObjectID)

	return user, nil
}

func (r *UserRepository) FindUserByEmail(email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserByID(userID primitive.ObjectID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateUser(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	update := bson.M{
		"$set": bson.M{
			"first_name":     user.FirstName,
			"last_name":      user.LastName,
			"address":        user.Address,
			"phone_number":   user.PhoneNumber,
			"date_of_birth":  user.DateOfBirth,
			"gender":         user.Gender,
			"reset_required": user.ResetRequired,
			"password":       user.Password,
		},
	}

	_, err := r.collection.UpdateByID(ctx, user.ID, update)
	return err
}
func (r *UserRepository) UpdateUserFields(userID primitive.ObjectID, fields bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": fields}
	_, err := r.collection.UpdateByID(ctx, userID, update)
	return err
}

func (r *UserRepository) UpdatePassword(userID primitive.ObjectID, hashedPassword string, resetRequired bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"password":       hashedPassword,
			"reset_required": resetRequired,
		},
	}

	_, err := r.collection.UpdateByID(ctx, userID, update)
	return err
}

func (r *UserRepository) DeleteUser(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": userID})
	return err
}

func (r *UserRepository) GetByRole(role string) ([]*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"role": role})
	if err != nil {
		return nil, err
	}

	var users []*models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) CountUsers(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *UserRepository) GetRating(userID primitive.ObjectID) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	return user.AverageRating, err
}

func (r *UserRepository) AddRating(userID primitive.ObjectID, rating int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
		"$set": bson.M{
			"ratings": bson.M{"$concatArrays": []interface{}{
				bson.M{"$ifNull": []interface{}{"$ratings", bson.A{}}},
				bson.A{rating},
			}},
		},
	})

	if err != nil {
		return err
	}

	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": userID}, mongo.Pipeline{
		{{"$set", bson.M{
			"average_rating": bson.M{"$avg": "$ratings"},
		}}},
	})

	return err
}

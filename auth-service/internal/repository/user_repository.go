package repositories

import (
	"cleaning-app/auth-service/internal/models"
	"context"
	"fmt"
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

func (r *UserRepository) AddRating(ctx context.Context, cleanerID string, rating int) error {
	oid, err := primitive.ObjectIDFromHex(cleanerID)
	if err != nil {
		return fmt.Errorf("invalid cleanerID %q: %w", cleanerID, err)
	}

	// 1) Инкрементируем count и sum
	update := bson.M{
		"$inc": bson.M{
			"rating_count": 1,
			"rating_sum":   rating,
		},
	}
	// Если вы хотите сразу пересчитать average в одном запросе и ваша MongoDB ≥4.2, можно использовать aggregation pipeline update:
	// update = mongo.Pipeline{
	//   {{ "$set": bson.M{
	//       "rating_count": bson.M{"$add": []interface{}{"$rating_count", 1}},
	//       "rating_sum":   bson.M{"$add": []interface{}{"$rating_sum", rating}},
	//   }}},
	//   {{ "$set": bson.M{"average_rating": bson.M{"$divide": []interface{}{"$rating_sum", "$rating_count"}}}}},
	// }

	_, err = r.collection.UpdateByID(ctx, oid, update)
	if err != nil {
		return fmt.Errorf("inc sum/count: %w", err)
	}

	// 2) Получаем свежие значения для подсчёта среднего
	var u models.User
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&u); err != nil {
		return fmt.Errorf("fetch user for avg: %w", err)
	}
	avg := float64(u.RatingSum) / float64(u.RatingCount)

	// 3) Обновляем поле average_rating
	if _, err := r.collection.UpdateByID(ctx, oid, bson.M{"$set": bson.M{"average_rating": avg}}); err != nil {
		return fmt.Errorf("set average_rating: %w", err)
	}

	return nil
}

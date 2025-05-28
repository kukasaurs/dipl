package repository

import (
	"context"
	"time"

	"cleaning-app/support-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type supportRepository struct {
	ticketsCol  *mongo.Collection
	messagesCol *mongo.Collection
}

// NewSupportRepository создаёт новый репозиторий
func NewSupportRepository(db *mongo.Database) *supportRepository {
	return &supportRepository{
		ticketsCol:  db.Collection("tickets"),
		messagesCol: db.Collection("messages"),
	}
}

func (r *supportRepository) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	ticket.ID = primitive.NewObjectID()
	ticket.CreatedAt = time.Now().UTC()
	ticket.UpdatedAt = ticket.CreatedAt
	_, err := r.ticketsCol.InsertOne(ctx, ticket)
	return err
}

func (r *supportRepository) GetTicketByID(ctx context.Context, id primitive.ObjectID) (*models.Ticket, error) {
	var t models.Ticket
	if err := r.ticketsCol.FindOne(ctx, bson.M{"_id": id}).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *supportRepository) GetTicketsByClient(ctx context.Context, clientID string) ([]models.Ticket, error) {
	cursor, err := r.ticketsCol.Find(ctx, bson.M{"client_id": clientID})
	if err != nil {
		return nil, err
	}
	var tickets []models.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *supportRepository) GetAllTickets(ctx context.Context) ([]models.Ticket, error) {
	cursor, err := r.ticketsCol.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var tickets []models.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *supportRepository) GetTicketsByClientAndStatus(ctx context.Context, clientID string, status models.TicketStatus) ([]models.Ticket, error) {
	filter := bson.M{"client_id": clientID, "status": status}
	cursor, err := r.ticketsCol.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var tickets []models.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *supportRepository) GetTicketsByStatus(ctx context.Context, status models.TicketStatus) ([]models.Ticket, error) {
	filter := bson.M{"status": status}
	cursor, err := r.ticketsCol.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var tickets []models.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *supportRepository) UpdateTicketStatus(ctx context.Context, ticketID primitive.ObjectID, status models.TicketStatus) error {
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now().UTC()}}
	_, err := r.ticketsCol.UpdateByID(ctx, ticketID, update)
	return err
}

func (r *supportRepository) AddMessage(ctx context.Context, msg *models.Message) error {
	msg.ID = primitive.NewObjectID()
	msg.Timestamp = time.Now().UTC()
	_, err := r.messagesCol.InsertOne(ctx, msg)
	return err
}

func (r *supportRepository) GetMessagesByTicket(ctx context.Context, ticketID primitive.ObjectID) ([]models.Message, error) {
	filter := bson.M{"ticket_id": ticketID}
	opts := options.Find().SetSort(bson.M{"timestamp": 1})
	cursor, err := r.messagesCol.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var msgs []models.Message
	if err := cursor.All(ctx, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

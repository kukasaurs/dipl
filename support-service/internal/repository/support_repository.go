package repository

import (
	"cleaning-app/support-service/internal/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SupportRepository struct {
	ticketsCol  *mongo.Collection
	messagesCol *mongo.Collection
}

func NewSupportRepository(db *mongo.Database) *SupportRepository {
	return &SupportRepository{
		ticketsCol:  db.Collection("support_tickets"),
		messagesCol: db.Collection("support_messages"),
	}
}

// Ticket CRUD

func (r *SupportRepository) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	ticket.ID = primitive.NewObjectID()
	ticket.Status = models.StatusOpen
	ticket.CreatedAt = time.Now()
	ticket.UpdatedAt = time.Now()
	_, err := r.ticketsCol.InsertOne(ctx, ticket)
	return err
}

func (r *SupportRepository) GetTicketByID(ctx context.Context, id primitive.ObjectID) (*models.Ticket, error) {
	var ticket models.Ticket
	err := r.ticketsCol.FindOne(ctx, bson.M{"_id": id}).Decode(&ticket)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *SupportRepository) GetTicketsByClient(ctx context.Context, clientID string) ([]models.Ticket, error) {
	cursor, err := r.ticketsCol.Find(ctx, bson.M{"client_id": clientID})
	if err != nil {
		return nil, err
	}
	var result []models.Ticket
	err = cursor.All(ctx, &result)
	return result, err
}

func (r *SupportRepository) GetAllTickets(ctx context.Context) ([]models.Ticket, error) {
	cursor, err := r.ticketsCol.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var result []models.Ticket
	err = cursor.All(ctx, &result)
	return result, err
}

func (r *SupportRepository) UpdateTicketStatus(ctx context.Context, id primitive.ObjectID, status models.TicketStatus) error {
	_, err := r.ticketsCol.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	})
	return err
}

// Chat messages

func (r *SupportRepository) AddMessage(ctx context.Context, msg *models.Message) error {
	msg.ID = primitive.NewObjectID()
	msg.Timestamp = time.Now()
	_, err := r.messagesCol.InsertOne(ctx, msg)
	return err
}

func (r *SupportRepository) GetMessagesByTicket(ctx context.Context, ticketID primitive.ObjectID) ([]models.Message, error) {
	cursor, err := r.messagesCol.Find(ctx, bson.M{"ticket_id": ticketID})
	if err != nil {
		return nil, err
	}
	var result []models.Message
	err = cursor.All(ctx, &result)
	return result, err
}

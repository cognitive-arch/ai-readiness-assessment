// internal/repository/assessment.go
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrNotFound is returned when a document cannot be found.
var ErrNotFound = errors.New("not found")

// AssessmentRepository defines the storage contract.
type AssessmentRepository interface {
	Create(ctx context.Context, a *models.Assessment) (*models.Assessment, error)
	GetByID(ctx context.Context, id string) (*models.Assessment, error)
	SaveAnswers(ctx context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error)
	SaveResult(ctx context.Context, id string, result *models.Result) (*models.Assessment, error)
	List(ctx context.Context, limit, offset int64) ([]*models.Assessment, int64, error)
	Delete(ctx context.Context, id string) error
}

// mongoAssessmentRepo is the MongoDB implementation.
type mongoAssessmentRepo struct {
	col *mongo.Collection
}

// NewAssessmentRepository returns a new MongoDB-backed repository and
// ensures required indexes are created.
func NewAssessmentRepository(ctx context.Context, db *mongo.Database) (AssessmentRepository, error) {
	col := db.Collection("assessments")

	// Indexes
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_created_at"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_status"),
		},
		{
			Keys:    bson.D{{Key: "client_ref", Value: 1}},
			Options: options.Index().SetName("idx_client_ref").SetSparse(true),
		},
	}

	_, err := col.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &mongoAssessmentRepo{col: col}, nil
}

// Create inserts a new assessment and returns the created document with its ID.
func (r *mongoAssessmentRepo) Create(ctx context.Context, a *models.Assessment) (*models.Assessment, error) {
	now := time.Now().UTC()
	a.ID = primitive.NewObjectID()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = models.StatusDraft
	}
	if a.Answers == nil {
		a.Answers = make(map[string]models.Answer)
	}

	_, err := r.col.InsertOne(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("insert assessment: %w", err)
	}
	return a, nil
}

// GetByID retrieves an assessment by its hex string ID.
func (r *mongoAssessmentRepo) GetByID(ctx context.Context, id string) (*models.Assessment, error) {
	oid, err := toObjectID(id)
	if err != nil {
		return nil, err
	}

	var a models.Assessment
	err = r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&a)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find assessment %s: %w", id, err)
	}
	return &a, nil
}

// SaveAnswers performs a bulk upsert of answers (merges with existing answers).
func (r *mongoAssessmentRepo) SaveAnswers(ctx context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error) {
	oid, err := toObjectID(id)
	if err != nil {
		return nil, err
	}

	// Build individual $set fields for each answer so we do a field-level merge
	// instead of replacing the entire answers map.
	setFields := bson.M{
		"updated_at": time.Now().UTC(),
		"status":     models.StatusInProgress,
	}
	for qID, ans := range answers {
		setFields["answers."+qID] = ans
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Assessment
	err = r.col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": setFields},
		opts,
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("save answers for %s: %w", id, err)
	}
	return &updated, nil
}

// SaveResult stores a computed result and transitions status to computed.
func (r *mongoAssessmentRepo) SaveResult(ctx context.Context, id string, result *models.Result) (*models.Assessment, error) {
	oid, err := toObjectID(id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result.ComputedAt = now

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Assessment
	err = r.col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{
			"result":     result,
			"status":     models.StatusComputed,
			"updated_at": now,
		}},
		opts,
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("save result for %s: %w", id, err)
	}
	return &updated, nil
}

// List returns paginated assessments sorted by created_at descending.
func (r *mongoAssessmentRepo) List(ctx context.Context, limit, offset int64) ([]*models.Assessment, int64, error) {
	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.col.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("list assessments: %w", err)
	}
	defer cursor.Close(ctx)

	var assessments []*models.Assessment
	if err = cursor.All(ctx, &assessments); err != nil {
		return nil, 0, fmt.Errorf("decode assessments: %w", err)
	}

	total, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, fmt.Errorf("count assessments: %w", err)
	}

	return assessments, total, nil
}

// Delete removes an assessment by ID.
func (r *mongoAssessmentRepo) Delete(ctx context.Context, id string) error {
	oid, err := toObjectID(id)
	if err != nil {
		return err
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return fmt.Errorf("delete assessment %s: %w", id, err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// toObjectID converts a hex string to a MongoDB ObjectID.
func toObjectID(id string) (primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("invalid id %q: %w", id, ErrNotFound)
	}
	return oid, nil
}

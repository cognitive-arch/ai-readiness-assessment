//go:build integration

// internal/repository/assessment_integration_test.go
package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestDB(t *testing.T) (*mongo.Database, func()) {
	t.Helper()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set — skipping repository integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}

	dbName := "ai_readiness_repo_test_" + t.Name()
	db := client.Database(dbName)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}
	return db, cleanup
}

func ptr(i int) *int { return &i }

func TestRepository_CreateAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, err := NewAssessmentRepository(ctx, db)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	a := &models.Assessment{
		Status:    models.StatusDraft,
		ClientRef: "test-org",
	}
	created, err := repo.Create(ctx, a)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID.IsZero() {
		t.Error("expected non-zero ID after create")
	}

	got, err := repo.GetByID(ctx, created.ID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, created.ID)
	}
	if got.ClientRef != "test-org" {
		t.Errorf("client_ref mismatch: got %q", got.ClientRef)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	_, err := repo.GetByID(ctx, "6643f1a2c3d4e5f6a7b8c9d0")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRepository_SaveAnswers_FieldLevelMerge(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	a, _ := repo.Create(ctx, &models.Assessment{Status: models.StatusDraft})
	id := a.ID.Hex()

	// Save first batch
	batch1 := map[string]models.Answer{
		"s1": {Score: ptr(4), Comment: "First comment"},
		"s2": {Score: ptr(3)},
	}
	updated, err := repo.SaveAnswers(ctx, id, batch1)
	if err != nil {
		t.Fatalf("save batch1: %v", err)
	}
	if updated.Answers["s1"].Score == nil || *updated.Answers["s1"].Score != 4 {
		t.Error("s1 score not saved correctly")
	}

	// Save second batch — should NOT overwrite s1
	batch2 := map[string]models.Answer{
		"t1": {Score: ptr(5)},
	}
	updated, err = repo.SaveAnswers(ctx, id, batch2)
	if err != nil {
		t.Fatalf("save batch2: %v", err)
	}

	if _, ok := updated.Answers["s1"]; !ok {
		t.Error("s1 was lost after second batch save (merge failed)")
	}
	if _, ok := updated.Answers["t1"]; !ok {
		t.Error("t1 not present after second batch save")
	}
	if updated.Status != models.StatusInProgress {
		t.Errorf("expected status=in_progress, got %s", updated.Status)
	}
}

func TestRepository_SaveResult(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	a, _ := repo.Create(ctx, &models.Assessment{Status: models.StatusInProgress})
	id := a.ID.Hex()

	result := &models.Result{
		Overall:       72.5,
		Maturity:      "AI Structured",
		Confidence:    88.9,
		TotalAnswered: 65,
		TotalQ:        72,
		Risks:         []string{"CRITICAL_GAPS"},
		DomainScores: map[string]models.DomainScore{
			"Strategic": {Score: 75.0, Answered: 10, Total: 12},
		},
	}

	updated, err := repo.SaveResult(ctx, id, result)
	if err != nil {
		t.Fatalf("save result: %v", err)
	}
	if updated.Status != models.StatusComputed {
		t.Errorf("expected computed status, got %s", updated.Status)
	}
	if updated.Result == nil {
		t.Fatal("result not saved")
	}
	if updated.Result.Overall != 72.5 {
		t.Errorf("overall mismatch: got %f", updated.Result.Overall)
	}
	if updated.Result.ComputedAt.IsZero() {
		t.Error("computed_at should be set")
	}
}

func TestRepository_List_Pagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	// Insert 5 documents
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, &models.Assessment{Status: models.StatusDraft})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	// Get page 1 (limit 3)
	page1, total, err := repo.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if total < 5 {
		t.Errorf("total should be >= 5, got %d", total)
	}
	if len(page1) != 3 {
		t.Errorf("page1 len: expected 3, got %d", len(page1))
	}

	// Get page 2 (limit 3, offset 3)
	page2, _, err := repo.List(ctx, 3, 3)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2) < 2 {
		t.Errorf("page2 len: expected >= 2, got %d", len(page2))
	}

	// Ensure no duplicates between pages
	seen := map[string]bool{}
	for _, a := range append(page1, page2...) {
		id := a.ID.Hex()
		if seen[id] {
			t.Errorf("duplicate ID %s across pages", id)
		}
		seen[id] = true
	}
}

func TestRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	a, _ := repo.Create(ctx, &models.Assessment{Status: models.StatusDraft})
	id := a.ID.Hex()

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repo.GetByID(ctx, id)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete again → ErrNotFound
	if err := repo.Delete(ctx, id); err != ErrNotFound {
		t.Errorf("double delete: expected ErrNotFound, got %v", err)
	}
}

func TestRepository_Timestamps(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo, _ := NewAssessmentRepository(ctx, db)

	before := time.Now().UTC().Add(-time.Second)
	a, _ := repo.Create(ctx, &models.Assessment{Status: models.StatusDraft})
	after := time.Now().UTC().Add(time.Second)

	if a.CreatedAt.Before(before) || a.CreatedAt.After(after) {
		t.Errorf("created_at %v out of expected range [%v, %v]", a.CreatedAt, before, after)
	}
	if a.UpdatedAt.Before(before) || a.UpdatedAt.After(after) {
		t.Errorf("updated_at %v out of expected range", a.UpdatedAt)
	}
}

// internal/repository/mock_test.go
package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

func ptr(i int) *int { return &i }

// ─────────────────────────────────────────────
// Create
// ─────────────────────────────────────────────

func TestMock_Create_AssignsID(t *testing.T) {
	repo := NewMock()
	a, err := repo.Create(context.Background(), &models.Assessment{Status: models.StatusDraft})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID.IsZero() {
		t.Error("expected non-zero ObjectID after create")
	}
}

func TestMock_Create_SetsTimestamps(t *testing.T) {
	repo := NewMock()
	before := time.Now().Add(-time.Second)
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	after := time.Now().Add(time.Second)

	if a.CreatedAt.Before(before) || a.CreatedAt.After(after) {
		t.Errorf("created_at %v out of expected window", a.CreatedAt)
	}
}

func TestMock_Create_DefaultsAnswersMap(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	if a.Answers == nil {
		t.Error("expected Answers map to be initialised")
	}
}

func TestMock_Create_DefaultsStatusToDraft(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	if a.Status != models.StatusDraft {
		t.Errorf("expected draft, got %s", a.Status)
	}
}

// ─────────────────────────────────────────────
// Get
// ─────────────────────────────────────────────

func TestMock_GetByID_Found(t *testing.T) {
	repo := NewMock()
	created, _ := repo.Create(context.Background(), &models.Assessment{ClientRef: "acme"})

	got, err := repo.GetByID(context.Background(), created.ID.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("id mismatch: got %v, want %v", got.ID, created.ID)
	}
	if got.ClientRef != "acme" {
		t.Errorf("client_ref mismatch: got %q", got.ClientRef)
	}
}

func TestMock_GetByID_NotFound(t *testing.T) {
	repo := NewMock()
	_, err := repo.GetByID(context.Background(), "6643f1a2c3d4e5f6a7b8c9d0")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMock_GetByID_ReturnsCopy(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{ClientRef: "original"})

	got, _ := repo.GetByID(context.Background(), a.ID.Hex())
	got.ClientRef = "mutated"

	// Re-fetch — should still be "original"
	got2, _ := repo.GetByID(context.Background(), a.ID.Hex())
	if got2.ClientRef != "original" {
		t.Error("GetByID should return a deep copy, not a pointer to the stored struct")
	}
}

// ─────────────────────────────────────────────
// SaveAnswers
// ─────────────────────────────────────────────

func TestMock_SaveAnswers_MergesFields(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	id := a.ID.Hex()

	// First batch
	_, _ = repo.SaveAnswers(context.Background(), id, map[string]models.Answer{
		"s1": {Score: ptr(4)},
		"s2": {Score: ptr(3)},
	})

	// Second batch — should not erase s1/s2
	updated, err := repo.SaveAnswers(context.Background(), id, map[string]models.Answer{
		"t1": {Score: ptr(5)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, qid := range []string{"s1", "s2", "t1"} {
		if _, ok := updated.Answers[qid]; !ok {
			t.Errorf("expected %s to be present after merge", qid)
		}
	}
}

func TestMock_SaveAnswers_SetsStatusInProgress(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})

	updated, _ := repo.SaveAnswers(context.Background(), a.ID.Hex(),
		map[string]models.Answer{"s1": {Score: ptr(3)}})

	if updated.Status != models.StatusInProgress {
		t.Errorf("expected in_progress, got %s", updated.Status)
	}
}

func TestMock_SaveAnswers_NotFound(t *testing.T) {
	repo := NewMock()
	_, err := repo.SaveAnswers(context.Background(), "6643f1a2c3d4e5f6a7b8c9d0",
		map[string]models.Answer{"s1": {Score: ptr(3)}})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMock_SaveAnswers_PreservesScorePointerIndependence(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	score := 3
	ans := models.Answer{Score: &score}

	repo.SaveAnswers(context.Background(), a.ID.Hex(), map[string]models.Answer{"s1": ans})

	// Mutate original score pointer — stored value should not change
	score = 99
	got, _ := repo.GetByID(context.Background(), a.ID.Hex())
	if *got.Answers["s1"].Score != 3 {
		t.Error("stored score should be independent of original pointer")
	}
}

// ─────────────────────────────────────────────
// SaveResult
// ─────────────────────────────────────────────

func TestMock_SaveResult_SetsComputedStatus(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})

	result := &models.Result{Overall: 72.5, Maturity: "AI Structured"}
	updated, err := repo.SaveResult(context.Background(), a.ID.Hex(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != models.StatusComputed {
		t.Errorf("expected computed, got %s", updated.Status)
	}
	if updated.Result == nil {
		t.Fatal("result should be set")
	}
	if updated.Result.Overall != 72.5 {
		t.Errorf("overall mismatch: %.2f", updated.Result.Overall)
	}
}

func TestMock_SaveResult_SetsComputedAt(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})

	before := time.Now().Add(-time.Second)
	updated, _ := repo.SaveResult(context.Background(), a.ID.Hex(), &models.Result{})
	after := time.Now().Add(time.Second)

	if updated.Result.ComputedAt.Before(before) || updated.Result.ComputedAt.After(after) {
		t.Errorf("computed_at %v out of expected window", updated.Result.ComputedAt)
	}
}

// ─────────────────────────────────────────────
// List
// ─────────────────────────────────────────────

func TestMock_List_ReturnsAll(t *testing.T) {
	repo := NewMock()
	for i := 0; i < 5; i++ {
		repo.Create(context.Background(), &models.Assessment{})
	}

	items, total, err := repo.List(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(items) != 5 {
		t.Errorf("expected 5 items, got %d", len(items))
	}
}

func TestMock_List_AppliesLimit(t *testing.T) {
	repo := NewMock()
	for i := 0; i < 10; i++ {
		repo.Create(context.Background(), &models.Assessment{})
	}

	items, total, _ := repo.List(context.Background(), 3, 0)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items with limit=3, got %d", len(items))
	}
}

func TestMock_List_AppliesOffset(t *testing.T) {
	repo := NewMock()
	for i := 0; i < 5; i++ {
		repo.Create(context.Background(), &models.Assessment{})
	}

	items, total, _ := repo.List(context.Background(), 100, 3)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with offset=3, got %d", len(items))
	}
}

func TestMock_List_OffsetBeyondEnd(t *testing.T) {
	repo := NewMock()
	repo.Create(context.Background(), &models.Assessment{})

	items, _, _ := repo.List(context.Background(), 100, 999)
	if len(items) != 0 {
		t.Errorf("expected 0 items for large offset, got %d", len(items))
	}
}

// ─────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────

func TestMock_Delete_Removes(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})

	if err := repo.Delete(context.Background(), a.ID.Hex()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if repo.Len() != 0 {
		t.Error("expected store to be empty after delete")
	}
}

func TestMock_Delete_NotFound(t *testing.T) {
	repo := NewMock()
	err := repo.Delete(context.Background(), "6643f1a2c3d4e5f6a7b8c9d0")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMock_Delete_DoubleDelete(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	id := a.ID.Hex()

	repo.Delete(context.Background(), id)
	err := repo.Delete(context.Background(), id)
	if err != ErrNotFound {
		t.Errorf("second delete: expected ErrNotFound, got %v", err)
	}
}

// ─────────────────────────────────────────────
// Concurrency safety
// ─────────────────────────────────────────────

func TestMock_ConcurrentAccess_NoRace(t *testing.T) {
	repo := NewMock()
	a, _ := repo.Create(context.Background(), &models.Assessment{})
	id := a.ID.Hex()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 4 {
			case 0:
				repo.GetByID(context.Background(), id)
			case 1:
				repo.SaveAnswers(context.Background(), id,
					map[string]models.Answer{"s1": {Score: ptr(n%5 + 1)}})
			case 2:
				repo.List(context.Background(), 10, 0)
			case 3:
				repo.Create(context.Background(), &models.Assessment{})
			}
		}(i)
	}
	wg.Wait()
}

// ─────────────────────────────────────────────
// Reset helper
// ─────────────────────────────────────────────

func TestMock_Reset_ClearsStore(t *testing.T) {
	repo := NewMock()
	for i := 0; i < 5; i++ {
		repo.Create(context.Background(), &models.Assessment{})
	}
	if repo.Len() != 5 {
		t.Fatalf("expected 5 before reset, got %d", repo.Len())
	}
	repo.Reset()
	if repo.Len() != 0 {
		t.Errorf("expected 0 after reset, got %d", repo.Len())
	}
}

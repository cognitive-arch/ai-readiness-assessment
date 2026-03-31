// scripts/migrate.go
// Run with: go run scripts/migrate.go
// Creates collections, indexes, and optionally seeds sample data.
//
// Usage:
//
//	MONGO_URI=mongodb://localhost:27017 go run scripts/migrate.go
//	MONGO_URI=... SEED=true go run scripts/migrate.go   # also insert sample
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	uri := os.Getenv("MONGO_URI")

	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dbName := getenv("MONGO_DB", "ai_readiness")
	seed := os.Getenv("SEED") == "true"
	bankPath := getenv("QUESTION_BANK_PATH", "question-bank-v1.json")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println(uri)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	if err := client.Ping(ctx, nil); err != nil {
		fatalf("ping: %v", err)
	}
	fmt.Printf("✓ Connected to MongoDB at %s / db=%s\n", uri, dbName)

	db := client.Database(dbName)

	// ── Migrate ──────────────────────────────────────────────
	if err := migrate(ctx, db); err != nil {
		fatalf("migrate: %v", err)
	}
	fmt.Println("✓ Migrations complete")

	// ── Seed ─────────────────────────────────────────────────

	fmt.Printf("seed value: %v\n", seed)

	if seed {
		bank, err := loadBank(bankPath)
		if err != nil {
			fatalf("load bank: %v", err)
		}
		if err := seedSample(ctx, db, bank); err != nil {
			fatalf("seed: %v", err)
		}
		fmt.Println("✓ Sample data seeded")
	}

	fmt.Println("\nDone.")
}

// migrate creates collections and all required indexes.
func migrate(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("assessments")

	// Drop + recreate isn't safe in prod — use CreateMany which is idempotent
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
		{
			// Compound: status + created_at — common list query pattern
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_status_created_at"),
		},
	}

	names, err := col.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	for _, n := range names {
		fmt.Printf("  index: %s\n", n)
	}
	return nil
}

// seedSample inserts one sample assessment in the "computed" state.
func seedSample(ctx context.Context, db *mongo.Database, bank *questionBank) error {
	col := db.Collection("assessments")

	// Don't seed if data already exists
	count, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		fmt.Printf("  skipped: %d document(s) already exist\n", count)
		return nil
	}

	// Build synthetic answers: score 3 for most, score 2 for a few critical ones
	answers := bson.M{}
	for _, q := range bank.Questions {
		score := 3
		if q.IsCritical && q.Domain == "Data" {
			score = 2
		}
		answers[q.ID] = bson.M{"score": score, "comment": ""}
	}

	doc := bson.M{
		"_id":        primitive.NewObjectID(),
		"status":     "computed",
		"client_ref": "sample-organization",
		"answers":    answers,
		"result": bson.M{
			"overall":    52.4,
			"maturity":   "AI Emerging",
			"confidence": 100.0,
			"risks":      bson.A{"CRITICAL_GAPS", "DATA_HIGH_RISK"},
			"domain_scores": bson.M{
				"Strategic":    bson.M{"score": 62.5, "answered": 12, "total": 12},
				"Technology":   bson.M{"score": 56.2, "answered": 12, "total": 12},
				"Data":         bson.M{"score": 38.9, "answered": 12, "total": 12},
				"Organization": bson.M{"score": 58.3, "answered": 12, "total": 12},
				"Security":     bson.M{"score": 47.2, "answered": 12, "total": 12},
				"UseCase":      bson.M{"score": 55.0, "answered": 12, "total": 12},
			},
			"recommendations": bson.A{
				bson.M{"domain": "Data", "text": "Implement a unified data catalog", "priority": "Critical", "phase": "Phase 1"},
				bson.M{"domain": "Security", "text": "Conduct AI threat modeling", "priority": "High", "phase": "Phase 1"},
			},
			"total_answered": len(bank.Questions),
			"total_q":        len(bank.Questions),
			"computed_at":    time.Now().UTC(),
		},
		"created_at": time.Now().UTC().Add(-24 * time.Hour),
		"updated_at": time.Now().UTC(),
	}

	_, err = col.InsertOne(ctx, doc)
	return err
}

// ─────────────────────────────────────────────
// Question bank loader (minimal, no models import)
// ─────────────────────────────────────────────

type questionBank struct {
	Domains   []string   `json:"domains"`
	Questions []question `json:"questions"`
}

type question struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	IsCritical bool   `json:"isCritical"`
}

func loadBank(path string) (*questionBank, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var b questionBank
	return &b, json.NewDecoder(f).Decode(&b)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

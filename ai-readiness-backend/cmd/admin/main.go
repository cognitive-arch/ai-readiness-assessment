// cmd/admin/main.go
// Admin CLI for the AI Readiness Assessment backend.
//
// Usage:
//   go run ./cmd/admin [command] [flags]
//
// Commands:
//   stats          — show database statistics
//   list           — list recent assessments
//   get <id>       — show a single assessment
//   recompute <id> — re-run scoring for an assessment
//   delete <id>    — permanently delete an assessment
//   export <id>    — export assessment result as JSON to stdout
//   validate-bank  — validate the question bank file
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"github.com/yourorg/ai-readiness-backend/internal/service"
)

const usage = `AI Readiness Admin CLI

Usage: admin <command> [args]

Commands:
  stats                   Show database statistics
  list                    List 20 most recent assessments
  get        <id>         Show full assessment document
  recompute  <id>         Re-run scoring engine for an assessment
  delete     <id>         Permanently delete an assessment
  export     <id>         Print computed result as JSON
  validate-bank           Validate the question bank file

Environment:
  MONGO_URI              MongoDB connection string (required)
  MONGO_DB               Database name (default: ai_readiness)
  QUESTION_BANK_PATH     Path to question-bank-v1.json
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	cmd := os.Args[1]

	// validate-bank doesn't need MongoDB
	if cmd == "validate-bank" {
		runValidateBank()
		return
	}

	// All other commands need MongoDB + service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongoURI := mustEnv("MONGO_URI")
	dbName := getenv("MONGO_DB", "ai_readiness")
	bankPath := getenv("QUESTION_BANK_PATH", "question-bank-v1.json")

	client := mustConnect(ctx, mongoURI)
	defer client.Disconnect(context.Background()) //nolint:errcheck

	db := client.Database(dbName)
	log, _ := zap.NewDevelopment()

	repo, err := repository.NewAssessmentRepository(ctx, db)
	must("init repo", err)

	svc, err := service.NewAssessmentService(repo, bankPath, log)
	must("init service", err)

	switch cmd {
	case "stats":
		runStats(ctx, db)
	case "list":
		runList(ctx, svc)
	case "get":
		runGet(ctx, svc, argAt(2, "id"))
	case "recompute":
		runRecompute(ctx, svc, argAt(2, "id"))
	case "delete":
		runDelete(ctx, svc, argAt(2, "id"))
	case "export":
		runExport(ctx, svc, argAt(2, "id"))
	default:
		fatalf("unknown command %q\n\n%s", cmd, usage)
	}
}

// ─────────────────────────────────────────────
// Command handlers
// ─────────────────────────────────────────────

func runStats(ctx context.Context, db *mongo.Database) {
	col := db.Collection("assessments")

	total, _ := col.CountDocuments(ctx, bson.M{})
	draft, _ := col.CountDocuments(ctx, bson.M{"status": "draft"})
	inProgress, _ := col.CountDocuments(ctx, bson.M{"status": "in_progress"})
	computed, _ := col.CountDocuments(ctx, bson.M{"status": "computed"})

	// Average overall score across computed assessments
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": "computed", "result": bson.M{"$exists": true}}}},
		{{Key: "$group", Value: bson.M{
			"_id":          nil,
			"avg_overall":  bson.M{"$avg": "$result.overall"},
			"avg_confidence": bson.M{"$avg": "$result.confidence"},
		}}},
	}
	cursor, _ := col.Aggregate(ctx, pipeline)
	var aggResult []struct {
		AvgOverall    float64 `bson:"avg_overall"`
		AvgConfidence float64 `bson:"avg_confidence"`
	}
	_ = cursor.All(ctx, &aggResult)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  AI Readiness — Database Stats")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Total assessments:\t%d\n", total)
	fmt.Fprintf(w, "  Draft:\t%d\n", draft)
	fmt.Fprintf(w, "  In Progress:\t%d\n", inProgress)
	fmt.Fprintf(w, "  Computed:\t%d\n", computed)
	if len(aggResult) > 0 {
		fmt.Fprintf(w, "  Avg overall score:\t%.1f / 100\n", aggResult[0].AvgOverall)
		fmt.Fprintf(w, "  Avg confidence:\t%.1f%%\n", aggResult[0].AvgConfidence)
	}
	w.Flush()
	fmt.Println()
}

func runList(ctx context.Context, svc *service.AssessmentService) {
	assessments, total, err := svc.ListAssessments(ctx, 20, 0)
	must("list", err)

	fmt.Printf("Showing %d of %d assessments (most recent first)\n\n", len(assessments), total)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ID\tStatus\tClient\tOverall\tMaturity\tCreated")
	fmt.Fprintln(w, "  ──────────────────────────\t──────────\t────────\t───────\t────────────────────\t───────────")
	for _, a := range assessments {
		overall := "—"
		maturity := "—"
		if a.Result != nil {
			overall = fmt.Sprintf("%.1f", a.Result.Overall)
			maturity = a.Result.Maturity
		}
		client := a.ClientRef
		if client == "" {
			client = "—"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			a.ID.Hex(), a.Status, client, overall, maturity,
			a.CreatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
	fmt.Println()
}

func runGet(ctx context.Context, svc *service.AssessmentService, id string) {
	a, err := svc.GetAssessment(ctx, id)
	must("get", err)

	fmt.Printf("Assessment: %s\n", a.ID.Hex())
	fmt.Printf("Status:     %s\n", a.Status)
	fmt.Printf("Client:     %s\n", strOrDash(a.ClientRef))
	fmt.Printf("Created:    %s\n", a.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:    %s\n", a.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Answers:    %d questions answered\n", countAnswered(a.Answers))

	if a.Result != nil {
		r := a.Result
		fmt.Printf("\n── Result ──────────────────────────\n")
		fmt.Printf("Overall:    %.2f / 100\n", r.Overall)
		fmt.Printf("Maturity:   %s\n", r.Maturity)
		fmt.Printf("Confidence: %.1f%%\n", r.Confidence)
		fmt.Printf("Risks:      %v\n", r.Risks)
		fmt.Printf("Computed:   %s\n", r.ComputedAt.Format(time.RFC3339))
		fmt.Printf("\n── Domain Scores ───────────────────\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for domain, ds := range r.DomainScores {
			fmt.Fprintf(w, "  %s\t%.1f\t(%d/%d answered)\n", domain, ds.Score, ds.Answered, ds.Total)
		}
		w.Flush()
	}
	fmt.Println()
}

func runRecompute(ctx context.Context, svc *service.AssessmentService, id string) {
	fmt.Printf("Recomputing assessment %s...\n", id)
	updated, err := svc.ComputeResult(ctx, id)
	must("recompute", err)

	fmt.Printf("✓ Done\n")
	fmt.Printf("  Overall:  %.2f / 100\n", updated.Result.Overall)
	fmt.Printf("  Maturity: %s\n", updated.Result.Maturity)
	fmt.Printf("  Risks:    %v\n", updated.Result.Risks)
	fmt.Println()
}

func runDelete(ctx context.Context, svc *service.AssessmentService, id string) {
	fmt.Printf("Are you sure you want to permanently delete assessment %s? [y/N] ", id)
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return
	}

	must("delete", svc.DeleteAssessment(ctx, id))
	fmt.Printf("✓ Assessment %s deleted.\n\n", id)
}

func runExport(ctx context.Context, svc *service.AssessmentService, id string) {
	result, err := svc.GetResult(ctx, id)
	must("get result", err)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must("encode", enc.Encode(result))
}

func runValidateBank() {
	bankPath := getenv("QUESTION_BANK_PATH", "question-bank-v1.json")
	f, err := os.Open(bankPath)
	must("open bank", err)
	defer f.Close()

	var bank models.QuestionBank
	must("decode bank", json.NewDecoder(f).Decode(&bank))

	domainCounts := map[string]int{}
	idsSeen := map[string]bool{}
	var issues []string

	for _, q := range bank.Questions {
		// Duplicate IDs
		if idsSeen[q.ID] {
			issues = append(issues, fmt.Sprintf("  duplicate question ID: %s", q.ID))
		}
		idsSeen[q.ID] = true

		// Unknown domain
		validDomain := false
		for _, d := range bank.Domains {
			if q.Domain == d {
				validDomain = true
				break
			}
		}
		if !validDomain {
			issues = append(issues, fmt.Sprintf("  question %s has unknown domain %q", q.ID, q.Domain))
		}

		// Weight range
		if q.Weight < 1 || q.Weight > 5 {
			issues = append(issues, fmt.Sprintf("  question %s has invalid weight %d (must be 1-5)", q.ID, q.Weight))
		}

		domainCounts[q.Domain]++
	}

	fmt.Printf("Question bank: %s\n", bankPath)
	fmt.Printf("Domains:       %d → %v\n", len(bank.Domains), bank.Domains)
	fmt.Printf("Questions:     %d total\n\n", len(bank.Questions))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, d := range bank.Domains {
		count := domainCounts[d]
		status := "✓"
		if count != 12 {
			status = fmt.Sprintf("⚠  expected 12, got %d", count)
		}
		fmt.Fprintf(w, "  %s\t%d questions\t%s\n", d, count, status)
	}
	w.Flush()

	if len(issues) > 0 {
		fmt.Printf("\n⚠  %d issue(s) found:\n", len(issues))
		for _, issue := range issues {
			fmt.Println(issue)
		}
		os.Exit(1)
	}
	fmt.Printf("\n✓ Question bank is valid.\n\n")
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func mustConnect(ctx context.Context, uri string) *mongo.Client {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetConnectTimeout(10*time.Second))
	must("connect", err)
	must("ping", client.Ping(ctx, nil))
	return client
}

func countAnswered(answers map[string]models.Answer) int {
	n := 0
	for _, a := range answers {
		if a.Score != nil {
			n++
		}
	}
	return n
}

func argAt(pos int, name string) string {
	if len(os.Args) <= pos {
		fatalf("missing required argument <%s>", name)
	}
	return os.Args[pos]
}

func must(op string, err error) {
	if err != nil {
		fatalf("%s: %v", op, err)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fatalf("%s environment variable is required", key)
	}
	return v
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func strOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

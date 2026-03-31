// tests/fixtures/fixtures.go
// Utility for loading test fixture files.
// Fixtures are canonical JSON payloads that represent specific assessment states.
// They make integration tests more readable and maintainable.
package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// FullAnswerSet is the JSON structure of an answer fixture file.
type FullAnswerSet struct {
	ClientRef string                   `json:"client_ref"`
	Answers   map[string]models.Answer `json:"answers"`
}

// Load reads a fixture JSON file from the fixtures directory by name (without extension).
// Example: fixtures.Load("full_assessment_score3")
func Load(name string) (*FullAnswerSet, error) {
	path := filepath.Join(fixtureDir(), name+".json")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var fixture FullAnswerSet
	if err := json.NewDecoder(f).Decode(&fixture); err != nil {
		return nil, err
	}
	return &fixture, nil
}

// MustLoad is like Load but panics on error. For use in test setup only.
func MustLoad(name string) *FullAnswerSet {
	f, err := Load(name)
	if err != nil {
		panic("fixtures.MustLoad: " + err.Error())
	}
	return f
}

// AllAnswersScored returns all 72 question IDs with the given score.
// Useful for building complete answer sets in unit tests without a fixture file.
func AllAnswersScored(score int) map[string]models.Answer {
	qids := allQuestionIDs()
	m := make(map[string]models.Answer, len(qids))
	s := score
	for _, qid := range qids {
		m[qid] = models.Answer{Score: &s}
	}
	return m
}

// fixtureDir returns the absolute path to the tests/fixtures directory.
func fixtureDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

func allQuestionIDs() []string {
	return []string{
		"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12",
		"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12",
		"d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9", "d10", "d11", "d12",
		"o1", "o2", "o3", "o4", "o5", "o6", "o7", "o8", "o9", "o10", "o11", "o12",
		"sec1", "sec2", "sec3", "sec4", "sec5", "sec6", "sec7", "sec8", "sec9", "sec10", "sec11", "sec12",
		"u1", "u2", "u3", "u4", "u5", "u6", "u7", "u8", "u9", "u10", "u11", "u12",
	}
}

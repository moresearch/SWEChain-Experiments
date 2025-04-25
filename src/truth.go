package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

// ------------------------------------------------------------
// Schemas remembered:
//
// data.csv columns:
//   question_id, variant, price, price_limit, prompt,
//   manager_data, manager_commit, acceptable_folders, cwd, canary
//
// manager_data field holds the ground‐truth manager(s) for each task,
// either as a JSON array or comma‑separated IDs.
// ------------------------------------------------------------

// GroundTruth holds the mapping of manager -> tasks assigned.
type GroundTruth struct {
	Assignments map[string][]string `json:"assignments"`
}

func main() {
	csvPath := flag.String("csv", "data.csv", "path to data.csv with manager_data")
	outPath := flag.String("out", "ground_truth.json", "where to write ground truth JSON")
	flag.Parse()

	f, err := os.Open(*csvPath)
	if err != nil {
		log.Fatalf("opening CSV: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		log.Fatalf("reading header: %v", err)
	}
	// build name->index map
	idx := map[string]int{}
	for i, col := range header {
		idx[strings.ToLower(col)] = i
	}

	qi := idx["question_id"]
	vi := idx["variant"]
	mdi := idx["manager_data"]

	gt := GroundTruth{Assignments: make(map[string][]string)}
	for rowNum := 1; ; rowNum++ {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("row %d: csv read error: %v", rowNum, err)
			continue
		}
		variant := strings.ToLower(rec[vi])
		// only SWE Manager tasks have ground truth
		if !strings.Contains(variant, "manager") {
			continue
		}
		taskID := rec[qi]
		mgrField := strings.TrimSpace(rec[mdi])
		managers := parseManagerData(mgrField)
		for _, m := range managers {
			gt.Assignments[m] = append(gt.Assignments[m], taskID)
		}
	}

	out, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(gt); err != nil {
		log.Fatalf("encode JSON: %v", err)
	}
	fmt.Printf("Wrote ground truth to %s\n", *outPath)
}

// parseManagerData handles either JSON arrays or comma‑separated strings:
//
//	e.g. '["agent2"]'  or  'agent2,agent5'
func parseManagerData(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}
	// detect JSON array
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
	}
	// fallback: split on commas
	parts := regexp.MustCompile(`\s*,\s*`).Split(s, -1)
	var out []string
	for _, p := range parts {
		if p = strings.Trim(p, `"' `); p != "" {
			out = append(out, p)
		}
	}
	return out
}
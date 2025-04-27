package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Speciality struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type TaskBrief struct {
	ID        string  `json:"id"`
	Desc      string  `json:"desc"`
	Specialty string  `json:"specialty"`
	PriceMin  float64 `json:"price_min"`
	PriceMax  float64 `json:"price_max"`
	Timestamp string  `json:"timestamp"`
}

type AgentSchema struct {
	AgentID      string       `json:"agent_id"`
	Specialities []Speciality `json:"specialities"`
	Tasks        []TaskBrief  `json:"tasks"`
}

func shuffleSpecialities(specialities []Speciality) []Speciality {
	shuffled := make([]Speciality, len(specialities))
	copy(shuffled, specialities)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled
}

func randomSubsetSpecialities(specs []Speciality) []Speciality {
	n := rand.Intn(len(specs)) + 1 // at least 1 specialty
	shuffled := shuffleSpecialities(specs)
	return shuffled[:n]
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Command-line flags
	inputFile := flag.String("input", "./data/data.csv", "Input CSV file")
	outputDir := flag.String("output", "./data", "Output directory for agent JSON files")
	model := flag.String("model", "granite3.3:8b", "LLM model name (optional)")
	ollamaURL := flag.String("ollama_url", "http://localhost:11434/api/generate", "Ollama API URL (optional)")
	numIssues := flag.Int("num_issues", 10, "Number of issues/tasks to process")
	numAgents := flag.Int("num_agents", 10, "Number of agents to create")
	llmRetries := flag.Int("llm_retries", 5, "Number of LLM call retries (optional)")
	flag.Parse()

	_ = model
	_ = ollamaURL
	_ = numIssues
	_ = llmRetries

	// Define specialties using provided descriptions and examples
	specialities := []Speciality{
		{
			Name:        "ApplicationLogic",
			Description: "This agent's speciality includes state management, client-side routing, form handling tasks.",
			Example:     "When attempting to log in, the app doesn't switch focus to the first digit field after clicking 'didn't receive a magic code?'",
		},
		{
			Name:        "ServerSideLogic",
			Description: "API endpoints, DB queries, authentication, data processing-related tasks.",
			Example:     "The tooltip displays the users' email instead of their display name when hovering over the counter in the split preview.",
		},
		{
			Name:        "BugFixes",
			Description: "This agent's speciality includes unexpected behaviours, errors, inconsistencies.",
			Example:     "Navigating back from the flag as offensive screen doesn't display the correct report page.",
		},
		{
			Name:        "UI/UX",
			Description: "This agent's speciality includes design changes, layout, interaction improvements.",
			Example:     "Overlay background color is different.",
		},
		{
			Name:        "SystemWideQualityAndReliability",
			Description: "This agent's speciality includes topics related to refactoring code, performance, optimization.",
			Example:     "Opening a thread calls the OpenReport API twice.",
		},
		{
			Name:        "NewFeaturesOrEnhancements",
			Description: "This agent's speciality includes new functionality, optimization of existing features.",
			Example:     "Add the ability to mention yourself and use @here in the mentions auto-suggestions list.",
		},
		{
			Name:        "ReliabilityImprovements",
			Description: "This agent's speciality includes logging, monitoring, testing.",
			Example:     "Add a character limit on the length of task names, room names, etc.",
		},
	}

	// Generate agents and assign them RANDOM specialties
	agents := make([]*AgentSchema, *numAgents)
	for i := 0; i < *numAgents; i++ {
		agents[i] = &AgentSchema{
			AgentID:      fmt.Sprintf("agent%d", i+1),
			Specialities: randomSubsetSpecialities(specialities),
			Tasks:        []TaskBrief{},
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	csvFile, err := os.Open(*inputFile)
	if err != nil {
		log.Fatalf("Failed to open CSV: %v", err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}
	if len(records) < 1 {
		log.Fatal("Input CSV is empty")
	}
	header := records[0]
	col := func(name string) int {
		for i, h := range header {
			if h == name {
				return i
			}
		}
		return -1
	}

	// Assign each task to a RANDOM agent
	for i, record := range records[1:] {
		if *numIssues > 0 && i >= *numIssues {
			break
		}
		taskID := record[col("question_id")]
		desc := record[col("prompt")]

		// For specialty, pick a random one from the full list:
		taskSpecialty := specialities[rand.Intn(len(specialities))].Name

		price := 0.0
		if pcol := col("price"); pcol != -1 {
			if v, err := strconv.ParseFloat(record[pcol], 64); err == nil {
				price = v
			}
		}
		priceLimit := 0.0
		if plcol := col("price_limit"); plcol != -1 {
			if v, err := strconv.ParseFloat(record[plcol], 64); err == nil {
				priceLimit = v
			}
		}

		taskBrief := TaskBrief{
			ID:        taskID,
			Desc:      desc,
			Specialty: taskSpecialty,
			PriceMin:  price,
			PriceMax:  priceLimit,
			Timestamp: now,
		}
		// Assign to a random agent
		randomAgentIdx := rand.Intn(*numAgents)
		agents[randomAgentIdx].Tasks = append(agents[randomAgentIdx].Tasks, taskBrief)
	}

	// Output agent files
	os.MkdirAll(*outputDir, 0755)
	for _, agent := range agents {
		data, _ := json.MarshalIndent(agent, "", "  ")
		outfile := filepath.Join(*outputDir, fmt.Sprintf("%s.json", agent.AgentID))
		err := os.WriteFile(outfile, data, 0644)
		if err != nil {
			log.Printf("Failed to write %s: %v", outfile, err)
		}
	}
	fmt.Printf("Agent JSON files written to %s/ (with randomized specialties and task assignments)\n", *outputDir)
}

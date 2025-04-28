package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Speciality struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Example     string  `json:"example"`
	Weight      float64 `json:"weight"`
}

type TaskBrief struct {
	ID        string  `json:"id"`
	Desc      string  `json:"desc"`
	Specialty string  `json:"specialty"`
	PriceMin  float64 `json:"price_min"`
	PriceMax  float64 `json:"price_max"`
}

type AgentSchema struct {
	AgentID      string       `json:"agent_id"`
	Specialities []Speciality `json:"specialities"`
	Tasks        []TaskBrief  `json:"tasks"`
}

// Call Ollama API to classify a task description into a specialty name
func classifySpecialtyOllama(prompt, ollamaURL, model string, specialties []Speciality) (string, error) {
	type OllamaRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	type OllamaResponse struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	// Build the prompt with all specialty names and descriptions
	promptText := prompt + "\n\nAvailable specialties:\n"
	for _, s := range specialties {
		promptText += fmt.Sprintf(" - %s: %s\n", s.Name, s.Description)
	}
	promptText += "\nReturn only the best matching specialty name from the above list. If none fit, pick the closest one."

	reqBody := OllamaRequest{
		Model:  model,
		Prompt: promptText,
	}
	jsonBody, _ := json.Marshal(reqBody)
	log.Println("[Ollama] Calling Ollama model:", model)
	resp, err := http.Post(ollamaURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Println("[Ollama] HTTP POST failed:", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read streaming response until end
	full := ""
	dec := json.NewDecoder(resp.Body)
	for {
		var msg OllamaResponse
		if err := dec.Decode(&msg); err == io.EOF {
			break
		} else if err != nil {
			log.Println("[Ollama] JSON decode failed:", err)
			break
		}
		full += msg.Response
		if msg.Done {
			break
		}
	}
	// Clean up and extract specialty name (first word matching one of the specialties)
	full = cleanString(full)
	log.Println("[Ollama] Model returned:", full)
	for _, s := range specialties {
		if eqIgnoreCase(full, s.Name) {
			return s.Name, nil
		}
	}
	// Fallback: fuzzy match
	for _, s := range specialties {
		if containsIgnoreCase(full, s.Name) {
			return s.Name, nil
		}
	}
	return "Unknown", nil
}

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'` \n\t")
	return s
}

func eqIgnoreCase(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func containsIgnoreCase(hay, needle string) bool {
	return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
}

func shuffleSpecialities(specialities []Speciality) []Speciality {
	shuffled := make([]Speciality, len(specialities))
	copy(shuffled, specialities)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled
}

func randomSubsetSpecialitiesWithWeights(specs []Speciality) []Speciality {
	n := rand.Intn(len(specs)) + 1
	shuffled := shuffleSpecialities(specs)
	selected := shuffled[:n]
	weights := randomWeights(len(selected))
	for i := range selected {
		selected[i].Weight = weights[i]
	}
	return selected
}

func randomWeights(n int) []float64 {
	r := make([]float64, n)
	sum := 0.0
	for i := range r {
		r[i] = rand.Float64()
		sum += r[i]
	}
	weights := make([]float64, n)
	total := 0.0
	for i := range r {
		w := (r[i] / sum) * 100.0
		w = mathRound(w, 2)
		weights[i] = w
		total += w
	}
	diff := mathRound(100.0-total, 2)
	weights[n-1] = mathRound(weights[n-1]+diff, 2)
	return weights
}

func mathRound(x float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return float64(int(x*pow+0.5)) / pow
}

func main() {
	rand.Seed(time.Now().UnixNano())

	inputFile := flag.String("input", "./data/data.csv", "Input CSV file")
	outputDir := flag.String("output", "./data", "Output directory for agent JSON files")
	model := flag.String("model", "cogito:14b", "LLM model name")
	ollamaURL := flag.String("ollama_url", "http://localhost:11434/api/generate", "Ollama API URL")
	numIssues := flag.Int("num_issues", 10, "Number of issues/tasks to process")
	numAgents := flag.Int("num_agents", 10, "Number of agents to create")
	llmRetries := flag.Int("llm_retries", 3, "Number of LLM call retries per task")
	flag.Parse()

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

	agents := make([]*AgentSchema, *numAgents)
	for i := 0; i < *numAgents; i++ {
		agents[i] = &AgentSchema{
			AgentID:      fmt.Sprintf("agent%d", i+1),
			Specialities: randomSubsetSpecialitiesWithWeights(specialities),
			Tasks:        []TaskBrief{},
		}
	}
	log.Println("[Main] Generated", *numAgents, "agents with random specialties and weights.")

	csvFile, err := os.Open(*inputFile)
	if err != nil {
		log.Fatalf("[Main] Failed to open CSV: %v", err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("[Main] Failed to read CSV: %v", err)
	}
	if len(records) < 1 {
		log.Fatal("[Main] Input CSV is empty")
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

	// Assign each task to a RANDOM agent, use Ollama to assign specialty
	for i, record := range records[1:] {
		if *numIssues > 0 && i >= *numIssues {
			log.Println("[Main] Reached issue limit:", *numIssues)
			break
		}
		taskID := record[col("question_id")]
		desc := record[col("prompt")]

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

		// Use Ollama to assign specialty for this task
		var taskSpecialty string
		var classifyErr error
		for attempt := 0; attempt < *llmRetries; attempt++ {
			log.Println("[Main] Calling LLM for task", taskID, "(attempt", attempt+1, ")")
			taskSpecialty, classifyErr = classifySpecialtyOllama(
				fmt.Sprintf("Classify this software issue description into a specialty:\n%s", desc),
				*ollamaURL, *model, specialities)
			if classifyErr == nil && taskSpecialty != "" && taskSpecialty != "Unknown" {
				break
			}
			log.Println("[Main] LLM failed or returned unknown specialty, retrying...")
		}
		if classifyErr != nil || taskSpecialty == "" || taskSpecialty == "Unknown" {
			taskSpecialty = specialities[rand.Intn(len(specialities))].Name // fallback
			log.Println("[WARN] LLM could not classify task", taskID, ", fallback to random specialty:", taskSpecialty)
		} else {
			log.Println("[Main] LLM assigned specialty for task", taskID, ":", taskSpecialty)
		}

		taskBrief := TaskBrief{
			ID:        taskID,
			Desc:      desc,
			Specialty: taskSpecialty,
			PriceMin:  price,
			PriceMax:  priceLimit,
		}
		// Assign to a random agent
		randomAgentIdx := rand.Intn(*numAgents)
		agents[randomAgentIdx].Tasks = append(agents[randomAgentIdx].Tasks, taskBrief)
	}

	log.Println("[Main] Writing agent JSON files to", *outputDir)
	os.MkdirAll(*outputDir, 0755)
	for _, agent := range agents {
		// Ensure agent's weights sum to 100.0 (float rounding fix)
		total := 0.0
		for _, s := range agent.Specialities {
			total += s.Weight
		}
		diff := mathRound(100.0-total, 2)
		if len(agent.Specialities) > 0 {
			agent.Specialities[len(agent.Specialities)-1].Weight = mathRound(agent.Specialities[len(agent.Specialities)-1].Weight+diff, 2)
		}
		data, _ := json.MarshalIndent(agent, "", "  ")
		outfile := filepath.Join(*outputDir, fmt.Sprintf("%s.json", agent.AgentID))
		err := os.WriteFile(outfile, data, 0644)
		if err != nil {
			log.Println("[Main] Failed to write", outfile, ":", err)
		} else {
			log.Println("[Main] Wrote", outfile)
		}
	}
	log.Println("[Main] All agent files written.")
	fmt.Printf("Agent JSON files written to %s/ (with LLM-assigned specialties, weights summing to 100%%, and task assignments)\n", *outputDir)
}

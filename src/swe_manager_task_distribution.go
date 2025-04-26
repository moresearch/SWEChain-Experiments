package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ColorReset       = "\033[0m"
	ColorBugfix      = "\033[37;41m"
	ColorFeatures    = "\033[37;42m"
	ColorReliability = "\033[37;46m"
	ColorWarning     = "\033[37;43m"
	ColorInfo        = "\033[37;44m"
	ColorStats       = "\033[37;45m"
	ColorAppLogic    = "\033[37;43m"
	ColorServerLogic = "\033[37;47;30m"
	ColorUIUX        = "\033[37;45m"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
}

type CategoryResponse struct {
	CategoryIDs []int `json:"category_ids"`
}

type Task struct {
	TaskID        string   `json:"task_id"`
	Description   string   `json:"description"`
	Categories    []string `json:"categories"`
	AuctionStatus string   `json:"auction_status,omitempty"`
	Status        string   `json:"status,omitempty"`
	Price         float64  `json:"price"`
	PriceLimit    float64  `json:"price_limit,omitempty"`
}

type TaskData struct {
	Description string
	Variant     string
	Price       float64
	PriceLimit  float64
	QuestionID  string
	OriginalRow int
}

type Agent struct {
	AgentID          string `json:"agent_id"`
	Specialty        string `json:"specialty"`
	TasksToOutsource []Task `json:"tasks_to_outsource"`
	OwnTasks         []Task `json:"own_tasks"`
}

type Category struct {
	ID   int
	Name string
}

var availableCategories = []Category{
	{ID: 1, Name: "ApplicationLogic"},
	{ID: 2, Name: "ServerSideLogic"},
	{ID: 3, Name: "UI/UX"},
	{ID: 4, Name: "SystemQuality/Reliability"},
	{ID: 5, Name: "Bugfix"},
	{ID: 6, Name: "NewFeatures/Enhancement"},
	{ID: 7, Name: "ReliabilityImprovements"},
}

var categoryIDToName = map[int]string{
	1: "ApplicationLogic",
	2: "ServerSideLogic",
	3: "UI/UX",
	4: "SystemQuality/Reliability",
	5: "Bugfix",
	6: "NewFeatures/Enhancement",
	7: "ReliabilityImprovements",
}

var categoryColors = map[string]string{
	"ApplicationLogic":          ColorAppLogic,
	"ServerSideLogic":           ColorServerLogic,
	"UI/UX":                     ColorUIUX,
	"SystemQuality/Reliability": ColorInfo,
	"Bugfix":                    ColorBugfix,
	"NewFeatures/Enhancement":   ColorFeatures,
	"ReliabilityImprovements":   ColorReliability,
}

func getCategoryIDsList() string {
	var ids []string
	for _, cat := range availableCategories {
		ids = append(ids, fmt.Sprintf("%d - %s", cat.ID, cat.Name))
	}
	return strings.Join(ids, "\n")
}

func colorForCategory(category string) string {
	if color, exists := categoryColors[category]; exists {
		return color
	}
	return ColorReset
}

func extractTaskContent(text string) string {
	contentMatch := regexp.MustCompile(`'content':\s*'([^']+)'`).FindStringSubmatch(text)
	if len(contentMatch) > 1 {
		return contentMatch[1]
	}
	contentMatch = regexp.MustCompile(`"content":\s*"([^"]+)"`).FindStringSubmatch(text)
	if len(contentMatch) > 1 {
		return contentMatch[1]
	}
	return strings.Trim(text, "[]{}'\"`")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func readTasksFromCSV(filepath string) ([]TaskData, error) {
	log.Printf("%s[CSV] Opening file: %s%s", ColorInfo, filepath, ColorReset)
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("%s[ERROR] Failed to open CSV file: %v%s", ColorBugfix, err, ColorReset)
		return nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("%s[ERROR] Failed to read CSV content: %v%s", ColorBugfix, err, ColorReset)
		return nil, fmt.Errorf("error reading CSV file: %w", err)
	}
	log.Printf("%s[CSV] Total rows read: %d%s", ColorInfo, len(records), ColorReset)
	hasHeader := len(records) > 0
	startRow := 0
	if hasHeader && len(records) > 0 {
		startRow = 1
	}
	columnMap := make(map[string]int)
	if hasHeader {
		header := records[0]
		for i, cell := range header {
			columnName := strings.ToLower(strings.TrimSpace(cell))
			columnMap[columnName] = i
		}
	}
	questionIDCol := getColumnIndex(columnMap, "question_id", 0)
	variantCol := getColumnIndex(columnMap, "variant", 1)
	priceCol := getColumnIndex(columnMap, "price", 2)
	priceLimitCol := getColumnIndex(columnMap, "price_limit", 3)
	promptCol := getColumnIndex(columnMap, "prompt", 4)
	var tasks []TaskData
	for i := startRow; i < len(records); i++ {
		record := records[i]
		maxCol := max(max(questionIDCol, variantCol), max(priceCol, promptCol))
		if len(record) <= maxCol {
			log.Printf("%s[WARNING] Row %d skipped: not enough columns%s", ColorWarning, i+1, ColorReset)
			continue
		}
		questionID := strings.TrimSpace(record[questionIDCol])
		variant := strings.TrimSpace(record[variantCol])
		description := cleanTaskDescription(record[promptCol])
		price := 0.0
		if priceCol < len(record) {
			price, _ = parseFloat(record[priceCol], 0.0)
		}
		priceLimit := 0.0
		if priceLimitCol < len(record) {
			priceLimit, _ = parseFloat(record[priceLimitCol], 0.0)
		}
		if description == "" {
			log.Printf("%s[WARNING] Row %d skipped: empty description%s", ColorWarning, i+1, ColorReset)
			continue
		}
		isSWEManagerTask := strings.Contains(strings.ToLower(variant), "manager") ||
			strings.Contains(strings.ToLower(variant), "swe_manager") ||
			strings.Contains(strings.ToLower(variant), "swe manager")
		if isSWEManagerTask {
			tasks = append(tasks, TaskData{
				QuestionID:  questionID,
				Description: description,
				Variant:     "SWE Manager",
				Price:       price,
				PriceLimit:  priceLimit,
				OriginalRow: i + 1,
			})
		}
	}
	log.Printf("%s[CSV] Loaded %d SWE Manager tasks%s", ColorStats, len(tasks), ColorReset)
	return tasks, nil
}

func cleanTaskDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	spaceRegex := regexp.MustCompile(`\s+`)
	desc = spaceRegex.ReplaceAllString(desc, " ")
	if (strings.HasPrefix(desc, "\"") && strings.HasSuffix(desc, "\"")) ||
		(strings.HasPrefix(desc, "'") && strings.HasSuffix(desc, "'")) {
		desc = desc[1 : len(desc)-1]
	}
	return desc
}

func categorizeTaskWithRetries(taskDescription, modelName, ollamaURL string, maxRetries int) ([]string, error) {
	cleanDescription := extractTaskContent(taskDescription)
	cleanDescription = truncateString(cleanDescription, 1000)
	prompt := fmt.Sprintf(
		`You are a software engineering manager responsible for categorizing engineering tasks.
I will provide you with a task description, and you need to categorize it into one or more of the predefined categories.

Categories (ID - Name):
%s

Task Description: "%s"

A task can belong to multiple categories. Based on the task description, which categories best fit this task? 
Return your answer as a JSON object with the following schema:
{
  "category_ids": [int, int, ...]
}
`, getCategoryIDsList(), cleanDescription)
	requestPayload := OllamaRequest{
		Model:  modelName,
		Prompt: prompt,
		Stream: false,
	}
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}
	timeoutSeconds := 900

	for retry := 0; retry <= maxRetries; retry++ {
		log.Printf("%s[LLM] Categorizing task (attempt %d/%d): %s%s",
			ColorInfo, retry+1, maxRetries+1, truncateString(cleanDescription, 60), ColorReset)
		client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
		req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("%s[ERROR] Failed to create HTTP request: %v%s", ColorBugfix, err, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("%s[RETRY] Ollama API request failed (attempt %d/%d): %v%s", ColorWarning, retry+1, maxRetries+1, err, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("%s[RETRY] Ollama API response read failed (attempt %d/%d): %v%s", ColorWarning, retry+1, maxRetries+1, err, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("%s[RETRY] Ollama API non-OK status %d (attempt %d/%d)%s", ColorWarning, resp.StatusCode, retry+1, maxRetries+1, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		var ollamaResp OllamaResponse
		err = json.Unmarshal(body, &ollamaResp)
		if err != nil {
			log.Printf("%s[RETRY] Ollama API response unmarshal failed (attempt %d/%d): %v%s", ColorWarning, retry+1, maxRetries+1, err, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		jsonResponse := extractJSONFromString(ollamaResp.Response)
		var categoryResp CategoryResponse
		if err = json.Unmarshal([]byte(jsonResponse), &categoryResp); err != nil {
			log.Printf("%s[RETRY] LLM JSON parse failed (attempt %d/%d): %v%s", ColorWarning, retry+1, maxRetries+1, err, ColorReset)
			time.Sleep(2 * time.Second)
			continue
		}
		var validCategoryNames []string
		for _, id := range categoryResp.CategoryIDs {
			if name, ok := categoryIDToName[id]; ok {
				validCategoryNames = append(validCategoryNames, name)
			}
		}
		if len(validCategoryNames) > 0 {
			log.Printf("%s[LLM] Categorization successful: %v%s", ColorStats, validCategoryNames, ColorReset)
			return validCategoryNames, nil
		}
		log.Printf("%s[RETRY] LLM returned no valid category IDs (attempt %d/%d)%s", ColorWarning, retry+1, maxRetries+1, ColorReset)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("llm categorization failed after %d retries", maxRetries+1)
}

func extractJSONFromString(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return "{}"
	}
	return text[start : end+1]
}

// createAgents produces one agent per category and distributes tasks.
func createAgents(tasks []Task) []Agent {
	agents := make([]Agent, 0, len(availableCategories))
	for _, category := range availableCategories {
		agent := Agent{
			AgentID:   fmt.Sprintf("agent%d", category.ID),
			Specialty: category.Name,
		}
		agents = append(agents, agent)
	}
	for _, task := range tasks {
		for i := range agents {
			isMatchingSpecialty := false
			for _, category := range task.Categories {
				if agents[i].Specialty == category {
					isMatchingSpecialty = true
					break
				}
			}
			if isMatchingSpecialty {
				agents[i].OwnTasks = append(agents[i].OwnTasks, task)
			} else {
				taskCopy := Task{
					TaskID:        task.TaskID,
					Description:   task.Description,
					Categories:    task.Categories,
					AuctionStatus: "open",
					Price:         task.Price,
					PriceLimit:    task.PriceLimit,
				}
				if rand.Float32() < 0.15 {
					agents[i].TasksToOutsource = append(agents[i].TasksToOutsource, taskCopy)
				}
			}
		}
	}
	return agents
}

func writeAgentToJSON(agent Agent, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	filename := filepath.Join(outputDir, fmt.Sprintf("%s.json", agent.AgentID))
	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling agent data: %w", err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("error writing agent file: %w", err)
	}
	log.Printf("%s[FILE] Agent data written: %s (%d tasks, %d outsourced)%s",
		ColorInfo, filename, len(agent.OwnTasks), len(agent.TasksToOutsource), ColorReset)
	return nil
}

func getColumnIndex(columnMap map[string]int, colName string, defaultIndex int) int {
	if idx, exists := columnMap[colName]; exists {
		return idx
	}
	return defaultIndex
}

func parseFloat(str string, defaultVal float64) (float64, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return defaultVal, nil
	}
	str = regexp.MustCompile(`[$,]`).ReplaceAllString(str, "")
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return defaultVal, err
	}
	return val, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	numIssuesPtr := flag.Int("num_issues", 0, "Total number of SWE Manager issues/tasks to use (random subset if less than available)")
	inputFilePath := flag.String("input", "", "Path to input data file (CSV only)")
	outputDir := flag.String("output", "data", "Directory to write agent JSON files")
	ollamaAPIURL := flag.String("ollama_url", "http://localhost:11434/api/generate", "Ollama API URL")
	modelToUse := flag.String("model", "granite3.3:8b", "Ollama model to use")
	maxRetries := flag.Int("llm_retries", 3, "Number of retries for Ollama LLM categorization")
	flag.Parse()

	log.Printf("%s[STARTUP] SWE Manager Task Distribution System initializing!%s", ColorStats, ColorReset)

	if *inputFilePath == "" {
		log.Fatalf("%s[ABORT] Please specify -input <yourfile.csv>%s", ColorBugfix, ColorReset)
	}
	if !strings.HasSuffix(strings.ToLower(*inputFilePath), ".csv") {
		log.Fatalf("%s[ABORT] Input file must be a CSV%s", ColorBugfix, ColorReset)
	}
	log.Printf("%s[CONFIG] CSV input: %s | Output dir: %s | Ollama: %s | Model: %s | Retries: %d | MaxTasks: %d%s",
		ColorInfo, *inputFilePath, *outputDir, *ollamaAPIURL, *modelToUse, *maxRetries, *numIssuesPtr, ColorReset)

	taskData, err := readTasksFromCSV(*inputFilePath)
	if err != nil {
		log.Fatalf("%s[ABORT] Failed to read tasks from input: %v%s", ColorBugfix, err, ColorReset)
	}
	if len(taskData) == 0 {
		log.Fatalf("%s[ABORT] No SWE Manager tasks found in the input file%s", ColorBugfix, ColorReset)
	}
	totalTasks := len(taskData)
	if *numIssuesPtr > 0 && *numIssuesPtr < totalTasks {
		rand.Seed(time.Now().UnixNano())
		perm := rand.Perm(totalTasks)
		sampled := make([]TaskData, 0, *numIssuesPtr)
		for i := 0; i < *numIssuesPtr; i++ {
			sampled = append(sampled, taskData[perm[i]])
		}
		taskData = sampled
		log.Printf("%s[CONFIG] Randomized %d tasks from %d available%s", ColorInfo, *numIssuesPtr, totalTasks, ColorReset)
	}
	log.Printf("%s[PIPELINE] Loaded %d SWE Manager tasks for processing%s", ColorStats, len(taskData), ColorReset)

	var categorizedTasks []Task
	for j, task := range taskData {
		log.Printf("%s[TASK] Processing %d/%d: %s%s", ColorInfo, j+1, len(taskData), truncateString(task.Description, 60), ColorReset)
		startTime := time.Now()
		categories, err := categorizeTaskWithRetries(task.Description, *modelToUse, *ollamaAPIURL, *maxRetries)
		elapsed := time.Since(startTime)
		if err != nil {
			log.Printf("%s[ERROR] LLM categorization failed after %d retries for task %d: %v. Categories: %v%s",
				ColorBugfix, *maxRetries+1, j+1, err, categories, ColorReset)
			categories = []string{}
		}
		log.Printf("%s[TASK] Categorization done in %.2fs for task %d. Categories: %v%s",
			ColorStats, elapsed.Seconds(), j+1, categories, ColorReset)
		newTask := Task{
			TaskID:      fmt.Sprintf("task-%03d", j+1),
			Description: extractTaskContent(task.Description),
			Categories:  categories,
			Status:      "",
			Price:       task.Price,
			PriceLimit:  task.PriceLimit,
		}
		categorizedTasks = append(categorizedTasks, newTask)
	}

	catCounts := make(map[string]int)
	for _, t := range categorizedTasks {
		for _, c := range t.Categories {
			catCounts[c]++
		}
	}
	log.Printf("%s[SUMMARY] Category Distribution:", ColorStats)
	for cat, count := range catCounts {
		log.Printf(" - %s%s%s: %d", colorForCategory(cat), cat, ColorReset, count)
	}
	agents := createAgents(categorizedTasks)

	type assigned struct {
		owner      string
		outsourcer string
	}
	taskAssignments := make(map[string]*assigned)
	for _, agent := range agents {
		for _, t := range agent.OwnTasks {
			a := taskAssignments[t.TaskID]
			if a == nil {
				a = &assigned{}
				taskAssignments[t.TaskID] = a
			}
			if a.owner != "" {
				log.Printf("%s[WARN] Task %s claimed as 'own' by multiple agents: %s and %s. Keeping only the first.%s", ColorWarning, t.TaskID, a.owner, agent.AgentID, ColorReset)
			} else {
				a.owner = agent.AgentID
			}
		}
		for _, t := range agent.TasksToOutsource {
			a := taskAssignments[t.TaskID]
			if a == nil {
				a = &assigned{}
				taskAssignments[t.TaskID] = a
			}
			if a.outsourcer != "" {
				log.Printf("%s[WARN] Task %s marked as 'outsourced' by multiple agents: %s and %s. Keeping only the first.%s", ColorWarning, t.TaskID, a.outsourcer, agent.AgentID, ColorReset)
			} else {
				a.outsourcer = agent.AgentID
			}
		}
	}
	for i := range agents {
		cleanOwn := make([]Task, 0, len(agents[i].OwnTasks))
		for _, t := range agents[i].OwnTasks {
			a := taskAssignments[t.TaskID]
			if a.owner == agents[i].AgentID && a.outsourcer == "" {
				cleanOwn = append(cleanOwn, t)
			} else {
				log.Printf("%s[DATAFIX] Removing ambiguous 'own' task %s from agent %s%s", ColorWarning, t.TaskID, agents[i].AgentID, ColorReset)
			}
		}
		agents[i].OwnTasks = cleanOwn

		cleanOut := make([]Task, 0, len(agents[i].TasksToOutsource))
		for _, t := range agents[i].TasksToOutsource {
			a := taskAssignments[t.TaskID]
			if a.outsourcer == agents[i].AgentID && a.owner == "" {
				cleanOut = append(cleanOut, t)
			} else {
				log.Printf("%s[DATAFIX] Removing ambiguous 'outsourced' task %s from agent %s%s", ColorWarning, t.TaskID, agents[i].AgentID, ColorReset)
			}
		}
		agents[i].TasksToOutsource = cleanOut
	}

	for _, agent := range agents {
		if err := writeAgentToJSON(agent, *outputDir); err != nil {
			log.Printf("%s[ERROR] Failed to write agent file for %s: %v%s", ColorBugfix, agent.AgentID, err, ColorReset)
		}
	}
	log.Printf("%s[COMPLETE] Process finished successfully. Agent files written to: %s%s", ColorStats, *outputDir, ColorReset)
}

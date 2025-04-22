package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ANSI color codes for terminal output - white text on different backgrounds
const (
	ColorReset       = "\033[0m"
	ColorBugfix      = "\033[37;41m"    // White on Red
	ColorFeatures    = "\033[37;42m"    // White on Green
	ColorReliability = "\033[37;46m"    // White on Cyan
	ColorWarning     = "\033[37;43m"    // White on Yellow
	ColorInfo        = "\033[37;44m"    // White on Blue
	ColorStats       = "\033[37;45m"    // White on Purple
	ColorAppLogic    = "\033[37;43m"    // White on Yellow
	ColorServerLogic = "\033[37;47;30m" // Black on White (for visibility)
	ColorUIUX        = "\033[37;45m"    // White on Purple
)

// Ollama API Request/Response Structures
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

// Category Response from LLM
type CategoryResponse struct {
	CategoryIDs []int `json:"category_ids"`
}

// Task represents a software engineering task
type Task struct {
	TaskID         string   `json:"task_id"`
	Description    string   `json:"description"`
	Categories     []string `json:"categories"` // Multiple categories per task
	EstimatedHours int      `json:"estimated_hours"`
	RequiredSkills []string `json:"required_skills,omitempty"`
	AuctionStatus  string   `json:"auction_status,omitempty"`
	Status         string   `json:"status,omitempty"`
	Price          float64  `json:"price"`                 // Task price from CSV
	PriceLimit     float64  `json:"price_limit,omitempty"` // Optional price limit
}

// Agent represents a software engineering team member
type Agent struct {
	AgentID          string         `json:"agent_id"`
	Specialty        string         `json:"specialty"` // Primary specialty
	Skills           []string       `json:"skills"`    // Skills the agent wants to bid on
	SkillLevels      map[string]int `json:"skill_levels"`
	TasksToOutsource []Task         `json:"tasks_to_outsource"`
	OwnTasks         []Task         `json:"own_tasks"`
}

// Category definition with ID and name
type Category struct {
	ID   int
	Name string
}

// Available categories with numerical IDs
var availableCategories = []Category{
	{ID: 1, Name: "ApplicationLogic"},
	{ID: 2, Name: "ServerSideLogic"},
	{ID: 3, Name: "UI/UX"},
	{ID: 4, Name: "SystemQuality/Reliability"},
	{ID: 5, Name: "Bugfix"},
	{ID: 6, Name: "NewFeatures/Enhancement"},
	{ID: 7, Name: "ReliabilityImprovements"},
}

// For convenience, mapping of ID to name
var categoryIDToName = map[int]string{
	1: "ApplicationLogic",
	2: "ServerSideLogic",
	3: "UI/UX",
	4: "SystemQuality/Reliability",
	5: "Bugfix",
	6: "NewFeatures/Enhancement",
	7: "ReliabilityImprovements",
}

// Skills associated with each category
var categorySkills = map[string][]string{
	"ApplicationLogic":          {"State Management", "Client-Side Routing", "Form Handling", "Workflow Design", "Data Validation", "Algorithm Design", "Frontend Logic", "Client-Side Processing"},
	"ServerSideLogic":           {"API Endpoints", "Database Queries", "Authentication", "Authorization", "Data Processing", "Server Operations", "Backend Frameworks", "Middleware Functions", "Rate Limiting"},
	"UI/UX":                     {"Design Changes", "Layout", "Styling", "Interaction Improvements", "User Experience", "Accessibility", "UI Components", "Animation", "Visual Design"},
	"SystemQuality/Reliability": {"Code Refactoring", "Performance Optimization", "Architecture Design", "Code Quality", "System Design", "Technical Debt", "Dependency Management"},
	"Bugfix":                    {"Unexpected Behaviors", "Errors", "Inconsistencies", "Issue Reproduction", "Root Cause Analysis", "Hotfix Implementation", "Error Handling", "Regression Testing"},
	"NewFeatures/Enhancement":   {"New Functionality", "Feature Optimization", "Extending Capabilities", "User Workflows", "Product Enhancements", "Feature Implementation", "User Stories"},
	"ReliabilityImprovements":   {"Logging", "Monitoring", "Testing", "Error Handling", "Recovery Mechanisms", "System Stability", "Performance Monitoring", "Resilience Design"},
}

// Skill level ranges for agents (1-10 scale)
var skillLevelRanges = map[string]struct{ min, max int }{
	"primary":   {7, 10}, // Skills in agent's specialty
	"secondary": {3, 6},  // Skills in other categories
}

// Map categories to colors for logging
var categoryColors = map[string]string{
	"ApplicationLogic":          ColorAppLogic,
	"ServerSideLogic":           ColorServerLogic,
	"UI/UX":                     ColorUIUX,
	"SystemQuality/Reliability": ColorInfo,
	"Bugfix":                    ColorBugfix,
	"NewFeatures/Enhancement":   ColorFeatures,
	"ReliabilityImprovements":   ColorReliability,
}

// Get list of category IDs as string
func getCategoryIDsList() string {
	var ids []string
	for _, cat := range availableCategories {
		ids = append(ids, fmt.Sprintf("%d - %s", cat.ID, cat.Name))
	}
	return strings.Join(ids, "\n")
}

// Get category name from ID
func getCategoryNameByID(id int) string {
	if name, exists := categoryIDToName[id]; exists {
		return name
	}
	return fmt.Sprintf("Unknown Category ID: %d", id)
}

// Colorize text with given color
func colorize(text string, color string) string {
	return color + text + ColorReset
}

// Get color for a category
func colorForCategory(category string) string {
	if color, exists := categoryColors[category]; exists {
		return color
	}
	return ColorReset
}

// Extract actual task content from JSON-like text
func extractTaskContent(text string) string {
	// Handle the case where the description appears to be JSON with a 'content' field
	contentMatch := regexp.MustCompile(`'content':\s*'([^']+)'`).FindStringSubmatch(text)
	if len(contentMatch) > 1 {
		return contentMatch[1]
	}

	// Try another format variation
	contentMatch = regexp.MustCompile(`"content":\s*"([^"]+)"`).FindStringSubmatch(text)
	if len(contentMatch) > 1 {
		return contentMatch[1]
	}

	// If we can't extract the content, return the original text (cleaned a bit)
	return strings.Trim(text, "[]{}'\"`")
}

// Smart categorization of a task - tries local analysis first to reduce API calls
func categorizeTaskSmartly(taskDescription string, modelName string, ollamaURL string) ([]string, error) {
	// Try to extract clean description if the format is complex
	cleanDescription := extractTaskContent(taskDescription)
	cleanDescription = truncateString(cleanDescription, 1000) // Limit size to reduce API errors

	// First, try to categorize without API using keyword matching
	initialCategories := findClosestCategories(cleanDescription)

	// If we have high confidence categories from keyword matching, use them
	if len(initialCategories) >= 1 && hasStrongKeywordMatch(cleanDescription, initialCategories) {
		log.Printf("%sOPTIMIZATION: Using local keyword categorization for efficiency: %s%s",
			ColorInfo, strings.Join(initialCategories, ", "), ColorReset)
		return initialCategories, nil
	}

	// If local categorization wasn't confident, try API with retries
	return categorizeTaskWithOllama(cleanDescription, modelName, ollamaURL)
}

// Check if we have strong keyword matches
func hasStrongKeywordMatch(text string, categories []string) bool {
	text = strings.ToLower(text)
	strongMatches := 0

	// Define very clear signals for each category
	strongSignals := map[string][]string{
		"ApplicationLogic":          {"state management", "frontend logic", "client-side", "form validation"},
		"ServerSideLogic":           {"api endpoint", "database", "server", "authentication", "rate limit", "middleware"},
		"UI/UX":                     {"user interface", "ui component", "styling", "layout", "design", "user experience"},
		"SystemQuality/Reliability": {"refactor", "optimize performance", "architecture", "system design"},
		"Bugfix":                    {"fix bug", "resolve issue", "fix error", "debug", "incorrect behavior"},
		"NewFeatures/Enhancement":   {"new feature", "enhance", "add capability", "implement new"},
		"ReliabilityImprovements":   {"logging", "monitoring", "error handling", "test coverage", "resilience"},
	}

	// Check for strong signals
	for _, category := range categories {
		if signals, exists := strongSignals[category]; exists {
			for _, signal := range signals {
				if strings.Contains(text, signal) {
					strongMatches++
					break
				}
			}
		}
	}

	return strongMatches >= len(categories)/2 // At least half the categories have strong signals
}

// Function to categorize a task using Ollama with JSON schema for response
func categorizeTaskWithOllama(taskDescription string, modelName string, ollamaURL string) ([]string, error) {
	startTime := time.Now()
	log.Printf("%sCATEGORIZATION: Starting API categorization for task: %s%s",
		ColorInfo, truncateString(taskDescription, 50), ColorReset)

	// Construct a detailed prompt that helps the LLM understand the categorization task
	prompt := fmt.Sprintf(`You are a software engineering manager responsible for categorizing engineering tasks.
I will provide you with a task description, and you need to categorize it into one or more of the predefined categories.

Categories (ID - Name):
%s

For each category, here are examples of what types of tasks would fall under it:

- 1 (ApplicationLogic): State management, client-side routing, form handling
- 2 (ServerSideLogic): API endpoints, database queries, authentication, rate limiting
- 3 (UI/UX): Design changes, layout, styling, interaction improvements
- 4 (SystemQuality/Reliability): Code refactoring, performance optimization, architecture design
- 5 (Bugfix): Unexpected behaviors, errors, inconsistencies, issue reproduction
- 6 (NewFeatures/Enhancement): New functionality, feature optimization, extending capabilities
- 7 (ReliabilityImprovements): Logging, monitoring, testing, error handling, recovery mechanisms

Task Description: "%s"

A task can belong to multiple categories. Based on the task description, which categories best fit this task? 
Return your answer as a JSON object with the following schema:
{
  "category_ids": [int, int, ...]  // Array of category IDs as integers (valid values: 1-7)
}

Make sure your response is a valid JSON object that can be parsed. Only include category IDs from 1 to 7.
Do not include any other text or explanation outside of the JSON object.`, getCategoryIDsList(), taskDescription)

	// Prepare the Request Body
	requestPayload := OllamaRequest{
		Model:  modelName,
		Prompt: prompt,
		Stream: false,
	}
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		log.Printf("%sERROR: Failed to marshal request payload: %v%s", ColorBugfix, err, ColorReset)
		return findClosestCategories(taskDescription), nil
	}

	// Try with retries and shorter timeouts
	var ollamaResp OllamaResponse
	success := false
	maxRetries := 2
	timeoutSeconds := 3000 // Start with shorter timeout

	for retry := 0; retry <= maxRetries; retry++ {
		if retry > 0 {
			log.Printf("%sRETRY: Attempt %d of %d with %d second timeout%s",
				ColorWarning, retry, maxRetries, timeoutSeconds, ColorReset)
			timeoutSeconds = 20 // Even shorter timeout for retries
		}

		// Make the HTTP POST Request
		req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("%sERROR: Failed to create HTTP request: %v%s", ColorBugfix, err, ColorReset)
			continue // Try next retry
		}
		req.Header.Set("Content-Type", "application/json")

		log.Printf("%sAPI: Sending request to Ollama API (%s model) with timeout: %ds%s",
			ColorInfo, modelName, timeoutSeconds, ColorReset)
		client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("%sERROR: API request failed: %v%s", ColorBugfix, err, ColorReset)
			continue // Try next retry
		}

		// Read response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("%sERROR: Failed to read API response body: %v%s", ColorBugfix, err, ColorReset)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("%sERROR: Ollama API returned non-OK status: %s%s",
				ColorBugfix, resp.Status, ColorReset)
			continue
		}

		err = json.Unmarshal(body, &ollamaResp)
		if err != nil {
			log.Printf("%sERROR: Failed to unmarshal API response: %v%s", ColorBugfix, err, ColorReset)
			continue
		}

		success = true
		break
	}

	if !success {
		log.Printf("%sERROR: All API attempts failed, using fallback categorization%s", ColorBugfix, ColorReset)
		return findClosestCategories(taskDescription), nil
	}

	// Extract the JSON from the response and clean it (sometimes LLMs add extra text)
	jsonResponse := extractJSONFromString(ollamaResp.Response)

	// Parse the JSON response
	var categoryResp CategoryResponse

	err = json.Unmarshal([]byte(jsonResponse), &categoryResp)
	if err != nil {
		log.Printf("%sERROR: Failed to parse category JSON response: %v%s", ColorBugfix, err, ColorReset)
		// Try fallback heuristic categorization
		return findClosestCategories(taskDescription), nil
	}

	// Convert category IDs to names and validate
	var validCategoryNames []string
	for _, id := range categoryResp.CategoryIDs {
		if id < 1 || id > 7 {
			log.Printf("%sWARNING: Invalid category ID: %d (must be 1-7)%s", ColorWarning, id, ColorReset)
			continue
		}

		categoryName, valid := categoryIDToName[id]
		if valid {
			validCategoryNames = append(validCategoryNames, categoryName)
		} else {
			log.Printf("%sWARNING: Unknown category ID: %d%s", ColorWarning, id, ColorReset)
		}
	}

	// If no valid categories, use fallback
	if len(validCategoryNames) == 0 {
		fallbackCategories := findClosestCategories(taskDescription)
		log.Printf("%sRECOVERY: No valid categories found in JSON. Using fallback categories: %s%s",
			ColorWarning, strings.Join(fallbackCategories, ", "), ColorReset)
		validCategoryNames = fallbackCategories
	}

	// Color-code the log output
	coloredCategories := make([]string, len(validCategoryNames))
	for i, cat := range validCategoryNames {
		colorCode := colorForCategory(cat)
		coloredCategories[i] = colorize(cat, colorCode)
	}

	elapsedTime := time.Since(startTime)
	log.Printf("%sCATEGORIZATION: Completed in %.2f seconds. Final categories: %s%s",
		ColorInfo, elapsedTime.Seconds(), strings.Join(coloredCategories, ", "), ColorReset)

	return validCategoryNames, nil
}

// Extract valid JSON from a string that might contain other text
func extractJSONFromString(text string) string {
	// Find the first { and the last } to extract the JSON object
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		log.Printf("%sWARNING: Could not find valid JSON structure in response%s", ColorWarning, ColorReset)
		return "{}" // Return empty JSON object as fallback
	}

	return text[start : end+1]
}

// Truncate long strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Find closest categories by keyword matching (fallback)
func findClosestCategories(response string) []string {
	response = strings.ToLower(response)
	matchedCategories := make(map[string]bool)

	categoryKeywords := map[string][]string{
		"ApplicationLogic":          {"state", "client", "form", "workflow", "validation", "algorithm", "frontend", "logic"},
		"ServerSideLogic":           {"api", "database", "endpoint", "server", "backend", "authentication", "data processing", "query", "rate limit"},
		"UI/UX":                     {"ui", "ux", "interface", "design", "layout", "styling", "interaction", "user experience"},
		"SystemQuality/Reliability": {"refactor", "quality", "performance", "optimization", "architecture", "technical debt"},
		"Bugfix":                    {"bug", "fix", "issue", "error", "crash", "debug", "problem", "broken", "unexpected"},
		"NewFeatures/Enhancement":   {"feature", "new", "add", "enhance", "implement", "create", "improve", "functionality"},
		"ReliabilityImprovements":   {"logging", "monitoring", "testing", "reliability", "stability", "resilience"},
	}

	// Analyze the text context more deeply
	if strings.Contains(response, "rate limit") || strings.Contains(response, "api endpoint") {
		matchedCategories["ServerSideLogic"] = true
	}

	if strings.Contains(response, "user interface") || strings.Contains(response, "frontend") {
		matchedCategories["UI/UX"] = true
	}

	if strings.Contains(response, "fix") && (strings.Contains(response, "bug") || strings.Contains(response, "issue")) {
		matchedCategories["Bugfix"] = true
	}

	// Check each category's keywords against the response
	for category, keywords := range categoryKeywords {
		for _, keyword := range keywords {
			if strings.Contains(response, keyword) {
				matchedCategories[category] = true
				break // Found a match for this category, move to next category
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(matchedCategories))
	for category := range matchedCategories {
		result = append(result, category)
	}

	// If still no matches, return the most common categories
	if len(result) == 0 {
		return []string{"ServerSideLogic", "ReliabilityImprovements"}
	}

	return result
}

// TaskData represents a row in the CSV file
type TaskData struct {
	Description string
	Variant     string
	Price       float64
	PriceLimit  float64
	QuestionID  string
	OriginalRow int
}

// Clean task description by removing excessive spaces, normalizing text
func cleanTaskDescription(desc string) string {
	// Trim whitespace
	desc = strings.TrimSpace(desc)

	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	desc = spaceRegex.ReplaceAllString(desc, " ")

	// Remove quotes if they wrap the entire string
	if (strings.HasPrefix(desc, "\"") && strings.HasSuffix(desc, "\"")) ||
		(strings.HasPrefix(desc, "'") && strings.HasSuffix(desc, "'")) {
		desc = desc[1 : len(desc)-1]
	}

	return desc
}

// Function to read tasks from a CSV file, filtering for SWE Manager tasks from the variant column
func readTasksFromCSV(filepath string) ([]TaskData, error) {
	log.Printf("%sCSV: Opening file %s for reading%s", ColorInfo, filepath, ColorReset)
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("%sERROR: Failed to open CSV file: %v%s", ColorBugfix, err, ColorReset)
		return nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("%sERROR: Failed to read CSV content: %v%s", ColorBugfix, err, ColorReset)
		return nil, fmt.Errorf("error reading CSV file: %w", err)
	}

	log.Printf("%sCSV: Read %d total rows from file%s", ColorInfo, len(records), ColorReset)

	// Determine if there's a header by checking first row
	hasHeader := len(records) > 0

	// Skip header if present
	startRow := 0
	if hasHeader && len(records) > 0 {
		startRow = 1
		log.Printf("%sCSV: Detected header row, will start processing from row 2%s", ColorInfo, ColorReset)
	}

	// Map column names to indices
	columnMap := make(map[string]int)

	if hasHeader {
		header := records[0]
		for i, cell := range header {
			columnName := strings.ToLower(strings.TrimSpace(cell))
			columnMap[columnName] = i
		}

		log.Printf("%sCSV: Found headers: %v%s", ColorInfo, columnMap, ColorReset)
	}

	// Get column positions - use map or default positions
	questionIDCol := getColumnIndex(columnMap, "question_id", 0)
	variantCol := getColumnIndex(columnMap, "variant", 1)
	priceCol := getColumnIndex(columnMap, "price", 2)
	priceLimitCol := getColumnIndex(columnMap, "price_limit", 3)
	promptCol := getColumnIndex(columnMap, "prompt", 4)

	log.Printf("%sCSV: Using columns - question_id:%d, variant:%d, price:%d, price_limit:%d, prompt:%d%s",
		ColorInfo, questionIDCol+1, variantCol+1, priceCol+1, priceLimitCol+1, promptCol+1, ColorReset)

	var tasks []TaskData
	icCount := 0
	sweManagerCount := 0
	invalidRows := 0

	// Process all rows
	for i := startRow; i < len(records); i++ {
		record := records[i]

		// Skip rows that don't have enough columns
		maxCol := max(max(questionIDCol, variantCol), max(priceCol, promptCol))
		if len(record) <= maxCol {
			log.Printf("%sWARNING: Row %d has fewer columns than expected. Skipping.%s",
				ColorWarning, i+1, ColorReset)
			invalidRows++
			continue
		}

		questionID := strings.TrimSpace(record[questionIDCol])
		variant := strings.TrimSpace(record[variantCol])
		description := cleanTaskDescription(record[promptCol])

		// Parse price and price limit with fallbacks
		price := 0.0
		if priceCol < len(record) {
			price, _ = parseFloat(record[priceCol], 0.0)
		}

		priceLimit := 0.0
		if priceLimitCol < len(record) {
			priceLimit, _ = parseFloat(record[priceLimitCol], 0.0)
		}

		// Skip empty descriptions
		if description == "" {
			log.Printf("%sWARNING: Row %d has empty description. Skipping.%s",
				ColorWarning, i+1, ColorReset)
			invalidRows++
			continue
		}

		// Check if the variant indicates a SWE Manager task
		isSWEManagerTask := strings.Contains(strings.ToLower(variant), "manager") ||
			strings.Contains(strings.ToLower(variant), "swe_manager") ||
			strings.Contains(strings.ToLower(variant), "swe manager")

		isICSWETask := strings.Contains(strings.ToLower(variant), "ic_swe") ||
			strings.Contains(strings.ToLower(variant), "ic swe")

		// Count both types based on variant column
		if isSWEManagerTask {
			sweManagerCount++
			// Only collect SWE Manager tasks for processing
			tasks = append(tasks, TaskData{
				QuestionID:  questionID,
				Description: description,
				Variant:     "SWE Manager",
				Price:       price,
				PriceLimit:  priceLimit,
				OriginalRow: i + 1,
			})

			log.Printf("ROW %d: SWE Manager Task - ID: %s, Price: $%.2f",
				i+1, questionID, price)
		} else if isICSWETask {
			icCount++
			// We count IC SWE tasks but don't collect them for processing
		} else {
			log.Printf("%sWARNING: Row %d has unknown variant '%s'. Skipping.%s",
				ColorWarning, i+1, variant, ColorReset)
			invalidRows++
		}
	}

	log.Printf("%sCSV: Processed %d rows: %d SWE Manager tasks, %d IC SWE tasks, %d invalid rows%s",
		ColorStats, len(records)-startRow, sweManagerCount, icCount, invalidRows, ColorReset)

	return tasks, nil
}

// Get column index from map or use default
func getColumnIndex(columnMap map[string]int, colName string, defaultIndex int) int {
	if idx, exists := columnMap[colName]; exists {
		return idx
	}
	return defaultIndex
}

// Parse float with default value
func parseFloat(str string, defaultVal float64) (float64, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return defaultVal, nil
	}

	// Remove any currency symbols or commas
	str = regexp.MustCompile(`[$,]`).ReplaceAllString(str, "")

	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return defaultVal, err
	}
	return val, nil
}

// Utility function for max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Utility function for min of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Generate skills for a task based on its categories
func getSkillsForTask(categories []string) []string {
	// Collect skills from all categories
	skillsMap := make(map[string]bool)

	for _, category := range categories {
		if skills, exists := categorySkills[category]; exists {
			// Add 2-3 skills from each category
			numSkills := rand.Intn(2) + 2 // 2-3 skills
			if numSkills > len(skills) {
				numSkills = len(skills)
			}

			for _, i := range rand.Perm(len(skills))[:numSkills] {
				skillsMap[skills[i]] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(skillsMap))
	for skill := range skillsMap {
		result = append(result, skill)
	}

	// If no skills were found, return a generic skill
	if len(result) == 0 {
		return []string{"General Technical Skills"}
	}

	return result
}

// Generate comprehensive skill profile for an agent based on their specialty
func generateAgentSkills(specialty string) ([]string, map[string]int) {
	allSkills := make(map[string]bool)
	skillLevels := make(map[string]int)

	// Add all skills from agent's specialty with high proficiency
	for _, skill := range categorySkills[specialty] {
		allSkills[skill] = true
		// Primary skills have higher proficiency (7-10)
		skillLevels[skill] = rand.Intn(skillLevelRanges["primary"].max-skillLevelRanges["primary"].min+1) +
			skillLevelRanges["primary"].min
	}

	// Add some skills from other categories with lower proficiency
	for cat, skills := range categorySkills {
		if cat != specialty {
			// Add 1-2 skills from other categories
			numSkills := rand.Intn(2) + 1
			if numSkills > len(skills) {
				numSkills = len(skills)
			}

			for _, i := range rand.Perm(len(skills))[:numSkills] {
				skill := skills[i]
				if !allSkills[skill] {
					allSkills[skill] = true
					// Secondary skills have lower proficiency (3-6)
					skillLevels[skill] = rand.Intn(skillLevelRanges["secondary"].max-skillLevelRanges["secondary"].min+1) +
						skillLevelRanges["secondary"].min
				}
			}
		}
	}

	// Convert map to slice
	skillsList := make([]string, 0, len(allSkills))
	for skill := range allSkills {
		skillsList = append(skillsList, skill)
	}

	return skillsList, skillLevels
}

// Generate random hours estimate based on price (higher price = more hours)
func getEstimatedHours(price float64) int {
	// Base hours (minimum 2)
	baseHours := 2

	// Calculate additional hours based on price tiers
	if price > 50000 {
		return baseHours + rand.Intn(15) + 25 // 27-42 hours for premium tasks
	} else if price > 20000 {
		return baseHours + rand.Intn(10) + 15 // 17-27 hours for high-value tasks
	} else if price > 10000 {
		return baseHours + rand.Intn(8) + 10 // 12-20 hours for medium-high tasks
	} else if price > 5000 {
		return baseHours + rand.Intn(6) + 6 // 8-14 hours for medium tasks
	} else if price > 2000 {
		return baseHours + rand.Intn(4) + 4 // 6-10 hours for medium-low tasks
	} else if price > 1000 {
		return baseHours + rand.Intn(3) + 2 // 4-7 hours for low-medium tasks
	} else {
		return baseHours + rand.Intn(3) // 2-5 hours for low-price tasks
	}
}

// Create agents with tasks distributed based on their specialties
func createAgents(tasks []Task) []Agent {
	log.Printf("%sAGENTS: Creating %d agent specialists%s",
		ColorInfo, len(availableCategories), ColorReset)

	// Create an agent for each category
	agents := make([]Agent, 0, len(availableCategories))

	for _, category := range availableCategories {
		// Generate skills profile for this agent
		skills, skillLevels := generateAgentSkills(category.Name)

		agent := Agent{
			AgentID:     fmt.Sprintf("agent%d", category.ID), // Numbering matches category ID
			Specialty:   category.Name,
			Skills:      skills,
			SkillLevels: skillLevels,
		}
		agents = append(agents, agent)
		categoryColor := colorForCategory(category.Name)
		log.Printf("AGENTS: Created agent%d with specialty %s%s%s and %d skills",
			category.ID, categoryColor, category.Name, ColorReset, len(skills))
	}

	log.Printf("%sDISTRIBUTION: Starting task distribution among %d agents%s",
		ColorInfo, len(agents), ColorReset)

	// Distribution statistics
	ownTaskCounts := make(map[string]int)
	outsourceCounts := make(map[string]int)

	// Distribute tasks to agents
	for _, task := range tasks {
		// For each task, determine which agents' specialties match any of the task categories
		for i := range agents {
			// Check if agent's specialty matches any of the task categories
			isMatchingSpecialty := false
			for _, category := range task.Categories {
				if agents[i].Specialty == category {
					isMatchingSpecialty = true
					break
				}
			}

			if isMatchingSpecialty {
				// This is a task the agent can do themselves
				task.Status = "assigned"
				agents[i].OwnTasks = append(agents[i].OwnTasks, task)
				ownTaskCounts[agents[i].Specialty]++
			} else {
				// This is a task the agent would outsource - simplify the outsourced task
				taskCopy := Task{
					TaskID:         task.TaskID,
					Description:    task.Description,
					Categories:     task.Categories,
					EstimatedHours: task.EstimatedHours,
					RequiredSkills: task.RequiredSkills,
					AuctionStatus:  "open",
					Price:          task.Price,
					PriceLimit:     task.PriceLimit,
				}

				// Only add to agents that don't have this specialty, and limit duplicates
				if rand.Float32() < 0.15 { // 15% chance to be assigned to this non-matching agent
					agents[i].TasksToOutsource = append(agents[i].TasksToOutsource, taskCopy)
					outsourceCounts[agents[i].Specialty]++
				}
			}
		}
	}

	// Log distribution statistics
	log.Printf("%sDISTRIBUTION: Task distribution complete. Results:%s", ColorStats, ColorReset)
	for _, agent := range agents {
		categoryColor := colorForCategory(agent.Specialty)
		log.Printf("AGENT %s: %s%s%s - %d own tasks, %d tasks to outsource",
			agent.AgentID,
			categoryColor, agent.Specialty, ColorReset,
			len(agent.OwnTasks), len(agent.TasksToOutsource))
	}

	return agents
}

// Write agents to JSON files
func writeAgentToJSON(agent Agent) error {
	filename := fmt.Sprintf("%s.json", agent.AgentID)
	categoryColor := colorForCategory(agent.Specialty)
	log.Printf("%sFILE: Writing agent data to %s (specialty: %s%s%s)%s",
		ColorInfo, filename, categoryColor, agent.Specialty, ColorReset, ColorReset)

	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		log.Printf("%sERROR: Failed to marshal agent data: %v%s", ColorBugfix, err, ColorReset)
		return fmt.Errorf("error marshalling agent data: %w", err)
	}

	log.Printf("FILE: Agent JSON data size: %d bytes", len(data))

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("%sERROR: Failed to write agent file: %v%s", ColorBugfix, err, ColorReset)
		return fmt.Errorf("error writing agent file: %w", err)
	}

	log.Printf("FILE: Successfully wrote agent data to %s", filename)
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("%sSTARTUP: SWE Manager Task Distribution System initializing%s", ColorStats, ColorReset)

	// Get current time for random seed
	now := time.Now()
	rand.Seed(now.UnixNano())
	log.Printf("STARTUP: Random seed initialized with timestamp")

	// Configuration
	ollamaAPIURL := "http://localhost:11434/api/generate"
	modelToUse := "granite3.3:8b"
	inputFilePath := "data.csv"

	log.Printf("%sCONFIG: Using Ollama API at %s with model %s%s",
		ColorInfo, ollamaAPIURL, modelToUse, ColorReset)
	log.Printf("CONFIG: Reading task data from %s", inputFilePath)

	// Read and filter tasks from CSV
	taskData, err := readTasksFromCSV(inputFilePath)
	if err != nil {
		log.Fatalf("%sFATAL: Failed to read tasks from CSV: %v%s", ColorBugfix, err, ColorReset)
	}

	if len(taskData) == 0 {
		log.Fatalf("%sFATAL: No SWE Manager tasks found in the CSV file%s", ColorBugfix, ColorReset)
	}

	log.Printf("%sTASKS: Successfully loaded %d SWE Manager tasks for processing%s",
		ColorStats, len(taskData), ColorReset)

	// Process and categorize each task
	var categorizedTasks []Task
	totalCategorizationTime := 0.0
	log.Printf("%sCATEGORIZATION: Starting task categorization using %s model%s",
		ColorInfo, modelToUse, ColorReset)

	// Process tasks in batches to reduce memory pressure
	batchSize := 10
	for i := 0; i < len(taskData); i += batchSize {
		endIdx := min(i+batchSize, len(taskData))
		log.Printf("%sBATCH: Processing tasks %d to %d of %d%s",
			ColorInfo, i+1, endIdx, len(taskData), ColorReset)

		// Process current batch
		for j := i; j < endIdx; j++ {
			task := taskData[j]
			startTime := time.Now()
			log.Printf("TASK %d/%d: %s (Price: $%.2f)",
				j+1, len(taskData), truncateString(task.Description, 50), task.Price)

			// Use smart categorization to reduce API calls
			categories, err := categorizeTaskSmartly(task.Description, modelToUse, ollamaAPIURL)
			taskTime := time.Since(startTime).Seconds()
			totalCategorizationTime += taskTime

			if err != nil {
				log.Printf("%sERROR: Failed to categorize task %d: %v%s", ColorBugfix, j+1, err, ColorReset)
				// Fallback based on context
				taskText := strings.ToLower(task.Description)
				if strings.Contains(taskText, "api") || strings.Contains(taskText, "server") {
					categories = []string{"ServerSideLogic", "ReliabilityImprovements"}
				} else if strings.Contains(taskText, "user") || strings.Contains(taskText, "interface") {
					categories = []string{"UI/UX"}
				} else if strings.Contains(taskText, "bug") || strings.Contains(taskText, "issue") {
					categories = []string{"Bugfix"}
				} else {
					categories = []string{"ApplicationLogic"}
				}

				categoryColors := make([]string, len(categories))
				for k, cat := range categories {
					catColor := colorForCategory(cat)
					categoryColors[k] = colorize(cat, catColor)
				}

				log.Printf("%sRECOVERY: Using context-based categories: %s%s",
					ColorWarning, strings.Join(categoryColors, ", "), ColorReset)
			}

			// Create a new task with the determined categories
			estimatedHours := getEstimatedHours(task.Price)

			newTask := Task{
				TaskID:         fmt.Sprintf("task-%03d", j+1),
				Description:    extractTaskContent(task.Description), // Extract clean content
				Categories:     categories,
				EstimatedHours: estimatedHours,
				RequiredSkills: getSkillsForTask(categories),
				Status:         "",
				Price:          task.Price,
				PriceLimit:     task.PriceLimit,
			}

			categorizedTasks = append(categorizedTasks, newTask)
		}

		// Small pause between batches
		if endIdx < len(taskData) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	avgCatTime := totalCategorizationTime / float64(len(taskData))
	log.Printf("%sCATEGORIZATION: Complete - %d tasks categorized in %.2f seconds (avg %.2f sec/task)%s",
		ColorStats, len(categorizedTasks), totalCategorizationTime, avgCatTime, ColorReset)

	// Count tasks per category
	catCounts := make(map[string]int)
	for _, task := range categorizedTasks {
		for _, category := range task.Categories {
			catCounts[category]++
		}
	}

	log.Printf("%sSTATS: Category Distribution Summary%s", ColorStats, ColorReset)
	for cat, count := range catCounts {
		percentage := float64(count) / float64(len(categorizedTasks)) * 100
		categoryColor := colorForCategory(cat)
		log.Printf("STATS: Category %s%s%s: %d tasks (%.1f%%)",
			categoryColor, cat, ColorReset, count, percentage)
	}

	// Create agents and distribute tasks
	log.Printf("%sDISTRIBUTION: Creating agents and distributing %d tasks%s",
		ColorInfo, len(categorizedTasks), ColorReset)
	agents := createAgents(categorizedTasks)

	// Write agent data to JSON files
	log.Printf("%sOUTPUT: Writing agent data to JSON files%s", ColorInfo, ColorReset)
	for _, agent := range agents {
		err := writeAgentToJSON(agent)
		if err != nil {
			log.Printf("%sERROR: Failed to write agent file for %s: %v%s",
				ColorBugfix, agent.AgentID, err, ColorReset)
		}
	}

	log.Printf("%sCOMPLETE: Task categorization and distribution process finished successfully%s",
		ColorStats, ColorReset)
}

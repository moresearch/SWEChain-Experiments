package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Specialty string `json:"specialty"`
	Group     string `json:"group"`
}
type Link struct {
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	Type      string  `json:"type"`
	Amount    float64 `json:"amount,omitempty"`
	Reasoning string  `json:"reasoning,omitempty"`
}
type Graph struct {
	Nodes   []Node        `json:"nodes"`
	Links   []Link        `json:"links"`
	Metrics *MarketMetric `json:"metrics,omitempty"`
}
type Agent struct {
	AgentID          string             `json:"agent_id"`
	Specialty        string             `json:"specialty"`
	Skills           []string           `json:"skills"`
	SkillLevels      map[string]float64 `json:"skill_levels"`
	TasksToOutsource []struct {
		TaskID         string   `json:"task_id"`
		Description    string   `json:"description"`
		Categories     []string `json:"categories"`
		RequiredSkills []string `json:"required_skills"`
		AuctionStatus  string   `json:"auction_status"`
		Price          float64  `json:"price"`
		PriceLimit     float64  `json:"price_limit"`
	} `json:"tasks_to_outsource"`
	OwnTasks []struct {
		TaskID         string   `json:"task_id"`
		Description    string   `json:"description"`
		Categories     []string `json:"categories"`
		RequiredSkills []string `json:"required_skills"`
		Status         string   `json:"status"`
		Price          float64  `json:"price"`
		PriceLimit     float64  `json:"price_limit"`
	} `json:"own_tasks"`
}
type TaskInfo struct {
	TaskID         string
	Description    string
	Categories     []string
	RequiredSkills []string
	Price          float64
	PriceLimit     float64
	Auctioneer     string
}

// MarketMetric holds aggregated market information/statistics.
type MarketMetric struct {
	NumAgents     int     `json:"num_agents"`
	NumTasks      int     `json:"num_tasks"`
	NumBids       int     `json:"num_bids"`
	NumOutsourced int     `json:"num_outsourced"`
	AvgBid        float64 `json:"avg_bid,omitempty"`
	MinBid        float64 `json:"min_bid,omitempty"`
	MaxBid        float64 `json:"max_bid,omitempty"`
	AvgPrice      float64 `json:"avg_price,omitempty"`
	MinPrice      float64 `json:"min_price,omitempty"`
	MaxPrice      float64 `json:"max_price,omitempty"`
}

func loadAgents() []Agent {
	files, err := filepath.Glob("data/agent*.json")
	if err != nil {
		log.Fatalf("[FATAL] Filepath glob error: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("[FATAL] No agent JSON files found in ./data/")
	}
	log.Printf("[DATA] Found %d agent files: %v", len(files), files)

	agents := make([]Agent, 0)
	for _, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			log.Printf("[ERROR] Reading %s: %v", fname, err)
			continue
		}
		var agent Agent
		if err := json.Unmarshal(data, &agent); err != nil {
			log.Printf("[ERROR] Parsing %s: %v", fname, err)
			continue
		}
		agents = append(agents, agent)
	}
	return agents
}

func gatherTasks(agents []Agent) map[string]*TaskInfo {
	taskMap := make(map[string]*TaskInfo)
	for _, agent := range agents {
		for _, t := range agent.OwnTasks {
			taskMap[t.TaskID] = &TaskInfo{
				TaskID:         t.TaskID,
				Description:    t.Description,
				Categories:     t.Categories,
				RequiredSkills: t.RequiredSkills,
				Price:          t.Price,
				PriceLimit:     t.PriceLimit,
				Auctioneer:     agent.AgentID,
			}
		}
		for _, t := range agent.TasksToOutsource {
			if _, exists := taskMap[t.TaskID]; !exists {
				taskMap[t.TaskID] = &TaskInfo{
					TaskID:         t.TaskID,
					Description:    t.Description,
					Categories:     t.Categories,
					RequiredSkills: t.RequiredSkills,
					Price:          t.Price,
					PriceLimit:     t.PriceLimit,
					Auctioneer:     agent.AgentID,
				}
			}
		}
	}
	return taskMap
}

func buildNodes(agents []Agent, taskMap map[string]*TaskInfo, links []Link) []Node {
	nodeMap := make(map[string]Node)
	for _, agent := range agents {
		nodeMap[agent.AgentID] = Node{
			ID:        agent.AgentID,
			Name:      agent.Specialty,
			Role:      "agent",
			Specialty: agent.Specialty,
			Group:     "agent",
		}
	}
	for _, task := range taskMap {
		nodeMap[task.TaskID] = Node{
			ID:    task.TaskID,
			Name:  task.TaskID,
			Role:  "task",
			Group: "task",
		}
	}
	// Do NOT add "outsourced" as a node
	nodes := make([]Node, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

func buildPrompt(agents []Agent, taskMap map[string]*TaskInfo) string {
	agentsJSON, _ := json.MarshalIndent(agents, "", "  ")
	tasks := make([]map[string]interface{}, 0, len(taskMap))
	for _, t := range taskMap {
		tasks = append(tasks, map[string]interface{}{
			"task_id":         t.TaskID,
			"description":     t.Description,
			"categories":      t.Categories,
			"required_skills": t.RequiredSkills,
			"price":           t.Price,
			"price_limit":     t.PriceLimit,
			"auctioneer":      t.Auctioneer,
		})
	}
	tasksJSON, _ := json.MarshalIndent(tasks, "", "  ")

	return `
ONLY output a JSON array of links as in the example below.
Do NOT output any explanation or extra text.

[
  {"source": "agent5", "target": "task-002", "type": "outsourced"},
  {"source": "agent2", "target": "task-006", "type": "bidded"}
]

Rules:
- Do not create a node for "outsourced". It is only a link type, not a node.
- For each task, an agent may have a link of type "bidded" to the task if they bid on it.
- If an agent outsources a task, create a link of type "outsourced" from that agent to the task.
- Only agents whose specialty matches the required skills or categories of the task may bid.
- The network should have a scale-free degree distribution (preferential attachment: some agents bid more often).
- Output must be a single JSON array of links with no extra commentary.

Agents:
` + string(agentsJSON) + `

Tasks:
` + string(tasksJSON) + `
`
}

const (
	LLMModel    = "granite3.3:8b"
	LLMEndpoint = "http://localhost:11434/api/generate"
)

func callLLM(model, endpoint, prompt string) (string, error) {
	payload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("LLM request build failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 12000 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return string(raw), fmt.Errorf("LLM JSON parse failed: %w\nRaw: %s", err, string(raw))
	}
	return r.Response, nil
}

// Calculate market metrics (bids, outsourced, price, bid distributions)
func calcMarketMetrics(agents []Agent, tasks map[string]*TaskInfo, links []Link) *MarketMetric {
	agentSet := make(map[string]struct{})
	taskSet := make(map[string]struct{})
	var bidAmounts []float64
	var prices []float64
	numBids := 0
	numOutsourced := 0

	for _, agent := range agents {
		agentSet[agent.AgentID] = struct{}{}
	}
	for _, t := range tasks {
		taskSet[t.TaskID] = struct{}{}
		prices = append(prices, t.Price)
	}

	for _, link := range links {
		if link.Type == "bidded" {
			numBids++
			if link.Amount > 0 {
				bidAmounts = append(bidAmounts, link.Amount)
			}
		}
		if link.Type == "outsourced" {
			numOutsourced++
		}
	}

	avgBid, minBid, maxBid := float64(0), float64(0), float64(0)
	if len(bidAmounts) > 0 {
		sum := 0.0
		minBid = math.MaxFloat64
		maxBid = -math.MaxFloat64
		for _, b := range bidAmounts {
			sum += b
			if b < minBid {
				minBid = b
			}
			if b > maxBid {
				maxBid = b
			}
		}
		avgBid = sum / float64(len(bidAmounts))
	}
	avgPrice, minPrice, maxPrice := float64(0), float64(0), float64(0)
	if len(prices) > 0 {
		sum := 0.0
		minPrice = math.MaxFloat64
		maxPrice = -math.MaxFloat64
		for _, p := range prices {
			sum += p
			if p < minPrice {
				minPrice = p
			}
			if p > maxPrice {
				maxPrice = p
			}
		}
		avgPrice = sum / float64(len(prices))
	}

	return &MarketMetric{
		NumAgents:     len(agentSet),
		NumTasks:      len(taskSet),
		NumBids:       numBids,
		NumOutsourced: numOutsourced,
		AvgBid:        avgBid,
		MinBid:        minBid,
		MaxBid:        maxBid,
		AvgPrice:      avgPrice,
		MinPrice:      minPrice,
		MaxPrice:      maxPrice,
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	outputFile := "./data/baseline_network.json"
	log.Printf("[BOOT] Ensuring scale-free, specialty-driven economic network using LLM.")

	agents := loadAgents()
	taskMap := gatherTasks(agents)
	prompt := buildPrompt(agents, taskMap)

	log.Printf("[PROMPT] Sending prompt to LLM (%d agents, %d tasks)", len(agents), len(taskMap))
	response, err := callLLM(LLMModel, LLMEndpoint, prompt)
	if err != nil {
		log.Fatalf("[FATAL] LLM call failed: %v", err)
	}
	log.Printf("[LLM] Raw response received (%d bytes)", len(response))

	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 {
		log.Fatalf("[FATAL] No JSON array found in LLM response:\n%s", response)
	}
	linksJSON := response[start : end+1]
	var links []Link
	if err := json.Unmarshal([]byte(linksJSON), &links); err != nil {
		log.Fatalf("[FATAL] Failed to parse links from LLM output: %v\n%s", err, linksJSON)
	}

	nodes := buildNodes(agents, taskMap, links)
	metrics := calcMarketMetrics(agents, taskMap, links)

	g := Graph{Nodes: nodes, Links: links, Metrics: metrics}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		log.Fatalf("[FATAL] Failed to create output dir: %v", err)
	}
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("[FATAL] Failed to create %s: %v", outputFile, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		log.Fatalf("[FATAL] Failed to write %s: %v", outputFile, err)
	}
	log.Printf("[SUCCESS] Generated %s with %d nodes, %d links, metrics: %+v", outputFile, len(nodes), len(links), metrics)
}

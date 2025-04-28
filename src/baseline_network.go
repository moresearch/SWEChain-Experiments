package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Speciality struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type AgentFile struct {
	AgentID      string        `json:"agent_id"`
	DisplayName  string        `json:"display_name"`
	Group        string        `json:"group"`
	Avatar       string        `json:"avatar"`
	Specialities []Speciality  `json:"specialities"`
	Tasks        []TaskSummary `json:"tasks"`
}

type TaskSummary struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Speciality  string  `json:"speciality"`
	PriceMin    float64 `json:"price_min"`
	PriceMax    float64 `json:"price_max"`
}

type Network struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Label        string          `json:"label"`
	Group        string          `json:"group,omitempty"`
	Avatar       string          `json:"avatar,omitempty"`
	Specialities []Speciality    `json:"specialities,omitempty"`
	Speciality   *TaskSpeciality `json:"speciality,omitempty"`
	PriceMin     float64         `json:"price_min,omitempty"`
	PriceMax     float64         `json:"price_max,omitempty"`
	Desc         string          `json:"desc,omitempty"`
}

type TaskSpeciality struct {
	Name string `json:"name"`
}

type Edge struct {
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	Type      string  `json:"type"` // "auction", "bid", "assigned"
	BidValue  float64 `json:"bid_value,omitempty"`
	Reasoning string  `json:"reasoning,omitempty"`
}

// Load all agents from JSON files
func loadAgents(folderPath string) ([]AgentFile, error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}

	var agents []AgentFile
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			path := fmt.Sprintf("%s/%s", folderPath, file.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var agent AgentFile
			if err := json.Unmarshal(data, &agent); err != nil {
				return nil, err
			}
			agents = append(agents, agent)
		}
	}
	return agents, nil
}

// Build network using pairwise agent interactions
func buildNetworkFromPairs(agents []AgentFile, ollamaURL string) ([]Node, []Edge) {
	nodes := []Node{}
	edges := []Edge{}
	nodeSet := make(map[string]Node)
	edgeSet := make(map[string]bool)

	// Basic agent nodes
	for _, agent := range agents {
		n := Node{
			ID:           agent.AgentID,
			Type:         "agent",
			Label:        agent.DisplayName,
			Group:        agent.Group,
			Avatar:       agent.Avatar,
			Specialities: agent.Specialities,
		}
		nodes = append(nodes, n)
		nodeSet[agent.AgentID] = n
	}

	// Agent pairs
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			agentA := agents[i]
			agentB := agents[j]

			prompt := createAgentPairPrompt(agentA, agentB)
			response, err := safeCallOllama(prompt, ollamaURL)
			if err != nil {
				log.Printf("Skipping agent pair %s-%s due to LLM error: %v", agentA.AgentID, agentB.AgentID, err)
				continue
			}

			newEdges, newNodes := parseLLMPairResponse(response)
			for _, n := range newNodes {
				if _, exists := nodeSet[n.ID]; !exists {
					nodes = append(nodes, n)
					nodeSet[n.ID] = n
				}
			}
			for _, e := range newEdges {
				key := fmt.Sprintf("%s->%s:%s", e.Source, e.Target, e.Type)
				if !edgeSet[key] {
					edges = append(edges, e)
					edgeSet[key] = true
				}
			}
		}
	}

	return nodes, edges
}

// Create prompt for 2 agents
func createAgentPairPrompt(agentA, agentB AgentFile) string {
	formatSpecialities := func(specs []Speciality) string {
		var b strings.Builder
		for _, s := range specs {
			fmt.Fprintf(&b, "- %s (Weight: %.2f)\n", s.Name, s.Weight)
		}
		return b.String()
	}

	return fmt.Sprintf(`
You are helping simulate a task auction network.

Agent A:
- ID: %s
- Name: %s
- Specialities:
%s

Agent B:
- ID: %s
- Name: %s
- Specialities:
%s

Instructions:
1. Propose if either agent would auction a task based on their skills.
2. Let the other agent bid on tasks if relevant.
3. Decide who wins each bid.
4. Output in strict JSON only:
{
  "tasks": [ { "id": "task_id", "desc": "desc", "speciality": { "name": "speciality" }, "price_min": 100, "price_max": 200 } ],
  "edges": [ { "source": "agent_id", "target": "task_id", "type": "auction" }, { "source": "agent_id", "target": "task_id", "type": "bid", "bid_value": 150, "reasoning": "reason" }, { "source": "task_id", "target": "agent_id", "type": "assigned" } ]
}
No extra text, only JSON!
`, agentA.AgentID, agentA.DisplayName, formatSpecialities(agentA.Specialities),
		agentB.AgentID, agentB.DisplayName, formatSpecialities(agentB.Specialities))
}

// Parse LLM JSON response
func parseLLMPairResponse(response string) ([]Edge, []Node) {
	// First, log the raw LLM response
	log.Println("LLM Raw Response: ", response)

	// First, unmarshal the raw string into a structure
	var result struct {
		Tasks []struct {
			ID         string `json:"id"`
			Desc       string `json:"desc"`
			Speciality struct {
				Name string `json:"name"`
			} `json:"speciality"`
			PriceMin float64 `json:"price_min"`
			PriceMax float64 `json:"price_max"`
		} `json:"tasks"`
		Edges []Edge `json:"edges"`
	}

	// First unmarshal the outer structure
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Printf("Failed to parse outer LLM JSON: %v", err)
		log.Println("Response body (raw):", response) // Log the full response for debugging
		return nil, nil
	}

	nodes := []Node{}
	for _, t := range result.Tasks {
		nodes = append(nodes, Node{
			ID:         t.ID,
			Type:       "issue",
			Label:      t.ID,
			Desc:       t.Desc,
			Speciality: &TaskSpeciality{Name: t.Speciality.Name},
			PriceMin:   t.PriceMin,
			PriceMax:   t.PriceMax,
		})
	}
	return result.Edges, nodes
}

// LLM call wrapper with retry
func safeCallOllama(prompt, ollamaURL string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := callOllamaLLM(prompt, ollamaURL)
		if err == nil && strings.TrimSpace(response) != "" {
			return response, nil
		}
		lastErr = err
		log.Printf("LLM call failed (attempt %d/%d): %v", attempt, maxRetries, err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	return "", fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// Actual call to Ollama server
func callOllamaLLM(prompt, ollamaURL string) (string, error) {
	payload := map[string]interface{}{
		"model":  "cogito:8b",
		"prompt": prompt,
		"stream": false,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Raw Response Body: %s", body) // Log the raw response

	var response struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	// Check if the response is in the expected format
	if !strings.HasPrefix(strings.TrimSpace(response.Response), "{") {
		return "", fmt.Errorf("Unexpected LLM output: %s", response.Response)
	}

	return response.Response, nil
}

// Save network
func saveNetwork(outputPath string, network Network) error {
	data, err := json.MarshalIndent(network, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// Main
func main() {
	agentsPath := "./data/agents" // Folder where agents are stored
	ollamaURL := "http://localhost:11434/api/generate"
	outputPath := "./data/baseline_network.json"

	agents, err := loadAgents(agentsPath)
	if err != nil {
		log.Fatalf("Error loading agents: %v", err)
	}

	nodes, edges := buildNetworkFromPairs(agents, ollamaURL)

	network := Network{Nodes: nodes, Edges: edges}

	if err := saveNetwork(outputPath, network); err != nil {
		log.Fatalf("Error saving network: %v", err)
	}

	log.Println("✅ Network generated and saved to", outputPath)
}

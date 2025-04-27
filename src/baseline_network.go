package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// -------- Data Types --------

type Speciality struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type TaskBrief struct {
	TaskID    string  `json:"task_id"`
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

type Node struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Label        string                 `json:"label,omitempty"`
	Group        string                 `json:"group,omitempty"`
	Specialities []Speciality           `json:"specialities,omitempty"`
	Avatar       string                 `json:"avatar,omitempty"`
	Desc         string                 `json:"desc,omitempty"`
	Specialty    string                 `json:"specialty,omitempty"`
	PriceMin     float64                `json:"price_min,omitempty"`
	PriceMax     float64                `json:"price_max,omitempty"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Priority     string                 `json:"priority,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type Edge struct {
	Source   string                 `json:"source"`
	Target   string                 `json:"target"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight,omitempty"`
	Directed bool                   `json:"directed,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type EconomicNetwork struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

// -------- Helpers --------

type AgentIndex struct {
	ID         string   `json:"id"`
	Speciality []string `json:"speciality"`
	Degree     int      `json:"degree"`
}

type TaskIndex struct {
	TaskID    string `json:"task_id"`
	Specialty string `json:"specialty"`
	Owner     string `json:"owner"`
}

func buildGlobalIndex(agentFiles []string, nodes map[string]Node, edges []Edge) ([]AgentIndex, []TaskIndex) {
	agentDegrees := map[string]int{}
	for _, edge := range edges {
		if edge.Source != "" {
			agentDegrees[edge.Source]++
		}
		if edge.Target != "" {
			agentDegrees[edge.Target]++
		}
	}
	var agentIndexes []AgentIndex
	var taskIndexes []TaskIndex
	for _, afile := range agentFiles {
		b, err := ioutil.ReadFile(afile)
		if err != nil {
			log.Printf("[index] Failed to read %s: %v", afile, err)
			continue
		}
		var agent AgentSchema
		if err := json.Unmarshal(b, &agent); err != nil {
			log.Printf("[index] Failed to parse %s: %v", afile, err)
			continue
		}
		snames := []string{}
		for _, s := range agent.Specialities {
			snames = append(snames, s.Name)
		}
		agentIndexes = append(agentIndexes, AgentIndex{
			ID:         agent.AgentID,
			Speciality: snames,
			Degree:     agentDegrees[agent.AgentID],
		})
		for _, t := range agent.Tasks {
			taskIndexes = append(taskIndexes, TaskIndex{
				TaskID:    t.TaskID,
				Specialty: t.Specialty,
				Owner:     agent.AgentID,
			})
		}
	}
	return agentIndexes, taskIndexes
}

func stripThinkBlocks(text string) string {
	re := regexp.MustCompile(`(?is)<think>.*?</think>`)
	return re.ReplaceAllString(text, "")
}

func stripCodeBlocks(text string) string {
	re := regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)\\s*```")
	if match := re.FindStringSubmatch(text); len(match) > 1 {
		return match[1]
	}
	return strings.TrimSpace(text)
}

func cleanOutputForJSON(text string) string {
	text = stripThinkBlocks(text)
	text = stripCodeBlocks(text)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "` \n\r\t")
	return text
}

func askOllama(prompt, ollamaUrl, model, agentID string) (string, error) {
	log.Printf("[LLM][%s] ----- Prompt Begin -----\n%s\n----- Prompt End -----\n", agentID, prompt)

	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
	}
	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ollamaUrl, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var builder strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg struct {
			Response string `json:"response"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("[LLM][%s] Skipping unparseable streaming chunk: %q", agentID, string(line))
			continue
		}
		builder.WriteString(msg.Response)
	}
	raw := builder.String()
	log.Printf("[LLM][%s] ----- Raw LLM response (before cleaning) -----\n%s\n----- Raw End -----\n", agentID, raw)

	cleaned := cleanOutputForJSON(raw)
	if cleaned != raw {
		log.Printf("[LLM][%s] Cleaned LLM response (after stripping <think> and code blocks):\n%s\n", agentID, cleaned)
	}
	return cleaned, nil
}

// -------- Utility Fixes --------

func normalizeType(id, t string) string {
	if t != "" {
		tt := strings.ToLower(t)
		switch tt {
		case "agent", "issue", "task":
			return tt
		}
	}
	idl := strings.ToLower(id)
	switch {
	case idl == "":
		return "unknown"
	case strings.HasPrefix(idl, "agent"):
		return "agent"
	case strings.HasPrefix(idl, "issue"):
		return "issue"
	case strings.HasPrefix(idl, "task"):
		return "issue" // treat "task" as "issue" for this schema
	default:
		return "unknown"
	}
}

func sanitizeNodes(nodes []Node) []Node {
	uniq := make(map[string]Node)
	for _, n := range nodes {
		if n.ID == "" {
			continue
		}
		n.Type = normalizeType(n.ID, n.Type)
		// Only keep agents and issues/tasks
		if n.Type != "agent" && n.Type != "issue" {
			continue
		}
		uniq[n.ID] = n
	}
	result := make([]Node, 0, len(uniq))
	for _, n := range uniq {
		result = append(result, n)
	}
	return result
}

func sanitizeEdges(edges []Edge, nodeIDs map[string]struct{}) []Edge {
	uniq := make(map[string]Edge)
	for _, e := range edges {
		if e.Source == "" || e.Target == "" {
			continue
		}
		// Only include if both source and target exist in nodes
		if _, ok := nodeIDs[e.Source]; !ok {
			continue
		}
		if _, ok := nodeIDs[e.Target]; !ok {
			continue
		}
		if e.Type == "" {
			e.Type = "bid" // default for your domain
		}
		key := e.Source + "|" + e.Target + "|" + e.Type
		uniq[key] = e
	}
	result := make([]Edge, 0, len(uniq))
	for _, e := range uniq {
		result = append(result, e)
	}
	return result
}

// -------- Main --------

func main() {
	dataDir := flag.String("data_dir", "./data", "Directory containing agent*.json files")
	outputFile := flag.String("output", "./data/baseline_network.json", "Output file for the network (JSON)")
	ollamaUrl := flag.String("ollama_url", "http://localhost:11434/api/generate", "Ollama API URL")
	ollamaModel := flag.String("model", "granite3.3:8b", "Ollama model to use")
	flag.Parse()

	log.Println("[startup] Scanning agent JSON files in:", *dataDir)
	files, err := filepath.Glob(filepath.Join(*dataDir, "agent*.json"))
	if err != nil {
		log.Fatal("[startup] Error scanning files:", err)
	}
	if len(files) == 0 {
		log.Fatal("[startup] No agent*.json files found in data directory")
	}

	allNodes := map[string]Node{}
	allEdges := []Edge{}

	agentIndexes, taskIndexes := buildGlobalIndex(files, allNodes, allEdges)

	for i, fname := range files {
		log.Printf("[network] Processing agent %d/%d: %s", i+1, len(files), fname)
		b, err := ioutil.ReadFile(fname)
		if err != nil {
			log.Printf("[network] Failed to read %s: %v", fname, err)
			continue
		}
		var agent AgentSchema
		if err := json.Unmarshal(b, &agent); err != nil {
			log.Printf("[network] Failed to parse %s: %v", fname, err)
			continue
		}

		idxAgents := []AgentIndex{}
		for _, idx := range agentIndexes {
			if idx.ID != agent.AgentID {
				idxAgents = append(idxAgents, idx)
			}
		}
		idxTasks := []TaskIndex{}
		for _, idx := range taskIndexes {
			if idx.Owner != agent.AgentID {
				idxTasks = append(idxTasks, idx)
			}
		}
		indexSummary := struct {
			Agents []AgentIndex `json:"agents"`
			Tasks  []TaskIndex  `json:"tasks"`
		}{Agents: idxAgents, Tasks: idxTasks}

		indexJson, _ := json.MarshalIndent(indexSummary, "", "  ")

		prompt := `
You are helping construct an economic agent-issue network for visualization (D3.js JSON schema).
Here is a summary of all other agents and issues:
` + string(indexJson) + `
Here is the full data for the current agent:
` + string(b) + `

Instructions:
- Propose new "edges" (edges) from this agent to other agents (for collaboration, etc.) and to issues matching their specialties (bidding, interest, etc.).
- Output a JSON object with two arrays: "nodes" (for this agent and any new issues created) and "edges" (edges from this agent to other agents/issues), matching the D3.js schema. Do not repeat nodes for agents/issues already in the summary.
- Do NOT include Markdown code blocks or formatting, output ONLY valid JSON.
- Use the following schema:
{
  "nodes": [ ... ],
  "edges": [ ... ]
}
- For all issues, use the "task_id" field instead of "id" to avoid confusion.
- All nodes must have a unique, non-empty "id", and all edges must have non-empty "source" and "target" that match nodes.
- Always set the "type" field for nodes to "agent" or "issue" as appropriate.
- DO NOT include "manager" nodes or any other node types.
`

		llmResp, err := askOllama(prompt, *ollamaUrl, *ollamaModel, agent.AgentID)
		if err != nil {
			log.Printf("[LLM][%s] LLM call failed: %v", agent.AgentID, err)
			continue
		}

		if len(strings.TrimSpace(llmResp)) == 0 {
			log.Printf("[LLM][%s] LLM response is empty after cleaning, skipping.", agent.AgentID)
			continue
		}

		var partial struct {
			Nodes []Node `json:"nodes"`
			Edges []Edge `json:"edges"`
		}
		if err := json.Unmarshal([]byte(llmResp), &partial); err != nil {
			rawPath := filepath.Join(*dataDir, agent.AgentID+"_llm_raw.txt")
			_ = ioutil.WriteFile(rawPath, []byte(llmResp), 0644)
			log.Printf("[parse][%s] Failed to parse cleaned LLM output: %v\nCleaned output saved to %s", agent.AgentID, err, rawPath)
			continue
		} else {
			log.Printf("[LLM][%s] Parsed %d nodes and %d edges from LLM output.", agent.AgentID, len(partial.Nodes), len(partial.Edges))
		}

		for _, n := range partial.Nodes {
			if n.ID == "" {
				continue
			}
			n.Type = normalizeType(n.ID, n.Type)
			// Only keep agents and issues/tasks
			if n.Type != "agent" && n.Type != "issue" {
				continue
			}
			allNodes[n.ID] = n
		}
		allEdges = append(allEdges, partial.Edges...)

		agentIndexes, taskIndexes = buildGlobalIndex(files, allNodes, allEdges)

		log.Printf("[network][%s] Integrated: %d nodes, %d edges (total nodes: %d, edges: %d)",
			agent.AgentID, len(partial.Nodes), len(partial.Edges), len(allNodes), len(allEdges))
	}

	// --- Final Sanity: Remove invalid/blank nodes, edges with missing nodes, and filter by only "agent" and "issue" types ---

	finalNodes := sanitizeNodes(mapToNodeSlice(allNodes))
	nodeIDs := make(map[string]struct{}, len(finalNodes))
	for _, n := range finalNodes {
		nodeIDs[n.ID] = struct{}{}
	}
	finalEdges := sanitizeEdges(allEdges, nodeIDs)

	econ := EconomicNetwork{
		SchemaVersion: "1.1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Nodes:         finalNodes,
		Edges:         finalEdges,
	}
	out, _ := json.MarshalIndent(econ, "", "  ")
	err = ioutil.WriteFile(*outputFile, out, 0644)
	if err != nil {
		log.Fatalf("[output] Failed to write output: %v", err)
	}
	log.Printf("[output] Economic network written to %s", *outputFile)
}

func mapToNodeSlice(m map[string]Node) []Node {
	out := make([]Node, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	return out
}

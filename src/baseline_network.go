package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Speciality struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Example     string  `json:"example"`
	Weight      float64 `json:"weight"`
	Color       string  `json:"color,omitempty"`
	Icon        string  `json:"icon,omitempty"`
	Level       string  `json:"level,omitempty"`
}

type TaskSpeciality struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Level       string `json:"level,omitempty"`
}

type Node struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Label        string          `json:"label"`
	Group        string          `json:"group,omitempty"`
	Specialities []Speciality    `json:"specialities,omitempty"`
	Speciality   *TaskSpeciality `json:"speciality,omitempty"`
	Avatar       string          `json:"avatar,omitempty"`
	Desc         string          `json:"desc,omitempty"`
	Status       string          `json:"status,omitempty"`
	Priority     string          `json:"priority,omitempty"`
	Owner        string          `json:"owner,omitempty"`
	AssignedTo   string          `json:"assigned_to,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	PriceMin     float64         `json:"price_min,omitempty"`
	PriceMax     float64         `json:"price_max,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

type Edge struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Type     string         `json:"type"`
	BidValue float64        `json:"bid_value,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Network struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

type AgentFile struct {
	AgentID      string       `json:"agent_id"`
	DisplayName  string       `json:"display_name,omitempty"`
	Group        string       `json:"group,omitempty"`
	Avatar       string       `json:"avatar,omitempty"`
	Specialities []Speciality `json:"specialities"`
	Tasks        []struct {
		ID         string         `json:"id"`
		Desc       string         `json:"desc"`
		Speciality TaskSpeciality `json:"speciality"`
		PriceMin   float64        `json:"price_min"`
		PriceMax   float64        `json:"price_max"`
		Status     string         `json:"status,omitempty"`
		Priority   string         `json:"priority,omitempty"`
		Owner      string         `json:"owner,omitempty"`
		AssignedTo string         `json:"assigned_to,omitempty"`
		Tags       []string       `json:"tags,omitempty"`
		Metadata   map[string]any `json:"metadata,omitempty"`
	} `json:"tasks"`
}

func main() {
	agentsDir := flag.String("agents_dir", "./data/agents", "Directory containing agent JSON files")
	outputFile := flag.String("output", "./data/baseline_network.json", "Output file for the network (JSON)")
	flag.Parse()

	files, err := ioutil.ReadDir(*agentsDir)
	if err != nil {
		log.Fatalf("Failed to read dir %s: %v", *agentsDir, err)
	}

	agents := []AgentFile{}
	for _, fi := range files {
		if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".json") {
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(*agentsDir, fi.Name()))
		if err != nil {
			log.Printf("Failed to read %s: %v", fi.Name(), err)
			continue
		}
		var agent AgentFile
		if err := json.Unmarshal(b, &agent); err != nil {
			log.Printf("Failed to parse %s: %v", fi.Name(), err)
			continue
		}
		agents = append(agents, agent)
	}

	// Build nodes
	nodes := []Node{}
	agentSpecialities := map[string]map[string]bool{}
	for _, agent := range agents {
		nodes = append(nodes, Node{
			ID:           agent.AgentID,
			Type:         "agent",
			Label:        agent.DisplayName,
			Group:        agent.Group,
			Avatar:       agent.Avatar,
			Specialities: agent.Specialities,
		})
		specSet := map[string]bool{}
		for _, s := range agent.Specialities {
			specSet[s.Name] = true
		}
		agentSpecialities[agent.AgentID] = specSet
	}

	taskNodes := []Node{}
	allTasks := []Node{}
	for _, agent := range agents {
		for _, t := range agent.Tasks {
			taskNode := Node{
				ID:         t.ID,
				Type:       "issue",
				Label:      t.ID,
				Desc:       t.Desc,
				Speciality: &t.Speciality,
				PriceMin:   t.PriceMin,
				PriceMax:   t.PriceMax,
				Status:     t.Status,
				Priority:   t.Priority,
				Owner:      t.Owner,
				AssignedTo: t.AssignedTo,
				Tags:       t.Tags,
				Metadata:   t.Metadata,
			}
			taskNodes = append(taskNodes, taskNode)
			allTasks = append(allTasks, taskNode)
		}
	}
	nodes = append(nodes, taskNodes...)

	// Build edges
	edges := []Edge{}
	edgeExists := map[string]bool{}
	addEdge := func(e Edge) {
		k := e.Source + "->" + e.Target + ":" + e.Type
		if !edgeExists[k] {
			edges = append(edges, e)
			edgeExists[k] = true
		}
	}

	// Bids: agent has speciality for task
	for _, agent := range agents {
		for _, t := range allTasks {
			if agentSpecialities[agent.AgentID][t.Speciality.Name] {
				bidValue := (t.PriceMin + t.PriceMax) / 2
				addEdge(Edge{
					Source:   agent.AgentID,
					Target:   t.ID,
					Type:     "bid",
					BidValue: bidValue,
				})
			}
		}
	}
	// Auctions: agent owns task but lacks the required speciality
	for _, agent := range agents {
		for _, t := range allTasks {
			if t.Owner == agent.AgentID && !agentSpecialities[agent.AgentID][t.Speciality.Name] {
				addEdge(Edge{
					Source: agent.AgentID,
					Target: t.ID,
					Type:   "auction",
				})
			}
		}
	}
	// Assigned: task explicitly assigned to agent
	for _, t := range allTasks {
		if t.AssignedTo != "" {
			addEdge(Edge{
				Source: t.ID,
				Target: t.AssignedTo,
				Type:   "assigned",
			})
		}
	}

	network := Network{
		SchemaVersion: "1.2",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Nodes:         nodes,
		Edges:         edges,
	}
	b, err := json.MarshalIndent(network, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal network: %v", err)
	}
	if err := os.WriteFile(*outputFile, b, 0644); err != nil {
		log.Fatalf("Failed to write %s: %v", *outputFile, err)
	}
	log.Printf("Wrote economic network with %d nodes, %d edges to %s", len(nodes), len(edges), *outputFile)
}

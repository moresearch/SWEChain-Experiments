package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// Node represents an entity (agent or task)
type Node struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

// Link represents an edge between nodes
type Link struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"`
	Weight     int     `json:"weight"`
	Label      string  `json:"label"`
	BidCount   int     `json:"bid_count"`
	WinningBid float64 `json:"winning_bid"`
	Specialist bool    `json:"specialist,omitempty"`
}

// Graph holds the full schema
type Graph struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// Agent mirrors the schema in agent_data/*.json
type Agent struct {
	AgentID          string             `json:"agent_id"`
	Specialty        string             `json:"specialty"`
	Skills           []string           `json:"skills"`
	SkillLevels      map[string]float64 `json:"skill_levels"`
	TasksToOutsource []struct {
		TaskID         string   `json:"task_id"`
		Description    string   `json:"description"`
		Categories     []string `json:"categories"`
		EstimatedHours int      `json:"estimated_hours"`
		RequiredSkills []string `json:"required_skills"`
		AuctionStatus  string   `json:"auction_status"`
		Price          float64  `json:"price"`
		PriceLimit     float64  `json:"price_limit"`
	} `json:"tasks_to_outsource"`
	OwnTasks []struct {
		TaskID         string   `json:"task_id"`
		Description    string   `json:"description"`
		Categories     []string `json:"categories"`
		EstimatedHours int      `json:"estimated_hours"`
		RequiredSkills []string `json:"required_skills"`
		Status         string   `json:"status"`
		Price          float64  `json:"price"`
		PriceLimit     float64  `json:"price_limit"`
	} `json:"own_tasks"`
}

func main() {
	// Initialize graph with empty slices
	g := Graph{
		Nodes: make([]Node, 0),
		Links: make([]Link, 0),
	}
	seen := make(map[string]bool)

	// Read all agent JSON files under agent_data/
	files, err := filepath.Glob("agent_data/*.json")
	if err != nil {
		log.Fatal(err)
	}
	if len(files) == 0 {
		log.Fatalf("no agent JSON files found in agent_data/")
	}

	for _, fname := range files {
		data, err := ioutil.ReadFile(fname)
		if err != nil {
			log.Printf("ERROR reading %s: %v", fname, err)
			continue
		}
		var agent Agent
		if err := json.Unmarshal(data, &agent); err != nil {
			log.Printf("ERROR parsing %s: %v", fname, err)
			continue
		}

		// Add agent node
		if !seen[agent.AgentID] {
			g.Nodes = append(g.Nodes, Node{
				ID:    agent.AgentID,
				Name:  agent.Specialty,
				Group: "agent",
			})
			seen[agent.AgentID] = true
		}

		// Process own tasks
		for _, t := range agent.OwnTasks {
			if !seen[t.TaskID] {
				g.Nodes = append(g.Nodes, Node{
					ID:    t.TaskID,
					Name:  t.TaskID, // use task ID as node name
					Group: "task",
				})
				seen[t.TaskID] = true
			}
			g.Links = append(g.Links, Link{
				Source:     agent.AgentID,
				Target:     t.TaskID,
				Type:       "assigned",
				Weight:     t.EstimatedHours,
				Label:      t.Categories[0],
				BidCount:   1,
				WinningBid: t.Price,
				Specialist: true,
			})
		}

		// Process outsourced tasks
		for _, t := range agent.TasksToOutsource {
			if !seen[t.TaskID] {
				g.Nodes = append(g.Nodes, Node{
					ID:    t.TaskID,
					Name:  t.TaskID, // use task ID as node name
					Group: "task",
				})
				seen[t.TaskID] = true
			}
			g.Links = append(g.Links, Link{
				Source:     agent.AgentID,
				Target:     t.TaskID,
				Type:       "outsourced",
				Weight:     t.EstimatedHours,
				Label:      t.Categories[0],
				BidCount:   1,
				WinningBid: t.Price,
			})
		}
	}

	// Write graph to truth.json
	outFile := "truth.json"
	f, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("failed to create %s: %v", outFile, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		log.Fatalf("failed to write %s: %v", outFile, err)
	}
	log.Printf("Generated %s with %d nodes and %d links", outFile, len(g.Nodes), len(g.Links))
}

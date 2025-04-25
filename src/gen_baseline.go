package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// Node represents an entity (agent or task) following the required schema
type Node struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Role       string `json:"role"`       // "task" or "agent"
	Specialist bool   `json:"specialist"` // true for agents, false for tasks
	Group      string `json:"group"`      // same as role, or can be used for further grouping
	Degree     int    `json:"degree"`     // count of links connected to this node
}

// Link represents an edge between nodes following the required schema
type Link struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"` // "bidded", "assigned", "outsourced"
	Weight     int     `json:"weight"`
	WinningBid float64 `json:"WinningBid"`
	BidCount   int     `json:"bid_count"`
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
	g := Graph{
		Nodes: make([]Node, 0),
		Links: make([]Link, 0),
	}
	seenNodes := make(map[string]*Node)
	degree := make(map[string]int)

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
		if _, ok := seenNodes[agent.AgentID]; !ok {
			node := Node{
				ID:         agent.AgentID,
				Name:       agent.Specialty,
				Role:       "agent",
				Specialist: true,
				Group:      "agent",
				Degree:     0, // will be updated later
			}
			g.Nodes = append(g.Nodes, node)
			seenNodes[agent.AgentID] = &g.Nodes[len(g.Nodes)-1]
		}

		// Process own tasks
		for _, t := range agent.OwnTasks {
			if _, ok := seenNodes[t.TaskID]; !ok {
				node := Node{
					ID:         t.TaskID,
					Name:       t.TaskID,
					Role:       "task",
					Specialist: false,
					Group:      "task",
					Degree:     0,
				}
				g.Nodes = append(g.Nodes, node)
				seenNodes[t.TaskID] = &g.Nodes[len(g.Nodes)-1]
			}
			link := Link{
				Source:     agent.AgentID,
				Target:     t.TaskID,
				Type:       "assigned",
				Weight:     t.EstimatedHours,
				WinningBid: t.Price,
				BidCount:   1,
			}
			g.Links = append(g.Links, link)
			degree[agent.AgentID]++
			degree[t.TaskID]++
		}

		// Process outsourced tasks
		for _, t := range agent.TasksToOutsource {
			if _, ok := seenNodes[t.TaskID]; !ok {
				node := Node{
					ID:         t.TaskID,
					Name:       t.TaskID,
					Role:       "task",
					Specialist: false,
					Group:      "task",
					Degree:     0,
				}
				g.Nodes = append(g.Nodes, node)
				seenNodes[t.TaskID] = &g.Nodes[len(g.Nodes)-1]
			}
			link := Link{
				Source:     agent.AgentID,
				Target:     t.TaskID,
				Type:       "outsourced",
				Weight:     t.EstimatedHours,
				WinningBid: t.Price,
				BidCount:   1,
			}
			g.Links = append(g.Links, link)
			degree[agent.AgentID]++
			degree[t.TaskID]++
		}
	}

	// Set degree for each node
	for i := range g.Nodes {
		g.Nodes[i].Degree = degree[g.Nodes[i].ID]
	}

	// Write graph to baseline.json
	outFile := "baseline_network.json"
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

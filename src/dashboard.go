package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
)

type Node struct {
	ID        string `json:"id"`
	Group     string `json:"group"`
	Name      string `json:"name"`
	Role      string `json:"role"`      // "agent" or "task"
	Specialty string `json:"specialty"` // For agents: their specialty (now always set)
	Degree    int    `json:"degree"`
}

type Link struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"` // "assigned", "bidded", "outsourced"
	Label      string  `json:"label,omitempty"`
	Weight     int     `json:"weight"`
	BidCount   int     `json:"bid_count"`
	WinningBid float64 `json:"winning_bid"`
}

type GraphData struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// Metrics and supporting types (unchanged)
type MarketMetrics struct {
	NetworkDensity          float64      `json:"network_density"`
	DegreeDistribution      []DegreeData `json:"degree_distribution"`
	BidVolume               float64      `json:"bid_volume"`
	BidVariance             float64      `json:"bid_variance"`
	AvgPriceEfficiency      float64      `json:"avg_price_efficiency"`
	ClientSurplus           float64      `json:"client_surplus"`
	GiniCoefficient         float64      `json:"gini_coefficient"`
	WinRateDistribution     []float64    `json:"win_rate_distribution"`
	AvgBidToWinRatio        float64      `json:"avg_bid_to_win_ratio"`
	RepeatMatchingRate      float64      `json:"repeat_matching_rate"`
	AllocationEntropy       float64      `json:"allocation_entropy"`
	ParticipationElasticity float64      `json:"participation_elasticity"`
}

type DegreeData struct {
	Degree int `json:"degree"`
	Count  int `json:"count"`
}

type AgentMetrics struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Role            string  `json:"role"`
	Specialty       string  `json:"specialty"`
	Bids            int     `json:"bids"`
	Wins            int     `json:"wins"`
	WinRate         float64 `json:"win_rate"`
	BidToWinRatio   float64 `json:"bid_to_win_ratio"`
	RepeatMatchRate float64 `json:"repeat_match_rate"`
	AvgBidValue     float64 `json:"avg_bid_value"`
	TotalValue      float64 `json:"total_value"`
}

type TaskMetrics struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	BidCount      int     `json:"bid_count"`
	AvgBid        float64 `json:"avg_bid"`
	StdDev        float64 `json:"std_dev"`
	Variance      float64 `json:"variance"`
	CoV           float64 `json:"cov"` // Coefficient of variation
	WinningBid    float64 `json:"winning_bid"`
	ClientSurplus float64 `json:"client_surplus"`
}

type LorenzPoint struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Agent     string  `json:"agent,omitempty"`
	Specialty string  `json:"specialty,omitempty"`
	Value     float64 `json:"value,omitempty"`
}

type PageData struct {
	NetworkDensity     string
	OutsourcingRatio   string
	BiddingRatio       string
	AvgTaskCost        string
	AvgBiddersPerTask  string
	AvgWinningBidPrice string
	BidVariance        string
	NoBidRate          string
	NoAuctionRate      string
	GiniCoefficient    string
	ClientSurplus      string
}

var processedData []byte
var advancedMetrics *MarketMetrics
var indexTemplate *template.Template

func degreeCentrality(graphData GraphData) map[string]int {
	degreeMap := make(map[string]int)
	for _, node := range graphData.Nodes {
		degreeMap[node.ID] = 0
	}
	for _, link := range graphData.Links {
		degreeMap[link.Source]++
		degreeMap[link.Target]++
	}
	return degreeMap
}

func calculateDegreeDistribution(degrees map[string]int) []DegreeData {
	degreeFreq := make(map[int]int)
	for _, degree := range degrees {
		degreeFreq[degree]++
	}
	distribution := make([]DegreeData, 0, len(degreeFreq))
	for degree, count := range degreeFreq {
		distribution = append(distribution, DegreeData{Degree: degree, Count: count})
	}
	sort.Slice(distribution, func(i, j int) bool {
		return distribution[i].Degree < distribution[j].Degree
	})
	return distribution
}

func calculateGiniCoefficient(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sort.Float64s(values)
	sumValues := 0.0
	for _, v := range values {
		sumValues += v
	}
	if sumValues == 0 {
		return 0
	}
	cumulativeSum := 0.0
	lorenzArea := 0.0
	for i, v := range values {
		cumulativeSum += v
		x1 := float64(i) / float64(n)
		x2 := float64(i+1) / float64(n)
		y1 := cumulativeSum / sumValues
		lorenzArea += (x2 - x1) * y1
	}
	return 1.0 - 2.0*lorenzArea
}

func calculateEntropy(probabilities []float64) float64 {
	var entropy float64
	for _, p := range probabilities {
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func calculateAllocationEntropy(graphData GraphData) float64 {
	agentAssignments := make(map[string]int)
	for _, link := range graphData.Links {
		if link.Type == "assigned" {
			agentAssignments[link.Source]++
		}
	}
	totalAssignments := 0
	for _, count := range agentAssignments {
		totalAssignments += count
	}
	if totalAssignments == 0 {
		return 0
	}
	probabilities := make([]float64, 0, len(agentAssignments))
	for _, count := range agentAssignments {
		probabilities = append(probabilities, float64(count)/float64(totalAssignments))
	}
	rawEntropy := calculateEntropy(probabilities)
	maxEntropy := math.Log2(float64(len(agentAssignments)))
	if maxEntropy > 0 {
		return rawEntropy / maxEntropy
	}
	return 0
}

func calculateAgentMetrics(graphData GraphData) []AgentMetrics {
	agents := make([]AgentMetrics, 0)
	for _, node := range graphData.Nodes {
		if node.Role == "agent" {
			bids := 0
			totalBidValue := 0.0
			for _, link := range graphData.Links {
				if link.Source == node.ID && link.Type == "bidded" {
					bids++
					totalBidValue += link.WinningBid
				}
			}
			wins := 0
			totalWinValue := 0.0
			taskCounts := make(map[string]int)
			for _, link := range graphData.Links {
				if link.Source == node.ID && link.Type == "assigned" {
					wins++
					totalWinValue += link.WinningBid
					taskCounts[link.Target]++
				}
			}
			repeatMatches := 0
			for _, count := range taskCounts {
				if count > 1 {
					repeatMatches++
				}
			}
			repeatMatchRate := 0.0
			if wins > 0 {
				repeatMatchRate = float64(repeatMatches) / float64(wins)
			}
			winRate := 0.0
			bidToWinRatio := 0.0
			avgBidValue := 0.0
			if bids > 0 {
				winRate = float64(wins) / float64(bids)
				avgBidValue = totalBidValue / float64(bids)
			}
			if wins > 0 {
				bidToWinRatio = float64(bids) / float64(wins)
			} else if bids > 0 {
				bidToWinRatio = float64(bids)
			}
			agents = append(agents, AgentMetrics{
				ID:              node.ID,
				Name:            node.Name,
				Role:            node.Role,
				Specialty:       node.Specialty,
				Bids:            bids,
				Wins:            wins,
				WinRate:         winRate,
				BidToWinRatio:   bidToWinRatio,
				RepeatMatchRate: repeatMatchRate,
				AvgBidValue:     avgBidValue,
				TotalValue:      totalWinValue,
			})
		}
	}
	return agents
}

func calculateTaskMetrics(graphData GraphData) []TaskMetrics {
	tasks := make([]TaskMetrics, 0)
	for _, node := range graphData.Nodes {
		if node.Role == "task" {
			var bidValues []float64
			for _, link := range graphData.Links {
				if link.Target == node.ID && link.Type == "bidded" && link.WinningBid > 0 {
					bidValues = append(bidValues, link.WinningBid)
				}
			}
			bidCount := len(bidValues)
			avgBid := 0.0
			variance := 0.0
			stdDev := 0.0
			cov := 0.0
			if bidCount > 0 {
				sum := 0.0
				for _, bid := range bidValues {
					sum += bid
				}
				avgBid = sum / float64(bidCount)
				sumSquaredDiff := 0.0
				for _, bid := range bidValues {
					diff := bid - avgBid
					sumSquaredDiff += diff * diff
				}
				if bidCount > 1 {
					variance = sumSquaredDiff / float64(bidCount-1)
					stdDev = math.Sqrt(variance)
					if avgBid > 0 {
						cov = stdDev / avgBid
					}
				}
			}
			winningBid := 0.0
			for _, link := range graphData.Links {
				if link.Target == node.ID && link.Type == "assigned" {
					winningBid = link.WinningBid
					break
				}
			}
			clientSurplus := 0.0
			if avgBid > 0 && winningBid > 0 {
				clientSurplus = avgBid - winningBid
			}
			tasks = append(tasks, TaskMetrics{
				ID:            node.ID,
				Name:          node.Name,
				BidCount:      bidCount,
				AvgBid:        avgBid,
				StdDev:        stdDev,
				Variance:      variance,
				CoV:           cov,
				WinningBid:    winningBid,
				ClientSurplus: clientSurplus,
			})
		}
	}
	return tasks
}

func calculateMarketMetrics(graphData GraphData) *MarketMetrics {
	metrics := &MarketMetrics{}
	degrees := degreeCentrality(graphData)
	degreeDistribution := calculateDegreeDistribution(degrees)
	metrics.DegreeDistribution = degreeDistribution
	agentCount := 0
	taskCount := 0
	for _, node := range graphData.Nodes {
		if node.Role == "agent" {
			agentCount++
		} else if node.Role == "task" {
			taskCount++
		}
	}
	possibleConnections := agentCount * taskCount
	actualConnections := 0
	for _, link := range graphData.Links {
		sourceNode := findNodeByID(graphData.Nodes, link.Source)
		targetNode := findNodeByID(graphData.Nodes, link.Target)
		if sourceNode != nil && targetNode != nil {
			if (sourceNode.Role == "agent" && targetNode.Role == "task") ||
				(sourceNode.Role == "task" && targetNode.Role == "agent") {
				actualConnections++
			}
		}
	}
	if possibleConnections > 0 {
		metrics.NetworkDensity = float64(actualConnections) / float64(possibleConnections)
	}
	bidCount := 0
	totalBidValue := 0.0
	bidValues := []float64{}
	for _, link := range graphData.Links {
		if link.Type == "bidded" {
			bidCount++
			if link.WinningBid > 0 {
				totalBidValue += link.WinningBid
				bidValues = append(bidValues, link.WinningBid)
			}
		}
	}
	metrics.BidVolume = float64(bidCount)
	if len(bidValues) > 1 {
		mean := totalBidValue / float64(len(bidValues))
		sumSquaredDiff := 0.0
		for _, bid := range bidValues {
			diff := bid - mean
			sumSquaredDiff += diff * diff
		}
		variance := sumSquaredDiff / float64(len(bidValues)-1)
		metrics.BidVariance = variance
	}
	agentMetrics := calculateAgentMetrics(graphData)
	winValues := make([]float64, 0, len(agentMetrics))
	winRates := make([]float64, 0, len(agentMetrics))
	for _, agent := range agentMetrics {
		if agent.TotalValue > 0 {
			winValues = append(winValues, agent.TotalValue)
		}
		if agent.Bids > 0 {
			winRates = append(winRates, agent.WinRate)
		}
	}
	metrics.GiniCoefficient = calculateGiniCoefficient(winValues)
	metrics.WinRateDistribution = winRates
	totalBidToWinRatio := 0.0
	totalRepeatMatchRate := 0.0
	agentsWithBids := 0
	for _, agent := range agentMetrics {
		if agent.Bids > 0 {
			agentsWithBids++
			totalBidToWinRatio += agent.BidToWinRatio
			totalRepeatMatchRate += agent.RepeatMatchRate
		}
	}
	if agentsWithBids > 0 {
		metrics.AvgBidToWinRatio = totalBidToWinRatio / float64(agentsWithBids)
		metrics.RepeatMatchingRate = totalRepeatMatchRate / float64(agentsWithBids)
	}
	taskMetrics := calculateTaskMetrics(graphData)
	totalClientSurplus := 0.0
	tasksWithBids := 0
	for _, task := range taskMetrics {
		if task.BidCount > 0 && task.WinningBid > 0 {
			tasksWithBids++
			totalClientSurplus += task.ClientSurplus
		}
	}
	if tasksWithBids > 0 {
		metrics.AvgPriceEfficiency = totalClientSurplus / float64(tasksWithBids)
		metrics.ClientSurplus = totalClientSurplus
	}
	metrics.AllocationEntropy = calculateAllocationEntropy(graphData)
	if agentCount > 0 && taskCount > 0 {
		metrics.ParticipationElasticity = metrics.NetworkDensity * (1.0 - metrics.GiniCoefficient) * 2.0
		if metrics.ParticipationElasticity > 2.0 {
			metrics.ParticipationElasticity = 2.0
		}
	}
	return metrics
}

func findNodeByID(nodes []Node, id string) *Node {
	for i, node := range nodes {
		if node.ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func calculateNetworkMetrics(graphData GraphData) map[string]interface{} {
	metrics := make(map[string]interface{})
	agentCount := 0
	taskCount := 0
	for _, node := range graphData.Nodes {
		if node.Role == "agent" {
			agentCount++
		} else if node.Role == "task" {
			taskCount++
		}
	}
	assignedCount := 0
	outsourcedCount := 0
	biddedCount := 0
	totalLinks := len(graphData.Links)
	totalBidCount := 0
	totalWinningBids := 0.0
	bidValues := []float64{}
	noBidCount := 0
	validBidCount := 0
	totalSpecialistTasks := 0
	for _, link := range graphData.Links {
		switch link.Type {
		case "assigned":
			assignedCount++
			if link.BidCount > 0 {
				totalBidCount += link.BidCount
				validBidCount++
			} else {
				noBidCount++
			}
			if link.WinningBid > 0 {
				totalWinningBids += link.WinningBid
				bidValues = append(bidValues, link.WinningBid)
			}
			sourceIsAgent := false
			targetIsTask := false
			for _, node := range graphData.Nodes {
				if node.ID == link.Source && node.Role == "agent" {
					sourceIsAgent = true
				}
				if node.ID == link.Target && node.Role == "task" {
					targetIsTask = true
				}
			}
			if sourceIsAgent && targetIsTask {
				totalSpecialistTasks++
			}
		case "outsourced":
			outsourcedCount++
		case "bidded":
			biddedCount++
		}
	}
	possibleLinks := agentCount * taskCount
	if possibleLinks > 0 {
		metrics["networkDensity"] = float64(assignedCount) / float64(possibleLinks)
	} else {
		metrics["networkDensity"] = 0.0
	}
	if totalLinks > 0 {
		metrics["outsourcingRatio"] = float64(outsourcedCount) / float64(totalLinks)
		metrics["biddingRatio"] = float64(biddedCount) / float64(totalLinks)
	} else {
		metrics["outsourcingRatio"] = 0.0
		metrics["biddingRatio"] = 0.0
	}
	metrics["specialistWinRate"] = 1.0 // All agents are specialists in this model
	if len(bidValues) > 0 {
		metrics["avgTaskCost"] = totalWinningBids / float64(len(bidValues))
	} else {
		metrics["avgTaskCost"] = 0.0
	}
	if validBidCount > 0 {
		metrics["avgBiddersPerTask"] = float64(totalBidCount) / float64(validBidCount)
	} else {
		metrics["avgBiddersPerTask"] = 0.0
	}
	metrics["avgWinningBidPrice"] = metrics["avgTaskCost"]
	if len(bidValues) > 1 {
		mean := totalWinningBids / float64(len(bidValues))
		sumSquaredDiff := 0.0
		for _, value := range bidValues {
			diff := value - mean
			sumSquaredDiff += diff * diff
		}
		variance := sumSquaredDiff / float64(len(bidValues))
		metrics["bidVariance"] = math.Sqrt(variance) / mean
	} else {
		metrics["bidVariance"] = 0.0
	}
	if assignedCount > 0 {
		metrics["noBidRate"] = float64(noBidCount) / float64(assignedCount)
	} else {
		metrics["noBidRate"] = 0.0
	}
	metrics["noAuctionRate"] = metrics["noBidRate"]
	advancedMetrics = calculateMarketMetrics(graphData)
	metrics["giniCoefficient"] = advancedMetrics.GiniCoefficient
	metrics["clientSurplus"] = advancedMetrics.ClientSurplus
	return metrics
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	var graphData GraphData
	if err := json.Unmarshal(processedData, &graphData); err != nil {
		log.Printf("Error parsing JSON data: %v", err)
		http.Error(w, "Error processing data", http.StatusInternalServerError)
		return
	}
	metrics := calculateNetworkMetrics(graphData)
	pageData := PageData{
		NetworkDensity:     fmt.Sprintf("%.2f", metrics["networkDensity"].(float64)),
		OutsourcingRatio:   fmt.Sprintf("%.0f%%", metrics["outsourcingRatio"].(float64)*100),
		BiddingRatio:       fmt.Sprintf("%.0f%%", metrics["biddingRatio"].(float64)*100),
		AvgTaskCost:        fmt.Sprintf("$%.2f", metrics["avgTaskCost"].(float64)),
		AvgBiddersPerTask:  fmt.Sprintf("%.1f", metrics["avgBiddersPerTask"].(float64)),
		AvgWinningBidPrice: fmt.Sprintf("$%.2f", metrics["avgWinningBidPrice"].(float64)),
		BidVariance:        fmt.Sprintf("%.1f%%", metrics["bidVariance"].(float64)*100),
		NoBidRate:          fmt.Sprintf("%.1f%%", metrics["noBidRate"].(float64)*100),
		NoAuctionRate:      fmt.Sprintf("%.1f%%", metrics["noAuctionRate"].(float64)*100),
		GiniCoefficient:    fmt.Sprintf("%.3f", metrics["giniCoefficient"].(float64)),
		ClientSurplus:      fmt.Sprintf("$%.2f", metrics["clientSurplus"].(float64)),
	}
	err := indexTemplate.Execute(w, pageData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(processedData)
}

func handleMarketMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if advancedMetrics == nil {
		http.Error(w, "Market metrics not calculated", http.StatusInternalServerError)
		return
	}
	jsonData, err := json.Marshal(advancedMetrics)
	if err != nil {
		http.Error(w, "Error serializing market metrics", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
}

func handleAgentMetrics(w http.ResponseWriter, r *http.Request) {
	var graphData GraphData
	if err := json.Unmarshal(processedData, &graphData); err != nil {
		log.Printf("Error parsing JSON data: %v", err)
		http.Error(w, "Error processing data", http.StatusInternalServerError)
		return
	}
	agentMetrics := calculateAgentMetrics(graphData)
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(agentMetrics)
	if err != nil {
		http.Error(w, "Error serializing agent metrics", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
}

func handleTaskMetrics(w http.ResponseWriter, r *http.Request) {
	var graphData GraphData
	if err := json.Unmarshal(processedData, &graphData); err != nil {
		log.Printf("Error parsing JSON data: %v", err)
		http.Error(w, "Error processing data", http.StatusInternalServerError)
		return
	}
	taskMetrics := calculateTaskMetrics(graphData)
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(taskMetrics)
	if err != nil {
		http.Error(w, "Error serializing task metrics", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
}

func handleLorenzCurve(w http.ResponseWriter, r *http.Request) {
	var graphData GraphData
	if err := json.Unmarshal(processedData, &graphData); err != nil {
		log.Printf("Error parsing JSON data: %v", err)
		http.Error(w, "Error processing data", http.StatusInternalServerError)
		return
	}
	agents := make([]struct {
		Agent     string
		Specialty string
		Value     float64
	}, 0)
	for _, node := range graphData.Nodes {
		if node.Role == "agent" {
			totalValue := 0.0
			for _, link := range graphData.Links {
				if link.Source == node.ID && link.Type == "assigned" {
					totalValue += link.WinningBid
				}
			}
			agents = append(agents, struct {
				Agent     string
				Specialty string
				Value     float64
			}{
				Agent:     node.Name,
				Specialty: node.Specialty,
				Value:     totalValue,
			})
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Value < agents[j].Value
	})
	lorenzCurve := make([]LorenzPoint, 0, len(agents)+1)
	lorenzCurve = append(lorenzCurve, LorenzPoint{X: 0, Y: 0})
	totalValue := 0.0
	for _, agent := range agents {
		totalValue += agent.Value
	}
	cumulativeValue := 0.0
	for i, agent := range agents {
		cumulativeValue += agent.Value
		lorenzCurve = append(lorenzCurve, LorenzPoint{
			X:         float64(i+1) / float64(len(agents)),
			Y:         cumulativeValue / totalValue,
			Agent:     agent.Agent,
			Specialty: agent.Specialty,
			Value:     agent.Value,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(lorenzCurve)
	if err != nil {
		http.Error(w, "Error serializing Lorenz curve data", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
}

func main() {
	port := flag.Int("port", 8080, "Port to serve on")
	jsonFile := flag.String("input", "", "Path to network data JSON file")
	templateFile := flag.String("template", "index.html", "Path to HTML template file")
	flag.Parse()
	if *jsonFile == "" {
		if len(flag.Args()) > 0 {
			*jsonFile = flag.Args()[0]
		} else {
			fmt.Println("Usage: go run dashboard.go -input <network_data.json> [-port 8080] [-template index.html]")
			os.Exit(1)
		}
	}
	jsonData, err := ioutil.ReadFile(*jsonFile)
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}
	var graphData GraphData
	if err := json.Unmarshal(jsonData, &graphData); err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}
	// Ensure specialty is set for each agent node
	for i, node := range graphData.Nodes {
		if node.Role == "agent" {
			if node.Specialty == "" {
				graphData.Nodes[i].Specialty = node.Name
			}
		}
	}
	degreeCentralities := degreeCentrality(graphData)
	for i, node := range graphData.Nodes {
		graphData.Nodes[i].Degree = degreeCentralities[node.ID]
	}
	processedData, err = json.Marshal(graphData)
	if err != nil {
		log.Fatalf("Error encoding JSON: %v", err)
	}
	advancedMetrics = calculateMarketMetrics(graphData)
	indexTemplate, err = template.ParseFiles(*templateFile)
	if err != nil {
		log.Fatalf("Error loading template: %v", err)
	}
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/data", handleData)
	http.HandleFunc("/api/market-metrics", handleMarketMetrics)
	http.HandleFunc("/api/agent-metrics", handleAgentMetrics)
	http.HandleFunc("/api/task-metrics", handleTaskMetrics)
	http.HandleFunc("/api/lorenz-curve", handleLorenzCurve)
	if _, err := os.Stat("static"); err == nil {
		fs := http.FileServer(http.Dir("static"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))
	}
	serverAddr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Starting server at http://localhost%s\n", serverAddr)
	fmt.Println("Press Ctrl+C to stop")
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
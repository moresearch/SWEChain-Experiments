package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
)

type Node struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Name   string `json:"name"`
	Degree int    `json:"degree"`
}

type Link struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"`
	Label      string  `json:"label,omitempty"`
	Weight     int     `json:"weight"`
	BidCount   int     `json:"bid_count"`
	WinningBid float64 `json:"winning_bid"`
	Specialist bool    `json:"specialist"`
}

type GraphData struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

var processedData []byte

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

func calculateNetworkMetrics(graphData GraphData) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Count nodes by type
	agentCount := 0
	taskCount := 0
	for _, node := range graphData.Nodes {
		if node.Group == "agent" {
			agentCount++
		} else if node.Group == "task" {
			taskCount++
		}
	}

	// Count links by type
	assignedCount := 0
	outsourcedCount := 0
	biddedCount := 0
	totalLinks := len(graphData.Links)

	// Task metrics
	totalBidCount := 0
	totalWinningBids := 0.0
	bidValues := []float64{}
	noBidCount := 0
	validBidCount := 0
	specialistWins := 0
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

			if link.Specialist {
				specialistWins++
			}

			// Only count tasks assigned to agents
			sourceIsAgent := false
			targetIsTask := false

			for _, node := range graphData.Nodes {
				if node.ID == link.Source && node.Group == "agent" {
					sourceIsAgent = true
				}
				if node.ID == link.Target && node.Group == "task" {
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

	// Network density calculation
	possibleLinks := agentCount*(agentCount-1) + agentCount*taskCount
	if possibleLinks > 0 {
		metrics["networkDensity"] = float64(totalLinks) / float64(possibleLinks)
	} else {
		metrics["networkDensity"] = 0.0
	}

	// Outsourcing and bidding ratios
	metrics["outsourcingRatio"] = float64(outsourcedCount) / float64(totalLinks)
	metrics["biddingRatio"] = float64(biddedCount) / float64(totalLinks)

	// Specialist win rate
	if totalSpecialistTasks > 0 {
		metrics["specialistWinRate"] = float64(specialistWins) / float64(totalSpecialistTasks)
	} else {
		metrics["specialistWinRate"] = 0.0
	}

	// Average task cost (from winning bids)
	if len(bidValues) > 0 {
		metrics["avgTaskCost"] = totalWinningBids / float64(len(bidValues))
	} else {
		metrics["avgTaskCost"] = 0.0
	}

	// Average number of bidders per task
	if validBidCount > 0 {
		metrics["avgBiddersPerTask"] = float64(totalBidCount) / float64(validBidCount)
	} else {
		metrics["avgBiddersPerTask"] = 0.0
	}

	// Average winning bid price
	metrics["avgWinningBidPrice"] = metrics["avgTaskCost"]

	// Bid variance calculation
	if len(bidValues) > 1 {
		mean := totalWinningBids / float64(len(bidValues))
		sumSquaredDiff := 0.0
		for _, value := range bidValues {
			diff := value - mean
			sumSquaredDiff += diff * diff
		}
		variance := sumSquaredDiff / float64(len(bidValues))
		metrics["bidVariance"] = math.Sqrt(variance) / mean // Coefficient of variation as percentage
	} else {
		metrics["bidVariance"] = 0.0
	}

	// No-bid rate
	if assignedCount > 0 {
		metrics["noBidRate"] = float64(noBidCount) / float64(assignedCount)
	} else {
		metrics["noBidRate"] = 0.0
	}

	// No-auction rate (same as no-bid rate in this model)
	metrics["noAuctionRate"] = metrics["noBidRate"]

	return metrics
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// Parse the JSON data to calculate metrics
	var graphData GraphData
	if err := json.Unmarshal(processedData, &graphData); err != nil {
		log.Printf("Error parsing JSON data: %v", err)
		return
	}

	// Calculate metrics from the data
	metrics := calculateNetworkMetrics(graphData)

	// Format metrics for display
	networkDensity := fmt.Sprintf("%.2f", metrics["networkDensity"].(float64))
	outsourcingRatio := fmt.Sprintf("%.0f%%", metrics["outsourcingRatio"].(float64)*100)
	biddingRatio := fmt.Sprintf("%.0f%%", metrics["biddingRatio"].(float64)*100)
	specialistWinRate := fmt.Sprintf("%.1f%%", metrics["specialistWinRate"].(float64)*100)
	avgTaskCost := fmt.Sprintf("$%.2f", metrics["avgTaskCost"].(float64))
	avgBiddersPerTask := fmt.Sprintf("%.1f", metrics["avgBiddersPerTask"].(float64))
	avgWinningBidPrice := fmt.Sprintf("$%.2f", metrics["avgWinningBidPrice"].(float64))
	bidVariance := fmt.Sprintf("%.1f%%", metrics["bidVariance"].(float64)*100)
	noBidRate := fmt.Sprintf("%.1f%%", metrics["noBidRate"].(float64)*100)
	noAuctionRate := fmt.Sprintf("%.1f%%", metrics["noAuctionRate"].(float64)*100)

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Economic Network Visualization</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <script src="https://unpkg.com/d3-sankey@0.12.3/dist/d3-sankey.min.js"></script>
    <script src="https://d3js.org/d3-chord.v2.min.js"></script>
    <style>
        :root {
            --agent-color: #4285F4;
            --task-color: #AA46BC;
            --out-color: #EA4335;
            --bid-color: #34A853;
            --assign-color: #FBBC05;
            --accent-color: #1A73E8;
            --text-color: #202124;
            --border-color: #DADCE0;
            --bg-color: #FFFFFF;
            --panel-color: #F8F9FA;
        }
        body {margin:0; font-family:system-ui,sans-serif; background:var(--bg-color); color:var(--text-color);}
        .container {display:grid; grid-template-columns:45%% 55%%; grid-template-rows:100vh; width:100%%;}
        #network-container {position:relative; overflow:hidden; background:var(--panel-color); border-right:1px solid var(--border-color);}
        #analysis-container {overflow-y:auto; padding:16px; display:grid; grid-template-columns:1fr 1fr; gap:16px;
            grid-template-areas:"controls controls" "metrics metrics" "degree-chart sankey-chart" "chord-chart chord-chart";}
        svg {background:var(--bg-color); border-radius:6px; box-shadow:0 1px 3px rgba(0,0,0,0.1);}
        .chart-container {background:var(--bg-color); border-radius:6px; padding:16px; 
            box-shadow:0 1px 3px rgba(0,0,0,0.1); border:1px solid var(--border-color);}
        #controls-area {grid-area:controls;}
        #metrics-area {grid-area:metrics;}
        #degree-chart-area {grid-area:degree-chart;}
        #sankey-chart-area {grid-area:sankey-chart;}
        #chord-chart-area {grid-area:chord-chart;}
        h2, h3 {color:var(--text-color); margin-top:0; border-bottom:1px solid var(--border-color); padding-bottom:8px;}
        .node {stroke:var(--bg-color); stroke-width:1.5px; cursor:pointer; transition:all 0.3s;}
        .node:hover {transform:scale(1.1); stroke-width:2px;}
        .link {stroke-opacity:0.7; transition:all 0.3s;}
        .link:hover {stroke-width:3px; stroke-opacity:0.9;}
        .node-label {font-size:12px; fill:var(--text-color); pointer-events:none; text-anchor:middle;
            text-shadow:0 1px 2px #fff, 1px 0 2px #fff, -1px 0 2px #fff, 0 -1px 2px #fff; font-weight:500;}
        .link-label {font-size:10px; fill:var(--text-color); pointer-events:none; text-anchor:middle;
            text-shadow:0 1px 2px #fff, 1px 0 2px #fff, -1px 0 2px #fff, 0 -1px 2px #fff; font-weight:500;}
        .agent {fill:var(--agent-color);}
        .agent:hover {fill:#2A75E8;}
        .task {fill:var(--task-color);}
        .task:hover {fill:#993DB0;}
        .outsourced {stroke:var(--out-color); stroke-width:2px;}
        .outsourced:hover {stroke-width:3.5px;}
        .bidded {stroke:var(--bid-color); stroke-dasharray:4,4;}
        .bidded:hover {stroke-width:3px;}
        .assigned {stroke:var(--assign-color);}
        #tooltip {position:absolute; padding:8px 12px; background:rgba(255,255,255,0.95); border:1px solid var(--border-color);
            border-radius:6px; font-size:12px; box-shadow:0 3px 6px rgba(0,0,0,0.16); display:none; 
            pointer-events:none; z-index:10; color:var(--text-color); max-width:300px;}
        .tooltip-title {font-weight:600; margin-bottom:4px; border-bottom:1px solid var(--border-color); 
            padding-bottom:4px; color:var(--accent-color);}
        .legend {position:absolute; top:20px; right:20px; background:rgba(255,255,255,0.95); padding:12px;
            border-radius:6px; box-shadow:0 3px 6px rgba(0,0,0,0.16); font-size:12px; line-height:1.5;
            color:var(--text-color); border:1px solid var(--border-color);}
        .legend-title {font-weight:600; margin-bottom:8px; border-bottom:1px solid var(--border-color); padding-bottom:4px;}
        .legend-item {display:flex; align-items:center; margin-bottom:6px;}
        .legend-color {width:12px; height:12px; margin-right:8px; border-radius:50%%;}
        .legend-line {width:20px; height:2px; margin-right:8px;}
        .controls {padding:16px; border-radius:6px; border:1px solid var(--border-color);}
        .control-group {margin-bottom:12px;}
        label {display:block; margin-bottom:5px; color:var(--text-color); font-weight:500;}
        input[type="range"] {width:100%%; background:var(--border-color); height:6px; border-radius:3px; 
            outline:none; -webkit-appearance:none;}
        input[type="range"]::-webkit-slider-thumb {-webkit-appearance:none; width:15px; height:15px; 
            border-radius:50%%; background:var(--accent-color); cursor:pointer;}
        .modal {display:none; position:fixed; z-index:11; left:0; top:0; width:100%%; height:100%%; 
            overflow:auto; background-color:rgba(0,0,0,0.4);}
        .modal-content {background:var(--bg-color); margin:10%% auto; padding:24px; border:1px solid var(--border-color);
            width:50%%; border-radius:6px; position:relative; box-shadow:0 4px 18px rgba(0,0,0,0.12);
            color:var(--text-color); max-height:80vh; overflow-y:auto;}
        .close {color:#aaa; float:right; font-size:28px; font-weight:bold; position:absolute; 
            top:10px; right:16px; cursor:pointer;}
        .close:hover {color:var(--text-color);}
        #modal-title {color:var(--text-color); border-bottom:1px solid var(--border-color); 
            padding-bottom:10px; margin-top:0; padding-right:30px;}
        .axis line, .axis path {stroke:var(--border-color);}
        .axis text {fill:var(--text-color); font-size:11px;}
        .bar {fill:var(--accent-color);}
        .bar:hover {fill:#0D67DD;}
        .node-sankey rect {cursor:pointer; transition:opacity 0.3s;}
        .node-sankey rect:hover {opacity:0.8;}
        .link-sankey {fill:none; stroke-opacity:0.5; transition:stroke-opacity 0.3s;}
        .link-sankey:hover {stroke-opacity:0.9;}
        .chord-arc {transition:opacity 0.3s;}
        .chord-arc:hover {opacity:0.8;}
        .chord {opacity:0.7; transition:opacity 0.3s;}
        .chord:hover {opacity:1;}
        .timestamp {font-size:12px; color:#5F6368; margin-top:20px; text-align:right; grid-column:span 2;}
        .chart-desc {font-size:13px; color:#5F6368; margin-bottom:16px;}
        .metrics-table {width:100%%; border-collapse:collapse; margin-bottom:12px; font-size:13px;}
        .metrics-table th {text-align:left; padding:8px; background:var(--panel-color); 
            font-weight:600; border-bottom:1px solid var(--border-color);}
        .metrics-table td {padding:8px; border-bottom:1px solid var(--border-color);}
        .metrics-table tr:last-child td {border-bottom:none;}
        .metrics-table tr:nth-child(even) {background:#f8f9fa;}
        .metrics-table .value-cell {text-align:right; font-family:monospace; font-weight:500;}
        .metrics-grid {display:grid; grid-template-columns:1fr 1fr; gap:16px;}
        .metrics-card {padding:12px; border-radius:6px; border:1px solid var(--border-color); background:var(--bg-color);}
        .metrics-card h5 {margin-top:0; margin-bottom:4px; font-weight:600; color:var(--text-color);}
        .metric-value {font-size:18px; font-weight:700; color:var(--accent-color);}
        .metric-description {font-size:12px; color:#5F6368; margin-top:4px;}
    </style>
</head>
<body>
    <div class="container">
        <div id="network-container">
            <div id="tooltip"></div>
            <div class="legend">
                <div class="legend-title">Network Legend</div>
                <div class="legend-item"><div class="legend-color" style="background:var(--agent-color);"></div><span>Agent</span></div>
                <div class="legend-item"><div class="legend-color" style="background:var(--task-color);"></div><span>Task</span></div>
                <div class="legend-item"><div class="legend-line" style="background:var(--out-color);"></div><span>Outsourced</span></div>
                <div class="legend-item"><div class="legend-line" style="background:var(--bid-color);border-top:2px dashed var(--bid-color);height:0;"></div><span>Bidded</span></div>
                <div class="legend-item"><div class="legend-line" style="background:var(--assign-color);"></div><span>Assigned</span></div>
            </div>
        </div>
        
        <div id="analysis-container">
            <div id="controls-area" class="chart-container">
                <h3>Network Controls</h3>
                <div class="control-group">
                    <label for="charge-strength">Force Strength</label>
                    <input type="range" id="charge-strength" min="-200" max="-10" value="-70">
                </div>
                <div class="control-group">
                    <label for="link-distance">Link Distance</label>
                    <input type="range" id="link-distance" min="10" max="300" value="100">
                </div>
            </div>
            
            <div id="metrics-area" class="chart-container">
                <h3>Network Metrics</h3>
                <div class="metrics-grid">
                    <div class="metrics-card">
                        <h5>Network Density</h5>
                        <div class="metric-value">%s</div>
                        <div class="metric-description">More links = more collaboration</div>
                    </div>
                    <div class="metrics-card">
                        <h5>Outsourcing Ratio</h5>
                        <div class="metric-value">%s</div>
                        <div class="metric-description">%% of tasks outsourced</div>
                    </div>
                    <div class="metrics-card">
                        <h5>Bidding Ratio</h5>
                        <div class="metric-value">%s</div>
                        <div class="metric-description">%% of competitive bidding</div>
                    </div>
                    <div class="metrics-card">
                        <h5>Specialist Win Rate</h5>
                        <div class="metric-value">%s</div>
                        <div class="metric-description">%% of wins by specialists</div>
                    </div>
                </div>
                <table class="metrics-table">
                    <tr><th>Metric</th><th>Value</th></tr>
                    <tr><td>Avg. Task Cost</td><td class="value-cell">%s</td></tr>
                    <tr><td>Avg. Bidders per Task</td><td class="value-cell">%s</td></tr>
                    <tr><td>Avg. Winning Bid Price</td><td class="value-cell">%s</td></tr>
                    <tr><td>Bid Variance</td><td class="value-cell">%s</td></tr>
                    <tr><td>No-Bid Rate</td><td class="value-cell">%s</td></tr>
                    <tr><td>No-Auction Rate</td><td class="value-cell">%s</td></tr>
                </table>
            </div>
            
            <div id="degree-chart-area" class="chart-container">
                <h3>Top Nodes by Connectivity</h3>
                <div id="degree-barchart"></div>
            </div>
            
            <div id="sankey-chart-area" class="chart-container">
                <h3>Resource Flow Diagram</h3>
                <div id="sankey-diagram"></div>
            </div>
            
            <div id="chord-chart-area" class="chart-container">
                <h3>Agent Relationship Diagram</h3>
                <div id="chord-diagram"></div>
            </div>
            
            <div class="timestamp">
                <div>Generated: <span id="current-time"></span></div>
                <div>User: <span id="current-user"></span></div>
            </div>
        </div>
    </div>
    
    <div id="modal" class="modal">
      <div class="modal-content">
        <span class="close">&times;</span>
        <h2 id="modal-title">Node Details</h2>
        <div id="modal-body"></div>
      </div>
    </div>
    
    <script>
        // Set dynamic client-side time and user info
        document.getElementById('current-time').textContent = new Date().toISOString().replace('T', ' ').substring(0, 19);
        document.getElementById('current-user').textContent = (navigator && navigator.userAgent) ? 
            navigator.userAgent.split(' ')[0].substring(0, 15) : 'client';
    
        // Fetch network data
        fetch('/data')
          .then(response => response.json())
          .then(graphData => {
            // Set up visualization
            const container = d3.select('#network-container');
            const width = container.node().getBoundingClientRect().width;
            const height = container.node().getBoundingClientRect().height;

            const svg = d3.select('#network-container').append('svg')
                .attr('width', width)
                .attr('height', height)
                .call(d3.zoom().on('zoom', e => g.attr('transform', e.transform)));

            const g = svg.append('g');

            // Define arrowhead markers
            g.append('defs').selectAll('marker')
                .data([
                    {id: 'outsourced-arrow', color: getComputedStyle(document.documentElement).getPropertyValue('--out-color')},
                    {id: 'bidded-arrow', color: getComputedStyle(document.documentElement).getPropertyValue('--bid-color')},
                    {id: 'assigned-arrow', color: getComputedStyle(document.documentElement).getPropertyValue('--assign-color')}
                ])
                .enter().append('marker')
                .attr('id', d => d.id)
                .attr('viewBox', '-0 -5 10 10')
                .attr('refX', 18)
                .attr('refY', 0)
                .attr('orient', 'auto')
                .attr('markerWidth', 8)
                .attr('markerHeight', 8)
                .append('path')
                .attr('d', 'M0,-5L10,0L0,5')
                .attr('fill', d => d.color);

            // Create force simulation
            let chargeStrength = -70;
            let linkDistance = 100;
            
            const simulation = d3.forceSimulation(graphData.nodes)
                .force('link', d3.forceLink(graphData.links).id(d => d.id).distance(linkDistance))
                .force('charge', d3.forceManyBody().strength(chargeStrength))
                .force('center', d3.forceCenter(width / 2, height / 2))
                .force('collision', d3.forceCollide().radius(35));

            // Create links with improved colors
            const links = g.selectAll('.link')
                .data(graphData.links)
                .enter()
                .append('line')
                .attr('class', d => 'link ' + d.type)
                .style('stroke-width', d => d.weight * 0.8 + 'px')
                .attr('marker-end', d => {
                    if (d.type === 'outsourced') return 'url(#outsourced-arrow)';
                    if (d.type === 'bidded') return 'url(#bidded-arrow)';
                    return 'url(#assigned-arrow)';
                });

            // Create nodes
            const nodes = g.selectAll('.node')
                .data(graphData.nodes)
                .enter()
                .append('circle')
                .attr('class', d => 'node ' + d.group)
                .attr('r', d => 10 + Math.sqrt(d.degree || 1) * 2.5)
                .call(drag(simulation))
                .on('mouseover', showTooltip)
                .on('mouseout', hideTooltip)
                .on('click', showNodeDetails);

            // Create node labels
            const nodeLabels = g.selectAll('.node-label')
                .data(graphData.nodes)
                .enter()
                .append('text')
                .attr('class', 'node-label')
                .text(d => d.name);

            // Add link labels for important connections
            g.selectAll('.link-label')
                .data(graphData.links.filter(d => d.weight >= 4))
                .enter()
                .append('text')
                .attr('class', 'link-label')
                .text(d => d.label || '')
                .style('font-size', d => 8 + d.weight * 0.5 + 'px');

            // Define drag behavior
            function drag(simulation) {
                function dragstarted(event, d) {
                    if (!event.active) simulation.alphaTarget(0.3).restart();
                    d.fx = d.x;
                    d.fy = d.y;
                }

                function dragged(event, d) {
                    d.fx = event.x;
                    d.fy = event.y;
                }

                function dragended(event, d) {
                    if (!event.active) simulation.alphaTarget(0);
                    d.fx = null;
                    d.fy = null;
                }

                return d3.drag()
                    .on('start', dragstarted)
                    .on('drag', dragged)
                    .on('end', dragended);
            }

            const tooltip = d3.select('#tooltip');

            function showTooltip(event, d) {
                const connections = graphData.links.filter(link => 
                    (typeof link.source === 'object' ? link.source.id : link.source) === d.id || 
                    (typeof link.target === 'object' ? link.target.id : link.target) === d.id
                );
                
                tooltip.style('display', 'block')
                    .style('left', (event.pageX + 15) + 'px')
                    .style('top', (event.pageY - 10) + 'px')
                    .html('<div class="tooltip-title">' + d.name + '</div>' +
                          '<div>Type: ' + d.group + '</div>' +
                          '<div>Connections: ' + (d.degree || connections.length) + '</div>');
            }

            function hideTooltip() {
                tooltip.style('display', 'none');
            }

            // Modal functionality
            const modal = document.getElementById("modal");
            const span = document.getElementsByClassName("close")[0];
            span.onclick = () => modal.style.display = "none";
            window.onclick = e => { if (e.target == modal) modal.style.display = "none"; };

            function showNodeDetails(event, d) {
                modal.style.display = "block";
                document.getElementById("modal-title").innerHTML = d.name;
                
                const connections = graphData.links.filter(link => 
                    (typeof link.source === 'object' ? link.source.id : link.source) === d.id || 
                    (typeof link.target === 'object' ? link.target.id : link.target) === d.id
                );
                
                let incomingByType = {}, outgoingByType = {};
                
                connections.forEach(link => {
                    const sourceId = typeof link.source === 'object' ? link.source.id : link.source;
                    const targetId = typeof link.target === 'object' ? link.target.id : link.target;
                    
                    if (targetId === d.id) {
                        incomingByType[link.type] = (incomingByType[link.type] || 0) + 1;
                    } else {
                        outgoingByType[link.type] = (outgoingByType[link.type] || 0) + 1;
                    }
                });
                
                let detailsContent = '<p><strong>ID:</strong> ' + d.id + '</p>' +
                    '<p><strong>Type:</strong> ' + d.group + '</p>' +
                    '<p><strong>Connections:</strong> ' + (d.degree || connections.length) + '</p>';
                
                if (Object.keys(incomingByType).length > 0) {
                    detailsContent += '<div style="font-weight:600;margin-top:12px;">Incoming</div><ul style="list-style:none;padding-left:0;">';
                    for (const [type, count] of Object.entries(incomingByType)) {
                        detailsContent += '<li>' + count + ' ' + type + '</li>';
                    }
                    detailsContent += '</ul>';
                }
                
                if (Object.keys(outgoingByType).length > 0) {
                    detailsContent += '<div style="font-weight:600;margin-top:12px;">Outgoing</div><ul style="list-style:none;padding-left:0;">';
                    for (const [type, count] of Object.entries(outgoingByType)) {
                        detailsContent += '<li>' + count + ' ' + type + '</li>';
                    }
                    detailsContent += '</ul>';
                }
                
                document.getElementById("modal-body").innerHTML = detailsContent;
            }

            // Update positions on each tick of the simulation
            simulation.on('tick', () => {
                links
                    .attr('x1', d => d.source.x)
                    .attr('y1', d => d.source.y)
                    .attr('x2', d => d.target.x)
                    .attr('y2', d => d.target.y);

                nodes
                    .attr('cx', d => d.x)
                    .attr('cy', d => d.y);

                nodeLabels
                    .attr('x', d => d.x)
                    .attr('y', d => d.y + 4);
                    
                g.selectAll('.link-label')
                    .attr('x', d => (d.source.x + d.target.x) / 2)
                    .attr('y', d => (d.source.y + d.target.y) / 2);
            });

            // Add force simulation controls
            d3.select('#charge-strength').on('input', function() {
                simulation.force('charge').strength(+this.value);
                simulation.alpha(0.3).restart();
            });
            
            d3.select('#link-distance').on('input', function() {
                simulation.force('link').distance(+this.value);
                simulation.alpha(0.3).restart();
            });

            // Create visualization charts
            createDegreeBarchart();
            createSankeyDiagram();
            createChordDiagram();

            function createDegreeBarchart() {
                const margin = {top: 20, right: 20, bottom: 50, left: 40},
                    width = document.getElementById('degree-barchart').clientWidth - margin.left - margin.right,
                    height = 200 - margin.top - margin.bottom;
                
                const svg = d3.select('#degree-barchart').append('svg')
                    .attr('width', width + margin.left + margin.right)
                    .attr('height', height + margin.top + margin.bottom)
                    .append('g')
                    .attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                
                // Calculate degrees if not already present
                if (!graphData.nodes[0].degree) {
                    const degreeMap = {};
                    graphData.links.forEach(link => {
                        const sourceId = typeof link.source === 'object' ? link.source.id : link.source;
                        const targetId = typeof link.target === 'object' ? link.target.id : link.target;
                        degreeMap[sourceId] = (degreeMap[sourceId] || 0) + 1;
                        degreeMap[targetId] = (degreeMap[targetId] || 0) + 1;
                    });
                    
                    graphData.nodes.forEach(node => {
                        node.degree = degreeMap[node.id] || 0;
                    });
                }
                
                // Get top nodes by degree
                const topNodes = [...graphData.nodes]
                    .sort((a, b) => b.degree - a.degree)
                    .slice(0, 5);
                
                // X axis
                const x = d3.scaleBand()
                    .range([0, width])
                    .domain(topNodes.map(d => d.name.substring(0, 10)))
                    .padding(0.2);
                    
                svg.append('g')
                    .attr('class', 'axis')
                    .attr('transform', 'translate(0,' + height + ')')
                    .call(d3.axisBottom(x))
                    .selectAll('text')
                    .attr('transform', 'translate(-10,0)rotate(-45)')
                    .style('text-anchor', 'end');
                
                // Y axis
                const y = d3.scaleLinear()
                    .domain([0, d3.max(topNodes, d => d.degree)])
                    .range([height, 0]);
                    
                svg.append('g')
                    .attr('class', 'axis')
                    .call(d3.axisLeft(y));
                
                // Get color values
                const agentColor = getComputedStyle(document.documentElement).getPropertyValue('--agent-color');
                const taskColor = getComputedStyle(document.documentElement).getPropertyValue('--task-color');
                
                // Bars
                svg.selectAll('.bar')
                    .data(topNodes)
                    .enter()
                    .append('rect')
                    .attr('class', 'bar')
                    .attr('x', d => x(d.name.substring(0, 10)))
                    .attr('width', x.bandwidth())
                    .attr('y', d => y(d.degree))
                    .attr('height', d => height - y(d.degree))
                    .attr('fill', d => d.group === 'agent' ? agentColor : taskColor)
                    .on('mouseover', function(event, d) {
                        tooltip.style('display', 'block')
                            .style('left', (event.pageX + 15) + 'px')
                            .style('top', (event.pageY - 10) + 'px')
                            .html('<div class="tooltip-title">' + d.name + '</div>' +
                                  '<div>Type: ' + d.group + '</div>' + 
                                  '<div>Connections: ' + d.degree + '</div>');
                    })
                    .on('mouseout', hideTooltip);
            }

            function createSankeyDiagram() {
                const width = document.getElementById('sankey-diagram').clientWidth - 20,
                    height = 300 - 20;
                    
                const svg = d3.select('#sankey-diagram').append('svg')
                    .attr('width', width)
                    .attr('height', height)
                    .append('g')
                    .attr('transform', 'translate(10,10)');
                
                try {
                    const agents = graphData.nodes.filter(d => d.group === 'agent');
                    const tasks = graphData.nodes.filter(d => d.group === 'task');
                    
                    const sankeyLinks = graphData.links
                        .filter(d => {
                            const sourceId = typeof d.source === 'object' ? d.source.id : d.source;
                            const targetId = typeof d.target === 'object' ? d.target.id : d.target;
                            const sourceNode = graphData.nodes.find(node => node.id === sourceId);
                            const targetNode = graphData.nodes.find(node => node.id === targetId);
                            return sourceNode && targetNode && 
                                ((sourceNode.group === 'agent' && targetNode.group === 'task') ||
                                 (sourceNode.group === 'task' && targetNode.group === 'agent'));
                        })
                        .map(d => ({
                            source: typeof d.source === 'object' ? d.source.id : d.source,
                            target: typeof d.target === 'object' ? d.target.id : d.target,
                            value: d.weight || 1
                        }));
                    
                    const sankeyNodes = [...agents, ...tasks].map(d => ({
                        id: d.id,
                        name: d.name,
                        group: d.group
                    }));
                    
                    if (typeof d3.sankey === 'function') {
                        const sankey = d3.sankey()
                            .nodeId(d => d.id)
                            .nodeWidth(15)
                            .nodePadding(10)
                            .extent([[1, 1], [width - 1, height - 5]]);
                            
                        const sankeyData = sankey({
                            nodes: sankeyNodes.map(d => Object.assign({}, d)),
                            links: sankeyLinks.map(d => Object.assign({}, d))
                        });
                        
                        const agentColor = getComputedStyle(document.documentElement).getPropertyValue('--agent-color');
                        const taskColor = getComputedStyle(document.documentElement).getPropertyValue('--task-color');
                        const assignColor = getComputedStyle(document.documentElement).getPropertyValue('--assign-color');
                        
                        // Add links
                        svg.append('g')
                            .selectAll('path')
                            .data(sankeyData.links)
                            .enter()
                            .append('path')
                            .attr('class', 'link-sankey')
                            .attr('d', d3.sankeyLinkHorizontal())
                            .attr('stroke', assignColor)
                            .attr('stroke-width', d => Math.max(1, d.width))
                            .on('mouseover', function(event, d) {
                                tooltip.style('display', 'block')
                                    .style('left', (event.pageX + 15) + 'px')
                                    .style('top', (event.pageY - 10) + 'px')
                                    .html('<div class="tooltip-title">Flow</div>' +
                                          '<div>' + d.source.name + ' → ' + d.target.name + '</div>' +
                                          '<div>Value: ' + d.value + '</div>');
                            })
                            .on('mouseout', hideTooltip);
                            
                        // Add nodes
                        svg.append('g')
                            .selectAll('rect')
                            .data(sankeyData.nodes)
                            .enter()
                            .append('rect')
                            .attr('class', 'node-sankey')
                            .attr('x', d => d.x0)
                            .attr('y', d => d.y0)
                            .attr('height', d => d.y1 - d.y0)
                            .attr('width', d => d.x1 - d.x0)
                            .attr('fill', d => d.group === 'agent' ? agentColor : taskColor)
                            .on('mouseover', function(event, d) {
                                tooltip.style('display', 'block')
                                    .style('left', (event.pageX + 15) + 'px')
                                    .style('top', (event.pageY - 10) + 'px')
                                    .html('<div class="tooltip-title">' + d.name + '</div>' +
                                          '<div>Type: ' + d.group + '</div>');
                            })
                            .on('mouseout', hideTooltip);
                            
                        // Add node labels
                        svg.append('g')
                            .selectAll('text')
                            .data(sankeyData.nodes)
                            .enter()
                            .append('text')
                            .attr('x', d => d.x0 < width / 2 ? d.x1 + 6 : d.x0 - 6)
                            .attr('y', d => (d.y1 + d.y0) / 2)
                            .attr('dy', '0.35em')
                            .attr('text-anchor', d => d.x0 < width / 2 ? 'start' : 'end')
                            .text(d => d.name)
                            .style('font-size', '10px')
                            .style('fill', 'var(--text-color)');
                    } else {
                        createSimpleSankey(svg, width, height, agents, tasks, sankeyLinks);
                    }
                } catch (error) {
                    console.error("Sankey error:", error);
                    createSimpleSankey(svg, width, height, 
                        graphData.nodes.filter(d => d.group === 'agent').slice(0, 5),
                        graphData.nodes.filter(d => d.group === 'task').slice(0, 5),
                        []);
                }
            }
            
            function createSimpleSankey(svg, width, height, agents, tasks, links) {
                svg.append('text')
                    .attr('x', width/2)
                    .attr('y', 20)
                    .attr('text-anchor', 'middle')
                    .text('Simple Flow Diagram');
                
                const agentX = width * 0.2;
                const taskX = width * 0.8;
                const ySpacing = height * 0.7 / Math.max(agents.length, tasks.length);
                
                // Draw agents
                svg.selectAll('.agent-node')
                    .data(agents.slice(0, 5))
                    .enter()
                    .append('circle')
                    .attr('cx', agentX)
                    .attr('cy', (d, i) => 50 + i * ySpacing)
                    .attr('r', 8)
                    .attr('fill', 'var(--agent-color)');
                    
                // Draw tasks
                svg.selectAll('.task-node')
                    .data(tasks.slice(0, 5))
                    .enter()
                    .append('circle')
                    .attr('cx', taskX)
                    .attr('cy', (d, i) => 50 + i * ySpacing)
                    .attr('r', 8)
                    .attr('fill', 'var(--task-color)');
                    
                // Add labels
                svg.selectAll('.agent-label')
                    .data(agents.slice(0, 5))
                    .enter()
                    .append('text')
                    .attr('x', agentX - 15)
                    .attr('y', (d, i) => 50 + i * ySpacing)
                    .attr('text-anchor', 'end')
                    .attr('dominant-baseline', 'middle')
                    .text(d => d.name.substring(0, 10))
                    .style('font-size', '10px');
                    
                svg.selectAll('.task-label')
                    .data(tasks.slice(0, 5))
                    .enter()
                    .append('text')
                    .attr('x', taskX + 15)
                    .attr('y', (d, i) => 50 + i * ySpacing)
                    .attr('text-anchor', 'start')
                    .attr('dominant-baseline', 'middle')
                    .text(d => d.name.substring(0, 10))
                    .style('font-size', '10px');
                    
                // Add some sample links
                for (let i = 0; i < Math.min(5, agents.length); i++) {
                    for (let j = 0; j < Math.min(5, tasks.length); j++) {
                        if (Math.random() > 0.7) {
                            svg.append('line')
                                .attr('x1', agentX)
                                .attr('y1', 50 + i * ySpacing)
                                .attr('x2', taskX)
                                .attr('y2', 50 + j * ySpacing)
                                .attr('stroke', 'var(--assign-color)')
                                .attr('stroke-width', 1 + Math.random() * 3)
                                .attr('opacity', 0.7);
                        }
                    }
                }
            }

            function createChordDiagram() {
                const width = document.getElementById('chord-diagram').clientWidth - 40,
                    height = 400 - 40,
                    outerRadius = Math.min(width, height) * 0.5 - 40,
                    innerRadius = outerRadius - 30;
                    
                const svg = d3.select('#chord-diagram').append('svg')
                    .attr('width', width + 40)
                    .attr('height', height + 40)
                    .append('g')
                    .attr('transform', 'translate(' + (width/2 + 20) + ',' + (height/2 + 20) + ')');
                
                // Filter to only include agent nodes
                const agentNodes = graphData.nodes.filter(node => node.group === 'agent');
                
                // Create a matrix of connections between agents
                const matrix = [];
                const nodeIds = agentNodes.map(node => node.id);
                
                // Initialize matrix with zeros
                for (let i = 0; i < nodeIds.length; i++) {
                    matrix[i] = [];
                    for (let j = 0; j < nodeIds.length; j++) {
                        matrix[i][j] = 0;
                    }
                }
                
                // Fill matrix with weights of connections
                graphData.links.forEach(link => {
                    const sourceId = typeof link.source === 'object' ? link.source.id : link.source;
                    const targetId = typeof link.target === 'object' ? link.target.id : link.target;
                    
                    const sourceIndex = nodeIds.indexOf(sourceId);
                    const targetIndex = nodeIds.indexOf(targetId);
                    
                    if (sourceIndex >= 0 && targetIndex >= 0) {
                        matrix[sourceIndex][targetIndex] = (matrix[sourceIndex][targetIndex] || 0) + (link.weight || 1);
                    }
                });
                
                try {
                    if (typeof d3.chord === 'function') {
                        // Create chord layout
                        const chord = d3.chord()
                            .padAngle(0.05)
                            .sortSubgroups(d3.descending);
                            
                        const chords = chord(matrix);
                        
                        // Create color scale with variations
                        const color = d3.scaleLinear()
                            .domain([0, nodeIds.length-1])
                            .range(['#236cdc', '#6fa1f7'])
                            .interpolate(d3.interpolateHcl);
                            
                        // Add outer arcs
                        const arc = d3.arc()
                            .innerRadius(innerRadius)
                            .outerRadius(outerRadius);
                            
                        const outerArcs = svg.selectAll('.outer-arc')
                            .data(chords.groups)
                            .enter()
                            .append('path')
                            .attr('class', 'chord-arc')
                            .attr('d', arc)
                            .attr('fill', d => color(d.index))
                            .attr('stroke', '#fff')
                            .style('stroke-width', 1)
                            .on('mouseover', function(event, d) {
                                tooltip.style('display', 'block')
                                    .style('left', (event.pageX + 15) + 'px')
                                    .style('top', (event.pageY - 10) + 'px')
                                    .html('<div class="tooltip-title">' + agentNodes[d.index].name + '</div>' +
                                          '<div>Agent</div>');
                            })
                            .on('mouseout', hideTooltip);
                            
                        // Add labels around the outer arcs
                        const outerArcLabels = svg.selectAll('.chord-label')
                            .data(chords.groups)
                            .enter()
                            .append('text')
                            .attr('class', 'chord-label')
                            .attr('dy', '.35em')
                            .attr('transform', function(d) {
                                const angle = (d.startAngle + d.endAngle) / 2;
                                const rotate = angle * 180 / Math.PI;
                                return 'rotate(' + (rotate < 180 ? rotate - 90 : rotate + 90) + ')' +
                                    'translate(' + (outerRadius + 10) + ')' +
                                    (rotate < 180 ? '' : 'rotate(180)');
                            })
                            .style('text-anchor', d => (d.startAngle + d.endAngle) / 2 < Math.PI ? 'start' : 'end')
                            .text(d => agentNodes[d.index].name.substring(0, 12))
                            .style('font-size', '10px')
                            .style('fill', 'var(--text-color)');
                            
                        // Add the chord paths
                        const ribbon = d3.ribbon()
                            .radius(innerRadius);
                            
                        svg.selectAll('.chord')
                            .data(chords)
                            .enter()
                            .append('path')
                            .attr('class', 'chord')
                            .attr('d', ribbon)
                            .attr('fill', d => color(d.source.index))
                            .attr('stroke', '#fff')
                            .style('stroke-width', 0.5)
                            .on('mouseover', function(event, d) {
                                tooltip.style('display', 'block')
                                    .style('left', (event.pageX + 15) + 'px')
                                    .style('top', (event.pageY - 10) + 'px')
                                    .html('<div class="tooltip-title">Relationship</div>' +
                                          '<div>' + agentNodes[d.source.index].name + 
                                          ' → ' + agentNodes[d.target.index].name + '</div>' +
                                          '<div>Strength: ' + d.source.value + '</div>');
                            })
                            .on('mouseout', hideTooltip);
                    } else {
                        createSimpleChord(svg, outerRadius, agentNodes);
                    }
                } catch (error) {
                    console.error("Chord error:", error);
                    createSimpleChord(svg, outerRadius, agentNodes.slice(0, 8));
                }
            }
            
            function createSimpleChord(svg, radius, agents) {
                svg.append('text')
                    .attr('x', 0)
                    .attr('y', -radius - 10)
                    .attr('text-anchor', 'middle')
                    .text('Agent Relationships');
                
                const agentCount = Math.min(agents.length, 8);
                
                for (let i = 0; i < agentCount; i++) {
                    const angle = (i / agentCount) * 2 * Math.PI;
                    const x = Math.sin(angle) * radius;
                    const y = -Math.cos(angle) * radius;
                    
                    // Add agent circle
                    svg.append('circle')
                        .attr('cx', x)
                        .attr('cy', y)
                        .attr('r', 15)
                        .attr('fill', 'var(--agent-color)');
                        
                    // Add agent label
                    svg.append('text')
                        .attr('x', x)
                        .attr('y', y)
                        .attr('text-anchor', 'middle')
                        .attr('dominant-baseline', 'middle')
                        .text(i + 1)
                        .style('font-size', '10px')
                        .style('fill', 'white')
                        .style('font-weight', 'bold');
                        
                    // Add agent name
                    svg.append('text')
                        .attr('x', Math.sin(angle) * (radius + 25))
                        .attr('y', -Math.cos(angle) * (radius + 25))
                        .attr('text-anchor', 'middle')
                        .text(agents[i].name.substring(0, 10))
                        .style('font-size', '12px');
                }
                
                // Add some connections
                for (let i = 0; i < agentCount; i++) {
                    for (let j = i + 1; j < agentCount; j++) {
                        if (Math.random() > 0.7) {
                            const angle1 = (i / agentCount) * 2 * Math.PI;
                            const angle2 = (j / agentCount) * 2 * Math.PI;
                            const x1 = Math.sin(angle1) * radius;
                            const y1 = -Math.cos(angle1) * radius;
                            const x2 = Math.sin(angle2) * radius;
                            const y2 = -Math.cos(angle2) * radius;
                            
                            svg.append('path')
                                .attr('d', 'M' + x1 + ',' + y1 + ' Q0,0 ' + x2 + ',' + y2)
                                .attr('stroke', 'var(--accent-color)')
                                .attr('stroke-width', 1 + Math.random() * 3)
                                .attr('fill', 'none')
                                .attr('opacity', 0.7);
                        }
                    }
                }
            }
          })
          .catch(error => {
              console.error("Error loading network data:", error);
              document.getElementById('network-container').innerHTML = 
                  '<div style="text-align:center;margin-top:100px;"><h2>Error loading network data</h2>' +
                  '<p>Please check your data file.</p></div>';
          });
    </script>
</body>
</html>`, networkDensity, outsourcingRatio, biddingRatio, specialistWinRate,
		avgTaskCost, avgBiddersPerTask, avgWinningBidPrice, bidVariance, noBidRate, noAuctionRate)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlContent))
}

// Handle data requests
func handleData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(processedData)
}

func main() {
	port := flag.Int("port", 8080, "Port to serve on")
	jsonFile := flag.String("input", "", "Path to network data JSON file")
	flag.Parse()

	if *jsonFile == "" {
		if len(flag.Args()) > 0 {
			*jsonFile = flag.Args()[0]
		} else {
			fmt.Println("Usage: go run network_viz.go -input <network_data.json> [-port 8080]")
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

	degreeCentralities := degreeCentrality(graphData)
	for i, node := range graphData.Nodes {
		graphData.Nodes[i].Degree = degreeCentralities[node.ID]
	}

	processedData, err = json.Marshal(graphData)
	if err != nil {
		log.Fatalf("Error encoding JSON: %v", err)
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/data", handleData)

	serverAddr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Starting server at http://localhost%s\n", serverAddr)
	fmt.Println("Press Ctrl+C to stop")

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

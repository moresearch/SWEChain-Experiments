package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Node represents an entity in the network (agent, task, etc.)
type Node struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Label    string            `json:"label"`
	Group    string            `json:"group,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Edge represents a connection or relationship between two nodes.
type Edge struct {
	Source   string            `json:"source"`
	Target   string            `json:"target"`
	Type     string            `json:"type"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Network holds nodes, edges, and (added) metrics.
type Network struct {
	Nodes   []Node            `json:"nodes"`
	Edges   []Edge            `json:"edges"`
	Metrics map[string]string `json:"metrics,omitempty"`
}

var (
	mu      sync.Mutex
	network Network
)

// computeMetrics calculates simple economic network metrics.
func computeMetrics(nodes []Node, edges []Edge) map[string]string {
	if len(nodes) == 0 {
		return map[string]string{}
	}
	// Node counts by type
	nodeTypeCount := map[string]int{}
	for _, n := range nodes {
		nodeTypeCount[n.Type]++
	}

	// Degree (connections per node)
	degree := map[string]int{}
	for _, e := range edges {
		degree[e.Source]++
		degree[e.Target]++
	}
	totalDegree := 0
	for _, d := range degree {
		totalDegree += d
	}
	avgDegree := float64(totalDegree) / float64(len(nodes))

	// Edge types
	edgeTypeCount := map[string]int{}
	for _, e := range edges {
		edgeTypeCount[e.Type]++
	}

	// Find the node with the maximum degree (network hub)
	maxDeg, hub := 0, ""
	for id, d := range degree {
		if d > maxDeg {
			maxDeg = d
			hub = id
		}
	}

	// Prepare metrics
	metrics := map[string]string{
		"Node count":          fmt.Sprintf("%d", len(nodes)),
		"Edge count":          fmt.Sprintf("%d", len(edges)),
		"Avg node degree":     fmt.Sprintf("%.2f", avgDegree),
		"Node type breakdown": formatMapInt(nodeTypeCount),
		"Edge type breakdown": formatMapInt(edgeTypeCount),
		"Network hub":         hub + " (degree " + fmt.Sprintf("%d", maxDeg) + ")",
	}
	return metrics
}

func formatMapInt(m map[string]int) string {
	parts := []string{}
	for k, v := range m {
		if k == "" {
			k = "(none)"
		}
		parts = append(parts, k+": "+fmt.Sprintf("%d", v))
	}
	return strings.Join(parts, ", ")
}

// serveDashboard renders the main dashboard page.
func serveDashboard(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if network.Edges == nil {
		network.Edges = []Edge{}
	}
	if network.Nodes == nil {
		network.Nodes = []Node{}
	}
	network.Metrics = computeMetrics(network.Nodes, network.Edges)
	networkJSON, err := json.Marshal(network)
	if err != nil {
		log.Printf("[serveDashboard] Failed to marshal network: %v", err)
	}
	mu.Unlock()
	page := dashboardHTML
	tmpl := template.Must(template.New("dashboard").Parse(page))
	err = tmpl.Execute(w, map[string]interface{}{
		"Network": template.JS(networkJSON),
	})
	if err != nil {
		log.Printf("[serveDashboard] Failed to execute template: %v", err)
	}
}

const dashboardHTML = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Network Dashboard</title>
  <script src="https://d3js.org/d3.v7.min.js"></script>
  <link href="https://fonts.googleapis.com/css?family=Inter:400,600&display=swap" rel="stylesheet">
  <style>
    body {
      font-family: 'Inter', Lato, Arial, sans-serif;
      background: #f8fafc;
      color: #1e293b;
      margin: 0;
      min-height: 100vh;
    }
    .dashboard-title {
      font-size: 2.1em;
      text-align: center;
      color: #2563eb;
      margin: 32px 0 8px 0;
      font-weight: 600;
      letter-spacing: 0.5px;
    }
    .summary-cards {
      display: flex;
      flex-wrap: wrap;
      justify-content: center;
      gap: 24px;
      margin-bottom: 32px;
      max-width: 1200px;
      margin-left: auto; margin-right: auto;
    }
    .card {
      background: #fff;
      border-radius: 16px;
      box-shadow: 0 2px 16px #e0e7ef;
      padding: 18px 36px 10px 36px;
      min-width: 220px;
      flex: 1 1 220px;
      max-width: 380px;
      margin: 0 8px;
    }
    .card-title { font-size: 1.18em; color: #2d3748; margin-bottom: 4px; font-weight: 600;}
    .card-content { font-size: 1.45em; font-weight: 500; }
    .visual-section {
      display: flex;
      flex-wrap: wrap;
      gap: 32px;
      justify-content: center;
      max-width: 1500px;
      margin: 0 auto 48px auto;
    }
    .viz-card {
      background: #fff;
      border-radius: 16px;
      box-shadow: 0 2px 16px #e0e7ef;
      padding: 22px 18px 10px 18px;
      min-width: 450px;
      flex: 1 1 680px;
      max-width: 820px;
      margin: 0 8px 30px 8px;
      text-align: center;
      position: relative;
    }
    .viz-title { font-size: 1.13em; color: #2d3748; font-weight: 600; margin-bottom: 8px;}
    .viz-tooltip {
      font-size: 0.94em;
      color: #444;
      background: #e3e9f6;
      border-radius: 8px;
      padding: 8px 18px;
      position: absolute;
      top: 12px;
      right: 12px;
      box-shadow: 0 2px 6px #eef3fa;
      opacity: 0.8;
      pointer-events: none;
      display: none;
    }
    .viz-title:hover + .viz-tooltip { display: block; }
    svg { width: 100%; height: 540px; background: #fafdff; border-radius: 10px; }
    .metrics-panel { background: #f3f7fc; border-radius: 12px; padding: 16px; margin: 0 auto 20px auto; width: max-content;}
    table.metrics-table { border-collapse: collapse; margin: 0 auto; }
    table.metrics-table td, th { border: none; padding: 6px 12px; }
    table.metrics-table th { color: #64748b; }
    .bar-label { font-size: 0.96em; fill: #374151; font-weight: 600; }
    .bar { fill: #2563eb; }
    .pie-label { font-size: 0.95em; fill: #374151; }
    @media (max-width: 950px) {
      .summary-cards, .visual-section { flex-direction: column; align-items: center; }
      .card, .viz-card { min-width: 90vw; max-width: 99vw; }
    }
  </style>
</head>
<body>
  <div class="dashboard-title">Network Dashboard</div>
  <div class="summary-cards">
    <div class="card">
      <div class="card-title">Node Count</div>
      <div class="card-content" id="node-count"></div>
    </div>
    <div class="card">
      <div class="card-title">Edge Count</div>
      <div class="card-content" id="edge-count"></div>
    </div>
    <div class="card">
      <div class="card-title">Avg Node Degree</div>
      <div class="card-content" id="avg-degree"></div>
    </div>
  </div>
  <div class="visual-section">
    <div class="viz-card">
      <div class="viz-title">Network Force Graph</div>
      <div class="viz-tooltip">Drag nodes, scroll to zoom. Color by node type.</div>
      <svg id="force-graph"></svg>
    </div>
    <div class="viz-card">
      <div class="viz-title">Node Type Breakdown</div>
      <div class="viz-tooltip">Distribution of agents, tasks, managers, etc.</div>
      <svg id="pie-node-type"></svg>
    </div>
    <div class="viz-card">
      <div class="viz-title">Degree Distribution</div>
      <div class="viz-tooltip">How many nodes have each number of connections?</div>
      <svg id="bar-degree"></svg>
    </div>
    <div class="viz-card">
      <div class="viz-title">Adjacency Matrix</div>
      <div class="viz-tooltip">Rows and columns are nodes; filled cell = connection.</div>
      <svg id="adj-matrix"></svg>
    </div>
  </div>
  <script>
    // Assumes you inject graph as {{.Network}}
    let graph = {{.Network}};

    // Set summary cards
    document.getElementById("node-count").textContent = graph.nodes.length;
    document.getElementById("edge-count").textContent = graph.edges.length;
    let degSum = 0, degMap = {};
    graph.nodes.forEach(n => degMap[n.id]=0);
    graph.edges.forEach(e => { degMap[e.source]++; degMap[e.target]++; });
    for (let k in degMap) degSum += degMap[k];
    document.getElementById("avg-degree").textContent = (degSum/graph.nodes.length).toFixed(2);

    // Helper: group by field
    function groupBy(arr, field) {
      return arr.reduce((acc, obj) => {
        let val = obj[field] || "unknown";
        acc[val] = (acc[val]||0)+1;
        return acc;
      }, {});
    }

    // --- Force-Directed Graph ---
    function renderForceGraph(nodes, links) {
      const svg = d3.select("#force-graph");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const color = d3.scaleOrdinal(d3.schemeTableau10);
      const g = svg.append("g");
      // Zoom/Pan
      svg.call(d3.zoom().scaleExtent([0.2,4]).on("zoom", function(e){g.attr("transform", "translate(" + e.transform.x + "," + e.transform.y + ") scale(" + e.transform.k + ")");}));
      // Simulation
      const sim = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(links).id(function(d){return d.id;}).distance(120))
        .force("charge", d3.forceManyBody().strength(-220))
        .force("center", d3.forceCenter(width/2, height/2));
      const link = g.append("g").selectAll("line").data(links).enter().append("line").attr("stroke", "#d1e7fd").attr("stroke-width", 2.1);
      const node = g.append("g").selectAll("circle").data(nodes).enter().append("circle")
        .attr("r", 16)
        .attr("fill", function(d){return color(d.type);})
        .attr("stroke", "#222").attr("stroke-width", 1.1)
        .on("mouseover", function(e,d){ d3.select(this).transition().attr("r",22); })
        .on("mouseout", function(e,d){ d3.select(this).transition().attr("r",16); })
        .call(d3.drag()
          .on("start", dragstarted)
          .on("drag", dragged)
          .on("end", dragended)
        );
      node.append("title").text(function(d){return d.label||d.id;});
      sim.on("tick", function(){
        link.attr("x1",function(d){return d.source.x;}).attr("y1",function(d){return d.source.y;})
            .attr("x2",function(d){return d.target.x;}).attr("y2",function(d){return d.target.y;});
        node.attr("cx",function(d){return d.x;}).attr("cy",function(d){return d.y;});
      });
      function dragstarted(event, d) { if(!event.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; }
      function dragged(event, d) { d.fx = event.x; d.fy = event.y; }
      function dragended(event, d) { if(!event.active) sim.alphaTarget(0); d.fx = null; d.fy = null; }
    }

    // --- Pie Chart for Node Types ---
    function renderPieNodeType(nodes) {
      const svg = d3.select("#pie-node-type");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const radius = Math.min(width, height)/2-24;
      const g = svg.append("g").attr("transform", "translate(" + (width/2) + "," + (height/2) + ")");
      const typeCounts = groupBy(nodes, "type");
      const pie = d3.pie().value(function(d){return d[1];})(Object.entries(typeCounts));
      const arc = d3.arc().innerRadius(0).outerRadius(radius);
      const color = d3.scaleOrdinal(d3.schemeTableau10);
      g.selectAll("path").data(pie).enter().append("path")
        .attr("d", arc)
        .attr("fill", function(d){return color(d.data[0]);})
        .attr("stroke", "#fff").attr("stroke-width", 1.5);
      g.selectAll("text").data(pie).enter().append("text")
        .attr("transform", function(d){return "translate(" + arc.centroid(d) + ")";})
        .attr("text-anchor","middle")
        .attr("dy", "0.33em")
        .attr("class","pie-label")
        .text(function(d){return d.data[0] + " (" + d.data[1] + ")";});
    }

    // --- Degree Distribution Bar Chart ---
    function renderBarDegree(nodes, links) {
      const svg = d3.select("#bar-degree");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      let degMap = {};
      nodes.forEach(n => degMap[n.id]=0);
      links.forEach(e => { degMap[e.source]++; degMap[e.target]++; });
      const degCounts = {};
      Object.values(degMap).forEach(deg => degCounts[deg]=(degCounts[deg]||0)+1);
      const data = Object.entries(degCounts).map(function(entry){return {deg:+entry[0], count:entry[1]};});
      data.sort(function(a,b){return a.deg-b.deg;});
      const margin = {top:30, right:20, bottom:50, left:55};
      const x = d3.scaleBand().domain(data.map(function(d){return d.deg;})).range([margin.left, width-margin.right]).padding(0.17);
      const y = d3.scaleLinear().domain([0,d3.max(data,function(d){return d.count;})]).range([height-margin.bottom, margin.top]);
      svg.append("g").selectAll("rect").data(data).enter().append("rect")
        .attr("class","bar").attr("x",function(d){return x(d.deg);}).attr("y",function(d){return y(d.count);})
        .attr("width",x.bandwidth()).attr("height",function(d){return height-margin.bottom-y(d.count);});
      svg.append("g").attr("transform","translate(0,"+(height-margin.bottom)+")").call(d3.axisBottom(x));
      svg.append("g").attr("transform","translate("+margin.left+",0)").call(d3.axisLeft(y));
      svg.append("text").attr("x",width/2).attr("y",height-8).attr("text-anchor","middle").attr("fill","#888").text("Node Degree");
      svg.append("text").attr("transform","rotate(-90)").attr("y",15).attr("x",-height/2).attr("text-anchor","middle").attr("fill","#888").text("Node Count");
    }

    // --- Adjacency Matrix ---
    function renderAdjMatrix(nodes, links) {
      const svg = d3.select("#adj-matrix");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const ids = nodes.map(function(n){return n.id;});
      const n = ids.length;
      const cellSize = Math.min((width-60)/n, (height-60)/n);
      const idIdx = {}; ids.forEach(function(id,i){idIdx[id]=i;});
      const matrix = Array.from({length:n}, function(){return Array(n).fill(0);});
      links.forEach(function(e){
        let i=idIdx[e.source], j=idIdx[e.target];
        if (i!=null && j!=null) matrix[i][j]=1;
      });
      const g = svg.append("g").attr("transform","translate(40,40)");
      g.selectAll("rect").data(d3.merge(matrix.map(function(row,i){return row.map(function(v,j){return {i:i,j:j,v:v};});})))
        .enter().append("rect")
        .attr("x",function(d){return d.j*cellSize;}).attr("y",function(d){return d.i*cellSize;})
        .attr("width",cellSize-1).attr("height",cellSize-1)
        .attr("fill",function(d){return d.v?"#2563eb":"#e2e8f0";}).attr("opacity",function(d){return d.v?0.88:0.65;});
      g.selectAll(".rowLabel").data(ids).enter().append("text")
        .attr("x",-6).attr("y",function(d,i){return i*cellSize+cellSize/2+2;})
        .attr("text-anchor","end").attr("font-size","0.93em").attr("fill","#555").text(function(d){return d;});
      g.selectAll(".colLabel").data(ids).enter().append("text")
        .attr("x",function(d,i){return i*cellSize+cellSize/2;}).attr("y",-6)
        .attr("text-anchor","middle").attr("font-size","0.93em").attr("fill","#555").text(function(d){return d;});
    }

    // --- Render all charts ---
    function renderAll(graph) {
      renderForceGraph(graph.nodes, graph.edges);
      renderPieNodeType(graph.nodes);
      renderBarDegree(graph.nodes, graph.edges);
      renderAdjMatrix(graph.nodes, graph.edges);
    }
    renderAll(graph);
  </script>
</body>
</html>
`

func main() {
	raw, err := os.ReadFile("data/baseline_network.json")
	if err != nil {
		log.Fatalf("Failed to read baseline_network.json: %v", err)
	}
	log.Printf("[main] Loaded baseline_network.json, %d bytes", len(raw))
	err = json.Unmarshal(raw, &network)
	if err != nil {
		log.Fatalf("[main] Failed to unmarshal baseline_network.json: %v", err)
	}
	if network.Edges == nil {
		log.Printf("[main] Edges is nil, initializing as empty slice")
		network.Edges = []Edge{}
	}
	if network.Nodes == nil {
		log.Printf("[main] Nodes is nil, initializing as empty slice")
		network.Nodes = []Node{}
	}
	network.Metrics = computeMetrics(network.Nodes, network.Edges)
	log.Printf("[main] After load: nodes=%d, edges=%d", len(network.Nodes), len(network.Edges))
	http.HandleFunc("/", serveDashboard)
	log.Println("[main] Dashboard running at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
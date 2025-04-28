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
	Nodes   []Node            `json:"nodes"`
	Edges   []Edge            `json:"edges"`
	Metrics map[string]string `json:"metrics,omitempty"`
}

var (
	mu      sync.Mutex
	network Network
)

func computeMetrics(nodes []Node, edges []Edge) map[string]string {
	if len(nodes) == 0 {
		return map[string]string{}
	}
	nodeTypeCount := map[string]int{}
	for _, n := range nodes {
		nodeTypeCount[n.Type]++
	}
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
	edgeTypeCount := map[string]int{}
	for _, e := range edges {
		edgeTypeCount[e.Type]++
	}
	maxDeg, hub := 0, ""
	for id, d := range degree {
		if d > maxDeg {
			maxDeg = d
			hub = id
		}
	}
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
      max-width: 1400px;
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
      gap: 40px;
      justify-content: center;
      max-width: 2200px;
      margin: 0 auto 48px auto;
    }
    .viz-card {
      background: #fff;
      border-radius: 16px;
      box-shadow: 0 2px 16px #e0e7ef;
      padding: 22px 18px 10px 18px;
      min-width: 900px;
      flex: 1 1 1200px;
      max-width: 1200px;
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
      z-index: 100;
    }
    .viz-title:hover + .viz-tooltip { display: block; }
    svg { width: 100%; height: 750px; background: #fafdff; border-radius: 10px; }
    .legend {
      font-size: 1em;
      text-align: left;
      margin: 0 auto 10px auto;
      display: flex;
      gap: 18px;
      flex-wrap: wrap;
      justify-content: center;
      align-items: center;
      width: fit-content;
      background: #f5f6fa;
      border-radius: 8px;
      box-shadow: 0 1px 5px #e0e7ef;
      padding: 8px 24px;
    }
    .legend-item {
      display: flex;
      align-items: center;
      gap: 5px;
      margin-right: 20px;
    }
    .legend-circle, .legend-rect, .legend-line {
      display: inline-block;
      vertical-align: middle;
    }
    .legend-circle {
      width: 18px; height: 18px; border-radius: 50%; margin-right:6px;
      border: 1.5px solid #444;
    }
    .legend-rect {
      width: 22px; height: 12px; border-radius: 3px;
      border: 1.5px solid #444;
      margin-right: 6px;
    }
    .legend-line {
      width: 32px; height: 4px; border-radius: 2px;
      margin-right: 6px;
    }
    .bar-label { font-size: 0.96em; fill: #374151; font-weight: 600; }
    .bar { fill: #2563eb; }
    .pie-label { font-size: 0.95em; fill: #374151; }
    @media (max-width: 1150px) {
      .summary-cards, .visual-section { flex-direction: column; align-items: center; }
      .card, .viz-card { min-width: 98vw; max-width: 99vw; }
      .legend { flex-direction: column; gap: 5px;}
      svg { height: 520px; }
    }
    /* Modal styles */
    .modal-bg {
      display: none;
      position: fixed;
      z-index: 9999;
      left: 0; top: 0; width: 100vw; height: 100vh;
      background: rgba(40,50,80,0.22);
      justify-content: center;
      align-items: center;
    }
    .modal-bg.active { display: flex; }
    .modal-content {
      background: #fff;
      border-radius: 18px;
      box-shadow: 0 4px 24px #1e293b44;
      padding: 26px 36px 20px 36px;
      min-width: 350px;
      max-width: 80vw;
      max-height: 80vh;
      overflow: auto;
      position: relative;
    }
    .modal-close {
      position: absolute;
      top: 10px; right: 16px;
      font-size: 1.9em;
      color: #888;
      background: none;
      border: none;
      cursor: pointer;
      font-weight: bold;
    }
    .modal-title {
      font-size: 1.2em;
      color: #2563eb;
      margin-bottom: 16px;
      font-weight: 700;
    }
    .modal-body {
      font-size: 1.05em;
      color: #263238;
    }
    .modal-body pre {
      background: #f4f7fa;
      padding: 8px;
      border-radius: 8px;
      font-size: 0.97em;
      overflow-x: auto;
    }
    .modal-body .bid-highlight {
      color: #28a745;
      font-weight: bold;
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
      <div class="viz-title">Network Graph</div>
      <div class="viz-tooltip">
        <b>Nodes:</b> blue = agent, orange = issue/task.<br>
        <b>Edges:</b> <span style="color:#28a745;font-weight:bold;">green</span> = <b>bid</b>, <span style="color:#fd7e14;font-weight:bold;">orange</span> = <b>auction</b>.<br>
        <b>Bid edges</b> show <b>bid_value</b> as a clickable label.<br>
        Click any node or edge for more details.<br>
        Drag nodes, scroll to zoom.
      </div>
      <svg id="force-graph"></svg>
      <div class="legend" id="legend-network">
        <span class="legend-item">
          <span class="legend-circle" style="background:#2563eb"></span> Agent
        </span>
        <span class="legend-item">
          <span class="legend-circle" style="background:#ff9800"></span> Issue/Task
        </span>
        <span class="legend-item">
          <span class="legend-line" style="background:#28a745;height:6px;"></span> Bid
        </span>
        <span class="legend-item">
          <span class="legend-line" style="background:#fd7e14;height:6px;"></span> Auction
        </span>
      </div>
    </div>
    <div class="viz-card">
      <div class="viz-title">Node Type Breakdown</div>
      <div class="viz-tooltip">Distribution of agents, issues, etc.</div>
      <svg id="pie-node-type"></svg>
    </div>
    <div class="viz-card">
      <div class="viz-title">Degree Distribution</div>
      <div class="viz-tooltip">How many nodes have each number of connections?</div>
      <svg id="bar-degree"></svg>
    </div>
    <div class="viz-card">
      <div class="viz-title">Specialities Map</div>
      <div class="viz-tooltip">
        Agents by their specialities (left) and tasks/issues by required specialities (right).<br>
        Lines show which agent has which speciality and which task requires which speciality.
      </div>
      <svg id="specialities-map"></svg>
    </div>
  </div>
  <!-- Modal for details -->
  <div class="modal-bg" id="modal-bg">
    <div class="modal-content">
      <button class="modal-close" onclick="hideModal()">&times;</button>
      <div class="modal-title" id="modal-title"></div>
      <div class="modal-body" id="modal-body"></div>
    </div>
  </div>
  <script>
    let graph = {{.Network}};

    // Set summary cards
    document.getElementById("node-count").textContent = graph.nodes.length;
    document.getElementById("edge-count").textContent = graph.edges.length;
    let degSum = 0, degMap = {};
    graph.nodes.forEach(n => degMap[n.id]=0);
    graph.edges.forEach(e => { degMap[e.source]++; degMap[e.target]++; });
    for (let k in degMap) degSum += degMap[k];
    document.getElementById("avg-degree").textContent = (degSum/graph.nodes.length).toFixed(2);

    function edgeColor(type) {
      if (!type) return "#888";
      switch (type.toLowerCase()) {
        case "bid": return "#28a745";
        case "auction": return "#fd7e14";
        default: return "#888";
      }
    }

    function nodeColor(type) {
      if (!type) return "#aaa";
      switch (type.toLowerCase()) {
        case "agent": return "#2563eb";
        case "issue":
        case "task": return "#ff9800";
        default: return "#aaa";
      }
    }

    // Helper: group by field
    function groupBy(arr, field) {
      return arr.reduce((acc, obj) => {
        let val = obj[field] || "unknown";
        acc[val] = (acc[val]||0)+1;
        return acc;
      }, {});
    }

    // Modal functions
    function showModal(title, bodyHtml) {
      document.getElementById("modal-title").innerHTML = title;
      document.getElementById("modal-body").innerHTML = bodyHtml;
      document.getElementById("modal-bg").classList.add("active");
    }
    function hideModal() {
      document.getElementById("modal-bg").classList.remove("active");
    }
    document.getElementById("modal-bg").onclick = function(e) {
      if (e.target === this) hideModal();
    };

    // --- Universal Zoom Handler for All SVGs ---
    function enableZoom(svg) {
      const g = svg.select("g");
      const zoom = d3.zoom()
        .scaleExtent([0.2, 8])
        .on("zoom", function(e) {
          g.attr("transform", e.transform);
        });
      svg.call(zoom);
      svg.call(zoom.transform, d3.zoomIdentity);
      return zoom;
    }

    // --- Force-Directed Graph with Edge Meaning and Legend ---
    function renderForceGraph(nodes, links) {
      const svg = d3.select("#force-graph");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const g = svg.append("g");
      enableZoom(svg);
      const sim = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(links).id(function(d){return d.id;}).distance(150))
        .force("charge", d3.forceManyBody().strength(-280))
        .force("center", d3.forceCenter(width/2, height/2));
      // Draw edges (lines)
      const link = g.append("g").selectAll("line").data(links).enter().append("line")
        .attr("stroke", d => edgeColor(d.type))
        .attr("stroke-width", 3.3)
        .attr("marker-end", d => "url(#arrow-"+d.type+")")
        .attr("opacity", 0.93)
        .on("click", function(e, d) {
          e.stopPropagation();
          // Show edge details in modal
          showModal("Edge: " + d.type,
            "<b>Source:</b> " + (getNodeById(d.source)?.label || d.source) + "<br>"
            + "<b>Target:</b> " + (getNodeById(d.target)?.label || d.target) + "<br>"
            + "<b>Type:</b> " + d.type + "<br>"
            + (d.type && d.type.toLowerCase() === "bid" && typeof d.bid_value === "number"
                ? "<b>Bid Value:</b> <span class='bid-highlight'>" + d.bid_value.toFixed(2) + "</span><br>"
                : "")
            + (d.Metadata ? "<pre>"+JSON.stringify(d.Metadata,null,2)+"</pre>" : "")
          );
        });
      // Edge markers (arrows)
      const defs = svg.append("defs");
      ["bid", "auction"].forEach(function(etype){
        defs.append("marker")
          .attr("id","arrow-"+etype)
          .attr("viewBox","0 -5 10 10")
          .attr("refX",22).attr("refY",0)
          .attr("markerWidth",8).attr("markerHeight",8)
          .attr("orient","auto")
          .append("path")
          .attr("d","M0,-5L10,0L0,5")
          .attr("fill", edgeColor(etype));
      });
      // Bid value labels as clickable
      g.append("g").selectAll("text")
        .data(links.filter(l => l.type && l.type.toLowerCase() === "bid" && typeof l.bid_value === "number"))
        .enter()
        .append("text")
        .attr("class", "edge-label")
        .attr("fill", "#28a745")
        .attr("font-size", "1em")
        .attr("dy", -6)
        .style("cursor", "pointer")
        .on("click", function(e, d){
          e.stopPropagation();
          showModal("Bid Edge Details",
            "<b>Bid Value:</b> <span class='bid-highlight'>" + d.bid_value.toFixed(2) + "</span><br>"
            + "<b>Agent:</b> " + (getNodeById(d.source)?.label || d.source) + "<br>"
            + "<b>Task:</b> " + (getNodeById(d.target)?.label || d.target) + "<br>"
            + (d.Metadata ? "<pre>"+JSON.stringify(d.Metadata,null,2)+"</pre>" : "")
          );
        })
        .text(d => d.bid_value.toFixed(2));
      // Draw nodes
      const node = g.append("g").selectAll("circle").data(nodes).enter().append("circle")
        .attr("r", 18)
        .attr("fill", function(d){return nodeColor(d.type);})
        .attr("stroke", "#222").attr("stroke-width", 1.5)
        .style("cursor", "pointer")
        .on("mouseover", function(e,d){ d3.select(this).transition().attr("r",25); })
        .on("mouseout", function(e,d){ d3.select(this).transition().attr("r",18); })
        .on("click", function(e, d) {
          e.stopPropagation();
          showModal(
            "Node: " + (d.label || d.id),
            "<b>Type:</b> " + d.type + "<br>"
            + (d.label ? "<b>Label:</b> " + d.label + "<br>" : "")
            + (d.Avatar ? "<b>Avatar:</b> <img src='"+d.Avatar+"' style='max-height:38px;vertical-align:middle;'><br>" : "")
            + (d.Group ? "<b>Group:</b> " + d.Group + "<br>" : "")
            + (d.Desc ? "<b>Description:</b> " + d.Desc + "<br>" : "")
            + (d.Status ? "<b>Status:</b> " + d.Status + "<br>" : "")
            + (d.Priority ? "<b>Priority:</b> " + d.Priority + "<br>" : "")
            + (d.Owner ? "<b>Owner:</b> " + d.Owner + "<br>" : "")
            + (d.AssignedTo ? "<b>Assigned To:</b> " + d.AssignedTo + "<br>" : "")
            + (d.Tags && d.Tags.length ? "<b>Tags:</b> " + d.Tags.join(", ") + "<br>" : "")
            + (d.PriceMin ? "<b>Price Min:</b> " + d.PriceMin + "<br>" : "")
            + (d.PriceMax ? "<b>Price Max:</b> " + d.PriceMax + "<br>" : "")
            + (d.Specialities && d.Specialities.length ?
                "<b>Specialities:</b> <ul style='margin:0 0 0 16px'>" +
                  d.Specialities.map(s => "<li>"+s.Name+(s.Level?" ("+s.Level+")":"")+"</li>").join("") +
                "</ul>" : "")
            + (d.Speciality ?
                "<b>Speciality:</b> " + d.Speciality.Name
                + (d.Speciality.Level?" ("+d.Speciality.Level+")":"")
                + (d.Speciality.Description ? "<br><b>About:</b> "+d.Speciality.Description : "")
                + "<br>" : "")
            + (d.Metadata ? "<pre>"+JSON.stringify(d.Metadata,null,2)+"</pre>" : "")
          );
        });
      node.append("title").text(function(d){return (d.type||"") + ": " + (d.label||d.id);});
      g.append("g").selectAll("text.node-label").data(nodes).enter().append("text")
        .attr("class", "node-label")
        .attr("x", d=>d.x).attr("y", d=>d.y)
        .attr("text-anchor","middle")
        .attr("dy",".35em").attr("fill","#233").attr("font-size","1em")
        .text(d=>d.label||d.id);
      sim.on("tick", function(){
        link.attr("x1",function(d){return d.source.x;}).attr("y1",function(d){return d.source.y;})
            .attr("x2",function(d){return d.target.x;}).attr("y2",function(d){return d.target.y;});
        node.attr("cx",function(d){return d.x;}).attr("cy",function(d){return d.y;});
        g.selectAll("text.node-label").attr("x",function(d){return d.x;}).attr("y",function(d){return d.y-24;});
        g.selectAll("text.edge-label")
          .attr("x", function(d){return (d.source.x+d.target.x)/2;})
          .attr("y", function(d){return (d.source.y+d.target.y)/2-6;});
      });
      function dragstarted(event, d) { if(!event.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; }
      function dragged(event, d) { d.fx = event.x; d.fy = event.y; }
      function dragended(event, d) { if(!event.active) sim.alphaTarget(0); d.fx = null; d.fy = null; }
    }

    // Utility to get node by id
    function getNodeById(id) {
      return graph.nodes.find(n => n.id == id);
    }

    function renderPieNodeType(nodes) {
      const svg = d3.select("#pie-node-type");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const g = svg.append("g").attr("transform", "translate(" + (width/2) + "," + (height/2) + ")");
      enableZoom(svg);
      const typeCounts = groupBy(nodes, "type");
      const pie = d3.pie().value(function(d){return d[1];})(Object.entries(typeCounts));
      const arc = d3.arc().innerRadius(0).outerRadius(Math.min(width, height)/2-24);
      const color = d3.scaleOrdinal().domain(["agent","issue","task"]).range(["#2563eb","#ff9800","#ff9800"]);
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

    function renderBarDegree(nodes, links) {
      const svg = d3.select("#bar-degree");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      enableZoom(svg);
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

    function renderSpecialitiesMap(nodes) {
      const svg = d3.select("#specialities-map");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      enableZoom(svg);
      const agents = nodes.filter(n => n.type === "agent");
      const tasks = nodes.filter(n => n.type === "issue" || n.type === "task");
      let agentSpecs = new Set();
      let taskSpecs = new Set();
      agents.forEach(a => {
        if (Array.isArray(a.specialities)) {
          a.specialities.forEach(s => agentSpecs.add(s.name));
        }
      });
      tasks.forEach(t => {
        if (t.speciality && t.speciality.name) taskSpecs.add(t.speciality.name);
      });
      const allSpecs = Array.from(new Set([...Array.from(agentSpecs), ...Array.from(taskSpecs)]));
      const pad = 30;
      const col1x = pad + 120;
      const col2x = width/2;
      const col3x = width - pad - 120;
      const rowH = Math.min(40, (height-60)/Math.max(agents.length, allSpecs.length, tasks.length));
      svg.append("text").attr("x",col1x).attr("y",30).attr("text-anchor","middle").attr("font-size","1.1em").attr("fill","#1e293b").text("Agents");
      svg.append("text").attr("x",col2x).attr("y",30).attr("text-anchor","middle").attr("font-size","1.1em").attr("fill","#1e293b").text("Specialities");
      svg.append("text").attr("x",col3x).attr("y",30).attr("text-anchor","middle").attr("font-size","1.1em").attr("fill","#1e293b").text("Tasks/Issues");
      agents.forEach((a,i) => {
        svg.append("circle").attr("cx",col1x).attr("cy",60+i*rowH).attr("r",14).attr("fill","#2563eb").attr("stroke","#1e293b").attr("stroke-width",1.2)
          .style("cursor","pointer").on("click", function(){
            showModal("Agent: "+(a.label||a.id), "<pre>"+JSON.stringify(a,null,2)+"</pre>");
          });
        svg.append("text").attr("x",col1x+20).attr("y",65+i*rowH).attr("fill","#234").attr("font-size","0.98em").attr("text-anchor","start").text(a.label||a.id);
      });
      allSpecs.forEach((s,i) => {
        svg.append("rect").attr("x",col2x-14).attr("y",60+i*rowH-14).attr("width",28).attr("height",28).attr("rx",7).attr("fill","#eab308").attr("stroke","#b45309").attr("stroke-width",1.1)
          .style("cursor","pointer").on("click", function(){
            showModal("Speciality: "+s, "<b>Name:</b> "+s);
          });
        svg.append("text").attr("x",col2x+20).attr("y",65+i*rowH).attr("fill","#784c03").attr("font-size","0.98em").attr("text-anchor","start").text(s);
      });
      tasks.forEach((t,i) => {
        svg.append("rect").attr("x",col3x-14).attr("y",60+i*rowH-14).attr("width",28).attr("height",28).attr("rx",5).attr("fill","#f59e42").attr("stroke","#b45309").attr("stroke-width",1.1)
          .style("cursor","pointer").on("click", function(){
            showModal("Task: "+(t.label||t.id), "<pre>"+JSON.stringify(t,null,2)+"</pre>");
          });
        svg.append("text").attr("x",col3x+20).attr("y",65+i*rowH).attr("fill","#a16207").attr("font-size","0.98em").attr("text-anchor","start").text(t.label||t.id);
      });
      agents.forEach((a,i) => {
        if (Array.isArray(a.specialities)) {
          a.specialities.forEach(s => {
            const specIdx = allSpecs.indexOf(s.name);
            if (specIdx >= 0) {
              svg.append("line")
                .attr("x1",col1x+14).attr("y1",60+i*rowH)
                .attr("x2",col2x-14).attr("y2",60+specIdx*rowH)
                .attr("stroke","#2563eb").attr("stroke-width",1.0).attr("opacity",0.6)
                .style("cursor","pointer").on("click", function(){
                  showModal("Agent-Speciality Link", "<b>Agent:</b> "+(a.label||a.id)+"<br><b>Speciality:</b> "+s.name);
                });
            }
          });
        }
      });
      tasks.forEach((t,i) => {
        if (t.speciality && t.speciality.name) {
          const spec = t.speciality.name;
          const specIdx = allSpecs.indexOf(spec);
          if (spec && specIdx >= 0) {
            svg.append("line")
              .attr("x1",col3x-14).attr("y1",60+i*rowH)
              .attr("x2",col2x+14).attr("y2",60+specIdx*rowH)
              .attr("stroke","#f59e42").attr("stroke-width",1.0).attr("opacity",0.7)
              .style("cursor","pointer").on("click", function(){
                showModal("Task-Speciality Link", "<b>Task:</b> "+(t.label||t.id)+"<br><b>Speciality:</b> "+spec);
              });
          }
        }
      });
    }

    function renderAll(graph) {
      renderForceGraph(graph.nodes, graph.edges);
      renderPieNodeType(graph.nodes);
      renderBarDegree(graph.nodes, graph.edges);
      renderSpecialitiesMap(graph.nodes);
    }
    renderAll(graph);
  </script>
</body>
</html>
`
package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
)

type Speciality struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type TaskSpeciality struct {
	Name string `json:"name"`
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

type Edge struct {
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	Type      string  `json:"type"`
	BidValue  float64 `json:"bid_value,omitempty"`
	Reasoning string  `json:"reasoning,omitempty"`
}

type Network struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

var (
	mu      sync.Mutex
	network Network
)

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	networkJSON, err := json.Marshal(network)
	mu.Unlock()
	if err != nil {
		log.Printf("[serveDashboard] Failed to marshal network: %v", err)
	}
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
  <title>Economic Network Dashboard</title>
  <script src="https://d3js.org/d3.v7.min.js"></script>
  <link href="https://fonts.googleapis.com/css?family=Inter:400,600&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xz/fonts@1/serve/bitstream-vera-sans-mono.min.css">
  <style>
    body { font-family: 'Bitstream Vera Sans Mono', 'Inter', Lato, Arial, sans-serif; background: #f8fafc; color: #1e293b; margin: 0; min-height: 100vh;}
    .dashboard-title { font-size: 2.1em; text-align: center; color: #2563eb; margin: 32px 0 8px 0; font-weight: 600; letter-spacing: 0.5px;}
    .summary-cards { display: flex; flex-wrap: wrap; justify-content: center; gap: 24px; margin-bottom: 32px;}
    .card { background: #fff; border-radius: 16px; box-shadow: 0 2px 16px #e0e7ef; padding: 18px 36px 10px 36px; min-width: 220px; flex: 1 1 220px; max-width: 380px; margin: 0 8px;}
    .card-title { font-size: 1.18em; color: #2d3748; margin-bottom: 4px; font-weight: 600;}
    .card-content { font-size: 1.45em; font-weight: 500; }
    .panel-section { display: flex; flex-direction: column; gap: 48px; max-width: 1800px; margin: 0 auto 48px auto;}
    .panel { background: #f3f7fd; border-radius: 18px; box-shadow: 0 2px 12px #e0e7ef; padding: 32px 30px 28px 30px; margin: 0 0 18px 0;}
    .panel-title { font-size: 1.55em; color: #2d3748; font-weight: 700; margin-bottom: 20px; letter-spacing: 1px;}
    .viz-row { width: 100%; max-width: 1600px; margin: 0 auto 34px auto; }
    .viz-card { background: #fff; border-radius: 14px; box-shadow: 0 2px 12px #e0e7ef; padding: 22px 10px 10px 10px; text-align: center; }
    .viz-title { font-size: 1.13em; color: #2d3748; font-weight: 600; margin-bottom: 10px;}
    svg { width: 100%; height: 480px; background: #fafdff; border-radius: 10px; }
    .legend { font-size: 1em; text-align: left; margin: 0 auto 10px auto; display: flex; gap: 18px; flex-wrap: wrap; justify-content: center; align-items: center; width: fit-content; background: #f5f6fa; border-radius: 8px; box-shadow: 0 1px 5px #e0e7ef; padding: 8px 24px;}
    .legend-item { display: flex; align-items: center; gap: 5px; margin-right: 20px;}
    .legend-circle { width: 18px; height: 18px; border-radius: 50%; margin-right:6px; border: 1.5px solid #444;}
    .legend-line { width: 32px; height: 4px; border-radius: 2px; margin-right: 6px;}
    .bar-label { font-size: 0.96em; fill: #374151; font-weight: 600; }
    .modal-bg { display: none; position: fixed; z-index: 9999; left: 0; top: 0; width: 100vw; height: 100vh; background: rgba(40,50,80,0.22); justify-content: center; align-items: center;}
    .modal-bg.active { display: flex; }
    .modal-content { background: #fff; border-radius: 18px; box-shadow: 0 4px 24px #1e293b44; padding: 26px 36px 20px 36px; min-width: 350px; max-width: 80vw; max-height: 80vh; overflow: auto; position: relative;}
    .modal-close { position: absolute; top: 10px; right: 16px; font-size: 1.9em; color: #888; background: none; border: none; cursor: pointer; font-weight: bold;}
    .modal-title { font-size: 1.2em; color: #2563eb; margin-bottom: 16px; font-weight: 700;}
    .modal-body { font-size: 1.05em; color: #263238;}
    .modal-body pre { background: #f4f7fa; padding: 8px; border-radius: 8px; font-size: 0.97em; overflow-x: auto;}
    .modal-body .bid-highlight { color: #28a745; font-weight: bold;}
    @media (max-width: 1200px) {
      .panel-section, .viz-row { max-width: 98vw; }
      .panel { min-width: 98vw; max-width: 99vw; }
      svg { height: 320px; }
    }
  </style>
</head>
<body>
  <div class="dashboard-title">Economic Network Dashboard</div>
  <div class="summary-cards">
    <div class="card">
      <div class="card-title">Nodes</div>
      <div class="card-content" id="node-count"></div>
    </div>
    <div class="card">
      <div class="card-title">Edges</div>
      <div class="card-content" id="edge-count"></div>
    </div>
    <div class="card">
      <div class="card-title">Avg Node Degree</div>
      <div class="card-content" id="avg-degree"></div>
    </div>
  </div>
  <div class="panel-section">
    <!-- Panel 1: Economic Network -->
    <div class="panel">
      <div class="panel-title">Economic Network</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Network Graph</div>
		  <svg id="network-overview" width="850" height="600"></svg>


          <div class="legend" id="legend-network">
            <span class="legend-item"><span class="legend-circle" style="background:#2563eb"></span> Agent</span>
            <span class="legend-item"><span class="legend-circle" style="background:#ff9800"></span> Issue/Task</span>
            <span class="legend-item"><span class="legend-line" style="background:#28a745;height:6px;"></span> Bid</span>
            <span class="legend-item"><span class="legend-line" style="background:#fd7e14;height:6px;"></span> Auction</span>
            <span class="legend-item"><span class="legend-line" style="background:#888;height:6px;"></span> Assigned</span>
          </div>
        </div>
      </div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Top Hubs (by Degree)</div>
          <div id="hubs-metrics"></div>
        </div>
      </div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Edge Type Breakdown</div>
          <div id="edge-type-metrics"></div>
        </div>
      </div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Assignment Rate</div>
          <div id="assignment-rate-metrics"></div>
        </div>
      </div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Centrality Distribution</div>
          <div id="centrality-metrics"></div>
        </div>
      </div>
    </div>
    <!-- Panel 2: Price vs. Demand Per Skill -->
    <div class="panel">
      <div class="panel-title">Price vs. Demand per Skill</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Skill Price and Demand</div>
		  <svg id="price-demand-bar" width="800" height="420"></svg>
        </div>
      </div>
    </div>
    <!-- Panel 3: Competition vs. Labor Supply -->
    <div class="panel">
      <div class="panel-title">Competition vs. Labor Supply</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Agents per Speciality & Avg. Bids per Task</div>
		  <svg id="supply-bar" width="800" height="420"></svg>
        </div>
      </div>
    </div>
    <!-- Panel 4: Market Efficiency by Skill -->
    <div class="panel">
      <div class="panel-title">Market Efficiency by Skill</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Assignment Rate per Skill</div>
          <svg id="market-efficiency-bar"></svg>
        </div>
      </div>
    </div>
    <!-- Panel 5: Market Engagement Patterns -->
    <div class="panel">
      <div class="panel-title">Market Engagement Patterns</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Agent–Skill Participation Heatmap</div>
          <svg id="engagement-heatmap"></svg>
        </div>
      </div>
    </div>
    <!-- Panel 6: Market Balance/Gaps -->
    <div class="panel">
      <div class="panel-title">Market Balance & Gaps</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Supply vs. Demand per Skill</div>
		  <svg id="market-balance-bar" width="800" height="420"></svg>
        </div>
      </div>
    </div>
  </div>
  <div class="modal-bg" id="modal-bg">
    <div class="modal-content">
      <button class="modal-close" onclick="hideModal()">&times;</button>
      <div class="modal-title" id="modal-title"></div>
      <div class="modal-body" id="modal-body"></div>
    </div>
  </div>

  <!-- Metrics and Data Preparation -->
  <script>
    // The Go backend will inject graph data as {{.Network}}
    let graph = window.graphData || {{.Network}};
    if (!graph.nodes) graph.nodes = [];
    if (!graph.edges) graph.edges = [];

    // --- Summary metrics ---
    let degMap = {}, degSum = 0;
    graph.nodes.forEach(function(n) { degMap[n.id]=0; });
    graph.edges.forEach(function(e) {
      if(degMap[e.source]!==undefined)degMap[e.source]++;
      if(degMap[e.target]!==undefined)degMap[e.target]++;
    });
    for (let k in degMap) degSum += degMap[k];
    let avgDegree = graph.nodes.length ? (degSum/graph.nodes.length).toFixed(2) : "0";
    document.getElementById("node-count").textContent = graph.nodes.length;
    document.getElementById("edge-count").textContent = graph.edges.length;
    document.getElementById("avg-degree").textContent = avgDegree;

    // Top hubs (by degree)
    let hubs = Object.entries(degMap).map(function(item) {
      let id = item[0], deg = item[1];
      let node = graph.nodes.find(function(n) { return n.id===id; });
      return {id:id, degree:deg, label:node && node.label ? node.label : id, type:node && node.type ? node.type : ""};
    }).sort(function(a,b){return b.degree-a.degree;}).slice(0,5);
    document.getElementById("hubs-metrics").innerHTML =
      "<ul style='list-style:none;padding:0;margin:0'>" +
      hubs.map(function(h) {
        return "<li><b>" + h.label + "</b> (" + h.type + "): <span style=\"color:#2563eb\">" + h.degree + "</span></li>";
      }).join("") +
      "</ul>";

    // Edge type breakdown
    let edgeTypeCounts = {};
    graph.edges.forEach(function(e) {
      let type = (e.type||"unknown").toLowerCase();
      edgeTypeCounts[type] = (edgeTypeCounts[type]||0)+1;
    });
    document.getElementById("edge-type-metrics").innerHTML =
      "<ul style='list-style:none;padding:0;margin:0'>" +
      Object.entries(edgeTypeCounts).map(function(kv) {
        var k = kv[0], v = kv[1];
        return "<li><b>" + k.charAt(0).toUpperCase() + k.slice(1) + "</b>: <span style=\"color:#fd7e14\">" + v + "</span></li>";
      }).join("") +
      "</ul>";

    // Assignment rate (overall and per agent/task)
    let totalAssigned = graph.edges.filter(function(e) { return (e.type||"").toLowerCase()==="assigned"; }).length;
    let assignmentRate = graph.edges.length ? (100*totalAssigned/graph.edges.length).toFixed(1) : 0;
    document.getElementById("assignment-rate-metrics").innerHTML =
      "<span><b>Assigned Edges:</b> " + totalAssigned + " (" + assignmentRate + "% of all edges)</span>";

    // Centrality distribution (degree histogram: #nodes per degree bucket)
    let degs = Object.values(degMap);
    let minDeg = Math.min.apply(null, degs), maxDeg = Math.max.apply(null, degs);
    let buckets = Array.from({length:Math.max(1,maxDeg-minDeg+1)}, function(_,i){return i+minDeg;});
    let degHist = {};
    buckets.forEach(function(b){degHist[b]=0;});
    degs.forEach(function(d){degHist[d]++;});
    document.getElementById("centrality-metrics").innerHTML =
      "<div style='display:flex;align-items:end;height:60px;margin-top:18px;'>" +
      buckets.map(function(b) {
        return "<div title='Degree " + b + "' style=\"width:34px;margin-right:2px;background:#2563eb;height:" + (degHist[b]*3) + "px;border-radius:4px 4px 0 0;text-align:center;color:#fff;font-size:0.8em;\">" + (degHist[b]>0?degHist[b]:"") + "</div>";
      }).join("") +
      "</div><div style='display:flex;justify-content:space-between;font-size:0.86em;color:#666;margin-top:2px;'>" +
      buckets.map(function(b) {
        return "<span style='width:34px;text-align:center'>" + b + "</span>";
      }).join("") +
      "</div>";

    // Modal logic
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

    // D3 helpers for all charts
    function nodeColor(type) {
      if (!type) return "#aaa";
      switch (type.toLowerCase()) {
        case "agent": return "#2563eb";
        case "issue":
        case "task": return "#ff9800";
        default: return "#aaa";
      }
    }
    function edgeColor(type) {
      if (!type) return "#888";
      switch (type.toLowerCase()) {
        case "bid": return "#28a745";
        case "auction": return "#fd7e14";
        case "assigned": return "#888";
        default: return "#bbb";
      }
    }
    // idToNode for all panels
    window.idToNode = {};
    graph.nodes.forEach(function(n) { idToNode[n.id] = n; });
  </script>















  <!-- Panel 1: Economic Network -->
  <script>
console.log("Panel 1: Enhanced Network Overview", graph && graph.nodes, graph && graph.edges);

// Debug: Check data presence and structure
if (!graph || !Array.isArray(graph.nodes) || !Array.isArray(graph.edges)) {
  console.error("Panel 1: graph, graph.nodes, or graph.edges is missing or malformed", graph);
} else {
  console.log("Panel 1: Number of nodes:", graph.nodes.length);
  console.log("Panel 1: Number of edges:", graph.edges.length);
  if (graph.nodes.length > 0) {
    console.log("Panel 1: First node sample:", graph.nodes[0]);
  }
  if (graph.edges.length > 0) {
    console.log("Panel 1: First edge sample:", graph.edges[0]);
  }
}

// Color and shape mapping
const nodeColors = {
  agent: "#2563eb",    // blue
  issue: "#fd7e14",    // orange
  skill: "#059669"     // teal/green
};
const nodeShapes = {
  agent: d3.symbolCircle,
  issue: d3.symbolSquare,
  skill: d3.symbolDiamond
};

const width = 850, height = 600;
const svg = d3.select("#network-overview");
svg.selectAll("*").remove(); // Clear previous

// Tooltip
const tooltip = d3.select("body").append("div")
  .attr("class", "network-tooltip")
  .style("position", "absolute")
  .style("background", "#fff")
  .style("border", "1px solid #bbb")
  .style("border-radius", "6px")
  .style("padding", "6px 12px")
  .style("pointer-events", "none")
  .style("font", "14px sans-serif")
  .style("box-shadow", "0 2px 12px #0002")
  .style("visibility", "hidden");

// Defensive: Only run if data is present
if (graph && Array.isArray(graph.nodes) && Array.isArray(graph.edges)) {
  // Prepare simulation
  const simulation = d3.forceSimulation(graph.nodes)
    .force("link", d3.forceLink(graph.edges).id(function(d){ return d.id; }).distance(70).strength(0.8))
    .force("charge", d3.forceManyBody().strength(-260))
    .force("center", d3.forceCenter(width / 2, height / 2))
    .force("collide", d3.forceCollide().radius(30));

  // Edges
  const link = svg.append("g")
    .attr("stroke", "#999")
    .attr("stroke-opacity", 0.55)
    .selectAll("line")
    .data(graph.edges)
    .join("line")
    .attr("stroke-width", function(d){ return d.type === "assigned" ? 3 : 1.5; })
    .attr("stroke-dasharray", function(d){ return d.type === "bid" ? "4,2" : null; })
    .attr("marker-end", function(d){ return d.type === "assigned" ? "url(#arrowhead)" : null; });

  // Arrowhead marker for assignment edges
  svg.append("defs").append("marker")
    .attr("id", "arrowhead")
    .attr("viewBox", "-0 -5 10 10")
    .attr("refX", 28)
    .attr("refY", 0)
    .attr("orient", "auto")
    .attr("markerWidth", 8)
    .attr("markerHeight", 8)
    .attr("xoverflow", "visible")
    .append("svg:path")
    .attr("d", "M 0,-5 L 10,0 L 0,5")
    .attr("fill", "#fd7e14")
    .style("stroke","none");

  // Nodes
  const node = svg.append("g")
    .attr("stroke", "#fff")
    .attr("stroke-width", 2)
    .selectAll("path")
    .data(graph.nodes)
    .join("path")
    .attr("d", d3.symbol().type(function(d){ return nodeShapes[d.type] || d3.symbolCircle; }).size(function(d){ return 600; }))
    .attr("fill", function(d){ return nodeColors[d.type] || "#aaa"; })
    .style("cursor", "pointer")
    .on("mouseover", function(e, d) {
      d3.select(this).attr("stroke", "#222").attr("stroke-width", 4);
      var html = "<b>" + d.type.charAt(0).toUpperCase() + d.type.slice(1) + "</b><br>";
      if (d.name) html += d.name + "<br>";
      if (d.speciality && d.speciality.name) html += "Skill: " + d.speciality.name + "<br>";
      if (d.skills) html += "Skills: " + d.skills.map(function(s){ return s.name; }).join(", ") + "<br>";
      if (typeof d.price_min === "number") html += "Min Price: " + d.price_min + "<br>";
      if (typeof d.price_max === "number") html += "Max Price: " + d.price_max + "<br>";
      tooltip.html(html).style("visibility", "visible");
      console.log("Panel 1: Hovered node", d);
    })
    .on("mousemove", function(e) {
      tooltip.style("top", (e.pageY + 12) + "px").style("left", (e.pageX + 12) + "px");
    })
    .on("mouseout", function() {
      d3.select(this).attr("stroke", "#fff").attr("stroke-width", 2);
      tooltip.style("visibility", "hidden");
    })
    .call(d3.drag()
      .on("start", dragstarted)
      .on("drag", dragged)
      .on("end", dragended)
    );

  // Node labels
  const label = svg.append("g")
    .selectAll("text")
    .data(graph.nodes)
    .join("text")
    .text(function(d){ return d.name || d.id; })
    .attr("font-size", 13)
    .attr("font-family", "sans-serif")
    .attr("fill", "#333")
    .attr("stroke", "white")
    .attr("stroke-width", 2)
    .attr("paint-order", "stroke")
    .attr("dy", 4)
    .attr("text-anchor", "middle");

  // Zoom/pan
  svg.call(d3.zoom()
    .extent([[0, 0], [width, height]])
    .scaleExtent([0.3, 2])
    .on("zoom", function(event) {
      svg.selectAll("g").attr("transform", event.transform);
      label.attr("transform", event.transform);
    })
  );

  // Simulation tick
  simulation.on("tick", function() {
    link
      .attr("x1", function(d){ return d.source.x; })
      .attr("y1", function(d){ return d.source.y; })
      .attr("x2", function(d){ return d.target.x; })
      .attr("y2", function(d){ return d.target.y; });

    node
      .attr("transform", function(d){ return "translate(" + d.x + "," + d.y + ")"; });

    label
      .attr("x", function(d){ return d.x; })
      .attr("y", function(d){ return d.y + 24; });
  });

  // Drag functions
  function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
    console.log("Panel 1: Drag start", d);
  }
  function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
  }
  function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    d.fx = null;
    d.fy = null;
    console.log("Panel 1: Drag end", d);
  }

  // Legend
  const legend = svg.append("g")
    .attr("transform", "translate(20," + (height - 120) + ")");
  const legendData = [
    { type: "agent", label: "Agent" },
    { type: "issue", label: "Issue" },
    { type: "skill", label: "Skill" }
  ];
  legend.selectAll("path")
    .data(legendData)
    .join("path")
    .attr("d", d3.symbol().type(function(d){ return nodeShapes[d.type]; }).size(400))
    .attr("fill", function(d){ return nodeColors[d.type]; })
    .attr("transform", function(d, i){ return "translate(0," + (i*32) + ")"; });
  legend.selectAll("text")
    .data(legendData)
    .join("text")
    .attr("x", 22)
    .attr("y", function(d, i){ return i*32 + 6; })
    .attr("font-size", 15)
    .attr("font-family", "sans-serif")
    .attr("fill", "#333")
    .text(function(d){ return d.label; });

  // Title
  svg.append("text")
    .attr("x", width/2)
    .attr("y", 34)
    .attr("text-anchor", "middle")
    .attr("font-size", 22)
    .attr("font-family", "sans-serif")
    .attr("font-weight", "bold")
    .attr("fill", "#222")
    .text("Market Network Overview");

  console.log("Panel 1: Render complete.");
} else {
  svg.append("text")
    .attr("x", width/2)
    .attr("y", 60)
    .attr("text-anchor", "middle")
    .attr("font-size", 20)
    .attr("fill", "red")
    .text("No data available for network overview.");
}
</script>
<style>
.network-tooltip {
  z-index: 99;
}
</style>



























<!-- PANEL 2: Price vs. Demand per Skill -->
<script>
console.log("Panel 2: START", graph && graph.nodes);

// Filter issues with speciality and price_min
const issues = (graph.nodes || []).filter(n =>
  (n.type || "").toLowerCase() === "issue" &&
  n.speciality && typeof n.speciality.name === "string" &&
  typeof n.price_min === "number"
);
console.log("Panel 2: filtered issues with speciality and price_min", issues);

// Calculate demand and avg price per skill
const skillData = {};
issues.forEach(issue => {
  const skill = issue.speciality.name;
  if (!skillData[skill]) {
    skillData[skill] = { demand: 0, prices: [] };
  }
  skillData[skill].demand += 1;
  skillData[skill].prices.push(issue.price_min);
});
const data = Object.entries(skillData).map(([skill, v]) => ({
  skill,
  demand: v.demand,
  avgPrice: v.prices.length ? (v.prices.reduce((a, b) => a + b, 0) / v.prices.length) : 0
})).sort((a, b) => b.demand - a.demand);

console.log("Panel 2: final data array for chart", data);

const svg = d3.select("#price-demand-bar");
svg.selectAll("*").remove();
const width = svg.node().clientWidth,
      height = 420,
      margin = { top: 40, right: 60, bottom: 120, left: 70 };

if (data.length === 0) {
  svg.append("text")
    .attr("x", width/2)
    .attr("y", height/2)
    .attr("text-anchor", "middle")
    .attr("fill", "gray")
    .text("No data available.");
  console.log("Panel 2: No data available to render.");
} else {
  const x = d3.scaleBand()
    .domain(data.map(d => d.skill))
    .range([margin.left, width - margin.right])
    .padding(0.2);

  const yLeft = d3.scaleLinear()
    .domain([0, d3.max(data, d => d.demand) || 1]).nice()
    .range([height - margin.bottom, margin.top]);

  const yRight = d3.scaleLinear()
    .domain([0, d3.max(data, d => d.avgPrice) || 1]).nice()
    .range([height - margin.bottom, margin.top]);

  // Bars for demand
  svg.append("g")
    .selectAll("rect")
    .data(data)
    .enter().append("rect")
    .attr("x", d => x(d.skill))
    .attr("y", d => yLeft(d.demand))
    .attr("width", x.bandwidth())
    .attr("height", d => yLeft(0) - yLeft(d.demand))
    .attr("fill", "#2dd4bf");

  // Line for average price
  const line = d3.line()
    .x(d => x(d.skill) + x.bandwidth()/2)
    .y(d => yRight(d.avgPrice));

  svg.append("path")
    .datum(data)
    .attr("fill", "none")
    .attr("stroke", "#f59e42")
    .attr("stroke-width", 3)
    .attr("d", line);

  // Dots for price
  svg.append("g")
    .selectAll("circle")
    .data(data)
    .enter().append("circle")
    .attr("cx", d => x(d.skill) + x.bandwidth()/2)
    .attr("cy", d => yRight(d.avgPrice))
    .attr("r", 5)
    .attr("fill", "#f59e42");

  // X Axis with rotated labels
  svg.append("g")
    .attr("transform", "translate(0," + (height - margin.bottom) + ")")
    .call(d3.axisBottom(x).tickSize(0))
    .selectAll("text")
    .style("text-anchor", "end")
    .attr("dx", "-0.6em")
    .attr("dy", "0.1em")
    .attr("transform", "rotate(-65)");

  // Y Axis left (Demand)
  svg.append("g")
    .attr("transform", "translate(" + margin.left + ",0)")
    .call(d3.axisLeft(yLeft).ticks(6))
    .append("text")
    .attr("x", -margin.left + 10)
    .attr("y", margin.top - 15)
    .attr("fill", "#2dd4bf")
    .attr("text-anchor", "start")
    .text("Demand (issues)");

  // Y Axis right (Price)
  svg.append("g")
    .attr("transform", "translate(" + (width - margin.right) + ",0)")
    .call(d3.axisRight(yRight).ticks(6))
    .append("text")
    .attr("x", 40)
    .attr("y", margin.top - 15)
    .attr("fill", "#f59e42")
    .attr("text-anchor", "end")
    .text("Average Price");

  // Title
  svg.append("text")
    .attr("x", width / 2)
    .attr("y", margin.top - 18)
    .attr("text-anchor", "middle")
    .text("Price vs Demand per Skill");

  // Legend
  svg.append("rect")
    .attr("x", width - margin.right - 150)
    .attr("y", margin.top - 35)
    .attr("width", 22)
    .attr("height", 16)
    .attr("fill", "#2dd4bf");
  svg.append("text")
    .attr("x", width - margin.right - 120)
    .attr("y", margin.top - 22)
    .text("Demand (bars)")
    .attr("alignment-baseline", "middle");

  svg.append("circle")
    .attr("cx", width - margin.right - 60 + 8)
    .attr("cy", margin.top - 27 + 8)
    .attr("r", 8)
    .attr("fill", "#f59e42");
  svg.append("text")
    .attr("x", width - margin.right - 40)
    .attr("y", margin.top - 22)
    .text("Avg Price")
    .attr("alignment-baseline", "middle");
}

console.log("Panel 2: Render complete.");
</script>











  <!-- Panel 3: Competition vs. Labor Supply -->
<script>
console.log("Panel 3: START", graph && graph.nodes);

function renderSupplyBar(nodes) {
  // Count agents per skill
  var skillCounts = {};
  nodes.filter(n => (n.type || "").toLowerCase() === "agent" && Array.isArray(n.specialities)).forEach(agent => {
    agent.specialities.forEach(spec => {
      if (spec && typeof spec.name === "string") {
        skillCounts[spec.name] = (skillCounts[spec.name] || 0) + 1;
      }
    });
  });

  var data = Object.entries(skillCounts).map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count);

  console.log("Panel 3: skillCounts", skillCounts);
  console.log("Panel 3: data", data);

  var svg = d3.select("#supply-bar");
  svg.selectAll("*").remove();
  var width = svg.node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:140, left:100}; // bottom margin increased for vertical labels

  if (data.length === 0) {
    svg.append("text")
      .attr("x", width/2)
      .attr("y", height/2)
      .attr("text-anchor", "middle")
      .attr("fill", "gray")
      .text("No data available.");
    console.log("Panel 3: No data available to render.");
    return;
  }

  var x = d3.scaleBand().domain(data.map(d => d.name)).range([margin.left, width-margin.right]).padding(0.18);
  var y = d3.scaleLinear().domain([0, d3.max(data, d => d.count) || 1]).nice().range([height-margin.bottom, margin.top]);

  svg.append("g")
    .selectAll("rect")
    .data(data)
    .enter().append("rect")
    .attr("x", d => x(d.name))
    .attr("y", d => y(d.count))
    .attr("width", x.bandwidth())
    .attr("height", d => y(0) - y(d.count))
    .attr("fill", "#2563eb");
  // X Axis with vertical labels
  svg.append("g")
    .attr("transform", "translate(0," + (height - margin.bottom) + ")")
    .call(d3.axisBottom(x).tickSize(0))
    .selectAll("text")
    .style("text-anchor", "end")
    .attr("dx", "-0.6em")
    .attr("dy", "0.1em")
    .attr("transform", "rotate(-90)");
  svg.append("g")
    .attr("transform", "translate(" + (margin.left) + ",0)")
    .call(d3.axisLeft(y).ticks(6));
  svg.append("text")
    .attr("x", width / 2)
    .attr("y", margin.top - 18)
    .attr("text-anchor", "middle")
    .text("Agent Supply per Skill");
  svg.append("text")
    .attr("x", -(height / 2))
    .attr("y", margin.left - 70)
    .attr("transform", "rotate(-90)")
    .attr("text-anchor", "middle")
    .text("Number of Agents");

  console.log("Panel 3: Render complete.");
}
renderSupplyBar(graph.nodes);
</script>

















<!-- PANEL 4: Market Efficiency by Skill -->
<script>
console.log("Panel 4: START", graph && graph.nodes, graph && graph.edges);

function renderMarketEfficiencyBar(nodes, edges) {
  // Debug: Show all 'issue' nodes with a 'speciality'
  const tasks = nodes.filter(function(n) {
    const isIssue = (n.type || "").toLowerCase() === "issue";
    const hasSpeciality = n.speciality && typeof n.speciality.name === "string";
    return isIssue && hasSpeciality;
  });
  console.log("Panel 4: Found issues with speciality:", tasks);

  // Tally total issues per skill
  var skillIssueCount = {};
  tasks.forEach(function(n) {
    var skill = n.speciality.name;
    if (!skillIssueCount[skill]) skillIssueCount[skill] = 0;
    skillIssueCount[skill]++;
  });
  console.log("Panel 4: skillIssueCount", skillIssueCount);

  // Tally assigned issues per skill
  var skillAssignedCount = {};
  edges.forEach(function(e) {
    if ((e.type || "").toLowerCase() === "assigned") {
      var issue = tasks.find(function(n){return n.id===e.target;});
      if (issue) {
        var skill = issue.speciality.name;
        if (!skillAssignedCount[skill]) skillAssignedCount[skill]=0;
        skillAssignedCount[skill]++;
      }
    }
  });
  console.log("Panel 4: skillAssignedCount", skillAssignedCount);

  var skills = Object.keys(skillIssueCount);
  var data = skills.map(function(skill){
    return {
      skill: skill,
      assignmentRate: skillAssignedCount[skill]? (100*skillAssignedCount[skill]/skillIssueCount[skill]) : 0
    };
  }).sort(function(a,b){return b.assignmentRate-a.assignmentRate;});
  console.log("Panel 4: final data", data);

  var svg = d3.select("#market-efficiency-bar");
  svg.selectAll("*").remove();
  var width = svg.node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:60, left:160};

  if (data.length === 0) {
    svg.append("text")
      .attr("x", width/2)
      .attr("y", height/2)
      .attr("text-anchor", "middle")
      .attr("fill", "gray")
      .text("No data available.");
    console.log("Panel 4: No data available to render.");
    return;
  }

  var y = d3.scaleBand().domain(data.map(function(d){return d.skill;})).range([margin.top, height-margin.bottom]).padding(0.18);
  var x = d3.scaleLinear().domain([0, 100]).range([margin.left, width-margin.right]);
  svg.append("g")
    .selectAll("rect")
    .data(data)
    .enter().append("rect")
    .attr("y",function(d){return y(d.skill);})
    .attr("x",x(0))
    .attr("height",y.bandwidth())
    .attr("width",function(d){return x(d.assignmentRate)-x(0);})
    .attr("fill","#28a745");
  svg.append("g")
    .attr("transform","translate("+(margin.left)+",0)")
    .call(d3.axisLeft(y));
  svg.append("g")
    .attr("transform","translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).ticks(6).tickFormat(function(d){return d+"%";}));
  svg.selectAll(".bar-label")
    .data(data)
    .enter().append("text")
    .attr("class","bar-label")
    .attr("y",function(d){return y(d.skill)+y.bandwidth()/2+3;})
    .attr("x",function(d){return x(d.assignmentRate)+8;})
    .text(function(d){return d.assignmentRate.toFixed(1)+"%";});

  console.log("Panel 4: Render complete.");
}
renderMarketEfficiencyBar(graph.nodes, graph.edges);
</script>









<!-- PANEL 5: Market Engagement Patterns -->
<script>
console.log("Panel 5: START", graph && graph.nodes, graph && graph.edges);

function renderEngagementHeatmap(nodes, edges) {
  // Step 1: Get agent nodes
  const agents = nodes.filter(function(n){
    return (n.type||"").toLowerCase()==="agent";
  });
  // Step 2: Get issue nodes (tasks) with a speciality
  const issues = nodes.filter(function(n){
    return (n.type||"").toLowerCase()==="issue" && n.speciality && typeof n.speciality.name === "string";
  });
  // Step 3: Get unique skill names
  const skills = Array.from(new Set(issues.map(function(n){return n.speciality.name;})));

  console.log("Panel 5: agents", agents);
  console.log("Panel 5: issues with speciality", issues);
  console.log("Panel 5: skills", skills);

  // Step 4: Build agent-skill engagement matrix
  var mat = {};
  agents.forEach(function(a){ mat[a.id]={}; skills.forEach(function(s){mat[a.id][s]=0;}); });
  edges.forEach(function(e) {
    if ((e.type||"").toLowerCase()==="bid") {
      var agent = agents.find(function(n){return n.id===e.source;});
      var issue = issues.find(function(n){return n.id===e.target;});
      if (agent && issue) mat[agent.id][issue.speciality.name]++;
    }
  });

  console.log("Panel 5: engagement matrix", mat);

  // Step 5: Build data array for D3
  var data = [];
  agents.forEach(function(a,i) {
    skills.forEach(function(s,j) {
      data.push({agent:a.label||a.id, skill:s, value:mat[a.id][s], i:i, j:j});
    });
  });

  console.log("Panel 5: data for D3", data);

  var svg = d3.select("#engagement-heatmap");
  svg.selectAll("*").remove();
  var width = svg.node().clientWidth,
      height = 420, margin = {top:70, right:30, bottom:80, left:180};

  if (!skills.length || !agents.length) {
    svg.append("text")
      .attr("x", width/2)
      .attr("y", height/2)
      .attr("text-anchor", "middle")
      .attr("fill", "gray")
      .text("No data available.");
    console.log("Panel 5: No data available to render.");
    return;
  }

  var x = d3.scaleBand().domain(skills).range([margin.left, width-margin.right]).padding(0.05);
  var y = d3.scaleBand().domain(agents.map(function(a){return a.label||a.id;})).range([margin.top, height-margin.bottom]).padding(0.05);
  var color = d3.scaleSequential(d3.interpolateBlues).domain([0, d3.max(data,function(d){return d.value;})||1]);
  svg.append("g").selectAll("rect")
    .data(data)
    .enter().append("rect")
    .attr("x",function(d){return x(d.skill);})
    .attr("y",function(d){return y(d.agent);})
    .attr("width",x.bandwidth())
    .attr("height",y.bandwidth())
    .attr("fill",function(d){return color(d.value);})
    .on("mouseover", function(e,d){ d3.select(this).attr("fill","#fd7e14"); })
    .on("mouseout", function(e,d){ d3.select(this).attr("fill",color(d.value)); })
    .append("title").text(function(d){return d.agent + " - " + d.skill + ": " + d.value;});
  svg.append("g")
    .attr("transform","translate(0,"+margin.top+")")
    .call(d3.axisLeft(y).tickSize(0));
  svg.append("g")
    .attr("transform","translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).tickSize(0));
  svg.append("text").attr("x", width/2).attr("y", margin.top-25).attr("text-anchor","middle").attr("fill","#374151").text("Skill");
  svg.append("text").attr("x", margin.left-80).attr("y", height/2).attr("text-anchor","middle").attr("fill","#374151").attr("transform","rotate(-90 "+(margin.left-80)+","+(height/2)+")").text("Agent");

  console.log("Panel 5: Render complete.");
}
renderEngagementHeatmap(graph.nodes, graph.edges);
</script>

















  <!-- Panel 6: Market Balance/Gaps -->
<script>
console.log("Panel 6: START", graph && graph.nodes);

function renderMarketBalanceBar(nodes) {
  // Supply: agents per skill
  var agentSkillCounts = {};
  nodes.filter(n => (n.type || "").toLowerCase() === "agent" && Array.isArray(n.specialities)).forEach(agent => {
    agent.specialities.forEach(spec => {
      if (spec && typeof spec.name === "string") {
        agentSkillCounts[spec.name] = (agentSkillCounts[spec.name] || 0) + 1;
      }
    });
  });

  // Demand: issues per skill
  var issueSkillCounts = {};
  nodes.filter(n => (n.type || "").toLowerCase() === "issue" && n.speciality && typeof n.speciality.name === "string").forEach(issue => {
    var skill = issue.speciality.name;
    issueSkillCounts[skill] = (issueSkillCounts[skill] || 0) + 1;
  });

  // Merge skill sets
  var allSkills = Array.from(new Set([...Object.keys(agentSkillCounts), ...Object.keys(issueSkillCounts)]));

  // Prepare data for both supply and demand
  var data = allSkills.map(skill => ({
    skill: skill,
    supply: agentSkillCounts[skill] || 0,
    demand: issueSkillCounts[skill] || 0,
  })).sort((a, b) => b.demand - a.demand);

  console.log("Panel 6: data", data);

  var svg = d3.select("#market-balance-bar");
  svg.selectAll("*").remove();
  var width = svg.node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:60, left:160};

  if (data.length === 0) {
    svg.append("text")
      .attr("x", width/2)
      .attr("y", height/2)
      .attr("text-anchor", "middle")
      .attr("fill", "gray")
      .text("No data available.");
    console.log("Panel 6: No data available to render.");
    return;
  }

  var y = d3.scaleBand().domain(data.map(d => d.skill)).range([margin.top, height-margin.bottom]).padding(0.15);
  var x = d3.scaleLinear().domain([0, d3.max(data, d => Math.max(d.supply, d.demand)) || 1]).nice().range([margin.left, width-margin.right]);

  // Supply bars
  svg.append("g")
    .selectAll("rect.supply")
    .data(data)
    .enter().append("rect")
    .attr("class", "supply")
    .attr("y", d => y(d.skill))
    .attr("x", x(0))
    .attr("height", y.bandwidth()/2)
    .attr("width", d => x(d.supply) - x(0))
    .attr("fill", "#2563eb");

  // Demand bars
  svg.append("g")
    .selectAll("rect.demand")
    .data(data)
    .enter().append("rect")
    .attr("class", "demand")
    .attr("y", d => y(d.skill) + y.bandwidth()/2)
    .attr("x", x(0))
    .attr("height", y.bandwidth()/2)
    .attr("width", d => x(d.demand) - x(0))
    .attr("fill", "#fd7e14");

  svg.append("g")
    .attr("transform", "translate("+(margin.left)+",0)")
    .call(d3.axisLeft(y));
  svg.append("g")
    .attr("transform", "translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).ticks(6));

  // Legends
  svg.append("rect")
    .attr("x", width-margin.right-120)
    .attr("y", margin.top-35)
    .attr("width", 22)
    .attr("height", 16)
    .attr("fill", "#2563eb");
  svg.append("text")
    .attr("x", width-margin.right-90)
    .attr("y", margin.top-22)
    .text("Supply")
    .attr("alignment-baseline", "middle");

  svg.append("rect")
    .attr("x", width-margin.right-60)
    .attr("y", margin.top-35)
    .attr("width", 22)
    .attr("height", 16)
    .attr("fill", "#fd7e14");
  svg.append("text")
    .attr("x", width-margin.right-30)
    .attr("y", margin.top-22)
    .text("Demand")
    .attr("alignment-baseline", "middle");

  console.log("Panel 6: Render complete.");
}
renderMarketBalanceBar(graph.nodes);
</script>














</body>
</html>
`
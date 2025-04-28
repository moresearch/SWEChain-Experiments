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
  <title>Network Dashboard</title>
  <script src="https://d3js.org/d3.v7.min.js"></script>
  <link href="https://fonts.googleapis.com/css?family=Inter:400,600&display=swap" rel="stylesheet">
  <style>
    body { font-family: 'Inter', Lato, Arial, sans-serif; background: #f8fafc; color: #1e293b; margin: 0; min-height: 100vh;}
    .dashboard-title { font-size: 2.1em; text-align: center; color: #2563eb; margin: 32px 0 8px 0; font-weight: 600; letter-spacing: 0.5px;}
    .summary-cards { display: flex; flex-wrap: wrap; justify-content: center; gap: 24px; margin-bottom: 32px; max-width: 1400px; margin-left: auto; margin-right: auto;}
    .card { background: #fff; border-radius: 16px; box-shadow: 0 2px 16px #e0e7ef; padding: 18px 36px 10px 36px; min-width: 220px; flex: 1 1 220px; max-width: 380px; margin: 0 8px;}
    .card-title { font-size: 1.18em; color: #2d3748; margin-bottom: 4px; font-weight: 600;}
    .card-content { font-size: 1.45em; font-weight: 500; }
    .viz-card { background: #fff; border-radius: 16px; box-shadow: 0 2px 16px #e0e7ef; padding: 22px 18px 10px 18px; min-width: 900px; max-width: 1200px; margin: 0 auto 30px auto; text-align: center; position: relative;}
    .viz-title { font-size: 1.13em; color: #2d3748; font-weight: 600; margin-bottom: 8px;}
    svg { width: 100%; height: 750px; background: #fafdff; border-radius: 10px; }
    .legend { font-size: 1em; text-align: left; margin: 0 auto 10px auto; display: flex; gap: 18px; flex-wrap: wrap; justify-content: center; align-items: center; width: fit-content; background: #f5f6fa; border-radius: 8px; box-shadow: 0 1px 5px #e0e7ef; padding: 8px 24px;}
    .legend-item { display: flex; align-items: center; gap: 5px; margin-right: 20px;}
    .legend-circle { width: 18px; height: 18px; border-radius: 50%; margin-right:6px; border: 1.5px solid #444;}
    .legend-line { width: 32px; height: 4px; border-radius: 2px; margin-right: 6px;}
    .modal-bg { display: none; position: fixed; z-index: 9999; left: 0; top: 0; width: 100vw; height: 100vh; background: rgba(40,50,80,0.22); justify-content: center; align-items: center;}
    .modal-bg.active { display: flex; }
    .modal-content { background: #fff; border-radius: 18px; box-shadow: 0 4px 24px #1e293b44; padding: 26px 36px 20px 36px; min-width: 350px; max-width: 80vw; max-height: 80vh; overflow: auto; position: relative;}
    .modal-close { position: absolute; top: 10px; right: 16px; font-size: 1.9em; color: #888; background: none; border: none; cursor: pointer; font-weight: bold;}
    .modal-title { font-size: 1.2em; color: #2563eb; margin-bottom: 16px; font-weight: 700;}
    .modal-body { font-size: 1.05em; color: #263238;}
    .modal-body pre { background: #f4f7fa; padding: 8px; border-radius: 8px; font-size: 0.97em; overflow-x: auto;}
    .modal-body .bid-highlight { color: #28a745; font-weight: bold;}
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
  <div class="viz-card">
    <div class="viz-title">Network Graph</div>
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
      <span class="legend-item">
        <span class="legend-line" style="background:#888;height:6px;"></span> Assigned
      </span>
    </div>
  </div>
  <div class="modal-bg" id="modal-bg">
    <div class="modal-content">
      <button class="modal-close" onclick="hideModal()">&times;</button>
      <div class="modal-title" id="modal-title"></div>
      <div class="modal-body" id="modal-body"></div>
    </div>
  </div>
  <script>
    let graph = {{.Network}};
    if (!graph.nodes) graph.nodes = [];
    if (!graph.edges) graph.edges = [];

    // Compute degree for summary
    let degSum = 0, degMap = {};
    graph.nodes.forEach(n => degMap[n.id]=0);
    graph.edges.forEach(e => {
      if(degMap[e.source]!==undefined)degMap[e.source]++;
      if(degMap[e.target]!==undefined)degMap[e.target]++;
    });
    for (let k in degMap) degSum += degMap[k];
    let avgDegree = graph.nodes.length ? (degSum/graph.nodes.length).toFixed(2) : "0";
    document.getElementById("node-count").textContent = graph.nodes.length;
    document.getElementById("edge-count").textContent = graph.edges.length;
    document.getElementById("avg-degree").textContent = avgDegree;

    function edgeColor(type) {
      if (!type) return "#888";
      switch (type.toLowerCase()) {
        case "bid": return "#28a745";
        case "auction": return "#fd7e14";
        case "assigned": return "#888";
        default: return "#bbb";
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
    function getNodeById(id) {
      return graph.nodes.find(n => n.id == id);
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

    // ----------- D3 expects source/target as node objects, not IDs ---------------
    // Robustly filter out edges with missing endpoints!
    let idToNode = {};
    graph.nodes.forEach(n => idToNode[n.id] = n);
    let filteredEdges = graph.edges.filter(e => idToNode[e.source] && idToNode[e.target]);
    let links = filteredEdges.map(function(e) {
      let src = idToNode[e.source];
      let tgt = idToNode[e.target];
      return {...e, source: src, target: tgt};
    });

    function renderForceGraph(nodes, links) {
      const svg = d3.select("#force-graph");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const g = svg.append("g");
      const sim = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(links).id(function(d){return d.id;}).distance(150))
        .force("charge", d3.forceManyBody().strength(-280))
        .force("center", d3.forceCenter(width/2, height/2));
      // Draw edges (lines)
      const link = g.append("g").selectAll("line").data(links).enter().append("line")
        .attr("stroke", d => edgeColor(d.type))
        .attr("stroke-width", 3.3)
        .attr("opacity", 0.93)
        .on("click", function(e, d) {
          e.stopPropagation();
          showModal("Edge: " + d.type,
            "<b>Source:</b> " + (d.source.label || d.source.id || d.source) + "<br>"
            + "<b>Target:</b> " + (d.target.label || d.target.id || d.target) + "<br>"
            + "<b>Type:</b> " + d.type + "<br>"
            + (d.type && d.type.toLowerCase() === "bid" && typeof d.bid_value === "number"
                ? "<b>Bid Value:</b> <span class='bid-highlight'>" + d.bid_value.toFixed(2) + "</span><br>"
                : "")
            + (d.reasoning ? "<br><b>Reasoning:</b> " + d.reasoning : "")
          );
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
            + "<b>Agent:</b> " + (d.source.label || d.source.id || d.source) + "<br>"
            + "<b>Task:</b> " + (d.target.label || d.target.id || d.target) + "<br>"
            + (d.reasoning ? "<br><b>Reasoning:</b> " + d.reasoning : "")
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
            + (d.Group ? "<b>Group:</b> " + d.Group + "<br>" : "")
            + (d.Desc ? "<b>Description:</b> " + d.Desc + "<br>" : "")
            + (d.PriceMin ? "<b>Price Min:</b> " + d.PriceMin + "<br>" : "")
            + (d.PriceMax ? "<b>Price Max:</b> " + d.PriceMax + "<br>" : "")
            + (d.Specialities && d.Specialities.length ?
                "<b>Specialities:</b> <ul style='margin:0 0 0 16px'>" +
                  d.Specialities.map(s => "<li>"+s.Name+"</li>").join("") +
                "</ul>" : "")
            + (d.Speciality ?
                "<b>Speciality:</b> " + d.Speciality.Name + "<br>" : "")
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
    }
    renderForceGraph(graph.nodes, links);
  </script>
</body>
</html>
`
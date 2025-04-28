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
          <svg id="force-graph" height="480"></svg>
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
          <svg id="price-demand-bar"></svg>
        </div>
      </div>
    </div>
    <!-- Panel 3: Competition vs. Labor Supply -->
    <div class="panel">
      <div class="panel-title">Competition vs. Labor Supply</div>
      <div class="viz-row">
        <div class="viz-card">
          <div class="viz-title">Agents per Speciality & Avg. Bids per Task</div>
          <svg id="supply-competition-scatter"></svg>
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
          <svg id="supply-demand-bar"></svg>
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
    function renderForceGraph(nodes, links) {
      const svg = d3.select("#force-graph");
      svg.selectAll("*").remove();
      const width = svg.node().clientWidth, height = svg.node().clientHeight;
      const g = svg.append("g");
      g.append("rect")
        .attr("width", width)
        .attr("height", height)
        .attr("fill", "#fafdff")
        .attr("stroke", "#e0e7ef")
        .attr("stroke-width", 2)
        .lower();
      svg.call(d3.zoom().scaleExtent([0.1, 3]).on("zoom", function(event) {
        g.attr("transform", "translate(" + event.transform.x + "," + event.transform.y + ") scale(" + event.transform.k + ")");
      }));

      const sim = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(links).id(function(d){return d.id;}).distance(180))
        .force("charge", d3.forceManyBody().strength(-320))
        .force("center", d3.forceCenter(width/2, height/2));

      const link = g.append("g").selectAll("line").data(links).enter().append("line")
        .attr("stroke", function(d) { return edgeColor(d.type); })
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

      const node = g.append("g").selectAll("circle").data(nodes).enter().append("circle")
        .attr("r", 21)
        .attr("fill", function(d){return nodeColor(d.type);})
        .attr("stroke", "#222").attr("stroke-width", 1.5)
        .style("cursor", "pointer")
        .on("mouseover", function(e,d){ d3.select(this).transition().attr("r",28); })
        .on("mouseout", function(e,d){ d3.select(this).transition().attr("r",21); })
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
                  d.Specialities.map(function(s){ return "<li>"+s.Name+"</li>"; }).join("") +
                "</ul>" : "")
            + (d.Speciality ?
                "<b>Speciality:</b> " + d.Speciality.Name + "<br>" : "")
          );
        });
      // Drag support
      const drag = d3.drag()
        .on("start", function (event, d) {
          if (!event.active) sim.alphaTarget(0.3).restart();
          d.fx = d.x;
          d.fy = d.y;
        })
        .on("drag", function (event, d) {
          d.fx = event.x;
          d.fy = event.y;
        })
        .on("end", function (event, d) {
          if (!event.active) sim.alphaTarget(0);
          d.fx = null;
          d.fy = null;
        });
      node.call(drag);

      g.append("g").selectAll("text.node-label").data(nodes).enter().append("text")
        .attr("class", "node-label")
        .attr("x", function(d){return d.x;})
        .attr("y", function(d){return d.y-28;})
        .attr("text-anchor","middle")
        .attr("dy",".35em").attr("fill","#233").attr("font-size","1em")
        .text(function(d){return d.label||d.id;});

      sim.on("tick", function(){
        link.attr("x1",function(d){return d.source.x;}).attr("y1",function(d){return d.source.y;})
            .attr("x2",function(d){return d.target.x;}).attr("y2",function(d){return d.target.y;});
        node.attr("cx",function(d){return d.x;}).attr("cy",function(d){return d.y;});
        g.selectAll("text.node-label").attr("x",function(d){return d.x;}).attr("y",function(d){return d.y-28;});
      });
    }
    // Prepare data for network graph
    let filteredEdges = graph.edges.filter(function(e) { return idToNode[e.source] && idToNode[e.target]; });
    let links = filteredEdges.map(function(e) {
      let src = idToNode[e.source];
      let tgt = idToNode[e.target];
      return Object.assign({}, e, {source: src, target: tgt});
    });
    renderForceGraph(graph.nodes, links);
  </script>








<!-- PANEL 2: Price vs. Demand per Skill -->
<script>
console.log("Panel 2: START", graph && graph.nodes);

function renderPriceDemandBar(nodes) {
  // Step 1: filter for issues with speciality and price_min
  var filtered = nodes.filter(function(n) {
    var isIssue = (n.type || "").toLowerCase() === "issue";
    var hasSpeciality = n.speciality && typeof n.speciality.name === "string";
    var hasPriceMin = typeof n.price_min === "number";
    return isIssue && hasSpeciality && hasPriceMin;
  });
  console.log("Panel 2: filtered issues with speciality and price_min", filtered);

  // Step 2: Aggregate by skill
  var specialities = {};
  filtered.forEach(function(n) {
    var skill = n.speciality.name;
    if (!specialities[skill]) specialities[skill] = {count:0, prices:[]};
    specialities[skill].count++;
    specialities[skill].prices.push(n.price_min);
  });

  // Step 3: Build data array for D3
  var data = Object.entries(specialities).map(function(entry){
    var name = entry[0], v = entry[1];
    return {
      name: name,
      count: v.count,
      avgPrice: v.prices.reduce(function(a,b){return a+b;},0)/v.prices.length
    };
  }).sort(function(a,b){return b.count-a.count;}).slice(0,15);

  console.log("Panel 2: final data array for chart", data);

  // Step 4: D3 rendering
  var svg = d3.select("#price-demand-bar");
  svg.selectAll("*").remove();
  var width = svg.node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:60, left:100};

  if (data.length === 0) {
    svg.append("text")
      .attr("x", width/2)
      .attr("y", height/2)
      .attr("text-anchor", "middle")
      .attr("fill", "gray")
      .text("No data available.");
    console.log("Panel 2: No data available to render.");
    return;
  }

  var x = d3.scaleBand().domain(data.map(function(d){return d.name;})).range([margin.left, width-margin.right]).padding(0.15);
  var yLeft = d3.scaleLinear().domain([0, d3.max(data,function(d){return d.count;}) || 1]).nice().range([height-margin.bottom, margin.top]);
  var yRight = d3.scaleLinear().domain([0, d3.max(data,function(d){return d.avgPrice;}) || 1]).nice().range([height-margin.bottom, margin.top]);

  // Demand bars
  svg.append("g")
    .selectAll("rect")
    .data(data)
    .enter().append("rect")
    .attr("x",function(d){return x(d.name);})
    .attr("y",function(d){return yLeft(d.count);})
    .attr("width",x.bandwidth()/2)
    .attr("height",function(d){return yLeft(0)-yLeft(d.count);})
    .attr("fill","#2563eb");

  // Price line
  var line = d3.line()
    .x(function(d){return x(d.name)+x.bandwidth()/4;})
    .y(function(d){return yRight(d.avgPrice);});
  svg.append("path")
    .datum(data)
    .attr("fill","none")
    .attr("stroke","#fd7e14")
    .attr("stroke-width",3)
    .attr("d",line);

  // Price dots
  svg.append("g").selectAll("circle")
    .data(data)
    .enter().append("circle")
    .attr("cx",function(d){return x(d.name)+x.bandwidth()/4;})
    .attr("cy",function(d){return yRight(d.avgPrice);})
    .attr("r",5)
    .attr("fill","#fd7e14");

  // X axis (skills)
  svg.append("g")
    .attr("transform","translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).tickFormat(function(d){return d;}).tickSizeOuter(0));

  // Left Y axis (demand)
  svg.append("g")
    .attr("transform","translate("+(margin.left)+",0)")
    .call(d3.axisLeft(yLeft).ticks(6))
    .append("text")
    .attr("fill","#2563eb").attr("x",-40).attr("y",margin.top-20)
    .attr("text-anchor","end").text("Demand (Issues)");

  // Right Y axis (price)
  svg.append("g")
    .attr("transform","translate("+(width-margin.right)+",0)")
    .call(d3.axisRight(yRight).ticks(6))
    .append("text")
    .attr("fill","#fd7e14").attr("x",40).attr("y",margin.top-20)
    .attr("text-anchor","start").text("Avg Price");

  console.log("Panel 2: Render complete.");
}
renderPriceDemandBar(graph.nodes);
</script>





  <!-- Panel 3: Competition vs. Labor Supply -->
<script>
function renderSupplyCompetitionScatter(nodes, edges) {
  // Supply: #agents per skill; Competition: avg bids per task per skill
  var agentSkills = {};
  nodes.forEach(function(n) {
    if ((n.type||"").toLowerCase() === "agent" && n.specialities) {
      n.specialities.forEach(function(s) {
        if (!agentSkills[s.name]) agentSkills[s.name]=0;
        agentSkills[s.name]++;
      });
    }
  });
  var taskSkills = {};
  nodes.forEach(function(n) {
    if ((n.type||"").toLowerCase() === "task" && n.speciality) {
      var key = n.speciality.name;
      if (!taskSkills[key]) taskSkills[key] = [];
      taskSkills[key].push(n.id);
    }
  });
  var bidsPerTask = {};
  Object.entries(taskSkills).forEach(function(entry) {
    var skill = entry[0], taskIds = entry[1];
    var bids = edges.filter(function(e){ return e.type && e.type.toLowerCase()==="bid" && taskIds.indexOf(e.target)!==-1; }).length;
    bidsPerTask[skill] = taskIds.length ? bids / taskIds.length : 0;
  });

  var allSkills = Array.from(new Set(Object.keys(agentSkills).concat(Object.keys(bidsPerTask))));
  var data = allSkills.map(function(skill){
    return {
      skill: skill,
      agents: agentSkills[skill]||0,
      avgBids: bidsPerTask[skill]||0
    };
  });

  var width = d3.select("#supply-competition-scatter").node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:60, left:70};
  var svg = d3.select("#supply-competition-scatter");
  svg.selectAll("*").remove();
  var x = d3.scaleLinear().domain([0, d3.max(data,function(d){return d.agents;})]).nice().range([margin.left, width-margin.right]);
  var y = d3.scaleLinear().domain([0, d3.max(data,function(d){return d.avgBids;})]).nice().range([height-margin.bottom, margin.top]);
  svg.append("g")
    .selectAll("circle")
    .data(data)
    .enter().append("circle")
    .attr("cx",function(d){return x(d.agents);})
    .attr("cy",function(d){return y(d.avgBids);})
    .attr("r",10)
    .attr("fill","#2563eb")
    .attr("opacity",0.7)
    .append("title").text(function(d){return d.skill;});
  svg.append("g")
    .attr("transform","translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).ticks(7));
  svg.append("g")
    .attr("transform","translate("+(margin.left)+",0)")
    .call(d3.axisLeft(y).ticks(7));
  svg.selectAll(".scatter-label")
    .data(data)
    .enter().append("text")
    .attr("x",function(d){return x(d.agents);})
    .attr("y",function(d){return y(d.avgBids)-14;})
    .text(function(d){return d.skill;})
    .attr("font-size","0.9em")
    .attr("fill","#374151")
    .attr("text-anchor","middle");
  svg.append("text").attr("x", width/2).attr("y", height-10).attr("text-anchor","middle").attr("fill","#374151").text("Labor Supply (Agents per Skill)");
  svg.append("text").attr("x", 20).attr("y", margin.top-20).attr("fill","#374151").text("Competition (Avg Bids per Task)");
}
renderSupplyCompetitionScatter(graph.nodes, graph.edges);
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
function renderSupplyDemandBar(nodes) {
  var skillDemand = {}, skillSupply = {};
  nodes.forEach(function(n) {
    if ((n.type||"").toLowerCase()==="task" && n.speciality) {
      var skill = n.speciality.name;
      if (!skillDemand[skill]) skillDemand[skill]=0;
      skillDemand[skill]++;
    }
    if ((n.type||"").toLowerCase()==="agent" && n.specialities) {
      n.specialities.forEach(function(s) {
        if (!skillSupply[s.name]) skillSupply[s.name]=0;
        skillSupply[s.name]++;
      });
    }
  });
  var allSkills = Array.from(new Set(Object.keys(skillDemand).concat(Object.keys(skillSupply))));
  var data = allSkills.map(function(skill){
    return {
      skill: skill,
      demand: skillDemand[skill]||0,
      supply: skillSupply[skill]||0
    };
  }).sort(function(a,b){return b.demand-a.demand;}).slice(0,15);

  var width = d3.select("#supply-demand-bar").node().clientWidth,
      height = 420, margin = {top:40, right:30, bottom:60, left:140};
  var svg = d3.select("#supply-demand-bar");
  svg.selectAll("*").remove();
  var y = d3.scaleBand().domain(data.map(function(d){return d.skill;})).range([margin.top, height-margin.bottom]).padding(0.18);
  var x = d3.scaleLinear().domain([0, d3.max(data,function(d){return Math.max(d.demand,d.supply);})]).nice().range([margin.left, width-margin.right]);
  // Demand bars
  svg.append("g")
    .selectAll("rect.demand")
    .data(data)
    .enter().append("rect")
    .attr("class","demand")
    .attr("y",function(d){return y(d.skill);})
    .attr("x",x(0))
    .attr("height",y.bandwidth()/2)
    .attr("width",function(d){return x(d.demand)-x(0);})
    .attr("fill","#fd7e14");
  // Supply bars
  svg.append("g")
    .selectAll("rect.supply")
    .data(data)
    .enter().append("rect")
    .attr("class","supply")
    .attr("y",function(d){return y(d.skill)+y.bandwidth()/2;})
    .attr("x",x(0))
    .attr("height",y.bandwidth()/2)
    .attr("width",function(d){return x(d.supply)-x(0);})
    .attr("fill","#2563eb");
  svg.append("g")
    .attr("transform","translate("+(margin.left)+",0)")
    .call(d3.axisLeft(y));
  svg.append("g")
    .attr("transform","translate(0,"+(height-margin.bottom)+")")
    .call(d3.axisBottom(x).ticks(7));
  svg.selectAll(".bar-label-demand")
    .data(data)
    .enter().append("text")
    .attr("class","bar-label-demand")
    .attr("y",function(d){return y(d.skill)+y.bandwidth()/2-5;})
    .attr("x",function(d){return x(d.demand)+8;})
    .text(function(d){return d.demand;});
  svg.selectAll(".bar-label-supply")
    .data(data)
    .enter().append("text")
    .attr("class","bar-label-supply")
    .attr("y",function(d){return y(d.skill)+y.bandwidth()-7;})
    .attr("x",function(d){return x(d.supply)+8;})
    .text(function(d){return d.supply;});
  svg.append("text").attr("x", width-150).attr("y", margin.top+10).attr("fill","#fd7e14").attr("font-size","1em").text("Demand (Tasks)");
  svg.append("text").attr("x", width-150).attr("y", margin.top+30).attr("fill","#2563eb").attr("font-size","1em").text("Supply (Agents)");
}
renderSupplyDemandBar(graph.nodes);
</script>
</body>
</html>
`

// --- Modal logic ---
const modal = document.getElementById("modal");
document.querySelector(".close").onclick = () => (modal.style.display = "none");
window.onclick = (e) => {
  if (e.target === modal) modal.style.display = "none";
};

// Color scale for specialties
let specialtyColorScale;

const groupMap = {
  Logic: ["ApplicationLogic", "ServerSideLogic"],
  "User Experience": ["UI/UX"],
  Quality: ["SystemQuality/Reliability", "ReliabilityImprovements"],
  Development: ["Bugfix", "NewFeatures/Enhancement"],
  External: ["Outsourced"],
};

let graphData = null, simulation = null;

document.addEventListener("DOMContentLoaded", function () {
  fetch("/data")
    .then((r) => r.json())
    .then((data) => {
      window.graphData = data;
      // Color scale for agent specialties
      const specialties = Array.from(
        new Set(data.nodes.filter((n) => n.role === "agent").map((n) => n.specialty || n.name))
      );
      specialtyColorScale = d3.scaleOrdinal(d3.schemeCategory10).domain(specialties);
      window.specialtyColorScale = specialtyColorScale;
      updateAgentLegend();
      createNetworkVisualization(data);
      renderMetrics(data);
      renderDegreeHistogram(data);
      renderLorenzCurve(data);
      renderWinrateDistribution(data);
      renderComponentHistogram(data);
      renderClusteringHistogram(data);
      renderCentralityHistogram(data);
    });
});

// --- Grouped legend ---
function updateAgentLegend() {
  const legendDiv = document.getElementById("legend-agents");
  if (!window.graphData || !window.specialtyColorScale) return;

  // specialty -> group
  let specialtyToGroup = {};
  Object.entries(groupMap).forEach(([group, specs]) => {
    specs.forEach((spec) => (specialtyToGroup[spec] = group));
  });

  // specialties present
  const specialties = Array.from(
    new Set(window.graphData.nodes.filter((n) => n.role === "agent").map((n) => n.specialty || n.name))
  );
  let grouped = {};
  specialties.forEach((spec) => {
    const group = specialtyToGroup[spec] || "Other";
    if (!grouped[group]) grouped[group] = [];
    grouped[group].push(spec);
  });

  // Render
  legendDiv.innerHTML = Object.entries(grouped)
    .map(
      ([group, specs]) =>
        `<div class="legend-group">
          <div class="legend-group-title">${group}</div>
          ${specs
            .map(
              (s) =>
                `<div class="legend-item"><span class="legend-color" style="background:${window.specialtyColorScale(
                  s
                )}"></span>${s}</div>`
            )
            .join("")}
        </div>`
    )
    .join("");
}

// --- Network graph ---
function createNetworkVisualization(data) {
  const container = d3.select("#network-container");
  container.selectAll("svg").remove();
  const width = container.node().clientWidth,
    height = container.node().clientHeight;
  const svg = container.append("svg").attr("width", width).attr("height", height);

  // Zoom/pan
  const zoom = d3.zoom().scaleExtent([0.2, 8]).on("zoom", (event) => {
    g.attr("transform", event.transform);
  });
  svg.call(zoom);
  const g = svg.append("g");

  // Arrow markers
  g.append("defs")
    .selectAll("marker")
    .data([
      { id: "assign", color: getComputedStyle(document.documentElement).getPropertyValue("--assign-color") },
      { id: "bid", color: getComputedStyle(document.documentElement).getPropertyValue("--bid-color") },
      { id: "out", color: getComputedStyle(document.documentElement).getPropertyValue("--out-color") },
    ])
    .enter()
    .append("marker")
    .attr("id", (d) => d.id)
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 17)
    .attr("refY", 0)
    .attr("markerWidth", 7)
    .attr("markerHeight", 7)
    .attr("orient", "auto")
    .append("path")
    .attr("d", "M0,-5L10,0L0,5")
    .attr("fill", (d) => d.color);

  // Force simulation
  simulation = d3
    .forceSimulation(data.nodes)
    .force("link", d3.forceLink(data.links).id((d) => d.id).distance(110))
    .force("charge", d3.forceManyBody().strength(-130))
    .force("center", d3.forceCenter(width / 2, height / 2))
    .force("collision", d3.forceCollide().radius(18));

  // Links
  const link = g
    .selectAll(".link")
    .data(data.links)
    .enter()
    .append("line")
    .attr("class", (d) => "link " + d.type)
    .attr("stroke", (d) => {
      if (d.type === "winner") return getComputedStyle(document.documentElement).getPropertyValue("--assign-color");
      if (d.type === "bid") return getComputedStyle(document.documentElement).getPropertyValue("--bid-color");
      if (d.type === "auctioneer") return "#ababab";
      return "#999";
    })
    .attr("stroke-width", (d) => (d.type === "auctioneer" ? 1.5 : 3.2))
    .attr("stroke-dasharray", (d) => (d.type === "bid" ? "4,4" : null))
    .attr("marker-end", (d) => {
      if (d.type === "winner") return "url(#assign)";
      if (d.type === "bid") return "url(#bid)";
      if (d.type === "auctioneer") return null;
    })
    .on("mouseover", showLinkTooltip)
    .on("mouseout", hideTooltip);

  // Nodes
  const nodes = g
    .selectAll(".node")
    .data(data.nodes)
    .enter()
    .append("circle")
    .attr("class", (d) => "node " + d.role)
    .attr("r", 13)
    .attr("fill", (d) => {
      if (d.role === "task") return getComputedStyle(document.documentElement).getPropertyValue("--task-color");
      return specialtyColorScale(d.specialty || d.name);
    })
    .call(drag(simulation))
    .on("mouseover", showNodeTooltip)
    .on("mouseout", hideTooltip)
    .on("click", showNodeDetails);

  // Node labels
  const nodeLabels = g
    .selectAll(".node-label")
    .data(data.nodes)
    .enter()
    .append("text")
    .attr("class", "node-label")
    .text((d) => d.name)
    .style("font-size", "11px")
    .style("text-anchor", "middle")
    .style("fill", "#222")
    .style("pointer-events", "none");

  simulation.on("tick", () => {
    link
      .attr("x1", (d) => d.source.x)
      .attr("y1", (d) => d.source.y)
      .attr("x2", (d) => d.target.x)
      .attr("y2", (d) => d.target.y);
    nodes.attr("cx", (d) => d.x).attr("cy", (d) => d.y);
    nodeLabels.attr("x", (d) => d.x).attr("y", (d) => d.y + 18);
  });
}

// --- Drag helpers ---
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
  return d3.drag().on("start", dragstarted).on("drag", dragged).on("end", dragended);
}

// --- Tooltip logic ---
function showNodeTooltip(event, d) {
  let html = `<div style="font-weight:600;">${d.name}</div>
    <div>Type: <b>${d.role === "task" ? "Task" : "Agent"}</b></div>`;
  if (d.role === "agent") {
    html += `<div>Specialty: <b>${d.specialty || d.name || "–"}</b></div>`;
  }
  const tip = d3.select("#tooltip");
  const container = document.getElementById("network-container");
  const rect = container.getBoundingClientRect();
  tip
    .style("display", "block")
    .style("left", event.clientX - rect.left + 15 + "px")
    .style("top", event.clientY - rect.top - 15 + "px")
    .html(html);
}
function showLinkTooltip(event, d) {
  let html = `<div style="font-weight:600;">Link</div>
    <div>Type: <b>${d.type || "–"}</b></div>`;
  if (d.amount != null) html += `<div>Amount: <b>${(+d.amount).toLocaleString(undefined, { maximumFractionDigits: 2 })}</b></div>`;
  if (d.reasoning) html += `<div style="color:#556;">${d.reasoning}</div>`;
  const tip = d3.select("#tooltip");
  const container = document.getElementById("network-container");
  const rect = container.getBoundingClientRect();
  tip
    .style("display", "block")
    .style("left", event.clientX - rect.left + 15 + "px")
    .style("top", event.clientY - rect.top - 15 + "px")
    .html(html);
}
function hideTooltip() {
  d3.select("#tooltip").style("display", "none");
}

// --- Modal node details ---
function showNodeDetails(e, d) {
  const modal = document.getElementById("modal");
  modal.style.display = "block";
  document.getElementById("modal-title").textContent = d.name;
  let html = `<p><b>ID:</b> ${d.id}</p>
    <p><b>Type:</b> ${d.role}</p>`;
  if (d.role === "agent") {
    html += `<p><b>Specialty:</b> ${d.specialty || d.name || "–"}</p>`;
  }
  document.getElementById("modal-body").innerHTML = html;
}

// --- Metrics ---
function renderMetrics(data) {
  if (!data) return;
  const agentCount = data.nodes.filter((n) => n.role === "agent").length;
  const taskCount = data.nodes.filter((n) => n.role === "task").length;
  const specialties = {};
  data.nodes.filter((n) => n.role === "agent").forEach((n) => {
    const spec = n.specialty || n.name;
    if (!specialties[spec]) specialties[spec] = 0;
    specialties[spec]++;
  });
  const winnerLinks = data.links.filter((l) => l.type === "winner");
  const bidLinks = data.links.filter((l) => l.type === "bid");
  const auctioneerLinks = data.links.filter((l) => l.type === "auctioneer");
  let metrics = [
    { label: "Agents", value: agentCount },
    { label: "Tasks", value: taskCount },
    { label: "Winners", value: winnerLinks.length },
    { label: "Bids", value: bidLinks.length },
    { label: "Auctioneers", value: auctioneerLinks.length },
  ];
  Object.entries(specialties).forEach(([spec, count]) => {
    metrics.push({ label: `Specialty: ${spec}`, value: count });
  });
  const panel = document.getElementById("metrics-panel");
  panel.innerHTML = metrics
    .map(
      (m) =>
        `<div class="metrics-card">
          <h5>${m.label}</h5>
          <div class="metric-value">${m.value}</div>
        </div>`
    )
    .join("");
}

// --- Economic/Structural Charts ---
// (same as before, but cleaned up for readability and robustness)

function renderDegreeHistogram(data) {
  d3.select("#degree-histogram").selectAll("*").remove();
  const nodes = data.nodes || [];
  if (!nodes.length) return;
  const degreeMap = {};
  (data.links || []).forEach((l) => {
    degreeMap[l.source] = (degreeMap[l.source] || 0) + 1;
    degreeMap[l.target] = (degreeMap[l.target] || 0) + 1;
  });
  nodes.forEach((n) => (n._deg = degreeMap[n.id] || 0));
  const degreeCounts = {};
  nodes.forEach((n) => {
    degreeCounts[n._deg] = (degreeCounts[n._deg] || 0) + 1;
  });
  const degrees = Object.keys(degreeCounts).map(Number).sort((a, b) => a - b);
  const counts = degrees.map((d) => degreeCounts[d]);
  const margin = { top: 30, right: 10, bottom: 25, left: 24 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#degree-histogram")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3.scaleBand().domain(degrees).range([margin.left, width + margin.left]).padding(0.18);
  const y = d3.scaleLinear().domain([0, d3.max(counts) || 1]).range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .selectAll(".bar")
    .data(degrees)
    .enter()
    .append("rect")
    .attr("x", (d) => x(d))
    .attr("y", (d) => y(degreeCounts[d]))
    .attr("width", x.bandwidth())
    .attr("height", (d) => height + margin.top - y(degreeCounts[d]))
    .attr("fill", "#4f8ffb");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Degree Dist")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

function renderLorenzCurve(data) {
  d3.select("#lorenz-curve").selectAll("*").remove();
  const agents = (data.nodes || []).filter((n) => n.role === "agent");
  const wins = agents.map((a) =>
    (data.links || [])
      .filter((l) => l.source === a.id && l.type === "winner")
      .reduce((s, l) => s + (l.amount || 0), 0)
  );
  if (!wins.length || wins.reduce((a, b) => a + b, 0) === 0) return;
  const sorted = wins.slice().sort((a, b) => a - b);
  const n = sorted.length,
    sum = sorted.reduce((a, b) => a + b, 0);
  let lorenzPoints = [{ x: 0, y: 0 }];
  let acc = 0;
  sorted.forEach((v, i) => {
    acc += v;
    lorenzPoints.push({ x: (i + 1) / n, y: acc / sum });
  });
  const margin = { top: 30, right: 10, bottom: 25, left: 28 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#lorenz-curve")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3.scaleLinear().domain([0, 1]).range([margin.left, width + margin.left]);
  const y = d3.scaleLinear().domain([0, 1]).range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x).ticks(3));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .append("path")
    .datum(lorenzPoints)
    .attr("fill", "none")
    .attr("stroke", "#e06a00")
    .attr("stroke-width", 2)
    .attr("d", d3.line().x((d) => x(d.x)).y((d) => y(d.y)));
  svg
    .append("line")
    .attr("x1", x(0))
    .attr("y1", y(0))
    .attr("x2", x(1))
    .attr("y2", y(1))
    .attr("stroke", "#aaa")
    .attr("stroke-dasharray", "3,3");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Lorenz Curve")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

function renderWinrateDistribution(data) {
  d3.select("#winrate-distribution").selectAll("*").remove();
  const agents = (data.nodes || []).filter((n) => n.role === "agent");
  const winrates = agents.map((a) => {
    const bids = (data.links || []).filter((l) => l.source === a.id && l.type === "bid").length;
    const wins = (data.links || []).filter((l) => l.source === a.id && l.type === "winner").length;
    return bids ? wins / bids : 0;
  });
  if (!winrates.length) return;
  const bins = d3.bin().domain([0, 1]).thresholds(7)(winrates);
  const margin = { top: 30, right: 10, bottom: 25, left: 28 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#winrate-distribution")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3.scaleLinear().domain([0, 1]).range([margin.left, width + margin.left]);
  const y = d3.scaleLinear().domain([0, d3.max(bins, (d) => d.length) || 1]).range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x).ticks(3));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .selectAll(".bar")
    .data(bins)
    .enter()
    .append("rect")
    .attr("x", (d) => x(d.x0))
    .attr("width", (d) => Math.max(0, x(d.x1) - x(d.x0) - 1))
    .attr("y", (d) => y(d.length))
    .attr("height", (d) => height + margin.top - y(d.length))
    .attr("fill", "#38a169");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Win Rate Dist")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

function getComponentSizes(nodes, links) {
  const adj = {};
  nodes.forEach((n) => (adj[n.id] = []));
  links.forEach((l) => {
    adj[l.source] && adj[l.source].push(l.target);
    adj[l.target] && adj[l.target].push(l.source);
  });
  const visited = new Set();
  let sizes = [];
  function dfs(id) {
    let stack = [id],
      count = 0;
    while (stack.length) {
      let curr = stack.pop();
      if (!visited.has(curr)) {
        visited.add(curr);
        count++;
        stack.push(...adj[curr].filter((n) => !visited.has(n)));
      }
    }
    return count;
  }
  nodes.forEach((n) => {
    if (!visited.has(n.id)) {
      sizes.push(dfs(n.id));
    }
  });
  return sizes;
}
function renderComponentHistogram(data) {
  d3.select("#component-histogram").selectAll("*").remove();
  const sizes = getComponentSizes(data.nodes || [], data.links || []);
  if (!sizes.length) return;
  const sizeCounts = {};
  sizes.forEach((s) => {
    sizeCounts[s] = (sizeCounts[s] || 0) + 1;
  });
  const uniqueSizes = Object.keys(sizeCounts)
    .map(Number)
    .sort((a, b) => a - b);
  const margin = { top: 30, right: 10, bottom: 25, left: 24 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#component-histogram")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3.scaleBand().domain(uniqueSizes).range([margin.left, width + margin.left]).padding(0.18);
  const y = d3
    .scaleLinear()
    .domain([0, d3.max(Object.values(sizeCounts)) || 1])
    .range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .selectAll(".bar")
    .data(uniqueSizes)
    .enter()
    .append("rect")
    .attr("x", (d) => x(d))
    .attr("y", (d) => y(sizeCounts[d]))
    .attr("width", x.bandwidth())
    .attr("height", (d) => height + margin.top - y(sizeCounts[d]))
    .attr("fill", "#e06a00");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Component Sizes")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

function getClusteringCoefficients(nodes, links) {
  const adj = {};
  nodes.forEach((n) => (adj[n.id] = new Set()));
  links.forEach((l) => {
    adj[l.source] && adj[l.source].add(l.target);
    adj[l.target] && adj[l.target].add(l.source);
  });
  const coeffs = [];
  nodes.forEach((n) => {
    const neighbors = Array.from(adj[n.id]);
    const k = neighbors.length;
    if (k < 2) {
      coeffs.push(0);
      return;
    }
    let linksBetween = 0;
    for (let i = 0; i < k; ++i)
      for (let j = i + 1; j < k; ++j) {
        if (adj[neighbors[i]].has(neighbors[j])) linksBetween++;
      }
    coeffs.push(linksBetween / (k * (k - 1) / 2));
  });
  return coeffs;
}
function renderClusteringHistogram(data) {
  d3.select("#clustering-histogram").selectAll("*").remove();
  const coeffs = getClusteringCoefficients(data.nodes || [], data.links || []);
  if (!coeffs.length) return;
  const bins = d3.bin().domain([0, 1]).thresholds(7)(coeffs);
  const margin = { top: 30, right: 10, bottom: 25, left: 28 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#clustering-histogram")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3.scaleLinear().domain([0, 1]).range([margin.left, width + margin.left]);
  const y = d3.scaleLinear().domain([0, d3.max(bins, (d) => d.length) || 1]).range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x).ticks(3));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .selectAll(".bar")
    .data(bins)
    .enter()
    .append("rect")
    .attr("x", (d) => x(d.x0))
    .attr("width", (d) => Math.max(0, x(d.x1) - x(d.x0) - 1))
    .attr("y", (d) => y(d.length))
    .attr("height", (d) => height + margin.top - y(d.length))
    .attr("fill", "#a259f7");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Clustering Coef")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

function getBetweennessCentrality(nodes, links) {
  const adj = {};
  nodes.forEach((n) => (adj[n.id] = []));
  links.forEach((l) => {
    adj[l.source] && adj[l.source].push(l.target);
    adj[l.target] && adj[l.target].push(l.source);
  });
  const bc = {};
  nodes.forEach((n) => (bc[n.id] = 0));
  nodes.forEach((s) => {
    const stack = [],
      pred = {},
      sigma = {},
      dist = {};
    nodes.forEach((v) => {
      pred[v.id] = [];
      sigma[v.id] = 0;
      dist[v.id] = -1;
    });
    sigma[s.id] = 1;
    dist[s.id] = 0;
    let queue = [s.id];
    while (queue.length) {
      let v = queue.shift();
      stack.push(v);
      adj[v].forEach((w) => {
        if (dist[w] < 0) {
          dist[w] = dist[v] + 1;
          queue.push(w);
        }
        if (dist[w] === dist[v] + 1) {
          sigma[w] += sigma[v];
          pred[w].push(v);
        }
      });
    }
    const delta = {};
    nodes.forEach((v) => (delta[v.id] = 0));
    while (stack.length) {
      let w = stack.pop();
      pred[w].forEach((v) => {
        delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w]);
      });
      if (w !== s.id) bc[w] += delta[w];
    }
  });
  return nodes.map((n) => bc[n.id]);
}
function renderCentralityHistogram(data) {
  d3.select("#centrality-histogram").selectAll("*").remove();
  const bc = getBetweennessCentrality(data.nodes || [], data.links || []);
  if (!bc.length) return;
  const bins = d3.bin().domain([0, d3.max(bc) || 1]).thresholds(7)(bc);
  const margin = { top: 30, right: 10, bottom: 25, left: 28 },
    width = 170,
    height = 80;
  const svg = d3
    .select("#centrality-histogram")
    .append("svg")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);
  const x = d3
    .scaleLinear()
    .domain([0, d3.max(bc) || 1])
    .range([margin.left, width + margin.left]);
  const y = d3
    .scaleLinear()
    .domain([0, d3.max(bins, (d) => d.length) || 1])
    .range([height + margin.top, margin.top]);
  svg.append("g").attr("transform", `translate(0,${height + margin.top})`).call(d3.axisBottom(x).ticks(3));
  svg.append("g").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(3));
  svg
    .selectAll(".bar")
    .data(bins)
    .enter()
    .append("rect")
    .attr("x", (d) => x(d.x0))
    .attr("width", (d) => Math.max(0, x(d.x1) - x(d.x0) - 1))
    .attr("y", (d) => y(d.length))
    .attr("height", (d) => height + margin.top - y(d.length))
    .attr("fill", "#fc3a3a");
  svg
    .append("text")
    .attr("x", width / 2)
    .attr("y", 18)
    .text("Betweenness Cent")
    .attr("text-anchor", "middle")
    .attr("font-size", "11px")
    .attr("fill", "#333");
}

// Dynamic timestamp and user identification - client-side generation
function updateTimestamp() {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    const hours = String(now.getHours()).padStart(2, '0');
    const minutes = String(now.getMinutes()).padStart(2, '0');
    const seconds = String(now.getSeconds()).padStart(2, '0');
    
    const formattedDate = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    document.getElementById('current-time').textContent = formattedDate;
}

function generateUserIdentifier() {
    // Create a unique identifier based on browser info and session
    const browserInfo = navigator.userAgent || '';
    const browserName = browserInfo.split(' ')[0] || 'browser';
    const randomId = Math.floor(Math.random() * 10000);
    const sessionId = sessionStorage.getItem('sessionId') || Math.floor(Math.random() * 1000000);
    
    // Store session ID if not already stored
    if (!sessionStorage.getItem('sessionId')) {
        sessionStorage.setItem('sessionId', sessionId);
    }
    
    return `${browserName.substring(0, 5)}_${sessionId}_${randomId}`;
}

function setUserIdentifier() {
    document.getElementById('current-user').textContent = generateUserIdentifier();
}

// Tooltip functionality
const tooltip = d3.select('#tooltip');

function hideTooltip() {
    tooltip.style('display', 'none');
}

function showTooltip(event, d, content) {
    tooltip.style('display', 'block')
        .style('left', (event.pageX + 15) + 'px')
        .style('top', (event.pageY - 10) + 'px')
        .html(content);
}

// Modal functionality
const modal = document.getElementById('detail-modal');
const closeModal = document.querySelector('.close-modal');

function showModal(title, content) {
    document.getElementById('modal-title').textContent = title;
    document.getElementById('modal-content').innerHTML = content;
    modal.style.display = 'block';
}

function hideModal() {
    modal.style.display = 'none';
}

// Close modal when clicking the X
if (closeModal) {
    closeModal.addEventListener('click', hideModal);
}

// Close modal when clicking outside the content
window.addEventListener('click', function(event) {
    if (event.target === modal) {
        hideModal();
    }
});

// Load and initialize visualizations on document ready
document.addEventListener('DOMContentLoaded', function() {
    // Initialize dynamic timestamp and user ID
    updateTimestamp();
    setUserIdentifier();
    
    // Update timestamp every minute
    setInterval(updateTimestamp, 60000);
    
    // Load data and create visualizations
    Promise.all([
        d3.json('/data'),
        d3.json('/api/market-metrics'),
        d3.json('/api/agent-metrics'),
        d3.json('/api/task-metrics'),
        d3.json('/api/lorenz-curve')
    ]).then(function([graphData, marketMetrics, agentMetrics, taskMetrics, lorenzData]) {
        // Create main network visualization
        createNetworkVisualization(graphData);
        
        // Create dimension-specific visualizations
        createMarketLiquidityVisualization(graphData, marketMetrics);
        createCompetitionVisualization(graphData, taskMetrics);
        createEfficiencyVisualization(graphData, taskMetrics);
        createEquityVisualization(lorenzData, agentMetrics);
        createStrategicFrictionVisualization(agentMetrics);
        createRobustnessVisualization(marketMetrics);
        
        // Set up event handlers for interactive elements
        setupEventHandlers(graphData);
        
    }).catch(function(error) {
        console.error('Error loading data:', error);
    });
});

// Network visualization
function createNetworkVisualization(graphData) {
    const container = d3.select('#network-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    
    // Create simulation for force-directed layout
    const simulation = d3.forceSimulation(graphData.nodes)
        .force('link', d3.forceLink(graphData.links).id(d => d.id).distance(80))
        .force('charge', d3.forceManyBody().strength(-120))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(20));
    
    // Clear existing SVG
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .call(d3.zoom().on('zoom', e => g.attr('transform', e.transform)));
    
    const g = svg.append('g');
    
    // Define arrow markers for link types
    g.append('defs').selectAll('marker')
        .data(['assigned', 'bidded', 'outsourced'])
        .enter().append('marker')
        .attr('id', d => `arrow-${d}`)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 15)
        .attr('refY', 0)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', d => {
            if (d === 'assigned') return 'var(--assign-color)';
            if (d === 'bidded') return 'var(--bid-color)';
            return 'var(--out-color)';
        });
    
    // Create links
    const links = g.selectAll('.link')
        .data(graphData.links)
        .enter().append('line')
        .attr('class', d => `link ${d.type}`)
        .attr('marker-end', d => `url(#arrow-${d.type})`)
        .on('mouseover', function(event, d) {
            d3.select(this).style('stroke-width', '3px');
            
            const sourceNode = graphData.nodes.find(n => n.id === d.source.id || n.id === d.source);
            const targetNode = graphData.nodes.find(n => n.id === d.target.id || n.id === d.target);
            
            let content = `<div class="tooltip-title">${d.type.charAt(0).toUpperCase() + d.type.slice(1)} Relationship</div>`;
            content += `<div>From: ${sourceNode ? sourceNode.name : d.source}</div>`;
            content += `<div>To: ${targetNode ? targetNode.name : d.target}</div>`;
            
            if (d.weight) content += `<div>Weight: ${d.weight}</div>`;
            if (d.WinningBid) content += `<div>Value: $${d.WinningBid.toFixed(2)}</div>`;
            if (d.BidCount) content += `<div>Bids: ${d.BidCount}</div>`;
            
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this).style('stroke-width', '1.5px');
            hideTooltip();
        });
    
    // Create nodes
    const nodes = g.selectAll('.node')
        .data(graphData.nodes)
        .enter().append('circle')
        .attr('class', d => `node ${d.role}${d.specialist ? ' specialist' : ''}`)
        .attr('r', d => 8 + Math.sqrt(d.degree || 1))
        .call(d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended))
        .on('mouseover', function(event, d) {
            d3.select(this).attr('stroke', '#000').attr('stroke-width', 2);
            
            let content = `<div class="tooltip-title">${d.name}</div>`;
            content += `<div>Type: ${d.role.charAt(0).toUpperCase() + d.role.slice(1)}</div>`;
            if (d.role === 'agent') content += `<div>Specialist: ${d.specialist ? 'Yes' : 'No'}</div>`;
            content += `<div>Connections: ${d.degree || 0}</div>`;
            
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this).attr('stroke', null).attr('stroke-width', null);
            hideTooltip();
        })
        .on('click', function(event, d) {
            // Show detailed information in modal
            let content = `<p><strong>ID:</strong> ${d.id}</p>`;
            content += `<p><strong>Type:</strong> ${d.role.charAt(0).toUpperCase() + d.role.slice(1)}</p>`;
            
            if (d.role === 'agent') {
                content += `<p><strong>Specialist:</strong> ${d.specialist ? 'Yes' : 'No'}</p>`;
                
                // Find agent's bids and assignments
                const bids = graphData.links.filter(link => 
                    (link.source.id === d.id || link.source === d.id) && link.type === 'bidded'
                );
                
                const assignments = graphData.links.filter(link => 
                    (link.source.id === d.id || link.source === d.id) && link.type === 'assigned'
                );
                
                const winRate = bids.length > 0 ? (assignments.length / bids.length * 100).toFixed(1) : 'N/A';
                
                content += `<h4>Performance Metrics</h4>`;
                content += `<p><strong>Total Bids:</strong> ${bids.length}</p>`;
                content += `<p><strong>Assignments Won:</strong> ${assignments.length}</p>`;
                content += `<p><strong>Win Rate:</strong> ${winRate}%</p>`;
                
                if (assignments.length > 0) {
                    const totalValue = assignments.reduce((sum, a) => sum + (a.WinningBid || 0), 0);
                    content += `<p><strong>Total Value:</strong> $${totalValue.toFixed(2)}</p>`;
                    content += `<p><strong>Average Value:</strong> $${(totalValue / assignments.length).toFixed(2)}</p>`;
                }
            } else if (d.role === 'task') {
                // Find bids and assignments for this task
                const bids = graphData.links.filter(link => 
                    (link.target.id === d.id || link.target === d.id) && link.type === 'bidded'
                );
                
                const assignment = graphData.links.find(link => 
                    (link.target.id === d.id || link.target === d.id) && link.type === 'assigned'
                );
                
                content += `<h4>Task Details</h4>`;
                content += `<p><strong>Bids Received:</strong> ${bids.length}</p>`;
                
                if (assignment) {
                    const winningAgent = graphData.nodes.find(n => 
                        n.id === (typeof assignment.source === 'object' ? assignment.source.id : assignment.source)
                    );
                    
                    content += `<p><strong>Status:</strong> Assigned</p>`;
                    content += `<p><strong>Assigned To:</strong> ${winningAgent ? winningAgent.name : 'Unknown'}</p>`;
                    if (assignment.WinningBid) content += `<p><strong>Value:</strong> $${assignment.WinningBid.toFixed(2)}</p>`;
                } else {
                    content += `<p><strong>Status:</strong> Unassigned</p>`;
                }
                
                if (bids.length > 0) {
                    const bidValues = bids.map(b => b.WinningBid || 0).filter(v => v > 0);
                    if (bidValues.length > 0) {
                        const avgBid = bidValues.reduce((sum, v) => sum + v, 0) / bidValues.length;
                        content += `<p><strong>Average Bid:</strong> $${avgBid.toFixed(2)}</p>`;
                    }
                }
            }
            
            showModal(d.name, content);
        });
    
    // Create node labels
    const labels = g.selectAll('.node-label')
        .data(graphData.nodes)
        .enter().append('text')
        .attr('class', 'node-label')
        .attr('dx', 12)
        .attr('dy', 4)
        .text(d => d.name);
    
    // Update positions on simulation tick
    simulation.on('tick', () => {
        links
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);
        
        nodes
            .attr('cx', d => d.x)
            .attr('cy', d => d.y);
        
        labels
            .attr('x', d => d.x)
            .attr('y', d => d.y);
    });
    
    // Drag functions
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
}

// Market Liquidity Visualization
function createMarketLiquidityVisualization(graphData, marketMetrics) {
    // Degree distribution visualization
    const container = d3.select('#market-liquidity-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 20, bottom: 50, left: 60};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Use degree distribution data
    const data = marketMetrics.degree_distribution;
    
    // Create scales
    const x = d3.scaleBand()
        .domain(data.map(d => d.degree))
        .range([0, innerWidth])
        .padding(0.1);
    
    const y = d3.scaleLog()
        .domain([0.9, d3.max(data, d => d.count) * 1.1])
        .range([innerHeight, 0]);
    
    // Create axes
    const xAxis = d3.axisBottom(x)
        .tickValues(x.domain().filter(d => d % 5 === 0)); // Show every 5th tick for readability
    
    const yAxis = d3.axisLeft(y)
        .ticks(5)
        .tickFormat(d => d3.format(',')(Math.round(d)));
    
    svg.append('g')
        .attr('transform', `translate(0,${innerHeight})`)
        .call(xAxis)
        .selectAll('text')
        .style('text-anchor', 'end')
        .attr('dx', '-.8em')
        .attr('dy', '.15em')
        .attr('transform', 'rotate(-45)');
    
    svg.append('g')
        .call(yAxis);
    
    // Create bars
    svg.selectAll('.bar')
        .data(data)
        .enter().append('rect')
        .attr('class', 'bar')
        .attr('x', d => x(d.degree))
        .attr('y', d => y(d.count))
        .attr('width', x.bandwidth())
        .attr('height', d => innerHeight - y(d.count))
        .attr('fill', 'var(--accent-color)')
        .on('mouseover', function(event, d) {
            d3.select(this).attr('fill', d3.color('var(--accent-color)').darker(0.5));
            const content = `<div class="tooltip-title">Degree: ${d.degree}</div>
                            <div>Nodes: ${d.count}</div>
                            <div>Percentage: ${(d.count / graphData.nodes.length * 100).toFixed(1)}%</div>`;
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this).attr('fill', 'var(--accent-color)');
            hideTooltip();
        });
    
    // Add axis labels
    svg.append('text')
        .attr('class', 'x-axis-label')
        .attr('x', innerWidth / 2)
        .attr('y', innerHeight + 40)
        .attr('text-anchor', 'middle')
        .text('Node Degree (Connections)');
    
    svg.append('text')
        .attr('class', 'y-axis-label')
        .attr('transform', 'rotate(-90)')
        .attr('x', -innerHeight / 2)
        .attr('y', -40)
        .attr('text-anchor', 'middle')
        .text('Count (log scale)');
    
    // Add title
    svg.append('text')
        .attr('class', 'chart-title')
        .attr('x', innerWidth / 2)
        .attr('y', -20)
        .attr('text-anchor', 'middle')
        .text('Market Liquidity: Degree Distribution');
    
    // Add network density indicator
    svg.append('text')
        .attr('class', 'network-density')
        .attr('x', innerWidth - 10)
        .attr('y', 10)
        .attr('text-anchor', 'end')
        .text(`Network Density: ${(marketMetrics.network_density * 100).toFixed(2)}%`);
}

// Competition Visualization (Boxplots)
function createCompetitionVisualization(graphData, taskMetrics) {
    const container = d3.select('#competition-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 20, bottom: 60, left: 70};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Filter to tasks with bids
    const tasksWithBids = taskMetrics.filter(t => t.bid_count > 0);
    
    // Sort by coefficient of variation
    tasksWithBids.sort((a, b) => b.cov - a.cov);
    
    // Only show top 15 tasks for readability
    const displayedTasks = tasksWithBids.slice(0, 15);
    
    // Create scales
    const x = d3.scaleBand()
        .domain(displayedTasks.map(d => d.name))
        .range([0, innerWidth])
        .padding(0.2);
    
    const y = d3.scaleLinear()
        .domain([0, d3.max(displayedTasks, d => d.avg_bid * 1.5) || 100])
        .range([innerHeight, 0]);
    
    // Create axes
    svg.append('g')
        .attr('transform', `translate(0,${innerHeight})`)
        .call(d3.axisBottom(x))
        .selectAll('text')
        .style('text-anchor', 'end')
        .attr('dx', '-.8em')
        .attr('dy', '.15em')
        .attr('transform', 'rotate(-45)');
    
    svg.append('g')
        .call(d3.axisLeft(y).tickFormat(d => `$${d}`));
    
    // Create boxplots
    displayedTasks.forEach(task => {
        // Calculate boxplot metrics using task.avg_bid and task.std_dev
        const q1 = task.avg_bid - task.std_dev;
        const q3 = task.avg_bid + task.std_dev;
        const median = task.avg_bid;
        const iqr = q3 - q1;
        const min = Math.max(0, q1 - 1.5 * iqr);
        const max = q3 + 1.5 * iqr;
        
        // Draw box
        svg.append('rect')
            .attr('x', x(task.name))
            .attr('y', y(q3))
            .attr('width', x.bandwidth())
            .attr('height', y(q1) - y(q3))
            .attr('fill', 'steelblue')
            .attr('opacity', 0.7)
            .attr('stroke', '#000')
            .on('mouseover', function(event, d) {
                d3.select(this).attr('opacity', 1);
                const content = `<div class="tooltip-title">${task.name}</div>
                                <div>Bids: ${task.bid_count}</div>
                                <div>Avg Bid: $${task.avg_bid.toFixed(2)}</div>
                                <div>Std Dev: $${task.std_dev.toFixed(2)}</div>
                                <div>CV: ${(task.cov * 100).toFixed(1)}%</div>`;
                showTooltip(event, task, content);
            })
            .on('mouseout', function() {
                d3.select(this).attr('opacity', 0.7);
                hideTooltip();
            });
        
        // Draw median line
        svg.append('line')
            .attr('x1', x(task.name))
            .attr('x2', x(task.name) + x.bandwidth())
            .attr('y1', y(median))
            .attr('y2', y(median))
            .attr('stroke', '#000')
            .attr('stroke-width', 2);
        
        // Draw whiskers
        svg.append('line')
            .attr('x1', x(task.name) + x.bandwidth()/2)
            .attr('x2', x(task.name) + x.bandwidth()/2)
            .attr('y1', y(min))
            .attr('y2', y(q1))
            .attr('stroke', '#000')
            .attr('stroke-width', 1);
        
        svg.append('line')
            .attr('x1', x(task.name) + x.bandwidth()/2)
            .attr('x2', x(task.name) + x.bandwidth()/2)
            .attr('y1', y(q3))
            .attr('y2', y(max))
            .attr('stroke', '#000')
            .attr('stroke-width', 1);
        
        // Draw caps
        svg.append('line')
            .attr('x1', x(task.name) + x.bandwidth()/4)
            .attr('x2', x(task.name) + x.bandwidth()*3/4)
            .attr('y1', y(min))
            .attr('y2', y(min))
            .attr('stroke', '#000')
            .attr('stroke-width', 1);
        
        svg.append('line')
            .attr('x1', x(task.name) + x.bandwidth()/4)
            .attr('x2', x(task.name) + x.bandwidth()*3/4)
            .attr('y1', y(max))
            .attr('y2', y(max))
            .attr('stroke', '#000')
            .attr('stroke-width', 1);
        
        // Draw winning bid
        if (task.winning_bid > 0) {
            svg.append('circle')
                .attr('cx', x(task.name) + x.bandwidth()/2)
                .attr('cy', y(task.winning_bid))
                .attr('r', 4)
                .attr('fill', 'red')
                .on('mouseover', function(event, d) {
                    d3.select(this).attr('r', 6);
                    const content = `<div class="tooltip-title">Winning Bid</div>
                                    <div>Value: $${task.winning_bid.toFixed(2)}</div>
                                    <div>Diff from Avg: ${((task.winning_bid - task.avg_bid) / task.avg_bid * 100).toFixed(1)}%</div>`;
                    showTooltip(event, task, content);
                })
                .on('mouseout', function() {
                    d3.select(this).attr('r', 4);
                    hideTooltip();
                });
        }
    });
    
    // Add title and axis labels
    svg.append('text')
        .attr('class', 'chart-title')
        .attr('x', innerWidth / 2)
        .attr('y', -20)
        .attr('text-anchor', 'middle')
        .text('Competition: Bid Variance Per Task');
    
    svg.append('text')
        .attr('x', innerWidth / 2)
        .attr('y', innerHeight + 50)
        .attr('text-anchor', 'middle')
        .text('Tasks');
    
    svg.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('x', -innerHeight / 2)
        .attr('y', -45)
        .attr('text-anchor', 'middle')
        .text('Bid Amount ($)');
    
    // Add legend
    const legend = svg.append('g')
        .attr('transform', `translate(${innerWidth - 120}, 10)`);
    
    legend.append('rect')
        .attr('x', 0)
        .attr('y', 0)
        .attr('width', 15)
        .attr('height', 15)
        .attr('fill', 'steelblue')
        .attr('opacity', 0.7);
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 7.5)
        .attr('dy', '.35em')
        .text('Bid Range');
    
    legend.append('circle')
        .attr('cx', 7.5)
        .attr('cy', 30)
        .attr('r', 4)
        .attr('fill', 'red');
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 30)
        .attr('dy', '.35em')
        .text('Winning Bid');
}

// Efficiency Visualization
function createEfficiencyVisualization(graphData, taskMetrics) {
    const container = d3.select('#efficiency-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 30, bottom: 60, left: 70};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Filter tasks with both winning bids and average bids
    const tasksWithSurplus = taskMetrics.filter(t => t.winning_bid > 0 && t.avg_bid > 0);
    
    // Sort by client surplus (descending)
    tasksWithSurplus.sort((a, b) => b.client_surplus - a.client_surplus);
    
    // Limit to top 15 tasks for readability
    const displayedTasks = tasksWithSurplus.slice(0, 15);
    
    // Create scales
    const x = d3.scaleBand()
        .domain(displayedTasks.map(d => d.name))
        .range([0, innerWidth])
        .padding(0.2);
    
    const y = d3.scaleLinear()
        .domain([0, d3.max(displayedTasks, d => Math.max(d.avg_bid, d.winning_bid)) * 1.1])
        .range([innerHeight, 0]);
    
    // Create axes
    svg.append('g')
        .attr('transform', `translate(0,${innerHeight})`)
        .call(d3.axisBottom(x))
        .selectAll('text')
        .style('text-anchor', 'end')
        .attr('dx', '-.8em')
        .attr('dy', '.15em')
        .attr('transform', 'rotate(-45)');
    
    svg.append('g')
        .call(d3.axisLeft(y).tickFormat(d => `$${d}`));
    
    // Draw average bid bars
    svg.selectAll('.avg-bar')
        .data(displayedTasks)
        .enter().append('rect')
        .attr('class', 'avg-bar')
        .attr('x', d => x(d.name))
        .attr('y', d => y(d.avg_bid))
        .attr('width', x.bandwidth())
        .attr('height', d => innerHeight - y(d.avg_bid))
        .attr('fill', '#ccc')
        .attr('opacity', 0.6);
    
    // Draw winning bid bars
    svg.selectAll('.win-bar')
        .data(displayedTasks)
        .enter().append('rect')
        .attr('class', 'win-bar')
        .attr('x', d => x(d.name))
        .attr('y', d => y(d.winning_bid))
        .attr('width', x.bandwidth())
        .attr('height', d => innerHeight - y(d.winning_bid))
        .attr('fill', 'var(--bid-color)')
        .attr('opacity', 0.8)
        .on('mouseover', function(event, d) {
            d3.select(this).attr('opacity', 1);
            const surplusPct = (d.client_surplus / d.avg_bid * 100).toFixed(1);
            const content = `<div class="tooltip-title">${d.name}</div>
                            <div>Avg Bid: $${d.avg_bid.toFixed(2)}</div>
                            <div>Winning Bid: $${d.winning_bid.toFixed(2)}</div>
                            <div>Client Surplus: $${d.client_surplus.toFixed(2)} (${surplusPct}%)</div>`;
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this).attr('opacity', 0.8);
            hideTooltip();
        });
    
    // Draw surplus line for each task
    displayedTasks.forEach(task => {
        if (task.client_surplus > 0) {
            svg.append('line')
                .attr('x1', x(task.name) + x.bandwidth()/2)
                .attr('x2', x(task.name) + x.bandwidth()/2)
                .attr('y1', y(task.winning_bid))
                .attr('y2', y(task.avg_bid))
                .attr('stroke', 'var(--accent-color)')
                .attr('stroke-width', 2)
                .attr('stroke-dasharray', '3,3');
            
            // Add surplus label
            svg.append('text')
                .attr('x', x(task.name) + x.bandwidth()/2)
                .attr('y', y((task.winning_bid + task.avg_bid)/2))
                .attr('text-anchor', 'middle')
                .attr('dy', '0.35em')
                .attr('font-size', '10px')
                .attr('fill', 'var(--accent-color)')
                .text(`${(task.client_surplus / task.avg_bid * 100).toFixed(0)}%`);
        }
    });
    
    // Calculate total client surplus
    const totalSurplus = displayedTasks.reduce((sum, t) => sum + t.client_surplus, 0);
    const avgSurplusPct = displayedTasks.reduce((sum, t) => sum + (t.client_surplus / t.avg_bid), 0) / displayedTasks.length * 100;
    
    // Add title and metrics
    svg.append('text')
        .attr('class', 'chart-title')
        .attr('x', innerWidth / 2)
        .attr('y', -20)
        .attr('text-anchor', 'middle')
        .text('Efficiency: Price Efficiency & Client Surplus');
    
    svg.append('text')
        .attr('x', innerWidth - 10)
        .attr('y', 10)
        .attr('text-anchor', 'end')
        .attr('font-size', '12px')
        .text(`Avg Surplus: ${avgSurplusPct.toFixed(1)}% ($${(totalSurplus/displayedTasks.length).toFixed(2)})`);
    
    // Add axis labels
    svg.append('text')
        .attr('x', innerWidth / 2)
        .attr('y', innerHeight + 50)
        .attr('text-anchor', 'middle')
        .text('Tasks');
    
    svg.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('x', -innerHeight / 2)
        .attr('y', -45)
        .attr('text-anchor', 'middle')
        .text('Bid Amount ($)');
    
    // Add legend
    const legend = svg.append('g')
        .attr('transform', `translate(10, 10)`);
    
    legend.append('rect')
        .attr('x', 0)
        .attr('y', 0)
        .attr('width', 15)
        .attr('height', 15)
        .attr('fill', '#ccc')
        .attr('opacity', 0.6);
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 7.5)
        .attr('dy', '.35em')
        .text('Average Bid');
    
    legend.append('rect')
        .attr('x', 0)
        .attr('y', 20)
        .attr('width', 15)
        .attr('height', 15)
        .attr('fill', 'var(--bid-color)')
        .attr('opacity', 0.8);
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 27.5)
        .attr('dy', '.35em')
        .text('Winning Bid');
    
    legend.append('line')
        .attr('x1', 0)
        .attr('x2', 15)
        .attr('y1', 40)
        .attr('y2', 40)
        .attr('stroke', 'var(--accent-color)')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '3,3');
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 40)
        .attr('dy', '.35em')
        .text('Client Surplus');
}

// Equity Visualization (Lorenz Curve)
function createEquityVisualization(lorenzData, agentMetrics) {
    const container = d3.select('#equity-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 120, bottom: 60, left: 60};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Create diagonal line of perfect equality
    const equalityLine = [
        {x: 0, y: 0},
        {x: 1, y: 1}
    ];
    
    // Set up scales
    const x = d3.scaleLinear()
        .domain([0, 1])
        .range([0, innerWidth]);
    
    const y = d3.scaleLinear()
        .domain([0, 1])
        .range([innerHeight, 0]);
    
    // Create line generator
    const line = d3.line()
        .x(d => x(d.x))
        .y(d => y(d.y))
        .curve(d3.curveBasis);
    
    // Draw equality line
    svg.append('path')
        .datum(equalityLine)
        .attr('class', 'equality-line')
        .attr('fill', 'none')
        .attr('stroke', '#999')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,4')
        .attr('d', line);
    
    // Draw Lorenz curve
    svg.append('path')
        .datum(lorenzData)
        .attr('class', 'lorenz-curve')
        .attr('fill', 'none')
        .attr('stroke', 'var(--accent-color)')
        .attr('stroke-width', 2)
        .attr('d', line);
    
    // Calculate and display Gini coefficient (area between equality line and Lorenz curve)
    let area = 0;
    for (let i = 1; i < lorenzData.length; i++) {
        const width = lorenzData[i].x - lorenzData[i-1].x;
        const avgHeight = (lorenzData[i].y + lorenzData[i-1].y) / 2;
        area += width * avgHeight;
    }
    
    const gini = 1 - 2 * area;
    
    svg.append('text')
        .attr('x', innerWidth - 10)
        .attr('y', 20)
        .attr('text-anchor', 'end')
        .attr('font-weight', 'bold')
        .text(`Gini Coefficient: ${gini.toFixed(3)}`);
    
    // Add points on the Lorenz curve
    svg.selectAll('.lorenz-point')
        .data(lorenzData.filter((d, i) => i > 0 && i < lorenzData.length - 1 && i % 5 === 0)) // Add points every 5th data point
        .enter().append('circle')
        .attr('class', 'lorenz-point')
        .attr('cx', d => x(d.x))
        .attr('cy', d => y(d.y))
        .attr('r', 3)
        .attr('fill', d => d.specialist ? 'var(--specialist-color)' : 'var(--bidder-color)')
        .attr('stroke', '#fff')
        .attr('stroke-width', 1)
        .on('mouseover', function(event, d) {
            d3.select(this).attr('r', 5);
            const content = `<div class="tooltip-title">Population: ${(d.x * 100).toFixed(1)}%</div>
                             <div>Income Share: ${(d.y * 100).toFixed(1)}%</div>`;
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this).attr('r', 3);
            hideTooltip();
        });
    
    // Calculate area to color in 
    const areaData = [...lorenzData];
    areaData.push({x: 1, y: 0});
    areaData.unshift({x: 0, y: 0});
    
    const areaGenerator = d3.area()
        .x(d => x(d.x))
        .y0(y(0))
        .y1(d => y(d.y))
        .curve(d3.curveBasis);
    
    svg.append('path')
        .datum(areaData)
        .attr('class', 'lorenz-area')
        .attr('fill', 'var(--accent-color)')
        .attr('opacity', 0.1)
        .attr('d', areaGenerator);
    
    // Add axes
    svg.append('g')
        .attr('transform', `translate(0,${innerHeight})`)
        .call(d3.axisBottom(x).tickFormat(d3.format('.0%')));
    
    svg.append('g')
        .call(d3.axisLeft(y).tickFormat(d3.format('.0%')));
    
    // Add axis labels
    svg.append('text')
        .attr('x', innerWidth / 2)
        .attr('y', innerHeight + 40)
        .attr('text-anchor', 'middle')
        .text('Cumulative % of Agents');
    
    svg.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('x', -innerHeight / 2)
        .attr('y', -40)
        .attr('text-anchor', 'middle')
        .text('Cumulative % of Value');
    
    // Add title
    svg.append('text')
        .attr('class', 'chart-title')
        .attr('x', innerWidth / 2)
        .attr('y', -20)
        .attr('text-anchor', 'middle')
        .text('Equity: Income Distribution (Lorenz Curve)');
    
    // Add legend
    const legend = svg.append('g')
        .attr('transform', `translate(${innerWidth + 10}, 50)`);
    
    legend.append('path')
        .attr('d', 'M0,0 L20,0')
        .attr('stroke', 'var(--accent-color)')
        .attr('stroke-width', 2);
    
    legend.append('text')
        .attr('x', 25)
        .attr('y', 0)
        .attr('dy', '.35em')
        .text('Lorenz Curve');
    
    legend.append('path')
        .attr('d', 'M0,20 L20,20')
        .attr('stroke', '#999')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,4');
    
    legend.append('text')
        .attr('x', 25)
        .attr('y', 20)
        .attr('dy', '.35em')
        .text('Perfect Equality');
    
    legend.append('circle')
        .attr('cx', 10)
        .attr('cy', 40)
        .attr('r', 3)
        .attr('fill', 'var(--specialist-color)');
    
    legend.append('text')
        .attr('x', 25)
        .attr('y', 40)
        .attr('dy', '.35em')
        .text('Specialist');
    
    legend.append('circle')
        .attr('cx', 10)
        .attr('cy', 60)
        .attr('r', 3)
        .attr('fill', 'var(--bidder-color)');
    
    legend.append('text')
        .attr('x', 25)
        .attr('y', 60)
        .attr('dy', '.35em')
        .text('Non-Specialist');
    
    // Add Gini interpretation
    let giniInterpretation = 'Low Inequality';
    if (gini > 0.3) giniInterpretation = 'Moderate Inequality';
    if (gini > 0.5) giniInterpretation = 'High Inequality';
    if (gini > 0.7) giniInterpretation = 'Extreme Inequality';
    
    legend.append('text')
        .attr('x', 0)
        .attr('y', 90)
        .text('Interpretation:');
    
    legend.append('text')
        .attr('x', 0)
        .attr('y', 110)
        .attr('font-style', 'italic')
        .text(giniInterpretation);
}

// Strategic Friction Visualization
function createStrategicFrictionVisualization(agentMetrics) {
    const container = d3.select('#strategic-friction-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 30, bottom: 60, left: 60};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Filter out agents with no bids
    const activeAgents = agentMetrics.filter(a => a.bids > 0);
    
    // Create scales
    const x = d3.scaleLinear()
        .domain([0, d3.max(activeAgents, d => d.bid_to_win_ratio) * 1.1 || 10])
        .range([0, innerWidth]);
    
    const y = d3.scaleLinear()
        .domain([0, d3.max(activeAgents, d => d.repeat_match_rate) * 1.1 || 1])
        .range([innerHeight, 0]);
    
    const size = d3.scaleLinear()
        .domain([0, d3.max(activeAgents, d => d.bids) || 10])
        .range([5, 15]);
    
    // Create reference lines
    svg.append('line')
        .attr('x1', 0)
        .attr('x2', innerWidth)
        .attr('y1', innerHeight / 2)
        .attr('y2', innerHeight / 2)
        .attr('stroke', '#ccc')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '3,3');
    
    svg.append('line')
        .attr('x1', innerWidth / 2)
        .attr('x2', innerWidth / 2)
        .attr('y1', 0)
        .attr('y2', innerHeight)
        .attr('stroke', '#ccc')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '3,3');
    
    // Add quadrant labels
    const quadrants = [
        {label: 'Low Competition\nHigh Trust', x: innerWidth/4, y: innerHeight/4},
        {label: 'High Competition\nHigh Trust', x: 3*innerWidth/4, y: innerHeight/4},
        {label: 'Low Competition\nLow Trust', x: innerWidth/4, y: 3*innerHeight/4},
        {label: 'High Competition\nLow Trust', x: 3*innerWidth/4, y: 3*innerHeight/4}
    ];
    
    quadrants.forEach(q => {
        const text = svg.append('text')
            .attr('x', q.x)
            .attr('y', q.y)
            .attr('text-anchor', 'middle')
            .attr('opacity', 0.5)
            .style('font-size', '12px');
        
        // Handle multiline labels
        const lines = q.label.split('\n');
        lines.forEach((line, i) => {
            text.append('tspan')
                .attr('x', q.x)
                .attr('dy', i === 0 ? 0 : '1.2em')
                .text(line);
        });
    });
    
    // Draw scatter plot points
    svg.selectAll('.agent-point')
        .data(activeAgents)
        .enter().append('circle')
        .attr('class', 'agent-point')
        .attr('cx', d => x(d.bid_to_win_ratio))
        .attr('cy', d => y(d.repeat_match_rate))
        .attr('r', d => size(d.bids))
        .attr('fill', d => d.specialist ? 'var(--specialist-color)' : 'var(--bidder-color)')
        .attr('opacity', 0.7)
        .attr('stroke', '#fff')
        .attr('stroke-width', 1)
        .on('mouseover', function(event, d) {
            d3.select(this)
                .attr('opacity', 1)
                .attr('stroke', '#000');
            
            const content = `<div class="tooltip-title">${d.name}</div>
                             <div>${d.specialist ? 'Specialist' : 'Non-Specialist'}</div>
                             <div>Bids: ${d.bids}</div>
                             <div>Wins: ${d.wins}</div>
                             <div>Bid-to-Win Ratio: ${d.bid_to_win_ratio.toFixed(1)}</div>
                             <div>Repeat Match Rate: ${(d.repeat_match_rate * 100).toFixed(1)}%</div>`;
            showTooltip(event, d, content);
        })
        .on('mouseout', function() {
            d3.select(this)
                .attr('opacity', 0.7)
                .attr('stroke', '#fff');
            hideTooltip();
        });
    
    // Add trend line (linear regression)
    if (activeAgents.length > 2) {
        // Extract data points for regression
        const points = activeAgents.map(d => [d.bid_to_win_ratio, d.repeat_match_rate]);
        
        // Simple linear regression
        let sumX = 0, sumY = 0, sumXY = 0, sumX2 = 0;
        const n = points.length;
        
        points.forEach(point => {
            sumX += point[0];
            sumY += point[1];
            sumXY += point[0] * point[1];
            sumX2 += point[0] * point[0];
        });
        
        const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX);
        const intercept = (sumY - slope * sumX) / n;
        
        // Calculate correlation coefficient
        let sumX2Y2 = 0;
        let sumY2 = 0;
        points.forEach(point => {
            sumY2 += point[1] * point[1];
        });
        
        const r = (n * sumXY - sumX * sumY) / 
                  Math.sqrt((n * sumX2 - sumX * sumX) * (n * sumY2 - sumY * sumY));
        
        // Draw trend line if it's within bounds
        const x1 = 0;
        const y1 = intercept;
        const x2 = d3.max(activeAgents, d => d.bid_to_win_ratio) * 1.1;
        const y2 = slope * x2 + intercept;
        
        if (y1 >= 0 && y1 <= 1 && y2 >= 0 && y2 <= 1) {
            svg.append('line')
                .attr('x1', x(x1))
                .attr('y1', y(y1))
                .attr('x2', x(x2))
                .attr('y2', y(y2))
                .attr('stroke', '#000')
                .attr('stroke-width', 1)
                .attr('stroke-dasharray', '3,3');
            
            // Add correlation text
            svg.append('text')
                .attr('x', innerWidth - 10)
                .attr('y', 20)
                .attr('text-anchor', 'end')
                .attr('font-size', '12px')
                .text(`Correlation: ${r.toFixed(2)}`);
        }
    }
    
    // Add axes
    svg.append('g')
        .attr('transform', `translate(0,${innerHeight})`)
        .call(d3.axisBottom(x));
    
    svg.append('g')
        .call(d3.axisLeft(y).tickFormat(d3.format('.0%')));
    
    // Add axis labels
    svg.append('text')
        .attr('x', innerWidth / 2)
        .attr('y', innerHeight + 40)
        .attr('text-anchor', 'middle')
        .text('Bid-to-Win Ratio (Competition Cost)');
    
    svg.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('x', -innerHeight / 2)
        .attr('y', -40)
        .attr('text-anchor', 'middle')
        .text('Repeat Match Rate (Trust)');
    
    // Add title
    svg.append('text')
        .attr('class', 'chart-title')
        .attr('x', innerWidth / 2)
        .attr('y', -20)
        .attr('text-anchor', 'middle')
        .text('Strategic Friction: Competition vs. Trust');
    
    // Add legend
    const legend = svg.append('g')
        .attr('transform', `translate(${innerWidth - 150}, ${innerHeight - 80})`);
    
    legend.append('circle')
        .attr('cx', 7.5)
        .attr('cy', 7.5)
        .attr('r', 7.5)
        .attr('fill', 'var(--specialist-color)')
        .attr('opacity', 0.7);
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 7.5)
        .attr('dy', '.35em')
        .text('Specialist');
    
    legend.append('circle')
        .attr('cx', 7.5)
        .attr('cy', 27.5)
        .attr('r', 7.5)
        .attr('fill', 'var(--bidder-color)')
        .attr('opacity', 0.7);
    
    legend.append('text')
        .attr('x', 20)
        .attr('y', 27.5)
        .attr('dy', '.35em')
        .text('Non-Specialist');
    
    // Size legend
    const sizeLegendData = [
        d3.min(activeAgents, d => d.bids),
        d3.median(activeAgents, d => d.bids),
        d3.max(activeAgents, d => d.bids)
    ].filter(d => d !== undefined);
    
    if (sizeLegendData.length > 0) {
        legend.append('text')
            .attr('x', 0)
            .attr('y', 50)
            .text('Bid Volume:');
        
        sizeLegendData.forEach((d, i) => {
            legend.append('circle')
                .attr('cx', 7.5)
                .attr('cy', 70 + i * 20)
                .attr('r', size(d))
                .attr('fill', '#999')
                .attr('opacity', 0.7);
            
            legend.append('text')
                .attr('x', 20)
                .attr('y', 70 + i * 20)
                .attr('dy', '.35em')
                .text(`${d} bids`);
        });
    }
}

// Robustness Visualization
function createRobustnessVisualization(marketMetrics) {
    const container = d3.select('#robustness-visualization');
    const width = container.node().getBoundingClientRect().width;
    const height = container.node().getBoundingClientRect().height;
    const margin = {top: 40, right: 120, bottom: 60, left: 60};
    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;
    
    container.select('svg').remove();
    
    const svg = container.append('svg')
        .attr('width', width)
        .attr('height', height)
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    
    // Create gauge for allocation entropy
    const entropy = marketMetrics.allocation_entropy;
    const gaugeRadius = 80;
    const gaugeX = innerWidth - gaugeRadius - 20;
    const gaugeY = gaugeRadius + 20;
    
    // Create gauge background
    const arc = d3.arc()
        .innerRadius(gaugeRadius - 20)
        .outerRadius(gaugeRadius)
        .startAngle(-Math.PI/2)
        .endAngle(Math.PI/2);
    
    svg.append('path')
        .attr('transform', `translate(${gaugeX},${gaugeY})`)
        .attr('d', arc())
        .attr('fill', '#eee');
    
    // Create gauge fill based on entropy
    const gaugeArc = d3.arc()
        .innerRadius(gaugeRadius - 20)
        .outerRadius(gaugeRadius)
        .startAngle(-Math.PI/2)
        .endAngle(-Math.PI/2 + Math.PI * entropy);
    
    // Color gradient based on value
    const gaugeColor = d3.scaleLinear()
        .domain([0, 0.5, 1])
        .range(['#ff4500', '#ffa500', '#00cc00']);
    
    svg.append('path')
        .attr('transform', `translate(${gaugeX},${gaugeY})`)
        .attr('d', gaugeArc())
        .attr('fill', gaugeColor(entropy));
    
    // Add gauge label
    svg.append('text')
        .attr('x', gaugeX)
        .attr('y', gaugeY - gaugeRadius - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', '14px')
        .attr('font-weight', 'bold')
        .text('Allocation Entropy');
    
    svg.append('text')
        .attr('x', gaugeX)
        .attr('y', gaugeY)
        .attr('text-anchor', 'middle')
        .attr('font-size', '24px')
        .attr('font-weight', 'bold')
        .text(`${(entropy * 100).toFixed(1)}%`);
    
    // Add interpretation text
    let entropyText = 'Unhealthy concentration';
    if (entropy > 0.3) entropyText = 'Moderate diversity';
    if (entropy > 0.7) entropyText = 'Healthy diversity';
    
    svg.append('text')
        .attr('x', gaugeX)
        .attr('y', gaugeY + 30)
        .attr('text-anchor', 'middle')
        .attr('font-style', 'italic')
        .text(entropyText);
    
    // Create elasticity visualization
    // We'll simulate elasticity curve since we don't have real data
    const elasticity = marketMetrics.participation_elasticity || 0.8;
    
    const elasticityData = [];
    for (let market = 0.5; market <= 1.5; market += 0.1) {
        let participation = 1;
        
        // Simple elasticity model: participation = market^elasticity
        if (market < 1) {
            participation = Math.pow(market, elasticity);
        } else {
            participation = Math.pow(market, 1/elasticity);
        }
        
        elasticityData.push({
            market: market,
            participation: participation
        });
    }
    
    // Set up scales
    const x = d3.scaleLinear()
        .domain([0.5, 1.5])
        .range([0, innerWidth - gaugeRadius * 2 - 40]);
    
    const y = d3.scaleLinear()
        .domain([0, d3.max(elasticityData, d => d.participation) * 1.1])
        .range([innerHeight, 0]);
    
    // Create line generator
    const line = d3.line()
        .x(d => x(d.market))
        .y(d => y(d.participation))
        .curve(d3.curveMonotoneX);
    
    // Draw elasticity curve
    svg.append('path')
        .datum(elasticityData)
        .attr('fill', 'none')
        .attr('stroke', 'var(--accent-color)')
        .attr('stroke-width', 2)
        .attr('d', line);
    
    // Add reference lines
    svg.append('line')
        .attr('x1', 0)
        .attr('x2', x(1.5))
        .attr('y1', y(1))
        .attr('y2', y(1))
        .attr('stroke', '#ccc')
        .attr('stroke-width', 1
document.addEventListener('DOMContentLoaded', () => {
    // Tab switching
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabContents = document.querySelectorAll('.tab-content');

    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            tabBtns.forEach(b => b.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));

            btn.classList.add('active');
            document.getElementById(btn.dataset.tab).classList.add('active');
        });
    });

    // Chart Explorer
    const chartList = document.getElementById('chart-list');
    const chartDetails = document.getElementById('chart-details');
    const refreshBtn = document.getElementById('refresh-charts');

    let selectedChart = null;

    async function loadCharts() {
        try {
            const res = await fetch('/api/charts');
            const data = await res.json();
            renderChartList(data.charts || []);
        } catch (err) {
            chartList.innerHTML = `<p class="placeholder">Error loading charts: ${err.message}</p>`;
        }
    }

    function renderChartList(charts) {
        if (charts.length === 0) {
            chartList.innerHTML = '<p class="placeholder">No umbrella charts found</p>';
            return;
        }

        chartList.innerHTML = charts.map(c => `
            <div class="chart-item" data-chart="${c.name}">
                <h3>${c.name} ${c.version}</h3>
                <p>${c.description || 'No description'}</p>
                <p>${c.dependencies.length} sub-charts</p>
            </div>
        `).join('');

        chartList.querySelectorAll('.chart-item').forEach(item => {
            item.addEventListener('click', () => {
                chartList.querySelectorAll('.chart-item').forEach(i => i.classList.remove('selected'));
                item.classList.add('selected');
                loadChartDetails(item.dataset.chart);
            });
        });
    }

    async function loadChartDetails(chartName) {
        try {
            const res = await fetch(`/api/chart?name=${encodeURIComponent(chartName)}`);
            const chart = await res.json();
            selectedChart = chart;
            renderChartDetails(chart);
        } catch (err) {
            chartDetails.innerHTML = `<p class="placeholder">Error: ${err.message}</p>`;
        }
    }

    function renderChartDetails(chart) {
        let html = `
            <h3>${chart.name} ${chart.version}</h3>
            <p>${chart.description || ''}</p>
            <p>App Version: ${chart.appVersion || 'N/A'}</p>
            <p>Path: ${chart.path}</p>
        `;

        if (chart.dependencies && chart.dependencies.length > 0) {
            html += `
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Version</th>
                            <th>Alias</th>
                            <th>Weight</th>
                            <th>Tags</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${chart.dependencies.map(d => `
                            <tr>
                                <td>${d.name}</td>
                                <td>${d.version || '*'}</td>
                                <td>${d.alias || '-'}</td>
                                <td>${d.weight}</td>
                                <td>${(d.tags || []).join(', ') || '-'}</td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            `;
        }

        if (chart.executionOrder && chart.executionOrder.length > 0) {
            html += `
                <div class="execution-order">
                    <h3>Execution Order</h3>
                    ${chart.executionOrder.map((step, i) => `
                        <div class="execution-step">${i + 1}. ${step}</div>
                    `).join('')}
                </div>
            `;
        }

        chartDetails.innerHTML = html;
    }

    refreshBtn.addEventListener('click', loadCharts);

    // Spray Execution
    const sprayForm = document.getElementById('spray-form');
    const logOutput = document.getElementById('log-output');

    let ws = null;

    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

        ws.onmessage = (event) => {
            const line = document.createElement('div');
            line.className = 'log-line';
            line.textContent = event.data;
            logOutput.appendChild(line);
            logOutput.scrollTop = logOutput.scrollHeight;
        };

        ws.onclose = () => {
            setTimeout(connectWebSocket, 3000);
        };

        ws.onerror = (err) => {
            console.error('WebSocket error:', err);
        };
    }

    sprayForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const formData = new FormData(sprayForm);
        const targets = formData.get('targets') ? formData.get('targets').split(',').map(s => s.trim()) : [];
        const excludes = formData.get('excludes') ? formData.get('excludes').split(',').map(s => s.trim()) : [];
        const valueFiles = formData.get('valueFiles') ? formData.get('valueFiles').split(',').map(s => s.trim()) : [];
        const values = formData.get('values') ? formData.get('values').split(',').map(s => s.trim()) : [];

        const request = {
            chartName: formData.get('chartName'),
            namespace: formData.get('namespace'),
            targets,
            excludes,
            valueFiles,
            values,
            timeout: parseInt(formData.get('timeout')) || 300,
            prefixReleases: formData.get('prefixReleases'),
            createNamespace: formData.has('createNamespace'),
            resetValues: formData.has('resetValues'),
            reuseValues: formData.has('reuseValues'),
            force: formData.has('force'),
            dryRun: formData.has('dryRun'),
            verbose: formData.has('verbose'),
            debug: formData.has('debug'),
        };

        try {
            const res = await fetch('/api/spray', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(request),
            });

            const result = await res.json();
            if (result.status === 'started') {
                logOutput.innerHTML = '<div class="log-line info">Spray started...</div>';
            }
        } catch (err) {
            logOutput.innerHTML = `<div class="log-line error">Error: ${err.message}</div>`;
        }
    });

    // Initialize
    loadCharts();
    connectWebSocket();
});

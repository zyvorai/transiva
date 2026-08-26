// HyperSDK Dashboard JavaScript

let ws = null;
let currentView = 'vms';
let vmsData = [];

// Initialize dashboard
document.addEventListener('DOMContentLoaded', () => {
    initWebSocket();
    loadVMs();
    loadOperations();
    loadSnapshots();
    loadTemplates();

    // Refresh data every 30 seconds
    setInterval(() => {
        if (currentView === 'vms') loadVMs();
        if (currentView === 'operations') loadOperations();
        if (currentView === 'snapshots') loadSnapshots();
        if (currentView === 'templates') loadTemplates();
    }, 30000);
});

// WebSocket connection
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log('WebSocket connected');
        updateStatus('Connected', 'success');
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        handleWebSocketMessage(data);
    };

    ws.onclose = () => {
        console.log('WebSocket disconnected');
        updateStatus('Disconnected', 'danger');
        // Reconnect after 5 seconds
        setTimeout(initWebSocket, 5000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

function handleWebSocketMessage(data) {
    switch (data.type) {
        case 'vm_update':
            loadVMs();
            break;
        case 'operation_update':
            loadOperations();
            break;
        case 'status':
            console.log('Status:', data.message);
            break;
        case 'heartbeat':
            // Connection alive
            break;
    }
}

function updateStatus(text, type) {
    const statusEl = document.getElementById('status');
    statusEl.textContent = `● ${text}`;
    statusEl.className = type;
}

// View management
function showView(viewName) {
    currentView = viewName;

    // Update nav buttons
    document.querySelectorAll('nav button').forEach(btn => {
        btn.classList.remove('active');
    });
    event.target.classList.add('active');

    // Update views
    document.querySelectorAll('.view').forEach(view => {
        view.classList.remove('active');
    });
    document.getElementById(`${viewName}-view`).classList.add('active');

    // Load data for the view
    switch (viewName) {
        case 'vms':
            loadVMs();
            break;
        case 'operations':
            loadOperations();
            break;
        case 'snapshots':
            loadSnapshots();
            break;
        case 'templates':
            loadTemplates();
            break;
    }
}

// Load VMs
async function loadVMs() {
    try {
        const response = await fetch('/api/vms');
        const vms = await response.json();
        vmsData = vms;
        renderVMs(vms);
    } catch (error) {
        console.error('Error loading VMs:', error);
        document.getElementById('vm-list').innerHTML =
            '<div class="loading">Error loading VMs</div>';
    }
}

function renderVMs(vms) {
    const container = document.getElementById('vm-list');

    if (vms.length === 0) {
        container.innerHTML = '<div class="loading">No VMs found</div>';
        return;
    }

    const html = vms.map(vm => `
        <div class="vm-card" data-name="${vm.name}" onclick="showVMDetails('${vm.name}')">
            <div>
                <div class="vm-name">${vm.name}</div>
                <div class="vm-namespace">${vm.namespace}</div>
            </div>
            <div>
                <span class="vm-status ${vm.status.toLowerCase()}">${vm.status}</span>
            </div>
            <div class="vm-specs">
                ${vm.cpus} CPUs / ${vm.memory}
            </div>
            <div class="vm-node">
                ${vm.node || '-'}
            </div>
            <div class="vm-age">
                ${vm.age}
            </div>
            <div class="vm-actions" onclick="event.stopPropagation()">
                ${vm.status === 'Running' ?
                    `<button onclick="stopVM('${vm.name}')">Stop</button>` :
                    `<button onclick="startVM('${vm.name}')">Start</button>`
                }
                <button onclick="showVMMenu('${vm.name}')">⋮</button>
            </div>
        </div>
    `).join('');

    container.innerHTML = html;
}

// Filter VMs
function filterVMs() {
    const searchText = document.getElementById('vm-filter').value.toLowerCase();
    const statusFilter = document.getElementById('status-filter').value;

    let filtered = vmsData;

    if (searchText) {
        filtered = filtered.filter(vm =>
            vm.name.toLowerCase().includes(searchText) ||
            vm.namespace.toLowerCase().includes(searchText)
        );
    }

    if (statusFilter) {
        filtered = filtered.filter(vm => vm.status === statusFilter);
    }

    renderVMs(filtered);
}

// VM Details
async function showVMDetails(name) {
    try {
        const response = await fetch(`/api/vms/${name}`);
        const vm = await response.json();

        const html = `
            <h2>${vm.name}</h2>
            <div style="margin-top: 1.5rem;">
                <div class="form-group">
                    <label>Status</label>
                    <div><span class="vm-status ${vm.status.toLowerCase()}">${vm.status}</span></div>
                </div>
                <div class="form-group">
                    <label>Namespace</label>
                    <div>${vm.namespace}</div>
                </div>
                <div class="form-group">
                    <label>Node</label>
                    <div>${vm.node || 'Not scheduled'}</div>
                </div>
                <div class="form-group">
                    <label>IP Address</label>
                    <div>${vm.ipAddress || 'Not assigned'}</div>
                </div>
                <div class="form-group">
                    <label>Resources</label>
                    <div>${vm.cpus} CPUs, ${vm.memory} Memory</div>
                </div>
                <div class="form-group">
                    <label>Uptime</label>
                    <div>${vm.uptime}</div>
                </div>

                <h3 style="margin-top: 1.5rem; margin-bottom: 1rem;">Disks</h3>
                ${vm.disks.map(disk => `
                    <div class="form-group">
                        <label>${disk.name}</label>
                        <div>${disk.size} (${disk.storageClass})</div>
                    </div>
                `).join('')}

                <h3 style="margin-top: 1.5rem; margin-bottom: 1rem;">Networks</h3>
                ${vm.networks.map(net => `
                    <div class="form-group">
                        <label>${net.name}</label>
                        <div>${net.type} - ${net.ipAddress || 'No IP'}</div>
                    </div>
                `).join('')}

                <div class="form-actions">
                    <button class="btn-primary" onclick="closeModal()">Close</button>
                </div>
            </div>
        `;

        showModal(html);
    } catch (error) {
        console.error('Error loading VM details:', error);
    }
}

// VM Operations
async function startVM(name) {
    if (!confirm(`Start VM ${name}?`)) return;

    try {
        const response = await fetch('/api/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                vmName: name,
                type: 'start'
            })
        });

        const result = await response.json();
        alert(result.message);
        loadVMs();
        loadOperations();
    } catch (error) {
        console.error('Error starting VM:', error);
        alert('Error starting VM');
    }
}

async function stopVM(name) {
    if (!confirm(`Stop VM ${name}?`)) return;

    try {
        const response = await fetch('/api/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                vmName: name,
                type: 'stop'
            })
        });

        const result = await response.json();
        alert(result.message);
        loadVMs();
        loadOperations();
    } catch (error) {
        console.error('Error stopping VM:', error);
        alert('Error stopping VM');
    }
}

function showVMMenu(name) {
    const html = `
        <h2>VM Actions: ${name}</h2>
        <div style="margin-top: 1.5rem; display: flex; flex-direction: column; gap: 0.75rem;">
            <button class="btn-primary" onclick="cloneVM('${name}')">Clone VM</button>
            <button class="btn-primary" onclick="snapshotVM('${name}')">Create Snapshot</button>
            <button class="btn-primary" onclick="migrateVM('${name}')">Migrate VM</button>
            <button class="btn-primary" onclick="resizeVM('${name}')">Resize VM</button>
            <button class="btn-danger" onclick="deleteVM('${name}')">Delete VM</button>
        </div>
        <div class="form-actions">
            <button class="btn-primary" onclick="closeModal()">Cancel</button>
        </div>
    `;
    showModal(html);
}

async function deleteVM(name) {
    closeModal();
    if (!confirm(`Delete VM ${name}? This cannot be undone.`)) return;

    try {
        const response = await fetch(`/api/vms/${name}`, {
            method: 'DELETE'
        });

        const result = await response.json();
        alert(result.message);
        loadVMs();
    } catch (error) {
        console.error('Error deleting VM:', error);
        alert('Error deleting VM');
    }
}

function cloneVM(name) {
    closeModal();
    const targetName = prompt(`Enter name for cloned VM (source: ${name}):`, `${name}-clone`);
    if (!targetName) return;

    fetch('/api/operations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            vmName: name,
            type: 'clone',
            params: { targetName }
        })
    }).then(r => r.json()).then(result => {
        alert(result.message);
        loadOperations();
    });
}

function snapshotVM(name) {
    closeModal();
    const snapshotName = prompt(`Enter snapshot name for VM ${name}:`, `${name}-snapshot`);
    if (!snapshotName) return;

    fetch('/api/operations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            vmName: name,
            type: 'snapshot',
            params: { snapshotName }
        })
    }).then(r => r.json()).then(result => {
        alert(result.message);
        loadSnapshots();
    });
}

function migrateVM(name) {
    closeModal();
    const targetNode = prompt(`Enter target node for VM ${name}:`);
    if (!targetNode) return;

    fetch('/api/operations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            vmName: name,
            type: 'migrate',
            params: { targetNode }
        })
    }).then(r => r.json()).then(result => {
        alert(result.message);
        loadOperations();
    });
}

function resizeVM(name) {
    closeModal();
    const html = `
        <h2>Resize VM: ${name}</h2>
        <form onsubmit="submitResize(event, '${name}')">
            <div class="form-group">
                <label>CPUs</label>
                <input type="number" id="resize-cpus" min="1" max="64" required>
            </div>
            <div class="form-group">
                <label>Memory (e.g., 8Gi, 16Gi)</label>
                <input type="text" id="resize-memory" pattern="[0-9]+Gi" required>
            </div>
            <div class="form-actions">
                <button type="button" onclick="closeModal()">Cancel</button>
                <button type="submit" class="btn-primary">Resize</button>
            </div>
        </form>
    `;
    showModal(html);
}

function submitResize(event, name) {
    event.preventDefault();
    const cpus = document.getElementById('resize-cpus').value;
    const memory = document.getElementById('resize-memory').value;

    fetch('/api/operations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            vmName: name,
            type: 'resize',
            params: { cpus: parseInt(cpus), memory }
        })
    }).then(r => r.json()).then(result => {
        alert(result.message);
        closeModal();
        loadOperations();
    });
}

// Create VM
function createVM() {
    const html = `
        <h2>Create Virtual Machine</h2>
        <form onsubmit="submitCreateVM(event)">
            <div class="form-group">
                <label>VM Name</label>
                <input type="text" id="vm-name" required pattern="[a-z0-9-]+">
            </div>
            <div class="form-group">
                <label>CPUs</label>
                <input type="number" id="vm-cpus" min="1" max="64" value="2" required>
            </div>
            <div class="form-group">
                <label>Memory</label>
                <input type="text" id="vm-memory" value="4Gi" pattern="[0-9]+Gi" required>
            </div>
            <div class="form-group">
                <label>Image or Template</label>
                <select id="vm-source-type">
                    <option value="image">Image</option>
                    <option value="template">Template</option>
                </select>
            </div>
            <div class="form-group">
                <label>Image/Template</label>
                <input type="text" id="vm-source" placeholder="ubuntu:22.04 or template name" required>
            </div>
            <div class="form-actions">
                <button type="button" onclick="closeModal()">Cancel</button>
                <button type="submit" class="btn-primary">Create</button>
            </div>
        </form>
    `;
    showModal(html);
}

async function submitCreateVM(event) {
    event.preventDefault();

    const data = {
        name: document.getElementById('vm-name').value,
        cpus: parseInt(document.getElementById('vm-cpus').value),
        memory: document.getElementById('vm-memory').value
    };

    const sourceType = document.getElementById('vm-source-type').value;
    const source = document.getElementById('vm-source').value;

    if (sourceType === 'image') {
        data.image = source;
    } else {
        data.template = source;
    }

    try {
        const response = await fetch('/api/vms', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        const result = await response.json();
        alert(result.message);
        closeModal();
        loadVMs();
    } catch (error) {
        console.error('Error creating VM:', error);
        alert('Error creating VM');
    }
}

// Load Operations
async function loadOperations() {
    try {
        const response = await fetch('/api/operations');
        const operations = await response.json();
        renderOperations(operations);
    } catch (error) {
        console.error('Error loading operations:', error);
        document.getElementById('operations-list').innerHTML =
            '<div class="loading">Error loading operations</div>';
    }
}

function renderOperations(operations) {
    const container = document.getElementById('operations-list');

    if (operations.length === 0) {
        container.innerHTML = '<div class="loading">No operations found</div>';
        return;
    }

    const html = operations.map(op => `
        <div class="operation-card">
            <div class="operation-header">
                <div class="operation-name">${op.name}</div>
                <div class="operation-type">${op.type}</div>
            </div>
            <div class="vm-namespace">VM: ${op.vmName}</div>
            <div class="operation-progress">
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${op.progress}%"></div>
                </div>
                <div class="progress-text">${op.status} - ${op.progress}%</div>
            </div>
        </div>
    `).join('');

    container.innerHTML = html;
}

// Load Snapshots
async function loadSnapshots() {
    try {
        const response = await fetch('/api/snapshots');
        const snapshots = await response.json();
        renderSnapshots(snapshots);
    } catch (error) {
        console.error('Error loading snapshots:', error);
        document.getElementById('snapshots-list').innerHTML =
            '<div class="loading">Error loading snapshots</div>';
    }
}

function renderSnapshots(snapshots) {
    const container = document.getElementById('snapshots-list');

    if (snapshots.length === 0) {
        container.innerHTML = '<div class="loading">No snapshots found</div>';
        return;
    }

    const html = snapshots.map(snap => `
        <div class="snapshot-card">
            <div class="snapshot-info">
                <h4>${snap.name}</h4>
                <div class="snapshot-meta">
                    VM: ${snap.vmName} | Size: ${snap.size} | Created: ${new Date(snap.created).toLocaleString()}
                </div>
            </div>
            <div>
                <button class="btn-primary" onclick="restoreSnapshot('${snap.name}')">Restore</button>
            </div>
        </div>
    `).join('');

    container.innerHTML = html;
}

function restoreSnapshot(name) {
    alert(`Restore snapshot ${name} - feature coming soon`);
}

// Load Templates
async function loadTemplates() {
    try {
        const response = await fetch('/api/templates');
        const templates = await response.json();
        renderTemplates(templates);
    } catch (error) {
        console.error('Error loading templates:', error);
        document.getElementById('templates-list').innerHTML =
            '<div class="loading">Error loading templates</div>';
    }
}

function renderTemplates(templates) {
    const container = document.getElementById('templates-list');

    if (templates.length === 0) {
        container.innerHTML = '<div class="loading">No templates found</div>';
        return;
    }

    const html = templates.map(tpl => `
        <div class="template-card" onclick="useTemplate('${tpl.name}')">
            <div class="template-header">
                <div class="template-name">${tpl.displayName}</div>
                <div class="template-os">${tpl.os} ${tpl.version}</div>
            </div>
            <div class="template-meta">
                <span>Size: ${tpl.size}</span>
                <span>${tpl.ready ? '✓ Ready' : '⏳ Preparing'}</span>
            </div>
        </div>
    `).join('');

    container.innerHTML = html;
}

function useTemplate(name) {
    // Pre-fill create VM form with template
    createVM();
    setTimeout(() => {
        document.getElementById('vm-source-type').value = 'template';
        document.getElementById('vm-source').value = name;
    }, 100);
}

// Modal
function showModal(html) {
    const modal = document.getElementById('modal');
    document.getElementById('modal-body').innerHTML = html;
    modal.classList.add('active');
}

function closeModal(event) {
    if (!event || event.target.id === 'modal' || event.target.classList.contains('close')) {
        document.getElementById('modal').classList.remove('active');
    }
}

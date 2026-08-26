// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	version = "0.1.0"
)

var (
	port       = flag.Int("port", 8090, "Dashboard port")
	kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file")
	namespace  = flag.String("namespace", "default", "Default namespace")

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}
)

// Dashboard server
type Dashboard struct {
	config    *rest.Config
	namespace string
	clients   map[*websocket.Conn]bool
}

func main() {
	flag.Parse()

	log.Printf("HyperSDK Dashboard v%s starting...", version)
	log.Printf("Port: %d", *port)
	log.Printf("Default namespace: %s", *namespace)

	// Create Kubernetes config
	config, err := getKubeConfig(*kubeconfig)
	if err != nil {
		log.Fatalf("Failed to get kube config: %v", err)
	}

	dashboard := &Dashboard{
		config:    config,
		namespace: *namespace,
		clients:   make(map[*websocket.Conn]bool),
	}

	// Setup routes
	http.HandleFunc("/", dashboard.handleIndex)
	http.HandleFunc("/api/vms", dashboard.handleVMs)
	http.HandleFunc("/api/vms/", dashboard.handleVM)
	http.HandleFunc("/api/operations", dashboard.handleOperations)
	http.HandleFunc("/api/snapshots", dashboard.handleSnapshots)
	http.HandleFunc("/api/templates", dashboard.handleTemplates)
	http.HandleFunc("/ws", dashboard.handleWebSocket)

	// Static files
	fs := http.FileServer(http.Dir("./cmd/transiva-dashboard/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Dashboard available at http://localhost%s", addr)
	srv := &http.Server{
		Addr:              addr,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// getKubeConfig creates Kubernetes config
func getKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		// Try in-cluster config first
		config, err := rest.InClusterConfig()
		if err == nil {
			return config, nil
		}

		// Fall back to default kubeconfig location
		homeDir, err := os.UserHomeDir()
		if err == nil {
			kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
		}
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// handleIndex serves the main dashboard page
func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HyperSDK Dashboard</title>
    <link rel="stylesheet" href="/static/dashboard.css">
</head>
<body>
    <div id="app">
        <header>
            <h1>🖥️ HyperSDK Dashboard</h1>
            <div class="header-info">
                <span id="namespace">Namespace: default</span>
                <span id="status">● Connected</span>
            </div>
        </header>

        <nav>
            <button onclick="showView('vms')" class="active">Virtual Machines</button>
            <button onclick="showView('operations')">Operations</button>
            <button onclick="showView('snapshots')">Snapshots</button>
            <button onclick="showView('templates')">Templates</button>
        </nav>

        <main>
            <!-- VM List View -->
            <div id="vms-view" class="view active">
                <div class="view-header">
                    <h2>Virtual Machines</h2>
                    <button onclick="createVM()" class="btn-primary">+ Create VM</button>
                </div>
                <div class="filters">
                    <input type="text" id="vm-filter" placeholder="Filter VMs..." onkeyup="filterVMs()">
                    <select id="status-filter" onchange="filterVMs()">
                        <option value="">All Status</option>
                        <option value="Running">Running</option>
                        <option value="Stopped">Stopped</option>
                        <option value="Pending">Pending</option>
                        <option value="Failed">Failed</option>
                    </select>
                </div>
                <div id="vm-list" class="resource-list">
                    <div class="loading">Loading VMs...</div>
                </div>
            </div>

            <!-- Operations View -->
            <div id="operations-view" class="view">
                <div class="view-header">
                    <h2>VM Operations</h2>
                </div>
                <div id="operations-list" class="resource-list">
                    <div class="loading">Loading operations...</div>
                </div>
            </div>

            <!-- Snapshots View -->
            <div id="snapshots-view" class="view">
                <div class="view-header">
                    <h2>VM Snapshots</h2>
                </div>
                <div id="snapshots-list" class="resource-list">
                    <div class="loading">Loading snapshots...</div>
                </div>
            </div>

            <!-- Templates View -->
            <div id="templates-view" class="view">
                <div class="view-header">
                    <h2>VM Templates</h2>
                </div>
                <div id="templates-list" class="resource-list">
                    <div class="loading">Loading templates...</div>
                </div>
            </div>
        </main>
    </div>

    <!-- Modals -->
    <div id="modal" class="modal" onclick="closeModal(event)">
        <div class="modal-content">
            <span class="close" onclick="closeModal()">&times;</span>
            <div id="modal-body"></div>
        </div>
    </div>

    <script src="/static/dashboard.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("failed to write index response: %v", err)
	}
}

// handleVMs handles VM list API
func (d *Dashboard) handleVMs(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	switch r.Method {
	case "GET":
		// List VMs
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = d.namespace
		}

		// TODO: Use actual Kubernetes client to list VMs
		// For now, return mock data
		vms := []VMResponse{
			{
				Name:      "web-server-1",
				Namespace: namespace,
				Status:    "Running",
				Phase:     "Running",
				CPUs:      4,
				Memory:    "8Gi",
				Node:      "worker-1",
				IPAddress: "10.244.1.5",
				Age:       "2d",
				Uptime:    "47h",
			},
			{
				Name:      "database-1",
				Namespace: namespace,
				Status:    "Running",
				Phase:     "Running",
				CPUs:      8,
				Memory:    "16Gi",
				Node:      "worker-2",
				IPAddress: "10.244.2.10",
				Age:       "5d",
				Uptime:    "120h",
			},
			{
				Name:      "test-vm",
				Namespace: namespace,
				Status:    "Stopped",
				Phase:     "Stopped",
				CPUs:      2,
				Memory:    "4Gi",
				Node:      "",
				IPAddress: "",
				Age:       "1d",
				Uptime:    "0h",
			},
		}

		_ = ctx // Use ctx when implementing real client

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(vms); err != nil {
			log.Printf("failed to encode VMs response: %v", err)
		}

	case "POST":
		// Create VM
		var req CreateVMRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO: Create VM using Kubernetes client
		log.Printf("Creating VM: %s", req.Name)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("VM %s created", req.Name),
		}); err != nil {
			log.Printf("failed to encode create VM response: %v", err)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVM handles single VM API
func (d *Dashboard) handleVM(w http.ResponseWriter, r *http.Request) {
	// Extract VM name from path
	name := r.URL.Path[len("/api/vms/"):]
	if name == "" {
		http.Error(w, "VM name required", http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = d.namespace
	}

	switch r.Method {
	case "GET":
		// Get VM details
		vm := VMDetailResponse{
			Name:      name,
			Namespace: namespace,
			Status:    "Running",
			Phase:     "Running",
			CPUs:      4,
			Memory:    "8Gi",
			Node:      "worker-1",
			IPAddress: "10.244.1.5",
			Age:       "2d",
			Uptime:    "47h",
			Disks: []DiskInfo{
				{Name: "root", Size: "20Gi", StorageClass: "standard"},
				{Name: "data", Size: "100Gi", StorageClass: "fast-ssd"},
			},
			Networks: []NetworkInfo{
				{Name: "default", Type: "pod-network", IPAddress: "10.244.1.5"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(vm); err != nil {
			log.Printf("failed to encode VM response: %v", err)
		}

	case "DELETE":
		// Delete VM
		// #nosec G706 -- %q escapes CR/LF and other control characters in namespace/name (which come from the request path/query), neutralizing log injection
		log.Printf("Deleting VM: %q/%q", namespace, name)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("VM %s deleted", name),
		}); err != nil {
			log.Printf("failed to encode delete VM response: %v", err)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleOperations handles VMOperation API
func (d *Dashboard) handleOperations(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = d.namespace
	}

	switch r.Method {
	case "GET":
		// List operations
		ops := []OperationResponse{
			{
				Name:      "web-server-1-start",
				Namespace: namespace,
				VMName:    "web-server-1",
				Type:      "start",
				Status:    "Succeeded",
				Progress:  100,
				StartTime: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			},
			{
				Name:      "database-1-clone",
				Namespace: namespace,
				VMName:    "database-1",
				Type:      "clone",
				Status:    "Running",
				Progress:  65,
				StartTime: time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ops); err != nil {
			log.Printf("failed to encode operations response: %v", err)
		}

	case "POST":
		// Create operation
		var req CreateOperationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Creating operation: %s on VM %s", req.Type, req.VMName)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("Operation %s started", req.Type),
		}); err != nil {
			log.Printf("failed to encode create operation response: %v", err)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSnapshots handles VMSnapshot API
func (d *Dashboard) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = d.namespace
	}

	snapshots := []SnapshotResponse{
		{
			Name:      "web-server-1-snap1",
			Namespace: namespace,
			VMName:    "web-server-1",
			Status:    "Ready",
			Size:      "8.5Gi",
			Created:   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshots); err != nil {
		log.Printf("failed to encode snapshots response: %v", err)
	}
}

// handleTemplates handles VMTemplate API
func (d *Dashboard) handleTemplates(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = d.namespace
	}

	templates := []TemplateResponse{
		{
			Name:        "ubuntu-22.04",
			Namespace:   namespace,
			DisplayName: "Ubuntu 22.04 LTS",
			OS:          "Ubuntu",
			Version:     "22.04",
			Size:        "2.5Gi",
			Ready:       true,
		},
		{
			Name:        "centos-9",
			Namespace:   namespace,
			DisplayName: "CentOS Stream 9",
			OS:          "CentOS",
			Version:     "9",
			Size:        "2.8Gi",
			Ready:       true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		log.Printf("failed to encode templates response: %v", err)
	}
}

// handleWebSocket handles WebSocket connections for real-time updates
func (d *Dashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("WebSocket close error: %v", cerr)
		}
	}()

	d.clients[conn] = true
	defer delete(d.clients, conn)

	log.Printf("WebSocket client connected (total: %d)", len(d.clients))

	// Send initial status
	if err := conn.WriteJSON(map[string]interface{}{
		"type":    "status",
		"message": "Connected to dashboard",
		"time":    time.Now().Format(time.RFC3339),
	}); err != nil {
		log.Printf("WebSocket write error: %v", err)
		return
	}

	// Keep connection alive and send periodic updates
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Send heartbeat
		if err := conn.WriteJSON(map[string]interface{}{
			"type": "heartbeat",
			"time": time.Now().Format(time.RFC3339),
		}); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}

// Response types
type VMResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Phase     string `json:"phase"`
	CPUs      int32  `json:"cpus"`
	Memory    string `json:"memory"`
	Node      string `json:"node"`
	IPAddress string `json:"ipAddress"`
	Age       string `json:"age"`
	Uptime    string `json:"uptime"`
}

type VMDetailResponse struct {
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Status    string        `json:"status"`
	Phase     string        `json:"phase"`
	CPUs      int32         `json:"cpus"`
	Memory    string        `json:"memory"`
	Node      string        `json:"node"`
	IPAddress string        `json:"ipAddress"`
	Age       string        `json:"age"`
	Uptime    string        `json:"uptime"`
	Disks     []DiskInfo    `json:"disks"`
	Networks  []NetworkInfo `json:"networks"`
}

type DiskInfo struct {
	Name         string `json:"name"`
	Size         string `json:"size"`
	StorageClass string `json:"storageClass"`
}

type NetworkInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	IPAddress string `json:"ipAddress"`
}

type OperationResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	VMName    string `json:"vmName"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	StartTime string `json:"startTime"`
}

type SnapshotResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	VMName    string `json:"vmName"`
	Status    string `json:"status"`
	Size      string `json:"size"`
	Created   string `json:"created"`
}

type TemplateResponse struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	DisplayName string `json:"displayName"`
	OS          string `json:"os"`
	Version     string `json:"version"`
	Size        string `json:"size"`
	Ready       bool   `json:"ready"`
}

// Request types
type CreateVMRequest struct {
	Name     string `json:"name"`
	CPUs     int32  `json:"cpus"`
	Memory   string `json:"memory"`
	Image    string `json:"image"`
	Template string `json:"template"`
}

type CreateOperationRequest struct {
	VMName string                 `json:"vmName"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

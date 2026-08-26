// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// VNCProxy handles VNC console connections to VMs
type VNCProxy struct {
	k8sClient  *kubernetes.Clientset
	k8sConfig  *rest.Config
	upgrader   websocket.Upgrader
	sessions   map[string]*ConsoleSession
	sessionsMu sync.RWMutex
}

// ConsoleSession represents an active console session
type ConsoleSession struct {
	ID          string
	VMName      string
	VMNamespace string
	Type        string // "vnc" or "serial"
	Started     time.Time
	LastActive  time.Time
	Active      bool
}

// NewVNCProxy creates a new VNC proxy
func NewVNCProxy(k8sClient *kubernetes.Clientset, k8sConfig *rest.Config) *VNCProxy {
	return &VNCProxy{
		k8sClient: k8sClient,
		k8sConfig: k8sConfig,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development
				// TODO: Implement proper CORS in production
				return true
			},
		},
		sessions: make(map[string]*ConsoleSession),
	}
}

// HandleVNCWebSocket handles VNC console WebSocket connections
func (vp *VNCProxy) HandleVNCWebSocket(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	vmNamespace := r.URL.Query().Get("namespace")
	vmName := r.URL.Query().Get("vm")
	consoleType := r.URL.Query().Get("type") // "vnc" or "serial"

	if vmNamespace == "" {
		vmNamespace = "default"
	}

	if vmName == "" {
		http.Error(w, "VM name required", http.StatusBadRequest)
		return
	}

	if consoleType == "" {
		consoleType = "serial"
	}

	// Upgrade to WebSocket
	conn, err := vp.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("WebSocket upgrade failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("vnc proxy: failed to close websocket connection: %v", closeErr)
		}
	}()

	// Create session
	sessionID := fmt.Sprintf("%s-%s-%d", vmNamespace, vmName, time.Now().Unix())
	session := &ConsoleSession{
		ID:          sessionID,
		VMName:      vmName,
		VMNamespace: vmNamespace,
		Type:        consoleType,
		Started:     time.Now(),
		LastActive:  time.Now(),
		Active:      true,
	}

	vp.sessionsMu.Lock()
	vp.sessions[sessionID] = session
	vp.sessionsMu.Unlock()

	defer func() {
		vp.sessionsMu.Lock()
		session.Active = false
		delete(vp.sessions, sessionID)
		vp.sessionsMu.Unlock()
	}()

	// Handle console connection based on type
	switch consoleType {
	case "vnc":
		vp.handleVNCConsole(conn, vmNamespace, vmName, session)
	case "serial":
		vp.handleSerialConsole(conn, vmNamespace, vmName, session)
	default:
		if err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Unknown console type: %s", consoleType))); err != nil {
			log.Printf("vnc proxy: failed to write message: %v", err)
		}
	}
}

// handleVNCConsole handles VNC console connections
func (vp *VNCProxy) handleVNCConsole(conn *websocket.Conn, namespace, vmName string, session *ConsoleSession) {
	// For VNC, we need to connect to the VNC server running in the VM pod
	// This is typically exposed via a service or port-forward

	// Find the VM pod
	podName, err := vp.findVMPod(namespace, vmName)
	if err != nil {
		if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Failed to find VM pod: %v", err))); writeErr != nil {
			log.Printf("vnc proxy: failed to write message: %v", writeErr)
		}
		return
	}

	// Send initial message
	if err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Connecting to VNC console for VM %s/%s (pod: %s)...\n", namespace, vmName, podName))); err != nil {
		log.Printf("vnc proxy: failed to write message: %v", err)
		return
	}

	// Note: Actual VNC protocol implementation would require vnc2websocket proxy
	// For now, provide a placeholder that shows the connection is established
	if err := conn.WriteMessage(websocket.TextMessage, []byte("VNC console connection established\n")); err != nil {
		log.Printf("vnc proxy: failed to write message: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("Note: Full VNC protocol support requires noVNC client integration\n")); err != nil {
		log.Printf("vnc proxy: failed to write message: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("Use serial console for text-based access\n")); err != nil {
		log.Printf("vnc proxy: failed to write message: %v", err)
		return
	}

	// Keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		default:
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}

// handleSerialConsole handles serial console connections
func (vp *VNCProxy) handleSerialConsole(conn *websocket.Conn, namespace, vmName string, session *ConsoleSession) {
	// Find the VM pod
	podName, err := vp.findVMPod(namespace, vmName)
	if err != nil {
		if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Failed to find VM pod: %v\n", err))); writeErr != nil {
			log.Printf("vnc proxy: failed to write message: %v", writeErr)
		}
		return
	}

	// Send connection message
	if err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Connecting to serial console for VM %s/%s...\n", namespace, vmName))); err != nil {
		log.Printf("vnc proxy: failed to write message: %v", err)
		return
	}

	// Set up exec request for serial console
	// For KubeVirt VMs, we use virtctl console equivalent
	req := vp.k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	// Execute console command
	req.VersionedParams(&corev1.PodExecOptions{
		Container: "compute", // KubeVirt compute container
		Command:   []string{"/usr/bin/virsh", "console", "1"},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(vp.k8sConfig, "POST", req.URL())
	if err != nil {
		if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Failed to create executor: %v\n", err))); writeErr != nil {
			log.Printf("vnc proxy: failed to write message: %v", writeErr)
		}
		return
	}

	// Create pipes for stdin/stdout
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	defer func() {
		if err := stdinWriter.Close(); err != nil {
			log.Printf("vnc proxy: failed to close stdin writer: %v", err)
		}
	}()
	defer func() {
		if err := stdoutWriter.Close(); err != nil {
			log.Printf("vnc proxy: failed to close stdout writer: %v", err)
		}
	}()

	// Handle WebSocket -> stdin
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if closeErr := stdinWriter.Close(); closeErr != nil {
					log.Printf("vnc proxy: failed to close stdin writer: %v", closeErr)
				}
				return
			}
			if _, err := stdinWriter.Write(message); err != nil {
				log.Printf("vnc proxy: failed to write to stdin: %v", err)
				return
			}
			session.LastActive = time.Now()
		}
	}()

	// Handle stdout -> WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutReader.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					log.Printf("vnc proxy: failed to write message: %v", err)
					return
				}
				session.LastActive = time.Now()
			}
		}
	}()

	// Execute the console command
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:  stdinReader,
		Stdout: stdoutWriter,
		Stderr: stdoutWriter,
		Tty:    true,
	})

	if err != nil {
		if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nConsole session ended: %v\n", err))); writeErr != nil {
			log.Printf("vnc proxy: failed to write message: %v", writeErr)
		}
	} else {
		if writeErr := conn.WriteMessage(websocket.TextMessage, []byte("\nConsole session ended\n")); writeErr != nil {
			log.Printf("vnc proxy: failed to write message: %v", writeErr)
		}
	}
}

// findVMPod finds the pod for a given VM
func (vp *VNCProxy) findVMPod(namespace, vmName string) (string, error) {
	// List pods with label selector for the VM
	labelSelector := fmt.Sprintf("kubevirt.io/domain=%s", vmName)

	pods, err := vp.k8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pod found for VM %s/%s", namespace, vmName)
	}

	// Return the first pod (there should only be one for a VM)
	return pods.Items[0].Name, nil
}

// GetActiveSessions returns all active console sessions
func (vp *VNCProxy) GetActiveSessions() []ConsoleSession {
	vp.sessionsMu.RLock()
	defer vp.sessionsMu.RUnlock()

	sessions := make([]ConsoleSession, 0, len(vp.sessions))
	for _, session := range vp.sessions {
		sessions = append(sessions, *session)
	}

	return sessions
}

// CleanupInactiveSessions removes inactive sessions
func (vp *VNCProxy) CleanupInactiveSessions(maxIdleTime time.Duration) {
	vp.sessionsMu.Lock()
	defer vp.sessionsMu.Unlock()

	now := time.Now()
	for id, session := range vp.sessions {
		if !session.Active || now.Sub(session.LastActive) > maxIdleTime {
			delete(vp.sessions, id)
		}
	}
}

// HandleConsoleInfo returns console session information
func (vp *VNCProxy) HandleConsoleInfo(w http.ResponseWriter, r *http.Request) {
	sessions := vp.GetActiveSessions()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	type SessionInfo struct {
		ID          string    `json:"id"`
		VMName      string    `json:"vm_name"`
		VMNamespace string    `json:"vm_namespace"`
		Type        string    `json:"type"`
		Started     time.Time `json:"started"`
		LastActive  time.Time `json:"last_active"`
		Duration    string    `json:"duration"`
	}

	infos := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		infos = append(infos, SessionInfo{
			ID:          session.ID,
			VMName:      session.VMName,
			VMNamespace: session.VMNamespace,
			Type:        session.Type,
			Started:     session.Started,
			LastActive:  session.LastActive,
			Duration:    time.Since(session.Started).Round(time.Second).String(),
		})
	}

	// Use encoding/json to write response
	if _, err := w.Write([]byte("[")); err != nil {
		log.Printf("vnc proxy: failed to write response: %v", err)
		return
	}
	for i, info := range infos {
		if i > 0 {
			if _, err := w.Write([]byte(",")); err != nil {
				log.Printf("vnc proxy: failed to write response: %v", err)
				return
			}
		}
		if _, err := fmt.Fprintf(w, `{"id":"%s","vm_name":"%s","vm_namespace":"%s","type":"%s","started":"%s","last_active":"%s","duration":"%s"}`,
			info.ID, info.VMName, info.VMNamespace, info.Type,
			info.Started.Format(time.RFC3339), info.LastActive.Format(time.RFC3339), info.Duration); err != nil {
			log.Printf("vnc proxy: failed to write response: %v", err)
			return
		}
	}
	if _, err := w.Write([]byte("]")); err != nil {
		log.Printf("vnc proxy: failed to write response: %v", err)
	}
}

// GetVNCURL generates a VNC connection URL for a VM
func (vp *VNCProxy) GetVNCURL(namespace, vmName string) string {
	// In production, this would return the URL to connect to the VNC proxy
	// For now, return a WebSocket URL
	return fmt.Sprintf("/ws/console?namespace=%s&vm=%s&type=vnc", namespace, vmName)
}

// GetSerialURL generates a serial console URL for a VM
func (vp *VNCProxy) GetSerialURL(namespace, vmName string) string {
	return fmt.Sprintf("/ws/console?namespace=%s&vm=%s&type=serial", namespace, vmName)
}

// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleExportVMMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vms/export?provider=nutanix", nil)
	w := httptest.NewRecorder()

	server.handleExportVM(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleExportVMUnsupportedProvider(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vms/export?provider=aws", nil)
	w := httptest.NewRecorder()

	server.handleExportVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleExportNutanixNotRegistered(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vms/export?provider=nutanix&vm=web01&output=/tmp/out", nil)
	w := httptest.NewRecorder()

	server.handleExportVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestParseVMExportRequestFromQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/vms/export?provider=nutanix&vm=web01&output=/tmp/out&format=qcow2&mounts=ctr:/mnt/ctr", nil)
	parsed, err := parseVMExportRequest(req)
	if err != nil {
		t.Fatalf("parseVMExportRequest: %v", err)
	}
	if parsed.VM != "web01" || parsed.OutputPath != "/tmp/out" || parsed.Format != "qcow2" {
		t.Fatalf("unexpected request: %+v", parsed)
	}
	if parsed.Mounts["ctr"] != "/mnt/ctr" {
		t.Fatalf("unexpected mounts: %v", parsed.Mounts)
	}
}

func TestParseMountSpecMap(t *testing.T) {
	m := parseMountSpecMap("default:/mnt/default,uuid-1:/mnt/other")
	if m["default"] != "/mnt/default" || m["uuid-1"] != "/mnt/other" {
		t.Fatalf("unexpected mounts: %v", m)
	}
}

package handler

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidDeviceName(t *testing.T) {
	t.Parallel()

	valid := []string{
		"recamera",
		"recamera-01",
		"re_camera_01",
		"A1",
		"a" + string(make([]byte, 61)) + "z", // length 63; filled later
	}
	// Fix the 63-length example deterministically.
	valid[4] = "a" + string(make([]byte, 61)) + "z"
	b := []byte(valid[4])
	for i := 1; i < len(b)-1; i++ {
		b[i] = 'b'
	}
	valid[4] = string(b)

	for _, v := range valid {
		if !isValidDeviceName(v) {
			t.Fatalf("expected valid device name %q", v)
		}
	}

	invalid := []string{
		"",
		"-starts-with-hyphen",
		"ends-with-hyphen-",
		"has space",
		"has/slash",
		"has:semicolon;",
		"contains\nnewline",
	}
	for _, v := range invalid {
		if isValidDeviceName(v) {
			t.Fatalf("expected invalid device name %q", v)
		}
	}

	tooLong := "a"
	for i := 0; i < 63; i++ {
		tooLong += "a"
	}
	if isValidDeviceName(tooLong) {
		t.Fatalf("expected too-long device name to be invalid")
	}
}

func TestIsValidTimezone(t *testing.T) {
	t.Parallel()

	valid := []string{
		"UTC",
		"America/New_York",
		"Asia/Shanghai",
		"Etc/GMT+5",
		"Europe/Berlin",
		"Australia/Sydney",
		"Foo_Bar-Quux/Baz",
	}
	for _, v := range valid {
		if !isValidTimezone(v) {
			t.Fatalf("expected valid timezone %q", v)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"..",
		"America//New_York",
		"/absolute/path",
		"has space",
		"Windows\\Path",
	}
	for _, v := range invalid {
		if isValidTimezone(v) {
			t.Fatalf("expected invalid timezone %q", v)
		}
	}
}

func TestDeviceHandler_GetCameraWebsocketUrl(t *testing.T) {
	t.Parallel()

	h := &DeviceHandler{}

	t.Run("ws", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/deviceMgr/getCameraWebsocketUrl", nil)
		req.Host = "example.com:8443"
		h.GetCameraWebsocketUrl(rec, req)

		var resp struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
		}
		if resp.Code != 0 {
			t.Fatalf("expected code=0, got %d; body=%s", resp.Code, rec.Body.String())
		}
		var data struct {
			WebsocketURL string `json:"websocketUrl"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v; data=%s", err, string(resp.Data))
		}
		if data.WebsocketURL != "ws://example.com:8443/ws/camera" {
			t.Fatalf("unexpected websocketUrl: %q", data.WebsocketURL)
		}
	})

	t.Run("wss_by_tls", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://example.com/api/deviceMgr/getCameraWebsocketUrl", nil)
		req.Host = "example.com"
		req.TLS = &tls.ConnectionState{}
		h.GetCameraWebsocketUrl(rec, req)

		var resp struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
		}
		var data struct {
			WebsocketURL string `json:"websocketUrl"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v; data=%s", err, string(resp.Data))
		}
		if data.WebsocketURL != "wss://example.com/ws/camera" {
			t.Fatalf("unexpected websocketUrl: %q", data.WebsocketURL)
		}
	})

	t.Run("wss_by_forwarded_proto", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/deviceMgr/getCameraWebsocketUrl", nil)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-Proto", "https")
		h.GetCameraWebsocketUrl(rec, req)

		var resp struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
		}
		var data struct {
			WebsocketURL string `json:"websocketUrl"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v; data=%s", err, string(resp.Data))
		}
		if data.WebsocketURL != "wss://example.com/ws/camera" {
			t.Fatalf("unexpected websocketUrl: %q", data.WebsocketURL)
		}
	})
}

func TestDeviceHandler_GetModelFile_PathTraversalDoesNotEscape(t *testing.T) {
	t.Parallel()

	modelDir := t.TempDir()
	outsideDir := t.TempDir()

	modelContent := []byte("model-data")
	if err := os.WriteFile(filepath.Join(modelDir, "model.cvimodel"), modelContent, 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}
	secretContent := []byte("SECRET")
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.cvimodel"), secretContent, 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	h := &DeviceHandler{modelDir: modelDir, modelSuffix: ".cvimodel"}

	t.Run("serves_in_dir", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/deviceMgr/getModelFile?name=model.cvimodel", nil)
		h.GetModelFile(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d; body=%s", rec.Code, rec.Body.String())
		}
		if string(rec.Body.Bytes()) != string(modelContent) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("does_not_escape", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/deviceMgr/getModelFile?name=../secret.cvimodel", nil)
		h.GetModelFile(rec, req)

		if rec.Code == http.StatusOK {
			// Should not be able to read from outside modelDir.
			if rec.Body.String() == string(secretContent) {
				t.Fatalf("path traversal served outside content")
			}
		}

		// Even if it 404s, it must not leak the secret.
		if rec.Body.String() == string(secretContent) {
			t.Fatalf("response body leaked secret")
		}
	})
}

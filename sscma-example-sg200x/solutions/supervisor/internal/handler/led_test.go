package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLEDHandler_getSafeLEDPath_PreventsTraversal(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	h := &LEDHandler{ledBasePath: base}

	cases := []struct {
		name     string
		ledName  string
		wantErr  bool
		wantBase string
	}{
		{"empty", "", true, ""},
		{"dot", ".", true, ""},
		{"dotdot", "..", true, ""},
		// These inputs should be sanitized via filepath.Base(), not rejected.
		{"traversal_unix_sanitized", "../../etc/passwd", false, "passwd"},
		// On Unix, backslashes are not path separators, so this stays a literal filename.
		{"traversal_windows_literal", `..\\..\\etc\\passwd`, false, `..\\..\\etc\\passwd`},
		{"normal", "led0", false, "led0"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p, err := h.getSafeLEDPath(tc.ledName)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path=%q", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Ensure resolved path stays within base.
			rel, relErr := filepath.Rel(base, p)
			if relErr != nil {
				t.Fatalf("filepath.Rel failed: %v", relErr)
			}
			if rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator)) {
				t.Fatalf("path escaped base dir: base=%q path=%q rel=%q", base, p, rel)
			}
			if filepath.Base(p) != tc.wantBase {
				t.Fatalf("unexpected base: want=%q got=%q (full=%q)", tc.wantBase, filepath.Base(p), p)
			}
		})
	}
}

func TestLEDHandler_GetLEDs_HandlesSymlinkDirAndParsesFields(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	// Create the target directory OUTSIDE of ledBasePath so we only see the symlink entry.
	targetRoot := t.TempDir()
	realLED := filepath.Join(targetRoot, "real-led")
	if err := os.MkdirAll(realLED, 0o755); err != nil {
		t.Fatalf("mkdir real led: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realLED, "brightness"), []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write brightness: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realLED, "max_brightness"), []byte("255\n"), 0o644); err != nil {
		t.Fatalf("write max_brightness: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realLED, "trigger"), []byte("none [heartbeat] timer\n"), 0o644); err != nil {
		t.Fatalf("write trigger: %v", err)
	}

	// Simulate sysfs-style symlink entry under /sys/class/leds.
	if err := os.Symlink(realLED, filepath.Join(base, "led0")); err != nil {
		t.Fatalf("symlink led0: %v", err)
	}

	// Non-directory entry should be ignored.
	if err := os.WriteFile(filepath.Join(base, "not-a-dir"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-dir: %v", err)
	}

	h := &LEDHandler{ledBasePath: base}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ledMgr/getLEDs", nil)
	h.GetLEDs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if resp.Code != 0 {
		t.Fatalf("expected code=0, got %d; body=%s", resp.Code, rec.Body.String())
	}

	var data struct {
		LEDs []LEDInfo `json:"leds"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v; data=%s", err, string(resp.Data))
	}
	if len(data.LEDs) != 1 {
		t.Fatalf("expected 1 led, got %d; leds=%v", len(data.LEDs), data.LEDs)
	}
	if data.LEDs[0].Name != "led0" {
		t.Fatalf("expected name=led0, got %q", data.LEDs[0].Name)
	}
	if data.LEDs[0].Brightness != 1 || data.LEDs[0].MaxBrightness != 255 {
		t.Fatalf("unexpected brightness values: %+v", data.LEDs[0])
	}
	if data.LEDs[0].Trigger != "heartbeat" {
		t.Fatalf("expected trigger=heartbeat, got %q", data.LEDs[0].Trigger)
	}
}

func TestLEDHandler_GetLEDTriggers_ParsesCurrentAndList(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	ledDir := filepath.Join(base, "led0")
	if err := os.MkdirAll(ledDir, 0o755); err != nil {
		t.Fatalf("mkdir led0: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledDir, "trigger"), []byte("none [heartbeat] timer\n"), 0o644); err != nil {
		t.Fatalf("write trigger: %v", err)
	}

	h := &LEDHandler{ledBasePath: base}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ledMgr/getLEDTriggers?name=led0", nil)
	h.GetLEDTriggers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if resp.Code != 0 {
		t.Fatalf("expected code=0, got %d; body=%s", resp.Code, rec.Body.String())
	}

	var data struct {
		Triggers []string `json:"triggers"`
		Current  string   `json:"current"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v; data=%s", err, string(resp.Data))
	}
	if data.Current != "heartbeat" {
		t.Fatalf("expected current=heartbeat, got %q", data.Current)
	}
	if len(data.Triggers) != 3 || data.Triggers[0] != "none" || data.Triggers[1] != "heartbeat" || data.Triggers[2] != "timer" {
		t.Fatalf("unexpected triggers: %#v", data.Triggers)
	}
}

func TestIsValidLEDTrigger(t *testing.T) {
	t.Parallel()

	valid := []string{"heartbeat", "timer-1", "foo_bar", "[none]"}
	for _, v := range valid {
		if !isValidLEDTrigger(v) {
			t.Fatalf("expected valid trigger %q", v)
		}
	}

	invalid := []string{"", "with space", "../x", "x/..", "x\\y", "..", "a\x00b"}
	for _, v := range invalid {
		if isValidLEDTrigger(v) {
			t.Fatalf("expected invalid trigger %q", v)
		}
	}
}

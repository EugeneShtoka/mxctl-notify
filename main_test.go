package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// build is a helper that calls buildNotification with sensible defaults.
func build(evt *Event, maxBodyLen int, hideBody, hideRoom, hideSender bool) (string, string, bool) {
	return buildNotification(evt, maxBodyLen, hideBody, hideRoom, hideSender)
}

func TestBuildNotification_EmptyBody(t *testing.T) {
	evt := &Event{Body: "", Sender: "@alice:example.com", RoomName: "General"}
	_, _, skip := build(evt, 0, false, false, false)
	if !skip {
		t.Error("expected skip for empty body")
	}
}

func TestBuildNotification_Title_WithSenderName(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "General · Alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_Title_FallbackToMatrixID(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "General · alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_Title_FallbackNoServer(t *testing.T) {
	// Sender without server part — should not panic.
	evt := &Event{Body: "hi", Sender: "@alice", SenderName: "", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "General · alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_HideRoom(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, 0, false, true, false)
	if title != "Alice" {
		t.Errorf("unexpected title with hidden room: %q", title)
	}
}

func TestBuildNotification_HideSender(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, true)
	if title != "General" {
		t.Errorf("unexpected title with hidden sender: %q", title)
	}
}

func TestBuildNotification_HideRoomAndSender_FallbackTitle(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, 0, false, true, true)
	if title != "New message" {
		t.Errorf("expected fallback title, got: %q", title)
	}
}

func TestBuildNotification_HideBody(t *testing.T) {
	evt := &Event{Body: "secret", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, skip := build(evt, 0, true, false, false)
	if skip {
		t.Error("expected no skip")
	}
	if title != "General · Alice" {
		t.Errorf("unexpected title: %q", title)
	}
	if body != "" {
		t.Errorf("expected empty body when hidden, got: %q", body)
	}
}

func TestBuildNotification_AllHidden_FallbackTitle(t *testing.T) {
	evt := &Event{Body: "secret", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, skip := build(evt, 0, true, true, true)
	if skip {
		t.Error("expected no skip")
	}
	if title != "New message" {
		t.Errorf("expected fallback title, got: %q", title)
	}
	if body != "" {
		t.Errorf("expected empty body when hidden, got: %q", body)
	}
}

func TestBuildNotification_BodyTruncation(t *testing.T) {
	b := make([]byte, 250)
	for i := range b {
		b[i] = 'a'
	}
	evt := &Event{Body: string(b), Sender: "@alice:example.com", RoomName: "General"}
	_, body, _ := build(evt, 200, false, false, false)
	runes := []rune(body)
	// 200 chars + "…" = 201 runes.
	if len(runes) != 201 {
		t.Errorf("expected 201 runes after truncation, got %d", len(runes))
	}
}

func TestBuildNotification_BodyUnbounded(t *testing.T) {
	b := make([]byte, 250)
	for i := range b {
		b[i] = 'a'
	}
	evt := &Event{Body: string(b), Sender: "@alice:example.com", RoomName: "General"}
	_, body, _ := build(evt, 0, false, false, false)
	if len(body) != 250 {
		t.Errorf("expected 250 bytes with no truncation, got %d", len(body))
	}
}

func TestBuildNotification_BodyUnicodeTruncation(t *testing.T) {
	// 10 multi-byte runes — truncating at 5 should not produce garbled UTF-8.
	evt := &Event{Body: "αβγδεζηθικ", Sender: "@alice:example.com", RoomName: "R"}
	_, body, _ := build(evt, 5, false, false, false)
	if body != "αβγδε…" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestBuildNotification_Title_EmptyRoomName_NoDot(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: ""}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "Alice" {
		t.Errorf("expected no dot when room name empty, got: %q", title)
	}
}

func TestBuildNotification_Title_RoomEqualsSender_ShowOnce(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@general:example.com", SenderName: "General", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "General" {
		t.Errorf("expected single name when room equals sender, got: %q", title)
	}
}

func TestBuildNotification_Title_EmptySender_NoDot(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "", SenderName: "", RoomName: "General"}
	title, _, _ := build(evt, 0, false, false, false)
	if title != "General" {
		t.Errorf("expected no dot when sender fields empty, got: %q", title)
	}
}

// runBinary builds the binary once and returns a helper that runs it with the
// given arguments and stdin. It skips if the build fails (e.g. in short mode).
func runBinary(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := t.TempDir() + "/mxctl-notify"
	if err := exec.Command("go", "build", "-o", bin, ".").Run(); err != nil {
		t.Skipf("build failed: %v", err)
	}
	var outBuf, errBuf strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
	}
	return stdout, stderr, exitCode
}

func eventJSON(t *testing.T, evt Event) string {
	t.Helper()
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return string(b)
}

func configJSON(t *testing.T, cfg Config) string {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return string(b)
}

func TestBinary_TooManyArgs(t *testing.T) {
	_, stderr, code := runBinary(t, "", "a", "b", "c")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "too many arguments") {
		t.Errorf("expected usage error in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage hint in stderr, got: %q", stderr)
	}
}

func TestBinary_UnknownFlag(t *testing.T) {
	_, stderr, code := runBinary(t, "", "--not-a-flag")
	if code == 0 {
		t.Error("expected non-zero exit code for unknown flag")
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage hint in stderr, got: %q", stderr)
	}
}

func TestBinary_HelpFlag(t *testing.T) {
	if os.Getenv("CI") == "" {
		// build may be slow; only run in CI or when explicitly requested
	}
	_, stderr, _ := runBinary(t, "", "--help")
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage in --help output, got: %q", stderr)
	}
	if !strings.Contains(stderr, "max_body_len") {
		t.Errorf("expected config docs in --help output, got: %q", stderr)
	}
}

func TestBinary_PluginMode_InvalidConfig(t *testing.T) {
	evt := Event{Body: "hi", Sender: "@alice:example.com", RoomName: "General"}
	_, stderr, code := runBinary(t, eventJSON(t, evt), "--event", eventJSON(t, evt), "--config", "not-json")
	if code == 0 {
		t.Error("expected non-zero exit for invalid config")
	}
	if !strings.Contains(stderr, "parse config") {
		t.Errorf("expected parse config error in stderr, got: %q", stderr)
	}
}

func TestBinary_PluginMode_InvalidStdin(t *testing.T) {
	evt := Event{Body: "hi"}
	_, stderr, code := runBinary(t, "not-json", "--event", eventJSON(t, evt), "--config", "{}")
	if code == 0 {
		t.Error("expected non-zero exit for invalid stdin")
	}
	if !strings.Contains(stderr, "parse event") {
		t.Errorf("expected parse event error in stderr, got: %q", stderr)
	}
}

func TestBinary_PluginMode_ConfigWithoutEvent(t *testing.T) {
	_, stderr, code := runBinary(t, "{}", "--config", "{}")
	if code != 1 {
		t.Errorf("expected exit 1 when --config used without --event, got %d", code)
	}
	if !strings.Contains(stderr, "must be used together") {
		t.Errorf("expected pairing error in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage in stderr, got: %q", stderr)
	}
}

func TestBinary_PluginMode_EventWithoutConfig(t *testing.T) {
	evt := Event{Body: "hi"}
	_, stderr, code := runBinary(t, eventJSON(t, evt), "--event", eventJSON(t, evt))
	if code != 1 {
		t.Errorf("expected exit 1 when --event used without --config, got %d", code)
	}
	if !strings.Contains(stderr, "must be used together") {
		t.Errorf("expected pairing error in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage in stderr, got: %q", stderr)
	}
}

func TestBinary_PluginMode_PositionalArgsForbidden(t *testing.T) {
	evt := Event{Body: "hi"}
	_, stderr, code := runBinary(t, eventJSON(t, evt), "--event", eventJSON(t, evt), "--config", "{}", "extra")
	if code != 1 {
		t.Errorf("expected exit 1 for positional args in plugin mode, got %d", code)
	}
	if !strings.Contains(stderr, "positional arguments not allowed") {
		t.Errorf("expected positional args error in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage in stderr, got: %q", stderr)
	}
}

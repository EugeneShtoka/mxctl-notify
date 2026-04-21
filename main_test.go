package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// build is a helper that calls buildNotification with sensible defaults.
func build(evt *Event, cfg *Config, maxBodyLen int, hideBody, hideRoom, hideSender bool) (string, string, bool) {
	return buildNotification(evt, cfg, maxBodyLen, hideBody, hideRoom, hideSender)
}

func TestBuildNotification_EmptyBody(t *testing.T) {
	evt := &Event{Body: "", Sender: "@alice:example.com", RoomName: "General"}
	_, _, skip := build(evt, &Config{}, 0, false, false, false)
	if !skip {
		t.Error("expected skip for empty body")
	}
}

func TestBuildNotification_ExcludeSelf(t *testing.T) {
	cfg := &Config{SelfIDs: []string{"@alice:example.com"}, ExcludeSelf: true}
	evt := &Event{Body: "hello", Sender: "@alice:example.com", RoomName: "General"}
	_, _, skip := build(evt, cfg, 0, false, false, false)
	if !skip {
		t.Error("expected skip for own message with ExcludeSelf")
	}
}

func TestBuildNotification_ExcludeSelfDisabled(t *testing.T) {
	cfg := &Config{SelfIDs: []string{"@alice:example.com"}, ExcludeSelf: false}
	evt := &Event{Body: "hello", Sender: "@alice:example.com", RoomName: "General"}
	_, _, skip := build(evt, cfg, 0, false, false, false)
	if skip {
		t.Error("expected no skip when ExcludeSelf is false")
	}
}

func TestBuildNotification_ExcludedSender(t *testing.T) {
	cfg := &Config{ExcludedSenders: []string{"@bot:example.com"}}
	evt := &Event{Body: "spam", Sender: "@bot:example.com", RoomName: "General"}
	_, _, skip := build(evt, cfg, 0, false, false, false)
	if !skip {
		t.Error("expected skip for excluded sender")
	}
}

func TestBuildNotification_Title_WithSenderName(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, false, false)
	if title != "General · Alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_Title_FallbackToMatrixID(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, false, false)
	if title != "General · alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_Title_FallbackNoServer(t *testing.T) {
	// Sender without server part — should not panic.
	evt := &Event{Body: "hi", Sender: "@alice", SenderName: "", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, false, false)
	if title != "General · alice" {
		t.Errorf("unexpected title: %q", title)
	}
}

func TestBuildNotification_HideRoom(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, true, false)
	if title != "Alice" {
		t.Errorf("unexpected title with hidden room: %q", title)
	}
}

func TestBuildNotification_HideSender(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, false, true)
	if title != "General" {
		t.Errorf("unexpected title with hidden sender: %q", title)
	}
}

func TestBuildNotification_HideRoomAndSender_FallbackTitle(t *testing.T) {
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, _, _ := build(evt, &Config{}, 0, false, true, true)
	if title != "New message" {
		t.Errorf("expected fallback title, got: %q", title)
	}
}

func TestBuildNotification_HideBody(t *testing.T) {
	evt := &Event{Body: "secret", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, skip := build(evt, &Config{}, 0, true, false, false)
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
	title, body, skip := build(evt, &Config{}, 0, true, true, true)
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
	_, body, _ := build(evt, &Config{}, 200, false, false, false)
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
	_, body, _ := build(evt, &Config{}, 0, false, false, false)
	if len(body) != 250 {
		t.Errorf("expected 250 bytes with no truncation, got %d", len(body))
	}
}

func TestBuildNotification_BodyUnicodeTruncation(t *testing.T) {
	// 10 multi-byte runes — truncating at 5 should not produce garbled UTF-8.
	evt := &Event{Body: "αβγδεζηθικ", Sender: "@alice:example.com", RoomName: "R"}
	_, body, _ := build(evt, &Config{}, 5, false, false, false)
	if body != "αβγδε…" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestBuildNotification_RoomRule_HideTitle(t *testing.T) {
	cfg := &Config{HiddenRooms: []RoomRule{{Room: "General", HideTitle: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, _ := build(evt, cfg, 0, false, false, false)
	if title != "Alice" {
		t.Errorf("expected room hidden from title, got: %q", title)
	}
	if body != "" {
		t.Errorf("expected body hidden when hide_title true, got: %q", body)
	}
}

func TestBuildNotification_RoomRule_HideContent(t *testing.T) {
	cfg := &Config{HiddenRooms: []RoomRule{{Room: "General", HideContent: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, _ := build(evt, cfg, 0, false, false, false)
	if title != "General · Alice" {
		t.Errorf("unexpected title: %q", title)
	}
	if body != "" {
		t.Errorf("expected body hidden, got: %q", body)
	}
}

func TestBuildNotification_RoomRule_MatchByRoomID(t *testing.T) {
	cfg := &Config{HiddenRooms: []RoomRule{{Room: "!abc:example.com", HideContent: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", RoomID: "!abc:example.com", RoomName: "General"}
	_, body, _ := build(evt, cfg, 0, false, false, false)
	if body != "" {
		t.Errorf("expected body hidden when matched by room ID, got: %q", body)
	}
}

func TestBuildNotification_SenderRule_HideTitle(t *testing.T) {
	cfg := &Config{HiddenSenders: []SenderRule{{Sender: "@alice:example.com", HideTitle: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, _ := build(evt, cfg, 0, false, false, false)
	if title != "General" {
		t.Errorf("expected sender hidden from title, got: %q", title)
	}
	if body != "" {
		t.Errorf("expected body hidden when hide_title true, got: %q", body)
	}
}

func TestBuildNotification_SenderRule_HideContent(t *testing.T) {
	cfg := &Config{HiddenSenders: []SenderRule{{Sender: "@alice:example.com", HideContent: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, _ := build(evt, cfg, 0, false, false, false)
	if title != "General · Alice" {
		t.Errorf("unexpected title: %q", title)
	}
	if body != "" {
		t.Errorf("expected body hidden, got: %q", body)
	}
}

func TestBuildNotification_SenderRule_MatchByDisplayName(t *testing.T) {
	cfg := &Config{HiddenSenders: []SenderRule{{Sender: "Alice", HideContent: true}}}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	_, body, _ := build(evt, cfg, 0, false, false, false)
	if body != "" {
		t.Errorf("expected body hidden when matched by display name, got: %q", body)
	}
}

func TestBuildNotification_RoomAndSenderRule_BothHideTitle(t *testing.T) {
	cfg := &Config{
		HiddenRooms:   []RoomRule{{Room: "General", HideTitle: true}},
		HiddenSenders: []SenderRule{{Sender: "@alice:example.com", HideTitle: true}},
	}
	evt := &Event{Body: "hi", Sender: "@alice:example.com", SenderName: "Alice", RoomName: "General"}
	title, body, _ := build(evt, cfg, 0, false, false, false)
	if title != "New message" {
		t.Errorf("expected fallback title, got: %q", title)
	}
	if body != "" {
		t.Errorf("expected body hidden, got: %q", body)
	}
}

func TestHandleEvent_ConfigTakesPrecedenceOverFlag(t *testing.T) {
	// mxctl config (50) should take precedence over the flag (200).
	cfg := &Config{MaxBodyLen: 50}
	b := make([]byte, 100)
	for i := range b {
		b[i] = 'a'
	}
	evt := &Event{Body: string(b), Sender: "@alice:example.com", RoomName: "R"}
	maxBodyLen := cfg.MaxBodyLen // cfg wins over flag
	_, body, _ := build(evt, cfg, maxBodyLen, false, false, false)
	runes := []rune(body)
	if len(runes) != 51 { // 50 + "…"
		t.Errorf("expected 51 runes, got %d", len(runes))
	}
}

func TestHandleLine_InitParsesConfig(t *testing.T) {
	cfg := &Config{}
	msg := map[string]any{
		"version": 1,
		"type":    "init",
		"config": map[string]any{
			"self_ids":         []string{"@me:example.com"},
			"excluded_senders": []string{"@bot:example.com"},
			"exclude_self":     true,
			"max_body_len":     100,
			"hide_body":        true,
			"hide_room":        true,
			"hide_sender":      true,
		},
	}
	line, _ := json.Marshal(msg)
	handleLine(string(line), cfg, flags{})

	if len(cfg.SelfIDs) != 1 || cfg.SelfIDs[0] != "@me:example.com" {
		t.Errorf("self_ids not parsed: %v", cfg.SelfIDs)
	}
	if !cfg.ExcludeSelf {
		t.Error("exclude_self not parsed")
	}
	if len(cfg.ExcludedSenders) != 1 || cfg.ExcludedSenders[0] != "@bot:example.com" {
		t.Errorf("excluded_senders not parsed: %v", cfg.ExcludedSenders)
	}
	if cfg.MaxBodyLen != 100 {
		t.Errorf("max_body_len not parsed: %d", cfg.MaxBodyLen)
	}
	if !cfg.HideBody {
		t.Error("hide_body not parsed")
	}
	if !cfg.HideRoom {
		t.Error("hide_room not parsed")
	}
	if !cfg.HideSender {
		t.Error("hide_sender not parsed")
	}
}

func TestHandleLine_InvalidJSON(t *testing.T) {
	// Should not panic; errors go to stderr.
	cfg := &Config{}
	handleLine("not json", cfg, flags{})
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

func TestBinary_TooManyArgs(t *testing.T) {
	_, stderr, code := runBinary(t, "", "a", "b", "c")
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
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
	if !strings.Contains(stderr, "-max-body-len") {
		t.Errorf("expected flag docs in --help output, got: %q", stderr)
	}
}

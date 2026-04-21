package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RoomRule configures notification behaviour for a specific room.
type RoomRule struct {
	// Room matches against the event's room ID or room name.
	Room        string `json:"room"`
	HideTitle   bool   `json:"hide_title"`
	HideContent bool   `json:"hide_content"`
}

// SenderRule configures notification behaviour for a specific sender.
type SenderRule struct {
	// Sender matches against the event's sender Matrix ID or display name.
	Sender      string `json:"sender"`
	HideTitle   bool   `json:"hide_title"`
	HideContent bool   `json:"hide_content"`
}

// Config is received from mxctl in the init message.
type Config struct {
	SelfIDs         []string `json:"self_ids"`
	ExcludedSenders []string `json:"excluded_senders"`
	ExcludeSelf     bool     `json:"exclude_self"`
	// MaxBodyLen truncates message bodies to this many runes. 0 means unbounded.
	MaxBodyLen    int          `json:"max_body_len"`
	HideBody      bool         `json:"hide_body"`
	HideRoom      bool         `json:"hide_room"`
	HideSender    bool         `json:"hide_sender"`
	HiddenRooms   []RoomRule   `json:"hidden_rooms"`
	HiddenSenders []SenderRule `json:"hidden_senders"`
}

type envelope struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Data    json.RawMessage `json:"data"`
}

type Event struct {
	EventID    string `json:"event_id"`
	RoomID     string `json:"room_id"`
	RoomName   string `json:"room_name"`
	Sender     string `json:"sender"`
	SenderName string `json:"sender_name"`
	Body       string `json:"body"`
	MsgType    string `json:"msg_type"`
	TS         int64  `json:"ts"`
}

type flags struct {
	maxBodyLen int
	hideBody   bool
	hideRoom   bool
	hideSender bool
}

func main() {
	var f flags
	flag.IntVar(&f.maxBodyLen, "max-body-len", 0, "truncate message bodies to this many characters (default: unbounded)")
	flag.BoolVar(&f.hideBody, "hide-body", false, "omit message body from notifications")
	flag.BoolVar(&f.hideRoom, "hide-room", false, "omit room name from the notification title")
	flag.BoolVar(&f.hideSender, "hide-sender", false, "omit sender name from the notification title")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mxctl-notify [flags] [title [body]]

mxctl-notify sends desktop notifications via notify-send.

It operates in two modes:

  Plugin mode (no arguments):
    Reads JSON event envelopes from mxctl on stdin and fires a desktop
    notification for each incoming message. Configuration is received in
    the init envelope from mxctl. Each flag below has a corresponding
    mxctl config field; set it via one source only — an error is thrown
    if both are provided.

  Standalone mode:
    mxctl-notify "Title" "Body"          — notify with explicit title and body
    echo "Body" | mxctl-notify "Title"   — body from stdin
    echo "Body" | mxctl-notify           — title defaults to 'mxctl-notify'

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()

	// Standalone mode: mxctl-notify [title] [body]
	switch len(args) {
	case 2:
		notify(args[0], args[1])
		return
	case 1:
		body := readAll(os.Stdin)
		notify(args[0], body)
		return
	case 0:
		// fall through to plugin / piped-stdin mode
	default:
		fmt.Fprintf(os.Stderr, "mxctl-notify: too many arguments\n\n")
		flag.Usage()
		os.Exit(2)
	}

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}
	firstLine := scanner.Text()

	// Plugin mode: first line is a versioned JSON envelope from mxctl.
	if strings.HasPrefix(firstLine, `{"version"`) {
		var cfg Config
		handleLine(firstLine, &cfg, f)
		for scanner.Scan() {
			handleLine(scanner.Text(), &cfg, f)
		}
		return
	}

	// Standalone mode: plain text piped in.
	lines := []string{firstLine}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	notify("mxctl-notify", strings.Join(lines, "\n"))
}

func handleLine(line string, cfg *Config, f flags) {
	var env envelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		fmt.Fprintf(os.Stderr, "mxctl-notify: parse: %v\n", err)
		return
	}

	switch env.Type {
	case "init":
		if env.Config != nil {
			if err := json.Unmarshal(env.Config, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "mxctl-notify: parse config: %v\n", err)
				return
			}
		}
		var conflicts []string
		if cfg.MaxBodyLen != 0 && f.maxBodyLen != 0 {
			conflicts = append(conflicts, "max_body_len / --max-body-len")
		}
		if cfg.HideBody && f.hideBody {
			conflicts = append(conflicts, "hide_body / --hide-body")
		}
		if cfg.HideRoom && f.hideRoom {
			conflicts = append(conflicts, "hide_room / --hide-room")
		}
		if cfg.HideSender && f.hideSender {
			conflicts = append(conflicts, "hide_sender / --hide-sender")
		}
		if len(conflicts) > 0 {
			fmt.Fprintf(os.Stderr, "mxctl-notify: set via flag and mxctl config, use only one: %s\n", strings.Join(conflicts, ", "))
			os.Exit(1)
		}

	case "event":
		var evt Event
		if err := json.Unmarshal(env.Data, &evt); err != nil {
			fmt.Fprintf(os.Stderr, "mxctl-notify: parse event: %v\n", err)
			return
		}
		handleEvent(&evt, cfg, f)
	}
}

func handleEvent(evt *Event, cfg *Config, f flags) {
	maxBodyLen := cfg.MaxBodyLen
	if maxBodyLen == 0 {
		maxBodyLen = f.maxBodyLen
	}
	hideBody := cfg.HideBody || f.hideBody
	hideRoom := cfg.HideRoom || f.hideRoom
	hideSender := cfg.HideSender || f.hideSender

	title, body, skip := buildNotification(evt, cfg, maxBodyLen, hideBody, hideRoom, hideSender)
	if skip {
		return
	}
	notify(title, body)
}

func buildNotification(evt *Event, cfg *Config, maxBodyLen int, hideBody, hideRoom, hideSender bool) (title, body string, skip bool) {
	if evt.Body == "" {
		return "", "", true
	}

	if cfg.ExcludeSelf {
		for _, id := range cfg.SelfIDs {
			if evt.Sender == id {
				return "", "", true
			}
		}
	}

	for _, id := range cfg.ExcludedSenders {
		if evt.Sender == id {
			return "", "", true
		}
	}

	// Apply per-room rules.
	for _, rule := range cfg.HiddenRooms {
		if rule.Room == evt.RoomID || rule.Room == evt.RoomName {
			if rule.HideTitle {
				hideRoom = true
			}
			if rule.HideTitle || rule.HideContent {
				hideBody = true
			}
			break
		}
	}

	// Apply per-sender rules.
	for _, rule := range cfg.HiddenSenders {
		if rule.Sender == evt.Sender || rule.Sender == evt.SenderName {
			if rule.HideTitle {
				hideSender = true
			}
			if rule.HideTitle || rule.HideContent {
				hideBody = true
			}
			break
		}
	}

	var titleParts []string
	if !hideRoom && evt.RoomName != "" {
		titleParts = append(titleParts, evt.RoomName)
	}
	if !hideSender {
		display := evt.SenderName
		if display == "" {
			s := strings.TrimPrefix(evt.Sender, "@")
			if i := strings.IndexByte(s, ':'); i >= 0 {
				s = s[:i]
			}
			display = s
		}
		if display != "" && display != evt.RoomName {
			titleParts = append(titleParts, display)
		}
	}
	title = strings.Join(titleParts, " · ")
	if title == "" {
		title = "New message"
	}

	if hideBody {
		return title, "", false
	}

	body = evt.Body
	if maxBodyLen > 0 {
		runes := []rune(body)
		if len(runes) > maxBodyLen {
			body = string(runes[:maxBodyLen]) + "…"
		}
	}

	return title, body, false
}

func notify(title, body string) {
	cmd := exec.Command("notify-send", "--app-name=mxctl", title, body)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mxctl-notify: notify-send: %v\n", err)
	}
}

func readAll(f *os.File) string {
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}

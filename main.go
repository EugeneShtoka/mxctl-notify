package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Config is received from mxctl via --config.
type Config struct {
	// MaxBodyLen truncates message bodies to this many runes. 0 means unbounded.
	MaxBodyLen int  `json:"max_body_len"`
	HideBody   bool `json:"hide_body"`
	HideRoom   bool `json:"hide_room"`
	HideSender bool `json:"hide_sender"`
}

// Event is the accumulated event payload received on stdin.
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

func main() {
	configJSON := flag.String("config", "", "JSON config object (mxctl pipe mode); must be paired with --event")
	eventFlag := flag.String("event", "", "original Matrix event JSON (mxctl pipe mode); must be paired with --config")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mxctl-notify [title [body]]

mxctl-notify sends desktop notifications via notify-send.

It operates in two modes:

  Plugin mode (invoked by mxctl):
    Activated by --config and --event together. Reads the accumulated event
    JSON from stdin and fires a desktop notification. These two flags are
    mutually exclusive with all other flags and positional arguments.
    stdin must be valid JSON.

  Standalone mode:
    mxctl-notify "Title" "Body"          — notify with explicit title and body
    echo "Body" | mxctl-notify "Title"   — body from stdin
    echo "Body" | mxctl-notify           — title defaults to 'mxctl-notify'

Plugin flags:
  --config  JSON config object (must be paired with --event)
  --event   Original Matrix event JSON (must be paired with --config)

Config fields:
  max_body_len  int   Truncate body to this many characters (0 = unbounded)
  hide_body     bool  Omit message body from the notification
  hide_room     bool  Omit room name from the notification title
  hide_sender   bool  Omit sender name from the notification title
`)
	}
	flag.Parse()
	args := flag.Args()

	configSet := *configJSON != ""
	eventSet := *eventFlag != ""

	if configSet != eventSet {
		fmt.Fprintf(os.Stderr, "mxctl-notify: --config and --event must be used together\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if configSet {
		// Plugin mode — no other flags or positional args allowed.
		var otherFlags bool
		flag.Visit(func(f *flag.Flag) {
			if f.Name != "config" && f.Name != "event" {
				otherFlags = true
			}
		})
		if otherFlags {
			fmt.Fprintf(os.Stderr, "mxctl-notify: --config and --event are mutually exclusive with all other flags\n\n")
			flag.Usage()
			os.Exit(1)
		}
		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "mxctl-notify: positional arguments not allowed in plugin mode\n\n")
			flag.Usage()
			os.Exit(1)
		}

		var cfg Config
		if err := json.Unmarshal([]byte(*configJSON), &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "mxctl-notify: parse config: %v\n", err)
			os.Exit(1)
		}

		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mxctl-notify: read stdin: %v\n", err)
			os.Exit(1)
		}
		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			fmt.Fprintf(os.Stderr, "mxctl-notify: parse event: %v\n", err)
			os.Exit(1)
		}
		handleEvent(&evt, cfg)
		return
	}

	// Standalone mode.
	switch len(args) {
	case 2:
		notify(args[0], args[1])
	case 1:
		notify(args[0], readAll(os.Stdin))
	case 0:
		notify("mxctl-notify", readAll(os.Stdin))
	default:
		fmt.Fprintf(os.Stderr, "mxctl-notify: too many arguments\n\n")
		flag.Usage()
		os.Exit(1)
	}
}

func handleEvent(evt *Event, cfg Config) {
	title, body, skip := buildNotification(evt, cfg.MaxBodyLen, cfg.HideBody, cfg.HideRoom, cfg.HideSender)
	if skip {
		return
	}
	notify(title, body)
}

func buildNotification(evt *Event, maxBodyLen int, hideBody, hideRoom, hideSender bool) (title, body string, skip bool) {
	if evt.Body == "" {
		return "", "", true
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

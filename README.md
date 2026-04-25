# mxctl-notify

Desktop notification plugin for [mxctl](https://github.com/EugeneShtoka/mxctl), the Matrix CLI client. Sends `notify-send` notifications for incoming messages and also works as a standalone notification tool.

## Requirements

- `notify-send` (part of `libnotify`, available on most Linux desktops)
- Go 1.21+ (to build)

## Installation

```sh
go install mxctl-notify@latest
# or build from source:
go build -o mxctl-notify .
```

## Usage

### Plugin mode (via mxctl)

Register as a pipe in your mxctl config:

```json
{
  "name": "notify",
  "pipes": [
    {
      "cmd": "mxctl-filter",
      "config": {
        "exclude_self": true,
        "self_ids": ["@me:matrix.example.com"]
      }
    },
    {
      "cmd": "mxctl-notify",
      "config": {
        "hide_body": false,
        "hide_room": false
      }
    }
  ]
}
```

mxctl invokes `mxctl-notify` once per event, passing the accumulated event JSON on stdin, `--event` with the original event, and `--config` with plugin-specific settings. `mxctl-notify` reads `body`, `sender_name`, and `room_name` from stdin, fires a desktop notification, and exits 0.

`--config` and `--event` must always be provided together and are mutually exclusive with all other flags and positional arguments.

Filtering (excluding self, specific senders or rooms by Matrix ID) is handled upstream by [mxctl-filter](https://github.com/EugeneShtoka/mxctl-filter) — `mxctl-notify` always notifies when invoked.

### Standalone mode

Send a notification directly:

```sh
# title and body as arguments
mxctl-notify "Title" "Body text"

# title as argument, body from stdin
echo "Body text" | mxctl-notify "Title"

# title defaults to 'mxctl-notify', body from stdin
echo "Body text" | mxctl-notify
```

## Configuration

Configuration is passed by mxctl as `--config '{"key":"value"}'` in plugin mode.

| Field          | Type   | Default | Description                                                      |
|----------------|--------|---------|------------------------------------------------------------------|
| `max_body_len` | `int`  | `0`     | Truncate message bodies to this many characters. 0 = unbounded. |
| `hide_body`    | `bool` | `false` | Omit the message body; show only the title.                      |
| `hide_room`    | `bool` | `false` | Omit the room name from the notification title.                  |
| `hide_sender`  | `bool` | `false` | Omit the sender name from the notification title.                |

If both room and sender are hidden the title falls back to `New message`.

## License

MIT — see [LICENSE](LICENSE).

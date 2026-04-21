# mxctl-notify

Desktop notification plugin for [mxctl](https://github.com/tulir/mxctl), the Matrix CLI client. Sends `notify-send` notifications for incoming messages and also works as a standalone notification tool.

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

Register it as a plugin in your mxctl config:

```yaml
plugins:
  - command: mxctl-notify
```

mxctl will pipe JSON event envelopes to stdin. `mxctl-notify` reads an `init` message to load config (self IDs, excluded senders, exclude-self flag) and fires a desktop notification for each subsequent message event.

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

### Flags

| Flag             | Default | Description                                                     |
|------------------|---------|-----------------------------------------------------------------|
| `--max-body-len` | 0       | Truncate message bodies to this many characters. 0 = unbounded. |
| `--hide-body`    | false   | Omit the message body; show only the title.                     |
| `--hide-room`    | false   | Omit the room name from the notification title.                 |
| `--hide-sender`  | false   | Omit the sender name from the notification title.               |

If both room and sender are hidden the title falls back to `New message`.

### mxctl init config (plugin mode)

mxctl passes the following fields in the `init` envelope:

| Field              | Type       | Description                                                     |
|--------------------|------------|-----------------------------------------------------------------|
| `self_ids`         | `[]string` | Your own Matrix user IDs                                        |
| `excluded_senders` | `[]string` | Sender IDs to suppress notifications for                        |
| `exclude_self`     | `bool`     | Skip notifications for your own messages                        |
| `max_body_len`     | `int`      | Truncate message bodies to this many characters. 0 = unbounded. |
| `hide_body`        | `bool`     | Omit the message body; show only the title.                     |
| `hide_room`        | `bool`     | Omit the room name from the notification title.                 |
| `hide_sender`      | `bool`     | Omit the sender name from the notification title.               |

Each option can be set via its flag **or** its mxctl config field, but not both — an error is thrown if both sources provide a value.

### Per-room and per-sender rules (plugin mode)

For granular control, `hidden_rooms` and `hidden_senders` accept a list of rules. Each rule targets a specific room or sender and can independently hide the title contribution and/or the message body. If either flag is true on a matching rule, the body is hidden.

```yaml
# mxctl plugin config
hidden_rooms:
  - room: "!roomid:example.com"   # matches room ID or room name
    hide_title: true               # omits room from notification title; also hides body
    hide_content: true             # omits message body

hidden_senders:
  - sender: "@bot:example.com"    # matches Matrix ID or display name
    hide_title: false
    hide_content: true
```

| Field          | Type     | Description                                                                 |
|----------------|----------|-----------------------------------------------------------------------------|
| `room`         | `string` | Room ID (`!id:server`) or room name to match                                |
| `sender`       | `string` | Matrix ID (`@user:server`) or display name to match                         |
| `hide_title`   | `bool`   | Omit room/sender from the notification title; also suppresses the body      |
| `hide_content` | `bool`   | Omit the message body                                                       |

## License

MIT — see [LICENSE](LICENSE).

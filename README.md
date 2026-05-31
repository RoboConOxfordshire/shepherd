# Shepherd

Shepherd is the web-based code management system for RoboCon robot brains.
It lets competitors upload, edit, and run Python code on their Raspberry Pi
robot through a browser.

## Architecture

```
cmd/shepherd/     → main binary
internal/
  config/         → environment-based configuration
  server/         → HTTP server + route registration
  run/            → user code execution (subprocess + reaper)
  upload/         → code upload (.py / .zip)
  editor/         → file CRUD API for web IDE
  ws/             → WebSocket hub (camera, logs, pyls proxy)
  gpio/           → GPIO start button handler
web/
  editor/         → CodeMirror 6 web IDE
  static/         → images, fonts, CSS
  docs/           → documentation (wiki markdown)
```

The Go binary embeds all frontend assets. The robot hardware library
(`../robot/`) remains in Python and is called via subprocess.

## Build

```bash
nix develop   # or: nix-shell
go build -o shepherd ./cmd/shepherd
```

## Run

```bash
export SHEPHERD_USER_CODE_PATH=/path/to/usercode
./shepherd
```

Open http://localhost:80/editor/ in a browser.

## Configuration

All config is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SHEPHERD_PORT` | 80 | HTTP port |
| `SHEPHERD_USER_CODE_PATH` | `./usercode` | User code directory |
| `SHEPHERD_ROBOT_LIB_PATH` | `~/robot` | Python robot library path |
| `SHEPHERD_OUTPUT_FILE` | `/media/RobotUSB/logs.txt` | User code output log |
| `SHEPHERD_CAMERA_IMAGE` | `./shepherd/static/image.jpg` | Camera snapshot path |
| `SHEPHERD_ROUND_LENGTH` | `180s` | Competition round duration |
| `SHEPHERD_REAP_GRACE` | `5s` | Grace period before SIGKILL |
| `SHEPHERD_START_BUTTON_PIN` | 26 | GPIO pin for start button |
| `SHEPHERD_ARENA_USB_PATH` | `/media/ArenaUSB` | Arena USB stick mount |

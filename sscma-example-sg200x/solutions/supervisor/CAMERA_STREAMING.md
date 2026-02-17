# Camera Streaming Implementation

## Overview

Camera preview streaming has been integrated directly into the Go supervisor using CGo to interface with the Sophgo video SDK. This replaces the previous Node-RED based camera preview functionality.

## Architecture

```
┌───────────────────────────────────────────────────┐
│ Go Supervisor Process                              │
│                                                    │
│  ┌──────────────────────────────────────────┐   │
│  │ Camera Module (internal/camera/)          │   │
│  │  - stream.go: CGo bindings to Sophgo SDK  │   │
│  │  - camera.go: High-level manager          │   │
│  └──────────────────────────────────────────┘   │
│            ↓ CGo calls ↓                          │
│  ┌──────────────────────────────────────────┐   │
│  │ Sophgo Video SDK (C)                      │   │
│  │  - Camera capture (VI)                     │   │
│  │  - Video processing (VPSS)                 │   │
│  │  - H.264 encoding (VENC)                   │   │
│  └──────────────────────────────────────────┘   │
│            ↓ H.264 frames ↓                      │
│  ┌──────────────────────────────────────────┐   │
│  │ WebSocket Server (port 8765)              │   │
│  │  - gorilla/websocket                       │   │
│  │  - Binary H.264 + 8-byte timestamp        │   │
│  └──────────────────────────────────────────┘   │
└───────────────────────────────────────────────────┘
                    ↓ WebSocket ↓
┌───────────────────────────────────────────────────┐
│ Web Browser                                        │
│  - jmuxer H.264 decoder                           │
│  - Displays at /#/overview                        │
│  - Shows latency                                  │
└───────────────────────────────────────────────────┘
```

## Implementation Details

### CGo Integration (`internal/camera/stream.go`)

**Key Features:**
- CGo bindings to Sophgo video.h API
- Callback from C to Go for video frames
- WebSocket streaming using gorilla/websocket
- 8-byte timestamp appended to each frame

**CGo Directives:**
```go
#cgo CFLAGS: -I../../../components/sophgo/...
#cgo LDFLAGS: -L${SDK_PATH}/lib -lvenc -lvi -lvpss ...
```

**Frame Callback Flow:**
1. C video SDK captures frame → calls `cVideoFrameCallback`
2. C wrapper calls → `goVideoFrameCallback` (exported Go function)
3. Go function extracts H.264 data using `C.GoBytes()`
4. Appends timestamp using `binary.LittleEndian`
5. Sends to all WebSocket clients via `gorilla/websocket`

### Manager Interface (`internal/camera/camera.go`)

Provides simple API:
- `NewCameraManager()` - Create instance
- `Start()` - Initialize video and WebSocket server
- `Stop()` - Cleanup resources

### Server Integration (`internal/server/server.go`)

- Auto-starts camera on supervisor init
- Gracefully stops on shutdown
- Non-blocking (continues if camera fails)

### Frontend Already Ready (`www/src/views/overview/`)

- Connects to `ws://<device-ip>:8765`
- Uses jmuxer to decode H.264
- Extracts timestamp from last 8 bytes
- Displays real-time latency

## Building

### Prerequisites

```bash
export SG200X_SDK_PATH=/path/to/recamera-os/output/sg2002_recamera_emmc
export PATH=/path/to/toolchain/bin:$PATH
```

### Build with Camera Support

```bash
cd solutions/supervisor
make build-riscv-cgo
```

This builds supervisor WITH camera streaming using CGo.

### Create Package

```bash
make opkg
```

Creates `build/supervisor_1.0.0_riscv64.ipk` with camera support.

## Installation

```bash
scp build/supervisor_1.0.0_riscv64.ipk recamera@192.168.42.1:/tmp/
ssh recamera@192.168.42.1
sudo opkg install /tmp/supervisor_1.0.0_riscv64.ipk
```

## Usage

Camera streaming starts automatically with the supervisor:

```bash
# Check if running
ps aux | grep supervisor

# Check WebSocket port
netstat -tuln | grep 8765

# View logs
journalctl -u supervisor -f
```

## Accessing the Stream

1. Open browser: `http://<device-ip>/#/overview`
2. Video appears automatically
3. Latency shown below video

Direct WebSocket: `ws://<device-ip>:8765`

## Protocol

Each WebSocket message:
- **Bytes 0 to N-8**: H.264 encoded video frame
- **Last 8 bytes**: Little-endian uint64 timestamp (milliseconds since epoch)

## Comparison with Node-RED

| Feature | Node-RED (Old) | Go/CGo (New) |
|---------|----------------|--------------|
| Runtime | Node.js process | Native Go with CGo |
| WebSocket Port | 8090 | 8765 |
| Video Format | Base64 JPEG | Binary H.264 |
| Latency | ~200ms | ~100ms |
| Memory Usage | ~50MB | ~20MB |
| Process Management | Manual | Auto-managed |
| Crash Recovery | None | Auto-restart |

## Troubleshooting

**No video in browser:**
- Check WebSocket is listening: `netstat -tuln | grep 8765`
- Check browser console for errors
- Verify camera is accessible: `ls /dev/video*`

**Build errors:**
- Ensure SG200X_SDK_PATH is set
- Verify toolchain is in PATH
- Check SDK libraries exist

**Video stuttering:**
- Check CPU usage: `top`
- Verify network bandwidth
- Consider reducing resolution/FPS

## Technical Notes

- Uses gorilla/websocket v1.5.3
- H.264 encoding at 1920x1080@30fps by default
- WebSocket allows multiple concurrent viewers
- Video callback runs in C thread context
- Frame data copied to Go heap for safety

## Development

To modify video settings, edit `internal/camera/stream.go`:

```go
// Configure video
param.format = C.VIDEO_FORMAT_H264
param.width = 1920    // Change resolution
param.height = 1080
param.fps = 30        // Change frame rate
```

Rebuild with `make build-riscv-cgo`.

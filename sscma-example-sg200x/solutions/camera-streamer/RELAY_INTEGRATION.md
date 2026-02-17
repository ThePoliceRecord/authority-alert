# Camera-Streamer Relay Integration

This document explains how to connect camera-streamer on the SG200x camera to the remote Go relay server.

## Architecture

```
Camera (SG200x) → camera-streamer → Relay Forwarder → WSS → Relay Server → Viewers
```

## Implementation Options

### Option 1: Direct Integration (Recommended)

Add WebSocket client to [`main/main.cpp`](main.cpp:1) that forwards H.264 frames to relay server.

**Modifications to main.cpp:**

```cpp
// Add at top
#include "relay_forwarder.h"

// Add global relay forwarder
static RelayForwarder* g_relay = nullptr;

// In main(), after video initialization:
const char* relay_url = std::getenv("RELAY_URL");
const char* camera_id = std::getenv("CAMERA_ID");
const char* jwt_token = std::getenv("RELAY_TOKEN");

if (relay_url && camera_id) {
    printf("%s: Enabling relay forwarding to %s\n", TAG, relay_url);
    g_relay = new RelayForwarder(relay_url, camera_id, jwt_token ? jwt_token : "");
    if (!g_relay->start()) {
        fprintf(stderr, "%s: Failed to start relay forwarder\n", TAG);
    }
}

// In video_frame_callback(), after sending to local clients:
if (g_relay && g_relay->isConnected()) {
    g_relay->sendFrame(frame.first, frame.second, is_keyframe);
}

// In cleanup:
if (g_relay) {
    g_relay->stop();
    delete g_relay;
}
```

### Option 2: External Proxy (Simpler, No Code Changes)

Run a separate proxy process that reads from local WebSocket and forwards to relay:

**proxy.sh:**
```bash
#!/bin/bash
# Simple WebSocket proxy using websocat
LOCAL_WS="ws://localhost:8765/?channel=0"
RELAY_WS="wss://relay.example.com/ws"
CAMERA_ID="camera_12345"
TOKEN="your_jwt_token"

# Install: apt-get install websocat
websocat -b \
  "$LOCAL_WS" \
  --header="Authorization: Bearer $TOKEN" \
  --header="Camera-ID: $CAMERA_ID" \
  "$RELAY_WS"
```

### Option 3: Node.js Proxy

```javascript
const WebSocket = require('ws');

const localWs = new WebSocket('ws://localhost:8765/?channel=0');
const relayWs = new WebSocket('wss://relay.example.com/ws', {
    headers: {
        'Authorization': 'Bearer ' + process.env.RELAY_TOKEN,
        'Camera-ID': process.env.CAMERA_ID
    }
});

localWs.on('message', (data) => {
    if (relayWs.readyState === WebSocket.OPEN) {
        relayWs.send(data);
    }
});

relayWs.on('close', () => {
    console.log('Relay disconnected, reconnecting...');
    setTimeout(() => process.exec('node proxy.js'), 5000);
});
```

## Configuration

### Environment Variables

```bash
# Camera configuration
export CAMERA_ID="camera_unique_12345"
export RELAY_URL="wss://relay.example.com/ws"
export RELAY_TOKEN="eyJhbGciOi...." # JWT token for auth

# Start camera-streamer with relay
./camera-streamer
```

### systemd Service

```ini
[Unit]
Description=Camera Streamer with Relay
After=network.target

[Service]
Type=simple
User=root
Environment="CAMERA_ID=camera_12345"
Environment="RELAY_URL=wss://relay.example.com/ws"
Environment="RELAY_TOKEN=<your_jwt_token>"
ExecStart=/usr/bin/camera-streamer
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Testing

### 1. Start Relay Server

```bash
cd video-relay-server
go run ./cmd/server/main.go -config config.yaml
```

### 2. Start Camera-Streamer (with proxy)

```bash
# Terminal 1: camera-streamer
./camera-streamer

# Terminal 2: proxy
CAMERA_ID="camera_test" RELAY_TOKEN="" \
  websocat ws://localhost:8765/?channel=0 \
  --header="Camera-ID: camera_test" \
  ws://localhost:8443/ws
```

### 3. Open Viewer

```
http://localhost:8443/

# Configure:
Server: ws://localhost:8443/ws
Camera ID: camera_test
Token: (leave empty if auth disabled)

# Click Connect
```

## Bandwidth Considerations

Camera-streamer sends frames to:
1. **Local WebSocket clients** (LAN)
2. **Relay server** (WAN)

**Upload bandwidth per channel:**
- CH0 (1920x1080 @ 30fps): ~3 Mbps
- CH1 (1280x720 @ 30fps): ~1.5 Mbps
- CH2 (640x480 @ 15fps): ~500 Kbps

**Recommendation:** Use CH2 (low res) for relay to conserve bandwidth:
```bash
websocat ws://localhost:8765/?channel=2 ws://relay:8443/ws
```

## Troubleshooting

**Camera won't connect to relay:**
- Check RELAY_URL is correct and reachable
- Verify JWT token is valid (if auth enabled)
- Check firewall allows outbound HTTPS/WSS
- Test locally first: `ws://localhost:8443/ws`

**High latency:**
- Use lower resolution channel (CH2)
- Check upload bandwidth (need ~500Kbps minimum)
- Verify relay server not overloaded

**Connection drops:**
- Add automatic reconnection logic
- Use persistent connection (TCP keepalive)
- Check NAT timeout settings on router

## Next Steps

For production deployment:
1. Build relay forwarder into camera-streamer binary
2. Add configuration file support (vs environment variables)
3. Implement automatic reconnection with exponential backoff
4. Add connection health monitoring
5. Support multiple relay servers for redundancy

## References

- [Camera-Streamer README](README.md)
- [Relay Server](../../../../video-relay-server/README.md)
- [WebSocket Streaming Architecture](../../../../docs/WEBSOCKET_STREAMING_ARCHITECTURE.md)

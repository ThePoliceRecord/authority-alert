# On-Demand Streaming Implementation

**Status:** Implemented  
**Last Updated:** 2026-01-01

## Overview

On-demand streaming enables the camera to only stream video when viewers are actively watching, dramatically reducing bandwidth usage and resource consumption. The streaming automatically starts when the first viewer connects and stops when the last viewer disconnects.

## Configuration

### Supervisor Configuration

The supervisor can operate in two modes:

#### 1. Always-On Mode (Default)
Continuously streams to relay regardless of viewer count:
```bash
export EXTERNAL_RELAY_URL="ws://relay.example.com/ws"
export CAMERA_ID="camera_12345"
export RELAY_TOKEN="your_token_here"
# ON_DEMAND_STREAMING not set or set to "false"
```

#### 2. On-Demand Mode
Starts streaming only when viewers are watching:
```bash
export EXTERNAL_RELAY_URL="ws://relay.example.com/ws"
export CAMERA_ID="camera_12345"
export RELAY_TOKEN="your_token_here"
export ON_DEMAND_STREAMING="true"
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
|`EXTERNAL_RELAY_URL` | Yes | WebSocket URL of the relay server |
| `CAMERA_ID` | Yes | Unique identifier for this camera |
| `RELAY_TOKEN` | No | Bearer token for authentication (if relay has auth enabled) |
| `ON_DEMAND_STREAMING` | No | Set to `"true"` to enable on-demand mode (default: always-on) |

## Architecture Components

### Relay Server

**New Components:**
- [`ConnectionTypeSupervisor`](../video-relay-server/internal/relay/relay.go:20) - New connection type for supervisor control
- [`supervisors`](../video-relay-server/internal/relay/relay.go:54) map - Tracks supervisor connections by camera ID
- [`viewerCounts`](../video-relay-server/internal/relay/relay.go:57) map - Tracks active viewer count per camera
- [`RegisterSupervisor()`](../video-relay-server/internal/relay/relay.go:128) - Registers supervisor control connections
- [`SendControlMessage()`](../video-relay-server/internal/relay/relay.go:157) - Sends start/stop commands to supervisor

**Modified Functions:**
- [`SubscribeViewer()`](../video-relay-server/internal/relay/relay.go:193) - Triggers `start_streaming` when first viewer connects
- [`UnregisterViewer()`](../video-relay-server/internal/relay/relay.go:252) - Triggers `stop_streaming` when last viewer disconnects

**New Handler:**
- [`handleSupervisorConnection()`](../video-relay-server/cmd/server/main.go:293) - Processes status messages from supervisor

### Supervisor

**New Files:**
- [`forwarder_manager.go`](../sscma-example-sg200x/solutions/supervisor/internal/handler/forwarder_manager.go) - Manages dynamic forwarder lifecycle
- [`supervisor_control.go`](../sscma-example-sg200x/solutions/supervisor/internal/handler/supervisor_control.go) - Handles control connection to relay

**Key Components:**
- `ForwarderManager` - Starts/stops individual channel forwarders on demand
- `StartSupervisorControl()` - Establishes persistent control connection
- `handleStartStreaming()` - Processes start_streaming commands
- `handleStopStreaming()` - Processes stop_streaming commands

## Message Protocol

### Control Messages (Relay → Supervisor)

#### Start Streaming
```json
{
  "type": "start_streaming",
  "camera_id": "camera_12345",
  "channels": ["high", "medium", "low"],
  "viewer_count": 1
}
```

#### Stop Streaming
```json
{
  "type": "stop_streaming",
  "camera_id": "camera_12345",
  "channels": ["high", "medium", "low"],
  "viewer_count": 0
}
```

### Status Messages (Supervisor → Relay)

#### Streaming Started
```json
{
  "type": "streaming_started",
  "camera_id": "camera_12345",
  "channels_active": ["high", "medium", "low"]
}
```

#### Streaming Stopped
```json
{
  "type": "streaming_stopped",
  "camera_id": "camera_12345"
}
```

## Connection Flow

### On-Demand Mode

1. **Supervisor Startup:**
   - Connects to relay with `Role: supervisor` header
   - Waits for control commands
   - No video streaming initially

2. **First Viewer Connects:**
   - Relay receives viewer subscription
   - Relay sends `start_streaming` to supervisor
   - Supervisor starts forwarders for requested channels
   - Supervisor sends `streaming_started` acknowledgment
   - Video begins flowing to relay

3. **Subsequent Viewers:**
   - Relay continues streaming
   - Viewer count incremented
   - No additional supervisor commands

4. **Last Viewer Disconnects:**
   - Relay detects viewer count = 0
   - Relay sends `stop_streaming` to supervisor
   - Supervisor stops all forwarders
   - Supervisor sends `streaming_stopped` acknowledgment
   - Camera returns to idle

## Benefits

### Bandwidth Savings
For a 1920x1080 @ 30fps stream (~3 Mbps):
- **Always-on:** 970 GB/month
- **On-demand (10% viewing time):** 97 GB/month
- **Savings:** 90% reduction

### Resource Savings
- Camera encoder idles when not needed
- Lower power consumption
- Reduced thermal load
- Extended hardware lifespan

### Scalability
- Relay server only processes active streams
- Can support more cameras per server instance

## Migration

The system is **backward compatible**. Existing deployments continue to work in always-on mode:

- **Phase 1:** Deploy updated relay server (supports both modes)
- **Phase 2:** Deploy updated supervisor (defaults to always-on)
- **Phase 3:** Enable on-demand per camera by setting `ON_DEMAND_STREAMING=true`

## Testing

### Manual Test Procedure

1. **Start relay server:**
   ```bash
   cd video-relay-server
   go run cmd/server/main.go
   ```

2. **Start supervisor with on-demand enabled:**
   ```bash
   export ON_DEMAND_STREAMING=true
   export EXTERNAL_RELAY_URL="ws://localhost:8080/ws"
   export CAMERA_ID="test_camera"
   cd sscma-example-sg200x/solutions/supervisor
   go run cmd/supervisor/main.go
   ```

3. **Connect first viewer:**
   - Open viewer page in browser
   - Subscribe to `test_camera`
   - Observe supervisor logs: "Starting forwarder for channel..."
   - Verify video playback

4. **Connect second viewer:**
   - Open another browser tab
   - Subscribe to same camera
   - Verify both viewers receive video
   - Observe: No new forwarders started

5. **Disconnect all viewers:**
   - Close both viewer tabs
   - Observe supervisor logs: "Stopping forwarder for channel..."
   - Verify no video being sent

6. **Reconnect viewer:**
   - Open viewer again
   - Observe forwarders restart
   - Verify video resumes

### Expected Log Output

**Supervisor (on-demand mode):**
```
Starting supervisor control connection for on-demand streaming
Connecting to relay as supervisor for camera test_camera
Supervisor control connection established
Received start_streaming command for camera test_camera, channels: [high medium low], viewers: 1
Starting forwarder for channel high
Started forwarder for channel high
...
Received stop_streaming command for camera test_camera, channels: [high medium low]
Stopping forwarder for channel high
```

**Relay Server:**
```
Supervisor registered: camera_id=test_camera
Viewer subscribed to camera: camera_id=test_camera viewer_count=1
Sent control message to supervisor: type=start_streaming viewer_count=1
Supervisor confirmed streaming started: channels=[high medium low]
...
Viewer unsubscribed from camera: camera_id=test_camera viewer_count=0
Sent control message to supervisor: type=stop_streaming viewer_count=0
Supervisor confirmed streaming stopped
```

## Troubleshooting

### Forwarders don't start on viewer connect
- Check supervisor logs for control connection status
- Verify `ON_DEMAND_STREAMING=true` is set
- Ensure relay server has supervisor connection registered

### Forwarders don't stop on viewer disconnect
- Check relay server viewer count tracking
- Verify supervisor receives stop_streaming command
- Check for network connectivity issues

### Always-on mode still active
- Ensure `ON_DEMAND_STREAMING=true` environment variable is set
- Restart supervisor after changing environment
- Check supervisor logs for "On-demand streaming disabled" message

## Robust Reconnection

The implementation includes enterprise-grade reconnection logic to handle network interruptions gracefully:

### Control Connection Reconnection

**Features:**
- **Exponential backoff:** Starts at 1s, doubles each attempt, caps at 60s
- **Jitter:** Random 0-2s added to prevent thundering herd
- **Ping/pong heartbeat:** 30s intervals with 90s timeout
- **Connection state tracking:** Monitors active connection status
- **Automatic cleanup:** Gracefully closes dead connections

**Behavior:**
```
Attempt 1: ~1-3s delay
Attempt 2: ~2-4s delay  
Attempt 3: ~4-6s delay
Attempt 4: ~8-10s delay
Attempt 5+: ~60-62s delay (capped)
```

When control connection is re-established, the reconnect counter resets to 0.

### Video Forwarder Reconnection

**Features:**
- **Per-channel exponential backoff:** Each channel (high/medium/low) reconnects independently
- **Context-aware cancellation:** Forwarders stop immediately when commanded
- **Connection timeouts:** 10s handshake, 30s read timeout for video, 90s for control
- **Deadline management:** Read/write deadlines prevent hanging connections
- **Error classification:** Distinguishes normal closures from errors

**Behavior:**
```
Attempt 1: ~1-2s delay
Attempt 2: ~2-3s delay
Attempt 3: ~4-5s delay
Attempt 4: ~8-9s delay
Attempt 5+: ~30-31s delay (capped)
```

### Network Failure Scenarios

**Scenario 1: Temporary Network Interruption (5s)**
1. Connection drops
2. Supervisor detects error within 10-30s (read timeout)
3. Reconnects in ~1-3s (first attempt)
4. Normal operation resumes
5. Total downtime: <35s

**Scenario 2: Relay Server Restart**
1. All connections drop
2. Control connection reconnects with backoff
3. If viewers present, relay immediately sends start_streaming
4. Video forwarders start within seconds
5. Streaming resumes automatically

**Scenario 3: Extended Outage (5 minutes)**
1. Connection drops
2. Multiple reconnect attempts with backoff
3. Eventually stabilizes at 60s retry interval
4. When network returns, reconnects on next attempt
5. No data loss (viewers re-buffer)

**Scenario 4: Supervisor Restart mid-stream**
1. Supervisor starts, establishes control connection
2. If viewers watching, relay sends start_streaming immediately
3. Forwarders start, streaming resumes
4. Viewers experience brief buffering but auto-recover

### Monitoring Reconnection

**Log Messages:**
```
INFO: Supervisor control connection established
DEBUG: Forwarder attempt 1 for camera test_camera_high
WARNING: Camera read error: EOF
INFO: Reconnecting forwarder for camera test_camera_high in 2.1s (attempt 2)
INFO: Reconnecting control connection in 4.3s... (attempt 3)
```

**Health Indicators:**
- `isConnected` flag tracks active control connection
- `reconnectAttempt` counter shows retry state (0 = healthy)
- Ping/pong messages every 30s confirm liveness
- Error logs indicate disruption type

### Best Practices

1. **Network Stability:** Place supervisor and relay on stable networks
2. **Monitoring:** Watch for frequent reconnects indicating network issues
3. **Timeouts:** Increase read timeouts if using high-latency links
4. **Logging:** Enable debug logs during troubleshooting
5. **Load Testing:** Verify reconnection under viewer load

## Future Enhancements

Potential improvements for future versions:

1. **Idle Timeout:** Continue streaming for 30s after last viewer (prevents reconnect flapping)
2. **Channel Selection:** Start only specific quality channels based on viewer preferences
3. **Metrics:** Track on-demand efficiency (bandwidth saved, idle time, etc.)
4. **Health Checks:** Periodic status updates from supervisor
5. **Graceful Shutdown:** Proper cleanup on supervisor termination

---

This implementation transforms the system from always-on to smart on-demand streaming, dramatically reducing operational costs while maintaining instant-on capability for viewers.

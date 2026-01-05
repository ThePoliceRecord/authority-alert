# On-Demand Streaming (Viewer Count-Based)

**Status:** Architecture Design  
**Last Updated:** 2026-01-01

## Goal

Only stream video from camera to relay server when viewers are actively watching, saving bandwidth and processing power.

## Current vs Desired Behavior

**Current (Always-On):**
```
Camera → Supervisor → Relay Server (always streaming)
                         ↓
                    (waiting for viewers)
```
- Wastes upload bandwidth
- Wastes server resources
- Camera encoder always running

**Desired (On-Demand):**
```
Viewer connects → Relay signals → Supervisor starts → Camera streams
Viewer disconnects → Relay signals → Supervisor stops → Camera idles
```

## Architecture

### Component Responsibilities

**Relay Server:**
1. Tracks viewer count per camera ID
2. When first viewer subscribes → send `start_streaming` to supervisor
3. When last viewer leaves → send `stop_streaming` to supervisor

**Supervisor:**
1. Maintains persistent control connection to relay
2. Receives stream control commands
3. Starts/stops external relay forwarders on-demand
4. Reports status back to relay

### Message Protocol

**Relay → Supervisor (Control Channel):**
```json
{
  "type": "start_streaming",
  "camera_id": "camera_12345",
  "channels": ["high", "medium", "low"],
  "viewer_count": 1
}

{
  "type": "stop_streaming",
  "camera_id": "camera_12345",
  "channels": ["high", "medium", "low"],
  "viewer_count": 0
}
```

**Supervisor → Relay (Status Channel):**
```json
{
  "type": "streaming_started",
  "camera_id": "camera_12345",
  "channels_active": ["high", "medium", "low"]
}

{
  "type": "streaming_stopped",
  "camera_id": "camera_12345"
}
```

## Implementation

### 1. Relay Server Changes

**Track viewer count:**
```go
// Add to Server struct
viewerCounts map[string]int  // cameraID -> count
controlConns map[string]*Connection  // cameraID -> supervisor control connection

// In SubscribeViewer:
if len(s.cameraMap[cameraID]) == 1 {  // First viewer
    s.sendControlMessage(cameraID, "start_streaming")
}

// In UnregisterViewer:
if len(s.cameraMap[cameraID]) == 0 {  // Last viewer left
    s.sendControlMessage(cameraID, "stop_streaming")
}
```

### 2. Supervisor Changes

**Control connection:**
```go
// Connect to relay with role="supervisor"
headers := http.Header{}
headers.Set("Camera-ID", cameraID)
headers.Set("Role", "supervisor")

controlConn, _, err := websocket.DefaultDialer.Dial(relayURL, headers)

// Handle control messages
go func() {
    for {
        var msg map[string]interface{}
        if err := controlConn.ReadJSON(&msg); err != nil {
            return
        }
        
        switch msg["type"] {
        case "start_streaming":
            startForwarders(msg["channels"])
        case "stop_streaming":
            stopForwarders()
        }
    }
}()
```

**Dynamic forwarder lifecycle:**
```go
type ForwarderManager struct {
    forwarders map[string]context.CancelFunc
    mu sync.Mutex
}

func (fm *ForwarderManager) Start(channel string) {
    fm.mu.Lock()
    defer fm.mu.Unlock()
    
    if _, exists := fm.forwarders[channel]; exists {
        return  // Already running
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    fm.forwarders[channel] = cancel
    
    go runForwarder(ctx, channel)
}

func (fm *ForwarderManager) Stop(channel string) {
    fm.mu.Lock()
    defer fm.mu.Unlock()
    
    if cancel, exists := fm.forwarders[channel]; exists {
        cancel()
        delete(fm.forwarders, channel)
    }
}
```

### 3. New Connection Role

**Add "supervisor" role:**
- Supervisor connects with `Role: supervisor` header
- Receives control messages (not video data)
- Can send status updates

## Benefits

**Bandwidth Savings:**
- 1920x1080 @ 30fps = ~3 Mbps upload
- If streaming 24/7 but only watched 10% of time:
  - Before: 970 GB/month
  - After: 97 GB/month
  - **Savings: 90%**

**Resource Savings:**
- Camera encoder can idle when not needed
- Lower power consumption
- Reduced thermal load
- Longer hardware lifespan

**Scalability:**
- Relay server only processes active streams
- Can support more cameras per server instance

## Migration Path

1. **Phase 1:** Add control connection to relay server (backward compatible)
2. **Phase 2:** Implement viewer count tracking in relay
3. **Phase 3:** Add dynamic forwarder management to supervisor
4. **Phase 4:** Add timeout (continue streaming for 30s after last viewer leaves, in case they reconnect)

## Configuration

```yaml
# In supervisor config
external_relay:
  enabled: true
  url: "ws://relay.example.com/ws"
  camera_id: "camera_12345"
  on_demand: true  # Enable on-demand streaming
  idle_timeout: 30  # Seconds to wait after last viewer before stopping
```

## Next Steps

1. Implement control message handling in relay server
2. Add forwarder lifecycle management to supervisor
3. Test viewer connect/disconnect scenarios
4. Add metrics for on-demand efficiency

---

This feature transforms the system from always-on to smart on-demand streaming, dramatically reducing operational costs while maintaining instant-on capability for viewers.

# Multi-Channel Camera Streamer Specification

## Overview

This specification describes the implementation of a unified multi-channel video streamer that outputs all three camera channels (CH0, CH1, CH2) with different resolutions and frame rates to serve various use cases.

## Current State

The [`camera-streamer`](../solutions/camera-streamer/main/main.cpp) currently supports only a single channel (VIDEO_CH0) at 1920x1080@30fps.

## Objectives

1. Enable simultaneous streaming from all three video channels
2. Provide channel-specific access via WebSocket and shared memory IPC
3. **Break existing single-channel clients** - no backward compatibility required
4. Add REST API endpoint for channel discovery
5. Update web frontend with channel selection UI
6. Optimize resource usage with unified process architecture

## Architecture

### System Design

```mermaid
graph TB
    subgraph Camera Input
        CAM[Camera SG200x]
    end
    
    subgraph Video SDK
        CAM --> CH0[VIDEO_CH0<br/>1920x1080@30fps]
        CAM --> CH1[VIDEO_CH1<br/>1280x720@30fps]
        CAM --> CH2[VIDEO_CH2<br/>640x480@15fps]
    end
    
    subgraph Unified Streamer Process
        CH0 --> Q0[Frame Queue 0]
        CH1 --> Q1[Frame Queue 1]
        CH2 --> Q2[Frame Queue 2]
        
        Q0 --> WS[WebSocket Server<br/>Port 8765]
        Q1 --> WS
        Q2 --> WS
        
        Q0 --> SHM0[Shared Memory<br/>/video_stream_ch0]
        Q1 --> SHM1[Shared Memory<br/>/video_stream_ch1]
        Q2 --> SHM2[Shared Memory<br/>/video_stream_ch2]
    end
    
    subgraph Clients
        WS --> WEB[Web Browser<br/>Channel Selection]
        SHM0 --> APP1[Local Apps<br/>High Res]
        SHM1 --> APP2[Local Apps<br/>Medium Res]
        SHM2 --> APP3[Local Apps<br/>Low Res]
    end
```

## Channel Configuration

### Channel 0: High Resolution
- **Resolution**: 1920x1080
- **Frame Rate**: 30 fps
- **Format**: H.264
- **Use Cases**: Recording, archival, high-quality streaming
- **Bitrate**: ~2-4 Mbps

### Channel 1: Medium Resolution
- **Resolution**: 1280x720
- **Frame Rate**: 30 fps
- **Format**: H.264
- **Use Cases**: Web viewing, remote monitoring
- **Bitrate**: ~1-2 Mbps

### Channel 2: Low Resolution
- **Resolution**: 640x480
- **Frame Rate**: 15 fps
- **Format**: H.264
- **Use Cases**: AI/ML processing, motion detection, analytics
- **Bitrate**: ~0.5-1 Mbps

## Data Structures

### Per-Channel State

```cpp
typedef struct {
    video_ch_index_t channel_id;
    std::queue<std::pair<uint8_t*, size_t>> frame_queue;
    std::mutex queue_mutex;
    std::set<struct mg_connection*> ws_clients;
    std::mutex clients_mutex;
    video_shm_producer_t shm_producer;
    bool shm_enabled;
    video_ch_param_t params;
} channel_state_t;
```

### Global State

```cpp
static volatile bool g_running = true;
static struct mg_mgr g_mgr;
static channel_state_t g_channels[3];  // Array for CH0, CH1, CH2
```

## WebSocket Protocol

### Connection URL Format

Clients **must** specify which channel to subscribe to:
- `ws://device-ip:8765/?channel=0` - High res (1920x1080@30fps)
- `ws://device-ip:8765/?channel=1` - Medium res (1280x720@30fps)
- `ws://device-ip:8765/?channel=2` - Low res (640x480@15fps)

**Breaking Change**: Connecting without `?channel=N` parameter will return an error.

### Frame Packet Format

```
┌──────────────┬────────────────────┬─────────────────┐
│ Channel ID   │ Frame Data         │ Timestamp       │
│ (1 byte)     │ (N bytes)          │ (8 bytes)       │
└──────────────┴────────────────────┴─────────────────┘
```

- **Channel ID**: 0, 1, or 2
- **Frame Data**: H.264 encoded video frame
- **Timestamp**: Little-endian uint64 milliseconds since epoch

### Client Behavior

1. Client connects with required `channel=N` parameter
2. Server validates channel number (0-2)
3. Server rejects connection if parameter missing or invalid
4. Server tracks client's subscribed channel
5. Server sends only frames from subscribed channel
6. Channel switching requires reconnection with different parameter

## Shared Memory IPC

### Overview

Three separate shared memory regions for zero-copy local access.

**Breaking Change**: The old `/video_stream` IPC path is removed. All consumers must migrate to new channel-specific names.

| Region | Name | Size |
|--------|------|------|
| CH0 | `/video_stream_ch0` | ~15MB |
| CH1 | `/video_stream_ch1` | ~15MB |
| CH2 | `/video_stream_ch2` | ~15MB |

### API Changes

Update shared memory API to accept channel parameter:

```c
// Updated consumer initialization
int video_shm_consumer_init(video_shm_consumer_t* consumer, int channel_id);

// Example: Consumer for medium resolution
video_shm_consumer_t consumer;
video_shm_consumer_init(&consumer, 1);  // Opens /video_stream_ch1
```

### Shared Memory Naming

Producer creates shared memory with channel suffix:
```c
char shm_name[64];
snprintf(shm_name, sizeof(shm_name), "/video_stream_ch%d", channel_id);
```

## REST API for Channel Discovery

### New Endpoint: GET /api/channels

Returns available channel information for client configuration.

**Request:**
```
GET /api/channels HTTP/1.1
Host: device-ip:5000
```

**Response:**
```json
{
  "channels": [
    {
      "id": 0,
      "name": "High Resolution",
      "resolution": "1920x1080",
      "fps": 30,
      "bitrate": "2-4 Mbps",
      "codec": "H.264",
      "use_case": "Recording, archival, high-quality streaming",
      "websocket_url": "ws://device-ip:8765/?channel=0",
      "shm_path": "/video_stream_ch0",
      "active": true
    },
    {
      "id": 1,
      "name": "Medium Resolution",
      "resolution": "1280x720",
      "fps": 30,
      "bitrate": "1-2 Mbps",
      "codec": "H.264",
      "use_case": "Web viewing, remote monitoring",
      "websocket_url": "ws://device-ip:8765/?channel=1",
      "shm_path": "/video_stream_ch1",
      "active": true
    },
    {
      "id": 2,
      "name": "Low Resolution",
      "resolution": "640x480",
      "fps": 15,
      "bitrate": "0.5-1 Mbps",
      "codec": "H.264",
      "use_case": "AI/ML processing, motion detection",
      "websocket_url": "ws://device-ip:8765/?channel=2",
      "shm_path": "/video_stream_ch2",
      "active": true
    }
  ],
  "default_channel": 1
}
```

This endpoint will be implemented in the Go supervisor backend.

## Web Frontend Updates

### Channel Selection UI

Update [`overview/index.tsx`](../solutions/supervisor/www/src/views/overview/index.tsx) to add channel selector:

**UI Changes:**
1. Add dropdown/button group above video player
2. Display channel resolution and FPS
3. Show active channel indicator
4. Allow user to switch between channels

**Component Structure:**
```tsx
<div>
  {/* Channel Selector */}
  <div className="channel-selector">
    <button onClick={() => switchChannel(0)}>CH0: High (1920x1080)</button>
    <button onClick={() => switchChannel(1)}>CH1: Medium (1280x720)</button>
    <button onClick={() => switchChannel(2)}>CH2: Low (640x480)</button>
  </div>
  
  {/* Video Player */}
  <video id="player" ... />
  
  {/* Info Display */}
  <div>Resolution: {channelInfo.resolution}</div>
  <div>FPS: {channelInfo.fps}</div>
</div>
```

### Hook Updates

Update [`overview/hook.ts`](../solutions/supervisor/www/src/views/overview/hook.ts):

**Changes:**
1. Add `selectedChannel` state
2. Fetch channel list from `/api/channels` on mount
3. Construct WebSocket URL with `?channel=N` parameter
4. Handle channel switching by reconnecting WebSocket
5. Update jmuxer fps based on selected channel

**Key Changes:**
```typescript
const [selectedChannel, setSelectedChannel] = useState(1); // Default to CH1
const [channels, setChannels] = useState([]);

// Fetch channel list
useEffect(() => {
  fetch('/api/channels')
    .then(res => res.json())
    .then(data => {
      setChannels(data.channels);
      setSelectedChannel(data.default_channel);
    });
}, []);

// Update WebSocket URL construction
const websocketUrl = `${data.websocketUrl}?channel=${selectedChannel}`;

// Channel switching function
function switchChannel(channelId) {
  setSelectedChannel(channelId);
  // WebSocket will reconnect with new URL
}
```

## Implementation Plan

### Phase 1: Core Infrastructure
1. Refactor data structures to support per-channel state
2. Create channel initialization and cleanup functions
3. Implement channel-specific frame queues

### Phase 2: Video Subsystem Integration
1. Initialize all three video channels with appropriate parameters
2. Register separate frame callbacks for each channel
3. Implement channel-specific frame processing

### Phase 3: WebSocket Enhancements
1. Parse WebSocket connection URL for channel parameter (REQUIRED)
2. Reject connections without valid channel parameter
3. Track client's subscribed channel
4. Multiplex frame delivery based on client subscriptions
5. Add channel ID prefix to frame packets

### Phase 4: Shared Memory Updates
1. **BREAKING**: Remove old `/video_stream` shared memory
2. Create three separate shared memory regions with channel suffixes
3. Update `video_shm_consumer_init()` to accept channel parameter
4. Write frames to appropriate shared memory based on channel

### Phase 5: REST API (Go Supervisor)
1. Create new handler in Go supervisor for `/api/channels` endpoint
2. Return hardcoded channel configuration (or query camera-streamer status)
3. Update API index to register new route

### Phase 6: Frontend Updates
1. Update `overview/index.tsx` to add channel selector UI
2. Update `overview/hook.ts` to fetch channel list and handle selection
3. Implement WebSocket reconnection on channel switch
4. Update jmuxer configuration based on selected channel FPS

### Phase 7: Testing & Validation
1. Unit test each channel independently
2. Test concurrent channel streaming
3. Validate shared memory IPC for all channels
4. Test frontend channel switching
5. Performance testing and optimization

## Resource Requirements

### CPU
- Expected load: ~20-30% for 3 concurrent H.264 encoders
- Current (single channel): ~8-10%

### Memory
- Current: ~15MB shared memory + ~5MB process
- New: ~45MB shared memory (15MB × 3) + ~10MB process
- Total: ~55MB

### Network Bandwidth
- CH0: 2-4 Mbps
- CH1: 1-2 Mbps  
- CH2: 0.5-1 Mbps
- Total: ~4-7 Mbps concurrent

## Breaking Changes

### ❌ Removed
1. `/video_stream` shared memory (use `/video_stream_ch0` instead)
2. WebSocket connections without `?channel=N` parameter
3. Single-channel assumptions throughout codebase

### ✅ Migration Path

**For WebSocket Clients:**
```diff
- ws://device-ip:8765/
+ ws://device-ip:8765/?channel=1
```

**For Shared Memory Consumers:**
```diff
- video_shm_consumer_init(&consumer);
+ video_shm_consumer_init(&consumer, 1);  // Channel 1
```

**For Frontend:**
- Web UI automatically updated with channel selector
- Default channel: CH1 (medium resolution)

## Success Criteria

1. All three channels stream simultaneously
2. WebSocket clients **must** specify channel parameter
3. Shared memory IPC available for all channels with new API
4. Web frontend displays channel selector and switches channels
5. `/api/channels` endpoint returns channel metadata
6. CPU usage < 35%
7. Memory usage < 60MB
8. No frame drops under normal load
9. **Breaking changes are acceptable** - existing clients must update

## Configuration

### Compile-Time Configuration
```cpp
#define MAX_CHANNELS 3
#define MAX_QUEUE_SIZE_PER_CHANNEL 30
#define ENABLE_CHANNEL_0 1
#define ENABLE_CHANNEL_1 1
#define ENABLE_CHANNEL_2 1
```

### Runtime Configuration (Future)
- Configuration file to enable/disable channels
- Per-channel resolution and framerate customization
- Dynamic quality adjustment

## Testing Strategy

### Unit Tests
1. Channel initialization and cleanup
2. Frame queue management per channel
3. WebSocket channel parameter parsing
4. Shared memory producer initialization per channel

### Integration Tests
1. All three channels streaming simultaneously
2. Multiple WebSocket clients on different channels
3. Multiple shared memory consumers per channel
4. Channel switching via reconnection

### Performance Tests
1. CPU utilization monitoring
2. Memory usage tracking
3. Frame drop rate measurement
4. Latency benchmarking (WebSocket and shared memory)

## Error Handling

### Channel Initialization Failures
- If any channel fails to initialize, log error but continue with available channels
- Provide API to query which channels are active

### Frame Queue Overflow
- Drop oldest frames (per-channel behavior)
- Increment per-channel dropped frame counter

### Shared Memory Failures
- Continue WebSocket streaming even if shared memory fails
- Log warnings for debugging

## Deployment

### Build Process
- Existing CMake build process unchanged
- Package includes multi-channel binary

### Installation
```bash
sudo opkg install camera-streamer-1.1.0-1.deb
```

### Startup
- Service starts automatically via `/etc/init.d/S95camera-streamer`
- All configured channels start simultaneously
- Health check API reports active channels

## API Extensions

### HTTP Status Endpoint (Future Enhancement)

```
GET /status
```

Response:
```json
{
  "channels": [
    {
      "id": 0,
      "resolution": "1920x1080",
      "fps": 30,
      "active_clients": 2,
      "frames_sent": 15234,
      "frames_dropped": 3
    },
    {
      "id": 1,
      "resolution": "1280x720",
      "fps": 30,
      "active_clients": 1,
      "frames_sent": 15230,
      "frames_dropped": 0
    },
    {
      "id": 2,
      "resolution": "640x480",
      "fps": 15,
      "active_clients": 0,
      "frames_sent": 7615,
      "frames_dropped": 1
    }
  ]
}
```

## Success Criteria

1. All three channels stream simultaneously
2. WebSocket clients can select any channel
3. Shared memory IPC available for all channels
4. CPU usage < 35%
5. Memory usage < 60MB
6. No frame drops under normal load
7. Backward compatible with existing single-channel clients

## References

- Current implementation: [`solutions/camera-streamer/main/main.cpp`](../solutions/camera-streamer/main/main.cpp)
- Video API: [`components/sophgo/video/video.h`](../components/sophgo/video/video.h)
- Shared Memory API: [`components/sophgo/video/include/video_shm.h`](../components/sophgo/video/include/video_shm.h)
- Video subsystem implementation: [`components/sophgo/video/src/video_shm.c`](../components/sophgo/video/src/video_shm.c)

## Revision History

| Version | Date | Author | Description |
|---------|------|--------|-------------|
| 1.0 | 2026-01-01 | Kilo Code | Initial specification |
| 1.1 | 2026-01-01 | Kilo Code | Remove backward compat, add REST API, add frontend updates |

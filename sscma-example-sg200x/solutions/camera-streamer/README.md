# Camera Streamer

## Overview

**camera-streamer** is a WebSocket video streaming server that captures H.264 video from **all three camera channels** simultaneously and streams them to web clients and local applications. This version supports multi-channel streaming with different resolutions for different use cases.

## Features

- **Multi-Channel Streaming**: Three simultaneous video channels (CH0, CH1, CH2)
- Real-time H.264 video streaming via WebSocket
- **Zero-copy shared memory IPC** for local applications (per-channel)
- Automatic timestamp appending for latency measurement
- Channel-specific resolutions and frame rates
- WebSocket channel selection via URL parameter
- Compatible with Supervisor web UI with channel selector
- Lightweight and efficient using Mongoose WebSocket server
- Based on proven Sophgo video SDK
- Multiple concurrent consumers supported per channel

## Channels Configuration

| Channel | Resolution | FPS | Use Case | WebSocket URL | Shared Memory |
|---------|------------|-----|----------|---------------|---------------|
| CH0 | 1920x1080 | 30 | High quality, recording, archival | `ws://device-ip:8765/?channel=0` | `/video_stream_ch0` |
| CH1 | 1280x720 | 30 | Web viewing, remote monitoring | `ws://device-ip:8765/?channel=1` | `/video_stream_ch1` |
| CH2 | 640x480 | 15 | AI/ML processing, motion detection | `ws://device-ip:8765/?channel=2` | `/video_stream_ch2` |

**Default Channel**: CH1 (Medium resolution, balanced for web viewing)

## Architecture

```
Camera (sg200x) → Video SDK → H.264 Encoders (x3) → ┬→ Shared Memory (/video_stream_ch0/ch1/ch2)
                                                      │   (zero-copy, <1ms latency)
                                                      │
                                                      └→ WebSocket (port 8765 + ?channel=N)
                                                          (+ channel_id + timestamp)
```

## Building

### Prerequisites

Ensure you have set up the **ReCamera-OS** environment:
```bash
export SG200X_SDK_PATH=<PATH_TO_RECAMERA-OS>/output/sg2002_recamera_emmc/
export PATH=<PATH_TO_RECAMERA-OS>/host-tools/gcc/riscv64-linux-musl-x86_64/bin:$PATH
```

### Build Steps

**Important:** Build from the solution directory, not the repository root.

```bash
cd solutions/camera-streamer
cmake -B build -DCMAKE_BUILD_TYPE=Release .
cmake --build build
```

The CMakeLists.txt will automatically find components relative to the repository root.

### Create Package

```bash
cd build && cpack
```

This creates `camera-streamer-1.1.0-1.deb`

## Installation

### Transfer to Device

```bash
scp build/camera-streamer-1.1.0-1.deb recamera@192.168.42.1:/tmp/
```

### Install on Device

```bash
ssh recamera@192.168.42.1
sudo opkg install /tmp/camera-streamer-1.1.0-1.deb
```

## Usage

### Manual Start

```bash
camera-streamer
```

The server will start on port **8765** and begin streaming all three H.264 video channels simultaneously.

### Auto-start with Supervisor

The camera-streamer can be automatically started by the supervisor service. The supervisor will manage its lifecycle.

## ⚠️ BREAKING CHANGES (v1.1.0)

### WebSocket Connections

**OLD (v1.0.x)**:
```javascript
const ws = new WebSocket('ws://device-ip:8765/');
```

**NEW (v1.1.0+)** - Channel parameter **REQUIRED**:
```javascript
const ws = new WebSocket('ws://device-ip:8765/?channel=1');  // Medium res
```

Connections without the `?channel=N` parameter will be rejected with HTTP 400.

### Shared Memory IPC

**OLD (v1.0.x)**:
```c
video_shm_consumer_t consumer;
video_shm_consumer_init(&consumer);  // Opens /video_stream
```

**NEW (v1.1.0+)** - Channel ID **REQUIRED**:
```c
video_shm_consumer_t consumer;
video_shm_consumer_init_channel(&consumer, 1);  // Opens /video_stream_ch1
```

The old `/video_stream` shared memory path no longer exists.

### Frame Packet Format

**OLD (v1.0.x)**:
```
[Frame Data (N bytes)][Timestamp (8 bytes)]
```

**NEW (v1.1.0+)**:
```
[Channel ID (1 byte)][Frame Data (N bytes)][Timestamp (8 bytes)]
```

## Shared Memory IPC

The camera-streamer now provides **zero-copy shared memory IPC for all three channels** for local applications to access video frames with minimal latency.

### Quick Start

```bash
# Build example consumer
cd examples
make

# Run consumer for channel 1 (in separate terminal)
./video_consumer 1
```

### API Usage

```c
#include "video_shm.h"

// Initialize consumer for specific channel
video_shm_consumer_t consumer;
if (video_shm_consumer_init_channel(&consumer, 1) != 0) {  // Channel 1
    fprintf(stderr, "Failed to init consumer\n");
    return -1;
}

// Read frames
uint8_t frame_buffer[VIDEO_SHM_MAX_FRAME_SIZE];
video_frame_meta_t meta;

while (running) {
    int size = video_shm_consumer_wait(&consumer, frame_buffer, &meta, 1000);
    if (size > 0) {
        // Process frame: frame_buffer[0..size-1]
        printf("Got frame: %dx%d, %u bytes, %s\n",
               meta.width, meta.height, meta.size,
               meta.is_keyframe ? "keyframe" : "frame");
    }
}

video_shm_consumer_destroy(&consumer);
```

### Documentation

- **[Shared Memory IPC Guide](SHARED_MEMORY_IPC.md)** - Complete API reference and integration guide
- **[Testing Guide](TESTING_GUIDE.md)** - Step-by-step testing procedures
- **[Example Consumer](examples/video_consumer_example.c)** - Reference implementation

### Use Cases

- ML inference (object detection, face recognition)
- Video recording to disk (select appropriate resolution)
- Motion detection (use low-res CH2 for efficiency)
- Frame analysis
- Custom video processing pipelines

### Performance

**Per Channel**:
- **Latency**: <1ms (vs ~50-100ms for WebSocket)
- **CPU**: <1% overhead per channel
- **Memory**: ~15MB shared buffer per channel
- **Throughput**: Varies by channel (30/30/15 fps)
- **Multiple readers**: Supported per channel

**Total System** (all 3 channels):
- **CPU**: ~20-30% (3 concurrent H.264 encoders)
- **Memory**: ~55MB (45MB shared + 10MB process)
- **Network**: ~4-7 Mbps total bandwidth

## Accessing the Streams

### Web UI with Channel Selector

1. Navigate to `http://<device-ip>/#/overview`
2. Use the channel selector buttons (CH0/CH1/CH2) above the video player
3. View real-time channel information (resolution, FPS, bitrate, use case)
4. Switch channels dynamically without page reload

### Direct WebSocket Connection

```javascript
// High resolution
const ws0 = new WebSocket('ws://<device-ip>:8765/?channel=0');

// Medium resolution (default)
const ws1 = new WebSocket('ws://<device-ip>:8765/?channel=1');

// Low resolution
const ws2 = new WebSocket('ws://<device-ip>:8765/?channel=2');

ws1.binaryType = 'arraybuffer';
ws1.onmessage = (event) => {
    const buffer = new Uint8Array(event.data);
    const channelId = buffer[0];  // First byte
    const frameData = buffer.subarray(1, buffer.length - 8);
    const timestamp = buffer.slice(-8);  // Last 8 bytes
    // Process frame...
};
```

### REST API - Channel Discovery

```bash
curl http://<device-ip>/api/channels
```

Response:
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
    // ... CH1, CH2
  ],
  "default_channel": 1
}
```

## Protocol

### WebSocket Frame Format

Each WebSocket message contains:
1. **Channel ID** (1 byte): 0, 1, or 2
2. **Binary data** (N bytes): H.264 encoded video frame
3. **Timestamp** (8 bytes): Little-endian uint64 milliseconds since epoch

Frontend uses jmuxer to decode H.264 and calculate display latency.

## Troubleshooting

### No video in browser
- Check if camera-streamer is running: `ps aux | grep camera-streamer`
- Verify WebSocket port is open: `netstat -tuln | grep 8765`
- Check browser console for WebSocket errors
- **Verify channel parameter is included in WebSocket URL**

### "channel parameter required" error
- Ensure WebSocket URL includes `?channel=0`, `?channel=1`, or `?channel=2`
- Update legacy clients to new URL format

### Shared memory consumer fails to connect
- Verify camera-streamer is running
- Check if correct channel-specific path is used: `/video_stream_ch0`, not `/video_stream`
- Verify permissions: `ls -l /dev/shm/`
- Use `video_shm_consumer_init_channel(consumer, channel_id)`, not legacy `video_shm_consumer_init()`

### Poor video quality
- Try a different channel (CH0 for highest quality)
- Check network bandwidth if using WebSocket
- Verify bitrate in logs matches expected values

### High latency
- Use shared memory IPC instead of WebSocket for local apps
- Check network connection for WebSocket clients
- Ensure device has sufficient resources (CPU/memory)
- Consider using lower resolution channel (CH2) if high FPS not needed

### High CPU usage
- Normal: ~20-30% for 3 concurrent H.264 encoders
- If higher, check for runaway processes
- Consider disabling unused channels (requires code modification)

## Integration with Go Supervisor

The camera-streamer is designed to work alongside the Go supervisor:
- Supervisor provides the web UI with channel selector and `/api/channels` endpoint
- Camera-streamer handles video streaming for all channels
- Both services run independently but cooperate
- Web UI automatically detects and displays available channels

See [`solutions/supervisor/`](../supervisor/) for supervisor implementation.

## Technical Details

### Components Used
- **Sophgo Video SDK**: Camera capture and H.264 encoding (3 instances)
- **Mongoose**: Lightweight WebSocket server with channel multiplexing
- **Video Subsystem**: Low-level video API with per-channel configuration

### Video Configuration

| Parameter | CH0 | CH1 | CH2 |
|-----------|-----|-----|-----|
| Format | H.264 | H.264 | H.264 |
| Resolution | 1920x1080 | 1280x720 | 640x480 |
| Frame Rate | 30 FPS | 30 FPS | 15 FPS |
| Channel ID | VIDEO_CH0 | VIDEO_CH1 | VIDEO_CH2 |

### Performance Metrics

**CPU Usage**: ~20-30% (all three encoders)
**Memory**: ~55MB total
**Network Bandwidth**: Variable by channel:
- CH0: ~2-4 Mbps
- CH1: ~1-2 Mbps
- CH2: ~0.5-1 Mbps

**Latency**:
- WebSocket: ~100ms (device to browser, includes network)
- Shared Memory: <1ms (local IPC)

## Migration Guide (v1.0.x → v1.1.0)

### WebSocket Clients

1. Add channel parameter to WebSocket URL:
   ```diff
   - ws://device-ip:8765/
   + ws://device-ip:8765/?channel=1
   ```

2. Extract channel ID from frame packet:
   ```javascript
   ws.onmessage = (event) => {
       const buffer = new Uint8Array(event.data);
       const channelId = buffer[0];  // NEW: First byte is channel ID
       const frameData = buffer.subarray(1, buffer.length - 8);  // Skip channel ID
       const timestamp = buffer.slice(-8);
       // ...
   };
   ```

### Shared Memory Consumers

1. Update API calls:
   ```diff
   - video_shm_consumer_init(&consumer);
   + video_shm_consumer_init_channel(&consumer, 1);  // Specify channel
   ```

2. Update shared memory paths in documentation/scripts:
   ```diff
   - /video_stream
   + /video_stream_ch0  (or ch1, ch2)
   ```

### Recommended Channel Selection

- **Recording/Archival**: CH0 (1920x1080@30fps)
- **Web Streaming**: CH1 (1280x720@30fps) - Default
- **AI/ML Processing**: CH2 (640x480@15fps) - Most efficient

## License

Same as parent project.

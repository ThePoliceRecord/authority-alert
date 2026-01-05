# WebSocket Video Streaming (camera-streamer Approach)

**Owner:** Authority Alert Team  
**Last Updated:** 2026-01-01  
**Status:** Production Ready Architecture

## Why WebSockets Over WebRTC

WebRTC is marketed as the solution for browser video, but in practice:

**WebRTC Problems:**
- Complex NAT traversal (STUN/TURN infrastructure)
- ICE negotiation failures (15-30% fail rate symmetric NAT)
- Requires signaling server + TURN relay servers
- Firewall issues, corporate networks block UDP
- Debugging is nightmare (chrome://webrtc-internals)
- Large library footprint (libwebrtc = 50MB+)
- Constant maintenance (API changes, codec support)

**WebSocket Streaming (camera-streamer approach):**
- ✅ Simple: One WebSocket connection, one relay server
- ✅ 100% NAT compatibility (outbound connection only)
- ✅ No ICE/STUN/TURN complexity
- ✅ Small footprint (<1MB code)
- ✅ Easy debugging (wireshark, browser devtools)
- ✅ Reliable: If WebSocket connects, streaming works
- ✅ Works on all corporate networks (HTTPS/WSS)

**Tradeoff:**
- Server bandwidth scales with viewers (WebRTC is P2P)
- But for <100 cameras, this is negligible (~$10-50/month)

If you have 10,000 cameras, use WebRTC. For everything else, use WebSockets.

---

## Architecture Overview

```
Camera (Behind NAT)              Relay Server (Public)           Browser/Mobile Client
     |                                   |                                |
     |                                   |                                |
     |--- WSS Connect (Camera ID) ------>|                                |
     |<-- Connected --------------------|                                |
     |                                   |                                |
     |                                   |<--- WSS Connect (Camera ID) ---|
     |                                   |------ Connected -------------->|
     |                                   |                                |
     |--- H.264 NAL Units (Binary) ----->|                                |
     |                                   |--- FMP4 Fragments (Binary) --->|
     |                                   |                                |
     |<-- Control Command (JSON) --------|<--- Control Command (JSON) ---|
     |--- Status Update (JSON) --------->|----- Status Update (JSON) ---->|
```

### Data Flow

1. **Camera connects** to relay server with camera ID
2. **Browser connects** to relay server, requests camera ID
3. **Relay subscribes** browser to camera's stream
4. **Camera streams** H.264 NAL units over WebSocket
5. **Relay** optionally repackages to FMP4 fragments
6. **Browser** receives FMP4, plays via MSE (Media Source Extensions)

---

## Implementation Details

### 1. Camera Side

#### A. H.264 Encoder Setup

```cpp
// Low-latency H.264 encoding parameters
struct H264EncoderConfig {
    int width = 1280;
    int height = 720;
    int fps = 30;
    int bitrate = 2000000; // 2 Mbps
    int gop_size = 60;     // 2 seconds @ 30fps
    
    // Low latency settings
    bool zero_latency = true;
    int bframes = 0;        // No B-frames
    bool cabac = false;     // Baseline profile
    int slices = 4;         // Slice threading
    
    // Tune for zerolatency
    const char* preset = "ultrafast";
    const char* tune = "zerolatency";
};
```

#### B. WebSocket Client

```cpp
#include <websocketpp/config/asio_client.hpp>
#include <websocketpp/client.hpp>

typedef websocketpp::client<websocketpp::config::asio_tls_client> client;

class CameraStreamer {
private:
    client ws_client_;
    client::connection_ptr connection_;
    std::string camera_id_;
    
public:
    void connect(const std::string& url, const std::string& camera_id) {
        camera_id_ = camera_id;
        
        ws_client_.init_asio();
        ws_client_.set_tls_init_handler([](websocketpp::connection_hdl) {
            return websocketpp::lib::make_shared<boost::asio::ssl::context>(
                boost::asio::ssl::context::tlsv12
            );
        });
        
        ws_client_.set_open_handler([this](websocketpp::connection_hdl hdl) {
            onConnected();
        });
        
        ws_client_.set_message_handler([this](websocketpp::connection_hdl hdl, client::message_ptr msg) {
            onMessage(msg->get_payload());
        });
        
        websocketpp::lib::error_code ec;
        connection_ = ws_client_.get_connection(url, ec);
        connection_->append_header("Camera-ID", camera_id_);
        
        ws_client_.connect(connection_);
        ws_client_.run();
    }
    
    void sendNALUnit(const uint8_t* data, size_t size, bool keyframe) {
        // Send binary NAL unit
        websocketpp::lib::error_code ec;
        ws_client_.send(connection_, data, size, 
                       websocketpp::frame::opcode::binary, ec);
        
        if (ec) {
            std::cerr << "Send failed: " << ec.message() << std::endl;
        }
    }
    
private:
    void onConnected() {
        std::cout << "Connected to relay server" << std::endl;
        // Start encoding and sending
        startCapture();
    }
    
    void onMessage(const std::string& msg) {
        // Handle control commands from server
        nl::json j = nl::json::parse(msg);
        if (j["type"] == "command") {
            handleCommand(j["command"]);
        }
    }
};
```

#### C. Capture and Stream Loop

```cpp
void CameraStreamer::startCapture() {
    // Initialize hardware encoder
    initEncoder();
    
    // Streaming loop
    while (running_) {
        // Get encoded frame from hardware encoder
        EncodedFrame frame = encoder_->getNextFrame();
        
        // Send NAL units
        for (const auto& nal : frame.nal_units) {
            bool is_keyframe = (nal.type == NAL_IDR || nal.type == NAL_SPS);
            sendNALUnit(nal.data, nal.size, is_keyframe);
        }
        
        // Adaptive bitrate based on network conditions
        if (shouldAdjustBitrate()) {
            adjustEncoderBitrate();
        }
    }
}
```

### 2. Relay Server

#### A. Connection Management

```javascript
// Node.js WebSocket relay server
const WebSocket = require('ws');
const server = new WebSocket.Server({ port: 8443 });

const cameras = new Map(); // camera_id -> WebSocket
const viewers =  new Map(); // viewer_id -> {ws, camera_ids: Set}

server.on('connection', (ws, req) => {
    const cameraId = req.headers['camera-id'];
    const viewerId = req.headers['viewer-id'];
    
    if (cameraId) {
        // Camera connection
        cameras.set(cameraId, ws);
        console.log(`Camera ${cameraId} connected`);
        
        ws.on('message', (data) => {
            // Forward binary data to all viewers
            relayToViewers(cameraId, data);
        });
        
        ws.on('close', () => {
            cameras.delete(cameraId);
            notifyViewers(cameraId, { type: 'camera_disconnected' });
});
    } else if (viewerId) {
        // Viewer connection
        viewers.set(viewerId, { ws, camera_ids: new Set() });
        
        ws.on('message', (msg) => {
            const data = JSON.parse(msg);
            
            if (data.type === 'subscribe') {
                subscribeViewer(viewerId, data.camera_id);
            } else if (data.type === 'command') {
                forwardCommand(data.camera_id, data.command);
            }
        });
        
        ws.on('close', () => {
            viewers.delete(viewerId);
        });
    }
});

function relayToViewers(cameraId, data) {
    for (const [viewerId, viewer] of viewers) {
        if (viewer.camera_ids.has(cameraId)) {
            viewer.ws.send(data, { binary: true });
        }
    }
}

function subscribeViewer(viewerId, cameraId) {
    const viewer = viewers.get(viewerId);
    if (viewer) {
        viewer.camera_ids.add(cameraId);
        
        // Send initial config (SPS/PPS if cached)
        const camera = cameras.get(cameraId);
        if (camera && camera.spsNalu && camera.ppsNalu) {
            viewer.ws.send(camera.spsNalu, { binary: true });
            viewer.ws.send(camera.ppsNalu, { binary: true });
        }
    }
}
```

#### B. Optional FMP4 Repackaging

```javascript
// Convert H.264 NAL units to FMP4 fragments for MSE
const MP4 = require('mp4box');

class FMP4Muxer {
    constructor() {
        this.mp4file = MP4.createFile();
        this.trackId = null;
        this.sequenceNumber = 0;
    }
    
    addNAL(nalData, timestamp, isKeyframe) {
        if (!this.trackId) {
            // Extract SPS/PPS and create track
            this.initializeTrack(nalData);
        }
        
        // Create sample
        const sample = {
            number: this.sequenceNumber++,
            track_id: this.trackId,
            timescale: 90000,
            description_index: 0,
            duration: 3000, // 33ms @ 90kHz for 30fps
            cts: timestamp,
            dts: timestamp,
            is_sync: isKeyframe,
            data: nalData
        };
        
        this.mp4file.addSample(this.trackId, sample);
        
        // Get FMP4 fragment
        return this.mp4file.getBuffer();
    }
    
    initializeTrack(spsNalu) {
        const track = {
            type: 'video',
            timescale: 90000,
            duration: 0,
            width: 1280,
            height: 720,
            sps: [spsNalu],
            pps: []
        };
        
        this.trackId = this.mp4file.addTrack(track);
        this.mp4file.initializeSegmentation();
    }
}
```

### 3. Browser Client

#### A. WebSocket Connection

```javascript
class VideoPlayer {
    constructor(cameraId, serverUrl) {
        this.cameraId = cameraId;
        this.ws = null;
        this.mediaSource = null;
        this.sourceBuffer = null;
        this.queue = [];
        
        this.connect(serverUrl);
    }
    
    connect(url) {
        this.ws = new WebSocket(url, {
            headers: { 'Viewer-ID': this.generateViewerId() }
        });
        
        this.ws.binaryType = 'arraybuffer';
        
        this.ws.onopen = () => {
            console.log('Connected to relay server');
            // Subscribe to camera
            this.ws.send(JSON.stringify({
                type: 'subscribe',
                camera_id: this.cameraId
            }));
            
            this.initMediaSource();
        };
        
        this.ws.onmessage = (event) => {
            if (event.data instanceof ArrayBuffer) {
                // Binary FMP4 fragment
                this.appendBuffer(event.data);
            } else {
                // JSON control message
                const msg = JSON.parse(event.data);
                this.handleMessage(msg);
            }
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
        
        this.ws.onclose = () => {
            console.log('Disconnected, reconnecting...');
            setTimeout(() => this.connect(url), 2000);
        };
    }
    
    initMediaSource() {
        const video = document.getElementById('videoPlayer');
        this.mediaSource = new MediaSource();
        video.src = URL.createObjectURL(this.mediaSource);
        
        this.mediaSource.addEventListener('sourceopen', () => {
            this.sourceBuffer = this.mediaSource.addSourceBuffer(
                'video/mp4; codecs="avc1.42E01E"' // H.264 Baseline
            );
            
            this.sourceBuffer.addEventListener('updateend', () => {
                if (this.queue.length > 0 && !this.sourceBuffer.updating) {
                    this.sourceBuffer.appendBuffer(this.queue.shift());
                }
            });
        });
    }
    
    appendBuffer(data) {
        if (this.sourceBuffer.updating || this.queue.length > 0) {
            // Queue if currently updating
            this.queue.push(data);
        } else {
            try {
                this.sourceBuffer.appendBuffer(data);
            } catch (e) {
                console.error('Append buffer failed:', e);
                this.queue.push(data);
            }
        }
        
        // Remove old buffered data to prevent memory bloat
        if (this.sourceBuffer.buffered.length > 0) {
            const currentTime = document.getElementById('videoPlayer').currentTime;
            const start = this.sourceBuffer.buffered.start(0);
            
            if (currentTime - start > 30) { // Keep only last 30s
                this.sourceBuffer.remove(start, currentTime - 30);
            }
        }
    }
    
    sendCommand(command) {
        this.ws.send(JSON.stringify({
            type: 'command',
            camera_id: this.cameraId,
            command: command
        }));
    }
}

// Usage
const player = new VideoPlayer('camera_12345', 'wss://relay.example.com');
```

#### B. HTML Integration

```html
<!DOCTYPE html>
<html>
<head>
    <title>Camera Viewer</title>
    <style>
        #videoPlayer {
            width: 100%;
            maxwidth: 1280px;
            height: auto;
        }
        .controls {
            margin-top: 10px;
        }
    </style>
</head>
<body>
    <video id="videoPlayer" autoplay muted playsinline></video>
    
    <div class="controls">
        <button onclick="player.sendCommand('snapshot')">Take Snapshot</button>
        <button onclick="player.sendCommand('start_recording')">Start Recording</button>
        <button onclick="player.sendCommand('stop_recording')">Stop Recording</button>
        <span id="status">Connecting...</span>
    </div>
    
    <script src="video-player.js"></script>
    <script>
        const cameraId = new URLParams(window.location.search).get('camera');
        const player = new VideoPlayer(cameraId || 'camera_12345', 
                                       'wss://relay.example.com');
    </script>
</body>
</html>
```

### 4. Mobile Apps

#### iOS (Swift)

```swift
import UIKit
import AVFoundation

class CameraViewController: UIViewController {
    private var webSocket: URLSessionWebSocketTask?
    private var videoLayer: AVSampleBufferDisplayLayer!
    private var decompressor: VTDecompressionSession?
    
    func connect(cameraId: String) {
        let url = URL(string: "wss://relay.example.com")!
        var request = URLRequest(url: url)
        request.addValue(UUID().uuidString, forHTTPHeaderField: "Viewer-ID")
        
        webSocket = URLSession.shared.webSocketTask(with: request)
        webSocket?.resume()
        
        // Subscribe to camera
        let subscribe = ["type": "subscribe", "camera_id": cameraId]
        let data = try! JSONSerialization.data(withJSONObject: subscribe)
        webSocket?.send(.string(String(data: data, encoding: .utf8)!)) { error in
            if let error = error {
                print("Send error: \(error)")
            }
        }
        
        receiveMessage()
    }
    
    func receiveMessage() {
        webSocket?.receive { [weak self] result in
            switch result {
            case .success(let message):
                switch message {
                case .data(let data):
                    self?.decodeNALUnit(data)
                case .string(let text):
                    self?.handleControlMessage(text)
                @unknown default:
                    break
                }
                self?.receiveMessage() // Continue receiving
                
            case .failure(let error):
                print("Receive error: \(error)")
            }
        }
    }
    
    func decodeNALUnit(_ data: Data) {
        // Decode H.264 NAL unit using VideoToolbox
        // ... implementation
    }
}
```

#### Android (Kotlin)

```kotlin
class CameraActivity : AppCompatActivity() {
    private lateinit var webSocket: WebSocket
    private lateinit var videoView: SurfaceView
    private lateinit var mediaCodec: MediaCodec
    
    fun connect(cameraId: String) {
        val request = Request.Builder()
            .url("wss://relay.example.com")
            .addHeader("Viewer-ID", UUID.randomUUID().toString())
            .build()
        
        webSocket = OkHttpClient().newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                // Subscribe to camera
                val subscribe = JSONObject()
                    .put("type", "subscribe")
                    .put("camera_id", cameraId)
                webSocket.send(subscribe.toString())
                
                initMediaCodec()
            }
            
            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                // H.264 NAL unit
                decodeFrame(bytes.toByteArray())
            }
            
            override fun onMessage(webSocket: WebSocket, text: String) {
                // Control message
                handleControlMessage(text)
            }
        })
    }
    
    fun initMediaCodec() {
        mediaCodec = MediaCodec.createDecoderByType(MediaFormat.MIMETYPE_VIDEO_AVC)
        val format = MediaFormat.createVideoFormat(
            MediaFormat.MIMETYPE_VIDEO_AVC, 1280, 720
        )
        mediaCodec.configure(format, videoView.holder.surface, null, 0)
        mediaCodec.start()
    }
    
    fun decodeFrame(data: ByteArray) {
        val inputIndex = mediaCodec.dequeueInputBuffer(10000)
        if (inputIndex >= 0) {
            val buffer = mediaCodec.getInputBuffer(inputIndex)
            buffer?.clear()
            buffer?.put(data)
            mediaCodec.queueInputBuffer(inputIndex, 0, data.size, 
                System.nanoTime() / 1000, 0)
        }
    }
}
```

---

## Why This Works Behind NAT

**Outbound WebSocket Connection:**
- Camera initiates HTTPS/WSS connection to relay server
- NAT allows outbound connections (creates NAT mapping)
- Return traffic uses established connection
- No inbound ports or port forwarding needed

**Server Relay:**
- All traffic flows through relay server
- Both camera and viewer use outbound connections
- Server has public IP, handles all routing
- Works through any firewall that allows HTTPS (443)

---

## Deployment

### Small Scale (Docker Compose)

```yaml
version: '3.8'

services:
  relay:
    image: node:18
    volumes:
      - ./relay-server:/app
    working_dir: /app
    command: node server.js
    ports:
      - "8443:8443"
    environment:
      - NODE_ENV=production
  
  nginx:
    image: nginx:alpine
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    ports:
      - "443:443"
    depends_on:
      - relay
```

### Production (Kubernetes)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: video-relay
spec:
  replicas: 3
  selector:
    matchLabels:
      app: relay
  template:
    metadata:
      labels:
        app: relay
    spec:
      containers:
      - name: relay
        image: video-relay:latest
        ports:
        - containerPort: 8443
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
---
apiVersion: v1
kind: Service
metadata:
  name: relay-service
spec:
  type: LoadBalancer
  ports:
  - port: 443
    targetPort: 8443
  selector:
    app: relay
```

---

## Bandwidth & Cost

### Per Camera

| Resolution | Bitrate | Bandwidth/Month | Server Cost* |
|------------|---------|-----------------|--------------|
| 640x480 @ 15fps | 500 Kbps | 160 GB | ~$2/camera |
| 1280x720 @ 30fps | 1.5 Mbps | 485 GB | ~$6/camera |
| 1920x1080 @ 30fps | 3 Mbps | 970 GB | ~$12/camera |

*Based on AWS/GCP egress pricing (~$0.12/GB)

### Cost Optimization

1. **Regional deployment:** Reduce egress by serving from nearest region
2. **Adaptive bitrate:** Lower quality when bandwidth constrained
3. **CDN caching:** Use Cloudflare/CloudFront for popular streams
4. **Viewer limits:** Cap concurrent viewers per camera (e.g. 10)

---

## Security

### Transport Security
- **TLS/WSS only:** All connections encrypted
- **Certificate validation:** Cameras validate server cert
- **No plaintext:** Never use ws://, always wss://

### Authentication
```javascript
// Camera registration token
const cameraToken = jwt.sign(
    { camera_id: 'camera_12345', role: 'camera' },
    SECRET_KEY,
    { expiresIn: '30d' }
);

// Viewer session token
const viewerToken = jwt.sign(
    { user_id: 'user_abc', allowed_cameras: ['camera_12345'], role: 'viewer' },
    SECRET_KEY,
    { expiresIn: '1h' }
);

// Server validates tokens
server.on('connection', (ws, req) => {
    const token = req.headers['authorization']?.replace('Bearer ', '');
    const decoded = jwt.verify(token, SECRET_KEY);
    
    if (decoded.role === 'camera') {
        handleCameraConnection(ws, decoded.camera_id);
    } else if (decoded.role === 'viewer') {
        handleViewerConnection(ws, decoded.user_id, decoded.allowed_cameras);
    } else {
        ws.close(1008, 'Invalid role');
    }
});
```

---

## Troubleshooting

### Common Issues

**1. Video not playing**
- Check browser console for MSE errors
- Verify FMP4 fragments are valid (use mp4box.js validator)
- Ensure codec string matches encoder output
- Check if SPS/PPS NAL units were sent first

**2. Lag/buffering**
- Reduce encoder bitrate
- Check server CPU usage (transcoding overhead)
- Monitor network latency (ping relay server)
- Implement buffer trimming (remove old segments)

**3. Connection drops**
- Add WebSocket ping/pong heartbeat
- Implement exponential backoff reconnection
- Check firewall timeout settings (increase if needed)
- Monitor relay server load

### Debug Tools

**Browser:**
```javascript
// Monitor MSE buffer
const video = document.getElementById('videoPlayer');
const sb = video.sourceBuffer;

setInterval(() => {
    console.log('Buffered:', sb.buffered.length,
                'Current time:', video.currentTime);
}, 1000);
```

**Wireshark:**
- Filter: `tcp.port == 8443`
- See WebSocket frames in clear text (before encryption)
- Verify NAL unit sizes and timing

---

## Comparison: WebSocket vs WebRTC

| Feature | WebSocket | WebRTC |
|---------|-----------|--------|
| **Setup Complexity** | ⭐ Simple | ⭐⭐⭐⭐⭐ Complex |
| **NAT Success Rate** | 100% | 85-95% |
| **Infrastructure** | 1 relay server | Signaling + STUN + TURN |
| **Latency** | 0.5-2s | 0.2-1s (P2P) |
| **Bandwidth Cost** | Linear with viewers | Zero (P2P) |
| **Corporate Firewall** | ✅ Works everywhere | ❌ Often blocked |
| **Debugging** | ✅ Easy | ❌ Nightmare |
| **Mobile Battery** | ✅ Efficient | ❌ Higher drain |
| **Code Footprint** | <1MB | 50MB+ |

**Verdict:** Use WebSocket unless you have 1000+ cameras.

---

## References

- [camera-streamer project](https://github.com/ayufan/camera-streamer)
- [Media Source Extensions API](https://developer.mozilla.org/en-US/docs/Web/API/Media_Source_Extensions_API)
- [FMP4 Format](https://www.w3.org/TR/mse-byte-stream-format-isobmff/)
- [H.264 Annex B format](https://en.wikipedia.org/wiki/Network_Abstraction_Layer)

---

**Document Version:** 1.0  
**Next Review:** 2026-03-01

# WebRTC Video Streaming from Camera Behind NAT

**Owner:** Authority Alert Team  
**Last Updated:** 2026-01-01  
**Status:** Architecture Reference

## Executive Summary

This document describes the architecture for streaming video from security cameras behind NAT (Network Address Translation) to browser and mobile applications using WebRTC. The solution provides peer-to-peer connectivity when possible, with TURN relay fallback for restrictive NAT environments.

**Key Benefits:**
- **Works Behind NAT:** No port forwarding required on camera side
- **Low Latency:** Sub-second latency with P2P connections
- **Encrypted:** End-to-end encryption via DTLS-SRTP
- **Adaptive:** Automatic bitrate adaptation based on network conditions
- **Universal Compatibility:** Works in browsers and native mobile apps

---

## System Overview

###  Architecture Diagram

```
Camera (Behind NAT)          Signaling Server (Public)       STUN/TURN Server       Client (Browser/Mobile)
      |                              |                              |                          |
      |                              |                              |                          |
      |--- WebSocket Connect ------->|                              |                          |
      |<-- Connected ---------------|                              |                          |
      |                              |                              |                          |
      |                              |<---- WebSocket Connect ------|                          |
      |                              |------- Connected ----------->|                          |
      |                              |                              |                          |
      |<---- SDP Offer -------------|<----- Send Offer ------------|                          |
      |                              |                              |                          |
      |-- STUN Request ------------- | ----------------------------->|                          |
      |<-- STUN Response ----------- | -----------------------------|                          |
      |                              |                              |                          |
      |----- SDP Answer ------------>|-------- Answer ------------->|                          |
      |                              |                              |                          |
      |<---- ICE Candidates ---------|<----- ICE Candidates --------|                          |
      |------ ICE Candidates ------->|------- ICE Candidates ------>|                          |
      |                              |                              |                          |
      |================================ P2P Connection (if possible) ===============================|
      |                              |                              |                          |
      OR (if P2P fails):             |                              |                          |
      |----------------------- TURN Relay ----------------------------->|                      |
```

### Component Roles

| Component | Responsibility | Public IP Required |
|-----------|---------------|-------------------|
| **Camera** | Video source, WebRTC peer | No |
| **Signaling Server** | Message relay for connection setup | Yes |
| **STUN Server** | NAT type discovery, public IP detection | Yes |
| **TURN Server** | Media relay when P2P fails | Yes |
| **Browser/Mobile Client** | VideoWebSocket playback, user interface | No |

---

## NAT Traversal Strategy

### NAT Types and Success Rates

| Camera NAT | Client NAT | Connection Type | Success Rate | Comments |
|------------|------------|----------------|--------------|----------|
| Full Cone | Any | Direct P2P | >99% | Best case |
| Restricted Cone | Port Restricted | Direct P2P | >95% | STUN sufficient |
| Port Restricted | Symmetric | Direct P2P or TURN | >90% | May need TURN |
| Symmetric | Symmetric | TURN Relay | >85% | TURN required |

### ICE (Interactive Connectivity Establishment)

WebRTC uses ICE to find the best path for media:

1. **Gather Candidates:**
   - Host candidates (local IP addresses)
   - Server reflexive candidates (public IP via STUN)
   - Relay candidates (TURN server addresses)

2. **Exchange Candidates:**
   - Camera and client exchange candidates via signaling
   - All gathered candidates sent to peer

3. **Connectivity Checks:**
   - Both sides test all candidate pairs
   - Find the lowest-latency working path
   - Prefer P2P over relay

4. **Select Best Path:**
   - P2P chosen when available
   - Relay used as fallback
   - Connection established

### Why This Works Behind NAT

**Outbound Connections:**
- NAT allows outbound connections by default
- Camera initiates WebSocket connection to signaling server
- Camera sends ICE candidates through existing connection
- No inbound ports need to be opened on camera network

**STUN Discovery:**
- Camera sends STUN request to public STUN server
- STUN server responds with camera's public IP and port
- This creates NAT binding that can receive return traffic

**TURN Fallback:**
- When P2P fails, media routed through TURN server
- Both camera and client connect to TURN server
- TURN relays packets between them
- Works even with symmetric NAT on both sides

---

## Component Details

### 1. Camera Implementation

#### WebRTC Stack Options

**Option A: libdatachannel (Recommended for embedded)**
```cpp
// Lightweight, modern C++ WebRTC implementation
#include <rtc/rtc.hpp>

rtc::Configuration config;
config.iceServers.emplace_back("stun:stun.l.google.com:19302");
config.iceServers.emplace_back("turn:turn.example.com:3478", "username", "credential");

auto pc = std::make_shared<rtc::PeerConnection>(config);
```

**Pros:**
- Small footprint (~500KB binary)
- No Google dependencies
- Modern C++17 code
- Active development

**Cons:**
- Less mature than libwebrtc
- Fewer platform-specific optimizations

**Option B: Pion WebRTC (Go)**
```go
// Pure Go implementation, great for gateway devices
import "github.com/pion/webrtc/v3"

config := webrtc.Configuration{
    ICEServers: []webrtc.ICEServer{
        {URLs: []string{"stun:stun.l.google.com:19302"}},
        {URLs: []string{"turn:turn.example.com:3478"}},
    },
}

peerConnection, err := webrtc.NewPeerConnection(config)
```

**Pros:**
- Pure Go, easy cross-compilation
- Excellent documentation
- Built-in utilities

**Cons:**
- Larger memory footprint
- Go runtime overhead

#### Video Pipeline

```
Camera Sensor → H.264 Encoder → RTP Packetizer → WebRTC → Network
```

**H.264 Encoding Parameters:**
- **Profile:** Baseline (max compatibility) or Main
- **Level:** 3.1 or 4.0
- **Bitrate:** 1-2 Mbps (adaptive)
- **Frame Rate:** 15-30 fps
- **Resolution:** 640x480 to 1920x1080
- **GOP Size:** 2 seconds (60 frames @ 30fps)
- **Low Latency:** tune=zerolatency

**Example libdatachannel Integration:**
```cpp
class CameraStreamer {
private:
    std::shared_ptr<rtc::PeerConnection> pc;
    std::shared_ptr<rtc::Track> videoTrack;
    
public:
    void addVideoTrack() {
        rtc::Description::Video video("video", rtc::Description::Direction::SendOnly);
        video.addH264Codec(96); // Payload type 96
        video.setBitrate(2000); // 2 Mbps
        
        videoTrack = pc->addTrack(video);
        
        // Send RTP packets
        videoTrack->onOpen([this]() {
            startCapture();
        });
    }
    
    void sendFrame(const uint8_t* data, size_t size, uint32_t timestamp) {
        // RTP timestamp: 90kHz clock
        videoTrack->send(reinterpret_cast<const std::byte*>(data), size);
    }
};
```

#### Signaling Client

**WebSocket Connection:**
```cpp
// Using websocketpp or similar
class SignalingClient {
    void connect(const std::string& url) {
        ws_client.connect(url);
        ws_client.set_message_handler([this](auto msg) {
            handleSignalingMessage(msg);
        });
    }
    
    void sendOffer(const std::string& sdp) {
        json msg = {
            {"type", "offer"},
            {"sdp", sdp},
            {"camera_id", camera_id_}
        };
        ws_client.send(msg.dump());
    }
};
```

### 2. Signaling Server

#### Purpose

The signaling server exchanges WebRTC setup messages between camera and clients:
- SDP offers and answers
- ICE candidates
- Connection state updates
- Does NOT relay media (media goes direct or via TURN)

#### Technology Options

**Option A: Node.js with Socket.io**
```javascript
const io = require('socket.io')(server);

io.on('connection', (socket) => {
    socket.on('join-room', (roomId) => {
        socket.join(roomId);
    });
    
    socket.on('offer', (data) => {
        socket.to(data.roomId).emit('offer', data);
    });
    
    socket.on('answer', (data) => {
        socket.to(data.roomId).emit('answer', data);
    });
    
    socket.on('ice-candidate', (data) => {
        socket.to(data.roomId).emit('ice-candidate', data);
    });
});
```

**Option B: Go with Gorilla WebSocket**
```go
type SignalingServer struct {
    clients map[string]*websocket.Conn
    rooms   map[string][]string
}

func (s *SignalingServer) handleMessage(conn *websocket.Conn, msg Message) {
    switch msg.Type {
    case "offer":
        s.relayToRoom(msg.RoomID, msg)
    case "answer":
        s.relayToClient(msg.TargetID, msg)
    case "ice-candidate":
        s.relayToRoom(msg.RoomID, msg)
    }
}
```

#### Message Protocol

**Room-Based Topology:**
- One room per camera (identified by camera serial/ID)
- Camera creates room when connecting
- Clients join room to view camera
- Messages broadcast within room

**Message Types:**
```json
{
  "type": "offer",
  "room_id": "camera_12345",
  "sdp": "v=0\r\no=- ...",
  "sender_id": "client_abc"
}

{
  "type": "answer",
  "room_id": "camera_12345",
  "sdp": "v=0\r\no=- ...",
  "sender_id": "camera_12345"
}

{
  "type": "ice-candidate",
  "room_id": "camera_12345",
  "candidate": "candidate:1 1 UDP ...",
  "sender_id": "client_abc"
}
```

### 3. STUN/TURN Server

#### Deployment

**coturn (Recommended)**
```bash
# Install
apt-get install coturn

# Configure /etc/turnserver.conf
listening-port=3478
tls-listening-port=5349
listening-ip=0.0.0.0
external-ip=YOUR_PUBLIC_IP

# Authentication
lt-cred-mech
use-auth-secret
static-auth-secret=YOUR_SECRET_KEY

# Limits
user-quota=12
total-quota=1200
max-bps=1000000

# Security
fingerprint
no-multicast-peers
no-loopback-peers
```

#### Time-Limited Credentials

Generate temporary credentials for each connection:

```javascript
// Server-side (Node.js example)
const crypto = require('crypto');

function generateTurnCredentials(username, secret, ttl = 3600) {
    const timestamp = Math.floor(Date.now() / 1000) + ttl;
    const turnUsername = `${timestamp}:${username}`;
    
    const hmac = crypto.createHmac('sha1', secret);
    hmac.update(turnUsername);
    const turnPassword = hmac.digest('base64');
    
    return {
        urls: [
            'stun:turn.example.com:3478',
            'turn:turn.example.com:3478?transport=udp',
            'turn:turn.example.com:3478?transport=tcp',
            'turns:turn.example.com:5349?transport=tcp'
        ],
        username: turnUsername,
        credential: turnPassword
    };
}
```

### 4. Browser Client

```html
<!DOCTYPE html>
<html>
<head>
    <title>Camera Viewer</title>
</head>
<body>
    <video id="remoteVideo" autoplay playsinline></video>
    <script>
        const cameraId = 'camera_12345';
        const signalingUrl = 'wss://signaling.example.com';
        
        const ws = new WebSocket(signalingUrl);
        const configuration = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                {
                    urls: 'turn:turn.example.com:3478',
                    username: '1735689600:user123',
                    credential: 'generated_password'
                }
            ]
        };
        
        const pc = new RTCPeerConnection(configuration);
        const remoteVideo = document.getElementById('remoteVideo');
        
        // Handle incoming tracks
        pc.ontrack = (event) => {
            remoteVideo.srcObject = event.streams[0];
        };
        
        // Handle ICE candidates
        pc.onicecandidate = (event) => {
            if (event.candidate) {
                ws.send(JSON.stringify({
                    type: 'ice-candidate',
                    room_id: cameraId,
                    candidate: event.candidate
                }));
            }
        };
        
        // Join room and request stream
        ws.on open = () => {
            ws.send(JSON.stringify({
                type: 'join',
                room_id: cameraId
            }));
        };
        
        // Handle signaling messages
        ws.onmessage = async (event) => {
            const msg = JSON.parse(event.data);
            
            switch (msg.type) {
                case 'offer':
                    await pc.setRemoteDescription(msg.sdp);
                    const answer = await pc.createAnswer();
                    await pc.setLocalDescription(answer);
                    ws.send(JSON.stringify({
                        type: 'answer',
                        room_id: cameraId,
                        sdp: answer
                    }));
                    break;
                    
                case 'ice-candidate':
                    await pc.addIceCandidate(msg.candidate);
                    break;
            }
        };
    </script>
</body>
</html>
```

### 5. Mobile Apps

#### iOS (Swift + WebRTC)

```swift
import WebRTC

class CameraStreamViewController: UIViewController {
    private var peerConnection: RTCPeerConnection?
    private var videoCapturer: RTCVideoCapturer?
    private var videoSource: RTCVideoSource?
    private var remoteVideoTrack: RTCVideoTrack?
    private var remoteVideoView: RTCMTLVideoView!
    
    func setupWebRTC() {
        let config = RTCConfiguration()
        config.iceServers = [
            RTCIceServer(urlStrings: ["stun:stun.l.google.com:19302"]),
            RTCIceServer(urlStrings: ["turn:turn.example.com:3478"],
                        username: "username",
                        credential: "password")
        ]
        
        let constraints = RTCMediaConstraints(mandatoryConstraints: nil,
                                             optionalConstraints: nil)
        peerConnection = peerConnectionFactory.peerConnection(with: config,
                                                              constraints: constraints,
                                                              delegate: self)
    }
}

extension CameraStreamViewController: RTCPeerConnectionDelegate {
    func peerConnection(_ peerConnection: RTCPeerConnection, didAdd stream: RTCMediaStream) {
        if let videoTrack = stream.videoTracks.first {
            remoteVideoTrack = videoTrack
            videoTrack.add(remoteVideoView)
        }
    }
    
    func peerConnection(_ peerConnection: RTCPeerConnection, didGenerate candidate: RTCIceCandidate) {
        // Send candidate via signaling
        signalingClient.send(candidate: candidate)
    }
}
```

#### Android (Kotlin + WebRTC)

```kotlin
class CameraStreamActivity : AppCompatActivity() {
    private var peerConnection: PeerConnection? = null
    private lateinit var videoTrack: VideoTrack
    private lateinit var surfaceViewRenderer: SurfaceViewRenderer
    
    fun setupWebRTC() {
        val iceServers = listOf(
            PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").create(),
            PeerConnection.IceServer.builder("turn:turn.example.com:3478")
                .setUsername("username")
                .setPassword("password")
                .create()
        )
        
        val rtcConfig = PeerConnection.RTCConfiguration(iceServers)
        peerConnection = peerConnectionFactory.createPeerConnection(
            rtcConfig,
            object : PeerConnection.Observer {
                override fun onAddStream(mediaStream: MediaStream?) {
                    mediaStream?.videoTracks?.firstOrNull()?.let { remoteVideoTrack ->
                        remoteVideoTrack.addSink(surfaceViewRenderer)
                    }
                }
                
                override fun onIceCandidate(iceCandidate: IceCandidate?) {
                    iceCandidate?.let {
                        signalingClient.sendIceCandidate(it)
                    }
                }
            }
        )
    }
}
```

---

## Security Considerations

### Transport Security

**TLS/DTLS Everywhere:**
- Signaling: WSS (WebSocket Secure)
- STUN: Can use plain UDP (only discovers public IP)
- TURN: Use TURNS (TLS) when possible
- Media: DTLS-SRTP (automatic in WebRTC)

### Authentication

**Camera Authentication:**
- Camera uses device certificate or pre-shared key
- Signaling server validates before allowing room creation
- JWT tokens with short expiration

**Client Authentication:**
- User authenticates with backend system
- Backend issues time-limited tokens for signaling access
- Token includes allowed camera IDs

**TURN Authentication:**
- Time-limited credentials (1 hour typical)
- HMAC-SHA1 signature prevents credential reuse
- Separate credentials per session

### Network Security

**Firewall Requirements:**

*Camera Network:*
- Outbound HTTPS/WSS (443)  to signaling server
- Outbound UDP (3478) to STUN/TURN server
- Outbound UDP (high ports) for P2P media
- No inbound ports required

*Signaling Server:*
- Inbound HTTPS/WSS (443)
- Optional: Separate port for API

*TURN Server:*
- Inbound UDP/TCP (3478) - TURN
- Inbound TCP (5349) - TURNS
- Inbound UDP (49152-65535) - Media relay ports

---

## Deployment Guide

### Small Scale (1-10 cameras)

**Single Server Setup:**
```
Server: 2 CPU, 4GB RAM, 20GB SSD
- Signaling server (Node.js/Go)
- coturn (STUN/TURN)
- Nginx (reverse proxy)
- Let's Encrypt SSL
Cost: ~$10-20/month (VPS)
```

**Setup Steps:**
1. Provision cloud VM with public IP
2. Install coturn, configure with secret
3. Deploy signaling server
4. Configure Nginx with SSL
5. Update camera configs with server URL
6. Test with browser client

### Medium Scale (10-100 cameras)

**Distributed Setup:**
```
Load Balancer (1) - Nginx or HAProxy
Signaling Servers (2+) - Horizontally scaled
TURN Servers (2+) - Geographically distributed
Database/Redis - Session state (if needed)
Cost: ~$100-300/month
```

### Large Scale (100+ cameras)

**Managed Infrastructure:**
```
Kubernetes cluster
Auto-scaling signaling pods
Managed TURN service (Twilio, Cloudflare)
CDN for static assets
Monitoring (Prometheus/Grafana)
Cost: Variable, ~$500+/month
```

---

## Bandwidth and Cost Considerations

### Bandwidth per Stream

| Resolution | Bitrate | Monthly (24/7) | TURN Relay Cost* |
|------------|---------|----------------|------------------|
| 640x480 @ 15fps | 500 Kbps | ~160 GB | ~$2-10/camera |
| 1280x720 @ 30fps | 1.5 Mbps | ~485 GB | ~$6-30/camera |
| 1920x1080 @ 30fps | 3 Mbps | ~970 GB | ~$12-60/camera |

*When P2P fails and TURN relay is used

### Cost Optimization

**Reduce TURN Usage:**
- Optimize camera NAT (enable UPnP if safe)
- Deploy TURN servers in camera-friendly locations
- Use adaptive bitrate to reduce bandwidth
- Encourage clients to view on same network

**P2P Success Rate Improvement:**
- Enable UPnP/NAT-PMP on camera network (if security allows)
- Use multiple STUN servers
- Implement ICE-TCP as fallback before TURN

---

## Troubleshooting

### Common Issues

**1. No Video Appears**
- Check WebRTC connection state in browser console
- Verify ICE candidates are being exchanged
- Check camera encoder is running
- Verify firewall allows outbound connections

**2. High Latency**
- Check if TURN relay is being used (indicates P2P failure)
- Verify encoder settings (use low-latency tuning)
- Check network congestion
- Reduce resolution/bitrate

**3. Connection Fails**
- Verify signaling server is reachable
- Check TURN credentials are valid
- Verify coturn is running and accessible
- Check symmetric NAT on both sides

### Debug Tools

**Browser DevTools:**
```javascript
// View connection state
pc.onconnectionstatechange = () => {
    console.log('Connection state:', pc.connectionState);
};

// View ICE state
pc.oniceconnectionstatechange = () => {
    console.log('ICE state:', pc.iceConnectionState);
};

// Get statistics
pc.getStats().then(stats => {
    stats.forEach(report => {
        console.log(report);
    });
});
```

**chrome://webrtc-internals:**
- Real-time stats and graphs
- ICE candidate pairs
- Codec information
- Bandwidth usage

---

## References

- [WebRTC Specification](https://www.w3.org/TR/webrtc/)
- [ICE RFC 8445](https://tools.ietf.org/html/rfc8445)
- [STUN RFC 5389](https://tools.ietf.org/html/rfc5389)
- [TURN RFC 5766](https://tools.ietf.org/html/rfc5766)
- [libdatachannel](https://github.com/paullouisageneau/libdatachannel)
- [Pion WebRTC](https://github.com/pion/webrtc)
- [coturn](https://github.com/coturn/coturn)

---

**Document Version:** 1.0  
**Next Review:** 2026-03-01

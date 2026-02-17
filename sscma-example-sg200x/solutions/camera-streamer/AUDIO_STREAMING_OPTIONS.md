# Audio Streaming Options for reCamera

This document outlines the available options for adding audio streaming to the reCamera platform.

## Available Hardware & SDK

### Sophgo Audio SDK
Located at: `components/sophgo/audio/`

**Capabilities:**
- I2S audio input from internal codec
- Hardware-accelerated audio encoding
- Voice Quality Enhancement (VQE) with AGC, ANR, AEC
- Multi-codec support

**Supported Codecs:**
| Codec | Bitrate | Sample Rates | Use Case |
|-------|---------|--------------|----------|
| AAC | 32kbps | 8/16/32/48kHz | Best quality/size ratio, browser native |
| G.711 A/U | 64kbps | 8kHz only | VoIP, high compatibility |
| G.726 | 16/24/32/40kbps | 8/16kHz | Telephony, low bandwidth |
| ADPCM | Variable | 8/16kHz | Simple compression |
| PCM (raw) | 128-768kbps | Any | No encoding, high bandwidth |

## Architecture Options

### Option 1: Separate audio-streamer Service (Recommended)

**Architecture:**
```
Microphone → Sophgo Audio SDK → AAC Encoder → Queue → WebSocket (port 8766)
                                                           ↓
                                              Supervisor Proxy (WSS)
                                                           ↓
                                              Browser Web Audio API
```

**Pros:**
- ✅ Modular - independent of video
- ✅ Proven pattern (same as camera-streamer)
- ✅ Easy to enable/disable
- ✅ Lower complexity
- ✅ Separate control of audio quality

**Cons:**
- ❌ Audio/video sync requires timestamp coordination
- ❌ Two WebSocket connections
- ❌ Slightly more overhead

**Implementation Effort:** ~4-6 hours

**Code Structure:**
```cpp
// solutions/audio-streamer/main/main.cpp
#include "audio.h"
#include "mongoose.h"

// Audio callback queues AAC frames
int audio_stream_callback(AUDIO_STREAM_S* pStream) {
    // Queue AAC-encoded audio
    g_audio_queue.push(pStream);
}

// Main thread sends queued audio
void process_audio_queue() {
    // Pop from queue, send via WebSocket
}

int main() {
    startAudioIn(AUDIO_SAMPLE_RATE_16000, PT_AAC, NULL, audio_stream_callback);
    // WebSocket server on port 8766
}
```

**Browser Decoding:**
```javascript
const audioContext = new AudioContext();
ws.onmessage = async (event) => {
    const arrayBuffer = await event.data.arrayBuffer();
    const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);
    const source = audioContext.createBufferSource();
    source.buffer = audioBuffer;
    source.connect(audioContext.destination);
    source.start();
};
```

---

### Option 2: Integrated A/V Streamer

**Architecture:**
```
Camera + Mic → H.264 + AAC → Multiplexed WebSocket → Browser
```

**Pros:**
- ✅ Single WebSocket connection
- ✅ Perfect A/V sync (same timestamps)
- ✅ Lower total bandwidth
- ✅ Simpler client code

**Cons:**
- ❌ More complex multiplexing protocol
- ❌ Tightly coupled (can't disable one without other)
- ❌ Higher implementation complexity

**Implementation Effort:** ~8-12 hours

**Protocol:**
```
Frame format: [type:1][timestamp:8][length:4][data:N]
  type: 0x01=video, 0x02=audio
```

---

### Option 3: WebRTC with DataChannel

**Architecture:**
```
Camera + Mic → RTP → WebRTC → Browser (native support)
```

**Pros:**
- ✅ Industry standard
- ✅ Native browser support
- ✅ Bidirectional audio (2-way communication)
- ✅ Adaptive bitrate
- ✅ NAT traversal (STUN/TURN)

**Cons:**
- ❌ Complex implementation (signaling, ICE, DTLS)
- ❌ Requires STUN server
- ❌ Overkill for local network
- ❌ Heavy dependencies

**Implementation Effort:** ~20-30 hours

---

### Option 4: Audio in Same Stream (MPEG-TS/FLV container)

**Architecture:**
```
Camera + Mic → MPEG-TS muxer → Single WebSocket → flv.js/mpegts.js
```

**Pros:**
- ✅ Standard container format
- ✅ Perfect sync
- ✅ Existing browser libraries

**Cons:**
- ❌ Need muxer library (not in SDK)
- ❌ Larger dependencies
- ❌ More complex than simple WebSocket

**Implementation Effort:** ~10-15 hours

---

## Recommendation: Option 1 (Separate audio-streamer)

### Why Option 1?

1. **Proven Architecture** - Identical pattern to camera-streamer
2. **AAC Codec Ready** - Already in SDK ([`audio_io.c:114-133`](components/sophgo/audio/src/audio_io.c:114))
3. **Low Complexity** - Reuse all the queue/threading patterns
4. **Browser Support** - Web Audio API handles AAC natively
5. **Modular** - Can ship video-only builds

### Implementation Plan

**Phase 1: Core Audio Capture** (2 hours)
- Create `solutions/audio-streamer/`
- Copy camera-streamer structure
- Replace video SDK with audio SDK
- AAC encoding @ 16kHz mono

**Phase 2: WebSocket Streaming** (1 hour)
- Reuse mongoose WebSocket code
- Queue-based thread-safe sending
- Port 8766

**Phase 3: Integration** (2 hours)
- Supervisor proxy for port 8766
- API endpoint for audio WebSocket URL
- Frontend Web Audio API integration

**Phase 4: A/V Sync** (1 hour, optional)
- Add timestamps to audio frames
- Frontend sync video/audio playback

### Technical Specifications

**Recommended Settings:**
- **Codec**: PT_AAC (AACLC)
- **Sample Rate**: 16kHz (good quality, VQE supported)
- **Channels**: Mono (lower bandwidth)
- **Bitrate**: 32kbps
- **Frame Size**: 1024 samples (64ms @ 16kHz)

**Bandwidth:**
- Video: ~2-3 Mbps (H.264 @ 1080p30)
- Audio: ~32 kbps (AAC @ 16kHz mono)
- **Total**: ~2.5-3.5 Mbps

**Latency:**
- Audio encoding: ~5ms
- Network: ~10-20ms
- Browser decode: ~10-20ms
- **Total**: ~25-45ms (acceptable for monitoring)

---

## SDK API Reference

### Audio Input ([`audio.h`](components/sophgo/audio/audio.h:28-32))

```c
int startAudioIn(
    AUDIO_SAMPLE_RATE_E rate,          // AUDIO_SAMPLE_RATE_16000
    PAYLOAD_TYPE_E enType,             // PT_AAC
    audio_frame_handler frame_out,     // Raw PCM callback (optional)
    audio_stream_handler stream_out    // Encoded stream callback
);
```

### Callback Types
```c
typedef int (*audio_frame_handler)(AUDIO_FRAME_S* pFrame);    // Raw PCM
typedef int (*audio_stream_handler)(AUDIO_STREAM_S* pStream);  // AAC encoded
```

### VQE Features ([`audio_io.c:11-35`](components/sophgo/audio/src/audio_io.c:11))
- **AGC** - Automatic Gain Control (normalize volume)
- **ANR** - Acoustic Noise Reduction
- **AEC** - Acoustic Echo Cancellation (for 2-way audio)
- **Supported**: 8kHz and 16kHz only

---

## Next Steps

If you want to proceed with audio streaming:
1. Decide on Option 1 (separate service) vs Option 2 (integrated)
2. I can implement following the camera-streamer pattern
3. Estimated timeline: 1 day for basic implementation, 2-3 days with full testing

Let me know if you'd like me to start implementing audio streaming!
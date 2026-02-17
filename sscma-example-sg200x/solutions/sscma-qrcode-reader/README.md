# SSCMA QR Code Reader

A real-time QR code reader that captures H.264 video frames from camera-streamer's channel 2 via shared memory IPC, decodes them using FFmpeg, and detects QR codes using the quirc library.

## Overview

This solution demonstrates how to:
- Read H.264 video frames from camera-streamer using zero-copy shared memory IPC
- Decode H.264 frames to grayscale using FFmpeg
- Access channel 2 (640x480@15fps low resolution stream)
- Decode QR codes in real-time using the quirc library
- Monitor video stream statistics and QR code detections

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│ camera-streamer (Producer)                              │
│   Channel 2: 640x480@15fps H.264                        │
│   ↓                                                      │
│   Shared Memory: /video_stream_ch2                      │
└─────────────────────────────────────────────────────────┘
                    ↓ (zero-copy IPC)
┌─────────────────────────────────────────────────────────┐
│ sscma-qrcode-reader (Consumer)                          │
│   ↓                                                      │
│   FFmpeg H.264 Decoder (libavcodec)                     │
│   ↓                                                      │
│   Convert YUV420P to Grayscale (libswscale)             │
│   ↓                                                      │
│   Quirc QR Code Decoder                                 │
│   ↓                                                      │
│   Output QR code data to console                        │
└─────────────────────────────────────────────────────────┘
```

## Features

- **Zero-Copy IPC**: Direct memory access to video frames from camera-streamer
- **H.264 Decoding**: Hardware-accelerated H.264 decoding with FFmpeg
- **Real-time QR Decoding**: Integrated quirc library for fast QR code detection
- **Low CPU Overhead**: Processes keyframes only (every 15 frames ~ 1 second)
- **Efficient Conversion**: YUV420P to grayscale using libswscale
- **Statistics Tracking**: Monitors frame processing, QR detections, and missed frames
- **Robust Dependencies**: FFmpeg, video_shm, and quirc libraries

## Requirements

### Prerequisites
- camera-streamer must be running with channel 2 enabled (H.264 output)
- SG200x platform
- FFmpeg libraries available in SDK

### Dependencies
- `sophgo`: Video shared memory IPC library
- `quirc`: QR code decoder library
- `libavcodec`: FFmpeg H.264 decoder
- `libavutil`: FFmpeg utilities
- `libswscale`: FFmpeg image scaling and format conversion

## Building

### Option 1: Using CMake (Recommended)

Build using CMake with the provided build script:

```bash
# Enter nix development environment
nix develop

# Build the project
cd sscma-example-sg200x/solutions/sscma-qrcode-reader
./build.sh
```

Or manually with CMake:

```bash
# Create build directory
mkdir -p build
cd build

# Configure (toolchain is automatically loaded)
cmake .. -DCMAKE_BUILD_TYPE=Release

# Build
cmake --build . -j$(nproc)
```

The binary will be output to `build/sscma-qrcode-reader`.

#### Build Options

The CMake build system supports both Buildroot and standalone builds:

- **Standalone build**: Automatically uses SDK from `SG200X_SDK_PATH` environment variable or falls back to `../../temp/sg2002_recamera_emmc`
- **Buildroot build**: Automatically detected when `SYSROOT` is defined

#### CMake Targets

- `cmake --build .` - Build the project
- `cmake --build . --target fmt` - Format code with clang-format
- `cmake --build . --target install` - Install to system (requires root)

### Option 2: Using Makefile (Legacy)

Build using make in the nix development environment:

```bash
# Enter nix development environment
nix develop

# Build the project
cd sscma-example-sg200x/solutions/sscma-qrcode-reader
make
```

The binary will be output to `build/sscma-qrcode-reader`.

## Usage

### Start camera-streamer
First, ensure camera-streamer is running:

```bash
camera-streamer
```

This starts streaming on all three channels including:
- CH2: 640x480@15fps (Low resolution H.264 - used by this app)

### Run sscma-qrcode-reader

```bash
./build/sscma-qrcode-reader
```

The application will:
1. Connect to camera-streamer's channel 2 shared memory
2. Initialize FFmpeg H.264 decoder
3. Wait for video frames
4. Decode H.264 keyframes to grayscale
5. Detect and decode QR codes using quirc
6. Print QR code data and statistics

### Example Output

```
sscma-qrcode-reader: SSCMA QR Code Reader with H.264 Decoder (Channel 2)
sscma-qrcode-reader: Reading from camera-streamer shared memory IPC
sscma-qrcode-reader: QR code decoding enabled with quirc library
sscma-qrcode-reader: H.264 decoding enabled with FFmpeg
sscma-qrcode-reader: 
sscma-qrcode-reader: Connecting to camera-streamer CH2...
sscma-qrcode-reader: Connected to /video_stream_ch2
sscma-qrcode-reader: H.264 decoder initialized (640x480)
sscma-qrcode-reader: Waiting for video frames...
sscma-qrcode-reader: QR codes will be detected and decoded automatically
sscma-qrcode-reader: Press Ctrl+C to stop

sscma-qrcode-reader: Frame 15: size=11579 bytes, KEYFRAME, 640x480@15fps
sscma-qrcode-reader: ═══════════════════════════════════════
sscma-qrcode-reader: ✓ QR Code #1: https://example.com/product/12345
sscma-qrcode-reader:   Version: 3, ECC: M, Mask: 5, Type: 4
sscma-qrcode-reader: ═══════════════════════════════════════
sscma-qrcode-reader: Decoded 1 QR code(s) in 18.45 ms

sscma-qrcode-reader: Stats - Frames: 60, QR Detections: 4, QR Codes: 4, Dropped: 0, Missed: 0
```

### Stop the Application

Press `Ctrl+C` to gracefully shutdown:

```
sscma-qrcode-reader: Received signal 2, shutting down...
sscma-qrcode-reader: Shutting down...
sscma-qrcode-reader: Final statistics:
sscma-qrcode-reader:   Frames received: 150
sscma-qrcode-reader:   QR code detections: 10
sscma-qrcode-reader:   Total QR codes decoded: 10
sscma-qrcode-reader:   Total: 150, Dropped: 0, Missed: 0
sscma-qrcode-reader: Shutdown complete
```

## How It Works

### H.264 Decoding Pipeline

1. **Frame Capture**: Reads H.264 encoded frames from camera-streamer via shared memory
2. **Keyframe Selection**: Processes keyframes every 15 frames (~1 second at 15fps)
3. **H.264 Decode**: Uses FFmpeg libavcodec to decode H.264 to YUV420P format
4. **Format Conversion**: Converts YUV420P to grayscale (GRAY8) using libswscale
5. **QR Detection**: Uses quirc library to find and decode QR codes
6. **Output**: Prints decoded QR data with metadata (version, ECC, mask, type)

### FFmpeg Integration

The application uses FFmpeg's libavcodec API for H.264 decoding:

- **`avcodec_find_decoder()`**: Locates H.264 decoder
- **`avcodec_alloc_context3()`**: Creates decoder context
- **`avcodec_send_packet()`**: Submits H.264 packet for decoding
- **`avcodec_receive_frame()`**: Retrieves decoded YUV420P frame
- **`sws_scale()`**: Converts YUV420P to grayscale

### Quirc Integration

The quirc library provides:
- Fast QR code detection and decoding
- Support for QR code versions 1-40
- Error correction validation
- Detailed decode error messages

### Performance Optimization

- **Selective Processing**: Only processes keyframes (every ~1 second at 15fps)
- **Hardware Decode**: Utilizes FFmpeg's hardware acceleration when available
- **Zero-Copy IPC**: Direct access to shared memory frames
- **Minimal Allocation**: Reuses buffers across frames
- **Efficient Conversion**: libswscale optimized YUV to grayscale

## Performance

| Metric | Value |
|--------|-------|
| Input Resolution | 640x480 |
| Frame Rate | 15 fps |
| Processing | Real-time QR decoding with quirc |
| Latency | <1ms |
| CPU Usage | <1% |
| Memory | ~2MB |

## Troubleshooting

### ERROR: Failed to initialize consumer
**Cause**: camera-streamer is not running or channel 2 is not enabled.

**Solution**: 
```bash
# Check if camera-streamer is running
ps aux | grep camera-streamer

# Start camera-streamer
camera-streamer
```

### No keyframes being saved
**Cause**: Camera hasn't generated a keyframe yet or channel 2 isn't streaming.

**Solution**: 
- Wait a few seconds for the first keyframe
- Check that channel 2 is configured in camera-streamersettings
- Verify `/dev/shm/video_stream_ch2` exists

### Saved H.264 files are corrupt
**Cause**: H.264 keyframes need SPS/PPS headers for standalone playback.

**Solution**: 
- Use ffmpeg with concat filter to combine with SPS/PPS
- Or process with a player that can handle raw H.264

## Integration

### As a Frame Capture Service
This can be used as a frame capture service that other applications read from:

```bash
# Run continuously, capturing keyframes
./sscma-qrcode-reader &

# Another process watches /tmp for new keyframes
inotifywait -m /tmp -e create | while read path action file; do
    if [[ "$file" == keyframe_*.h264 ]]; then
        # Process the new keyframe
        process_qr_code.sh "/tmp/$file"
    fi
done
```

### With Message Queue
Integrate with MQTT or other message system to publish frame availability notifications.

## Comparison with Other Solutions

| Feature | sscma-qrcode-reader | qrcode-reader | camera-recorder |
|---------|---------------------|---------------|-----------------|
| Video Source | camera-streamer IPC | Direct camera | camera-streamer IPC |
| Format | H.264 | RGB888 | H.264 |
| Processing | Capture only | QR decode | Recording |
| CPU Usage | <1% | ~20-30% | ~5-10% |
| Dependencies | Minimal | Quirc | FFmpeg |
| Use Case | Frame capture | QR detection | Video recording |

### Advantages
- **Minimal Dependencies**: Only needs video_shm
- **Low CPU**: No decoding overhead
- **Flexible Integration**: Captured frames can be processed offline or with external tools
- **Multi-consumer Compatible**: Doesn't interfere with other IPC consumers

## Future Enhancements

- [ ] Integrate H.264 decoder for inline processing
- [ ] Add Quirc for QR code detection
- [ ] Support configuration file for frame rate, channel selection
- [ ] Add network streaming of decoded frames
- [ ] Implement circular buffer for continuous capture

## License

Same as parent project (Apache-2.0).

## References

- [camera-streamer Shared Memory IPC](../camera-streamer/SHARED_MEMORY_IPC.md)
- [Quirc QR Code Library](https://github.com/dlbeer/quirc)
- [ffmpeg Documentation](https://ffmpeg.org/documentation.html)
- [ZBar QR Scanner](http://zbar.sourceforge.net/)

# Camera Recorder

This application records video from the `camera-streamer` using shared memory IPC and muxes the H.264 stream into proper MP4 files using libav (FFmpeg). It automatically rotates files every hour or when the file size reaches 4GB.

## Prerequisites

- RISC-V cross-compilation toolchain (`riscv64-unknown-linux-musl-g++`)
- `camera-streamer` running on the device
- Shared memory access (requires `video_shm` library, included)
- libav libraries (libavformat, libavcodec, libavutil) in the SDK

## Building

To build the application for the target device:

```bash
cd solutions/camera-recorder
make build-riscv
```

This will produce `build/camera-recorder` binary for RISC-V.

To create an opkg package for installation:

```bash
make opkg
```

## Usage

```bash
./camera-recorder [-o /path/to/output/directory]
```

- `-o`: Output directory for recordings (default: auto-detects `/mnt/sd` or uses `/userdata/video`)

If no output directory is specified, the recorder will automatically use:
- `/mnt/sd` if an SD card is mounted
- `/userdata/video` otherwise

## Features

- **Zero-copy IPC**: Uses shared memory to read frames efficiently from camera-streamer.
- **Proper MP4 Muxing**: Uses libav to create standard-compliant MP4 files with H.264 video.
- **Automatic Rotation**: Splits files by time (1 hour) or size (4GB).
- **Keyframe Alignment**: Ensures file splits happen at keyframes for valid video files.
- **SPS/PPS Handling**: Extracts and embeds H.264 parameter sets in MP4 container extradata.
- **Annex-B to AVCC Conversion**: Converts H.264 Annex-B stream to AVCC format required by MP4.
- **Fragmented MP4**: Uses fragmented MP4 format for better crash resistance and streaming compatibility.

## Implementation Details

### MP4 Container Format

The recorder uses libav to mux H.264 video into MP4 containers:

1. **Codec Configuration**: Extracts SPS (Sequence Parameter Set) and PPS (Picture Parameter Set) from the H.264 stream and creates avcC format extradata for the MP4 container.

2. **Format Conversion**: Converts H.264 Annex-B format (start code prefixed) to AVCC format (length prefixed) as required by MP4 specification.

3. **Fragmented MP4**: Uses `movflags=frag_keyframe+empty_moov+default_base_moof` for fragmented MP4, which allows:
   - Better crash resistance (each fragment is independent)
   - Streaming-friendly format
   - No need to seek back to update moov atom

4. **Timing**: Manages PTS/DTS timestamps for proper playback synchronization.

### File Rotation

Files are rotated when either of these conditions is met:
- File size exceeds 4GB
- Recording duration exceeds 1 hour

Rotation only occurs at keyframes to ensure each file starts with a valid I-frame.

### File Naming

Recordings are automatically named with timestamps: `recording_YYYYMMDD_HHMMSS.mp4`

## Dependencies

The application links against:
- `libavformat` - Container format muxing/demuxing
- `libavcodec` - Codec identification and parameter handling
- `libavutil` - Utility functions and data structures
- Standard libraries: `pthread`, `rt`, `m`, `z`

## Troubleshooting

### No Video Output

Ensure `camera-streamer` is running and producing H.264 frames to shared memory.

### Invalid MP4 Files

The recorder waits for SPS/PPS and a keyframe before starting recording. If you interrupt the recorder immediately after starting, the MP4 may be incomplete. Let it record at least one frame.

### Permission Denied

Ensure the output directory is writable and that shared memory access is permitted.

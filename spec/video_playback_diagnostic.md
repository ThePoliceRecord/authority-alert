# Video Playback Diagnostic Guide

## Issue
Videos recorded to `/mnt/sd` cannot be played in the supervisor web interface.

## Root Cause Analysis

The camera recorder creates fragmented MP4 files optimized for crash resistance but requires proper HTTP server configuration for browser playback.

### Camera Recorder Configuration
The recorder uses these FFmpeg mux flags:
- `frag_keyframe`: New fragment at each keyframe
- `empty_moov`: Minimal movie header without duration
- `omit_tfhd_offset`: Simplified fragment format
- `default_base_moof`: Use moof-relative timestamps

## Required Server Configuration

The supervisor's `/api/fileMgr/download` endpoint must support:

### 1. **Correct Content-Type Header**
```
Content-Type: video/mp4
```

### 2. **Range Request Support** (Critical for video playback)
```
Accept-Ranges: bytes
```

For range requests, the server must:
- Accept `Range: bytes=start-end` request headers
- Respond with HTTP 206 (Partial Content) status
- Include `Content-Range: bytes start-end/total` response header
- Return only the requested byte range

### 3. **CORS Headers** (if needed)
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, HEAD, OPTIONS
Access-Control-Expose-Headers: Content-Length, Content-Range, Accept-Ranges
```

## Testing Commands

### Test 1: Check File Integrity
```bash
# On the camera device
ffprobe /mnt/sd/recording_20260101_002638.mp4

# Expected: Should show video stream info without errors
```

### Test 2: Check HTTP Headers
```bash
curl -I "https://192.168.10.116/api/fileMgr/download?path=recording_20260101_002638.mp4&storage=sd&authorization=YOUR_TOKEN"

# Expected headers:
# Content-Type: video/mp4
# Accept-Ranges: bytes
# Content-Length: <size>
```

### Test 3: Test Range Request
```bash
curl -H "Range: bytes=0-1023" -I "https://192.168.10.116/api/fileMgr/download?path=recording_20260101_002638.mp4&storage=sd&authorization=YOUR_TOKEN"

# Expected:
# HTTP/1.1 206 Partial Content
# Content-Range: bytes 0-1023/<total>
# Content-Length: 1024
```

## Alternative Solutions

### Option 1: Modify Recorder to Use Non-Fragmented MP4
Change the movflags in camera-recorder/main.cpp line 240:
```cpp
// Non-fragmented (but write moov at end - not crash resistant)
av_dict_set(&opts, "movflags", "faststart", 0);
```

### Option 2: Re-mux Videos Post-Recording
Create a post-processing script to convert fragmented MP4 to standard MP4:
```bash
ffmpeg -i /mnt/sd/recording.mp4 -c copy -movflags faststart /mnt/sd/recording_web.mp4
```

### Option 3: Fix the File Server
Update the supervisor's file serving API to properly support:
- Range requests (HTTP 206)
- Correct Content-Type
- Accept-Ranges header

## Quick Test

Try downloading the file directly and playing it locally:
```bash
wget --no-check-certificate "https://192.168.10.116/api/fileMgr/download?path=recording_20260101_002638.mp4&storage=sd&authorization=YOUR_TOKEN" -O test.mp4
ffplay test.mp4  # or open in VLC
```

If it plays locally but not in browser, the issue is definitely the HTTP server configuration.

## Recommended Fix Priority

1. **First**: Test range request support on the file server
2. **If range requests not supported**: Either fix the server OR modify the recorder to use `faststart` movflags
3. **Long-term**: Implement proper range request support in the supervisor file API

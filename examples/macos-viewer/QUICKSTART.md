# Quick Start Guide

Get your macOS video viewer up and running in 5 minutes.

## Step 1: Choose Your Approach

### Option A: SwiftUI (Modern, Recommended)
- File: `WebSocketVideoViewer.swift`
- Best for: Modern macOS apps with settings UI
- Requires: macOS 12.0+

### Option B: AppKit (Simple, Minimal)
- File: `SimpleVideoViewer.swift`  
- Best for: Learning, minimal dependencies
- Requires: macOS 10.15+

## Step 2: Create Xcode Project

### For SwiftUI Version:

1. Open Xcode
2. File → New → Project
3. Choose **App** under macOS
4. Settings:
   - Product Name: `VideoViewer`
   - Interface: **SwiftUI**
   - Language: **Swift**
5. Click **Next**, choose location, click **Create**
6. Delete the default `ContentView.swift` 
7. Add `WebSocketVideoViewer.swift` to project:
   - Right-click project → Add Files
   - Or copy contents into `ContentView.swift`
8. Build and Run (⌘R)

### For AppKit Version:

1. Open Xcode
2. File → New → Project
3. Choose **App** under macOS
4. Settings:
   - Product Name: `SimpleVideoViewer`
   - Interface: **Storyboard** (or SwiftUI, doesn't matter)
   - Language: **Swift**
5. Delete `ContentView.swift` and `Main.storyboard` (if exists)
6. Add `SimpleVideoViewer.swift` to project
7. In project settings → Info tab:
   - Delete "Main storyboard file base name" entry
   - Or set Application Scene Manifest → Scene Configuration → No scenes
8. Build and Run (⌘R)

## Step 3: Start Your Relay Server

In a terminal:

```bash
cd video-relay-server
go run cmd/server/main.go
```

Server should start on `http://localhost:8080`

## Step 4: Start Camera Streaming

In another terminal:

```bash
cd sscma-example-sg200x/solutions/supervisor
export EXTERNAL_RELAY_URL="ws://localhost:8080/ws"
export CAMERA_ID="camera_12345"
export ON_DEMAND_STREAMING="true"  # Optional: enable on-demand mode
go run cmd/supervisor/main.go
```

## Step 5: View the Stream

1. Launch your macOS app
2. Click **Connect** button
3. Video should appear within 2-3 seconds

## Troubleshooting

### App doesn't build
- Error: "Module not found" → Check you're targeting macOS, not iOS
- Error: "Storyboard entry" → Remove storyboard reference from Info.plist

### Can't connect to server
```bash
# Test relay server is running:
curl http://localhost:8080/health

# Should return: {"status":"ok"}
```

### No video appears
- Check camera is actually streaming: look for relay logs showing frames
- Verify camera_id matches exactly (case-sensitive)
- Try with ON_DEMAND_STREAMING disabled for testing

### Black screen with "Waiting for video"
- This means connected but no frames received
- Check supervisor logs for errors
- Ensure camera hardware is working

## Command Line Testing (AppKit version only)

```bash
# Build
swiftc -framework Cocoa SimpleVideoViewer.swift -o VideoViewer

# Run with custom URL and camera ID
./VideoViewer ws://192.168.1.100:8080/ws my_camera_id
```

## Next Steps

Once basic viewing works:

1. **Add authentication**: Set auth token in settings
2. **Try multiple cameras**: Modify code to show grid of cameras  
3. **Record video**: Add frame saving functionality
4. **Add overlays**: Draw detection boxes, timestamps, etc.
5. **Performance tune**: Adjust buffer sizes for your network

## Example Configurations

### Local Development
```
Relay URL: ws://localhost:8080/ws
Camera ID: camera_12345
Auth Token: (leave empty)
```

### Production Server
```
Relay URL: wss://relay.example.com/ws
Camera ID: prod_camera_001
Auth Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Multiple Cameras
```
Camera 1: camera_front_door
Camera 2: camera_backyard  
Camera 3: camera_driveway
```

## Performance Tips

- **Frame Rate**: Typical MJPEG streams are 10-30 fps
- **Latency**: Expect 200-500ms delay over internet
- **Bandwidth**: ~1-3 Mbps per camera for 1080p
- **CPU Usage**: JPEG decoding is efficient, ~5-10% per stream

## See Also

- [Full README](README.md) - Detailed documentation
- [Protocol Spec](../../spec/ON_DEMAND_STREAMING_IMPLEMENTATION.md) - WebSocket protocol details
- [Relay Server](../../video-relay-server/) - Server implementation

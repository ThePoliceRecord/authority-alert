# macOS WebSocket Video Viewer

A native macOS app to view video streams from the Authority Alert relay server.

## Features

- Real-time video streaming over WebSocket
- Connection status indicators
- Viewer count display
- Automatic reconnection handling
- Settings UI for configuration
- MJPEG frame decoding

## Requirements

- macOS 12.0 or later
- Xcode 14.0 or later
- Swift 5.7 or later

## Setup

### Option 1: Create Xcode Project

1. Open Xcode
2. Create a new **macOS App** project
3. Choose SwiftUI for Interface
4. Name it "AuthorityAlertViewer" (or any name you prefer)
5. Replace the contents of `ContentView.swift` with `WebSocketVideoViewer.swift`
6. Build and run (⌘ + R)

### Option 2: Swift Package Manager (Command Line)

Create a simple package to test:

```bash
# Create package directory
mkdir -p macos-viewer-app
cd macos-viewer-app

# Create Package.swift
cat > Package.swift << 'PKGEOF'
// swift-tools-version: 5.7
import PackageDescription

let package = Package(
    name: "VideoViewer",
    platforms: [.macOS(.v12)],
    dependencies: [],
    targets: [
        .executableTarget(
            name: "VideoViewer",
            dependencies: [])
    ]
)
PKGEOF

# Create source directory and copy the Swift file
mkdir -p Sources/VideoViewer
cp ../WebSocketVideoViewer.swift Sources/VideoViewer/main.swift

# Build
swift build

# Note: This creates a command-line tool, not a GUI app
# For GUI apps, use Xcode
```

## Usage

### 1. Configure Connection

The app comes with default settings:
- **Relay URL**: `ws://localhost:8080/ws`
- **Camera ID**: `camera_12345`
- **Auth Token**: (empty)

To change these:
1. Click the gear icon (⚙️) in the bottom-right
2. Update the values
3. Click Save

### 2. Connect to Stream

1. Ensure your relay server is running
2. Click **Connect** button
3. The app will automatically subscribe to the camera
4. Video should start displaying within a few seconds

### 3. Monitor Status

- **Green dot**: Connected to relay server
- **Red dot**: Disconnected
- **Eye icon + number**: Current number of viewers watching this camera

## Connection Protocol

The app implements the relay server protocol:

### 1. WebSocket Connection
```
GET /ws HTTP/1.1
Role: viewer
Authorization: Bearer <token>  (if using auth)
```

### 2. Subscribe to Camera
```json
{
  "type": "subscribe",
  "camera_id": "camera_12345"
}
```

### 3. Receive Video Frames
- Binary WebSocket messages containing JPEG frames
- Each frame is displayed as received

### 4. Control Messages
Text messages for status updates:
```json
{
  "type": "camera_offline",
  "camera_id": "camera_12345"
}
```

```json
{
  "type": "error",
  "message": "Access denied"
}
```

## Troubleshooting

### No Video Displayed

**Problem**: Connected but no frames showing

**Solutions**:
1. Check camera is streaming to relay server
2. Verify camera ID matches exactly
3. Check relay server logs for connection
4. Ensure camera is in "always-on" mode or has on-demand streaming triggered

### Connection Fails

**Problem**: Cannot connect to relay server

**Solutions**:
1. Verify relay URL is correct (must start with `ws://` or `wss://`)
2. Check relay server is running: `curl http://localhost:8080/health`
3. Check firewall settings
4. Try without auth token first

### Authentication Errors

**Problem**: "Access denied" or "Unauthorized"

**Solutions**:
1. Verify auth token is valid
2. Check token hasn't expired
3. Ensure token has viewer permissions for this camera
4. Test with auth disabled on relay server first

## Code Structure

### `WebSocketVideoClient`
The core networking class that:
- Manages WebSocket connection
- Sends subscribe messages
- Receives and decodes video frames
- Handles control messages
- Publishes state updates via Combine

### `ContentView`
Main SwiftUI view with:
- Video display area
- Connection controls
- Status indicators
- Settings panel

### `SettingsView`
Configuration UI for:
- Relay server URL
- Camera ID
- Authentication token

## Advanced Usage

### Multiple Cameras

To view multiple cameras simultaneously:

```swift
struct MultiCameraView: View {
    @StateObject private var camera1 = WebSocketVideoClient(
        relayURL: URL(string: "ws://localhost:8080/ws")!,
        cameraID: "camera_1",
        authToken: nil
    )
    
    @StateObject private var camera2 = WebSocketVideoClient(
        relayURL: URL(string: "ws://localhost:8080/ws")!,
        cameraID: "camera_2",
        authToken: nil
    )
    
    var body: some View {
        HStack {
            VideoDisplayView(client: camera1)
            VideoDisplayView(client: camera2)
        }
        .onAppear {
            camera1.connect()
            camera2.connect()
        }
    }
}
```

### Recording Frames

To save frames:

```swift
private func processVideoFrame(_ data: Data) {
    if let image = NSImage(data: data) {
        // Save to disk
        if let tiffData = image.tiffRepresentation,
           let bitmapImage = NSBitmapImageRep(data: tiffData),
           let pngData = bitmapImage.representation(using: .png, properties: [:]) {
            let filename = "frame_\(Date().timeIntervalSince1970).png"
            try? pngData.write(to: URL(fileURLWithPath: filename))
        }
        
        DispatchQueue.main.async {
            self.currentFrame = image
        }
    }
}
```

### Custom Video Processing

Add image processing before display:

```swift
private func processVideoFrame(_ data: Data) {
    if let image = NSImage(data: data) {
        // Apply filters, overlays, etc.
        let processedImage = applyFilters(to: image)
        
        DispatchQueue.main.async {
            self.currentFrame = processedImage
        }
    }
}

func applyFilters(to image: NSImage) -> NSImage {
    // Use Core Image for filters
    guard let tiffData = image.tiffRepresentation,
          let ciImage = CIImage(data: tiffData) else {
        return image
    }
    
    let filter = CIFilter(name: "CIColorControls")
    filter?.setValue(ciImage, forKey: kCIInputImageKey)
    filter?.setValue(1.2, forKey: kCIInputBrightnessKey)
    
    // Convert back to NSImage...
    return image
}
```

## Performance Tips

1. **Frame Rate**: The app displays frames as fast as they arrive
2. **Memory**: Old frames are automatically released
3. **Threading**: Frame decoding happens on background thread
4. **Buffer Size**: Adjust send channel size if frames are dropped

## Related Files

- [`relay.go`](../../video-relay-server/internal/relay/relay.go) - Server-side relay implementation
- [`ON_DEMAND_STREAMING_IMPLEMENTATION.md`](../../spec/ON_DEMAND_STREAMING_IMPLEMENTATION.md) - Protocol specification

## License

Same as parent project.

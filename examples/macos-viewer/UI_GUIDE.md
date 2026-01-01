# UI Guide - macOS Video Viewer

Visual guide to the three UI options provided.

## Option 1: Enhanced Video Viewer (Recommended)

**File:** `EnhancedVideoViewer.swift`

### Main Window Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  🎥 camera_12345         📊 📐 ⚙️                               │ Top Toolbar
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────┐                                            │
│  │ FPS:      24.5  │                                            │
│  │ Frames:   1250  │      [VIDEO DISPLAY AREA]                 │
│  │ Data:    45.2MB │                                            │
│  │ Uptime: 00:52   │                                            │
│  └─────────────────┘                                            │
│                            1920x1080                             │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│  🟢 Connected    👁️ 3                     [▶️ Disconnect]      │ Bottom Bar
└─────────────────────────────────────────────────────────────────┘
```

### Features

#### Top Toolbar
- **Camera Name Badge** - Shows current camera ID
- **Stats Toggle** 📊 - Show/hide statistics overlay
- **Fullscreen** 📐 - Toggle fullscreen mode
- **Settings** ⚙️ - Open configuration panel

#### Video Area
- Full-screen video display
- Automatic aspect ratio preservation
- **Statistics Overlay** (toggleable):
  - Real-time FPS counter
  - Total frames received
  - Data transferred
  - Connection uptime
- **Loading Indicator** - When waiting for video
- **Error Alerts** - Red banner for errors

#### Bottom Control Bar
- **Connection Status** - 🟢 Green (connected) / 🔴 Red (disconnected)
- **Viewer Count** - 👁️ Shows how many people watching
- **Connect/Disconnect Button** - Large, color-coded action button

### Settings Panel

```
┏━━━━━━━━━━━━━━━━━━━ Connection Settings ━━━━━━━━━━━━━━━━━━━┓
┃                                                              ┃
┃  Relay Server                                                ┃
┃  ┌────────────────────────────────────────────────────────┐ ┃
┃  │ ws://localhost:8080/ws                                 │ ┃
┃  └────────────────────────────────────────────────────────┘ ┃
┃                                                              ┃
┃  Camera                                                      ┃
┃  ┌────────────────────────────────────────────────────────┐ ┃
┃  │ camera_12345                                           │ ┃
┃  └────────────────────────────────────────────────────────┘ ┃
┃                                                              ┃
┃  Authentication                                              ┃
┃  ┌────────────────────────────────────────────────────────┐ ┃
┃  │ •••••••••••••••                                        │ ┃
┃  └────────────────────────────────────────────────────────┘ ┃
┃  Leave empty if authentication is disabled                   ┃
┃                                                              ┃
┃  ────────────────────────────────────────────────────────    ┃
┃                                                              ┃
┃  [Cancel]                            [Save & Connect]        ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

### Connection States

#### 1. Disconnected
```
┌─────────────────────┐
│                     │
│      🚫             │
│   video.slash       │
│                     │
│  Not connected      │
│                     │
└─────────────────────┘
```

#### 2. Connecting / Waiting for Video
```
┌─────────────────────┐
│                     │
│      ⏳             │
│  (spinner)          │
│                     │
│  Waiting for video  │
│                     │
└─────────────────────┘
```

#### 3. Streaming
```
┌─────────────────────┐
│                     │
│   ┌─────────────┐   │
│   │ Live Video  │   │
│   │   Playing   │   │
│   └─────────────┘   │
│                     │
└─────────────────────┘
```

#### 4. Error State
```
┌─────────────────────────────┐
│                             │
│      [VIDEO]                │
│                             │
│  ┌─────────────────────┐   │
│  │ ⚠️ Camera offline   │   │
│  └─────────────────────┘   │
└─────────────────────────────┘
```

---

## Option 2: Basic SwiftUI Viewer

**File:** `WebSocketVideoViewer.swift`

### Layout

```
┌───────────────────────────────────────────────┐
│                                               │
│                                               │
│                                               │
│          [VIDEO DISPLAY AREA]                │
│                                               │
│                                               │
│                                               │
├───────────────────────────────────────────────┤
│ 🟢 Connected  👁️ 2     [Connect] ⚙️         │
└───────────────────────────────────────────────┘
```

### Features
- Clean, minimal interface
- Connection status indicator
- Viewer count
- Connect/disconnect button
- Settings button

---

## Option 3: Simple AppKit Viewer

**File:** `SimpleVideoViewer.swift`

### Layout

```
┌───────────────────── Video Viewer ────────────────────┐
│                                                        │
│                                                        │
│              [VIDEO DISPLAY AREA]                     │
│                                                        │
│                                                        │
├────────────────────────────────────────────────────────┤
│  Disconnected                          [Connect]       │
└────────────────────────────────────────────────────────┘
```

### Features
- Ultra-simple interface
- Text-based status
- Single connect button
- Command-line argument support

---

## Comparison Matrix

| Feature | Enhanced | Basic SwiftUI | Simple AppKit |
|---------|----------|---------------|---------------|
| **UI Framework** | SwiftUI | SwiftUI | AppKit |
| **Video Display** | ✅ | ✅ | ✅ |
| **Connection Status** | 🟢 Badge | 🟢 Badge | Text |
| **FPS Counter** | ✅ | ❌ | ❌ |
| **Frame Counter** | ✅ | ❌ | ❌ |
| **Data Stats** | ✅ | ❌ | ❌ |
| **Uptime** | ✅ | ❌ | ❌ |
| **Viewer Count** | ✅ | ✅ | ❌ |
| **Settings Panel** | ✅ Rich | ✅ Basic | ❌ |
| **Fullscreen** | ✅ | ❌ | ❌ |
| **Stats Toggle** | ✅ | ❌ | ❌ |
| **Error Display** | Banner | Toast | None |
| **Loading State** | Spinner | Spinner | None |
| **Code Lines** | ~420 | ~250 | ~180 |
| **Complexity** | High | Medium | Low |

## UI Color Scheme

### Enhanced Viewer Colors
- **Background**: `Color.black` (video area)
- **Overlays**: `Color.black.opacity(0.7)` (semi-transparent)
- **Connected**: `Color.green` 🟢
- **Disconnected**: `Color.red` 🔴
- **Error**: `Color.red.opacity(0.9)` (alert background)
- **Text**: `Color.white` (all labels)
- **Accent**: `Color.blue` (buttons, links)

### System Integration
All viewers respect macOS system appearance:
- **Light Mode**: Window chrome is light
- **Dark Mode**: Window chrome is dark (recommended)
- Video area always black for best contrast

## Keyboard Shortcuts

### Enhanced Viewer
- **⌘ + ,** - Open Settings
- **⌘ + K** - Connect/Disconnect
- **⌘ + I** - Toggle Stats
- **⌘ + F** - Fullscreen
- **ESC** - Exit Fullscreen

### Basic Viewers
- **⌘ + ,** - Settings (SwiftUI version only)
- **⌘ + W** - Close Window
- **⌘ + Q** - Quit App

## Recommended Use Cases

### Enhanced Viewer
**Best for:**
- Production monitoring dashboards
- Security camera viewing
- Professional streaming applications
- Users who need detailed statistics
- Multi-hour viewing sessions

### Basic SwiftUI Viewer
**Best for:**
- Simple viewing needs
- Learning SwiftUI
- Quick camera checks
- Embedding in larger apps

### Simple AppKit Viewer
**Best for:**
- Learning purposes
- Command-line integration
- Automated testing
- Minimal resource usage
- Understanding WebSocket basics

## Screenshots (Text Representation)

### Enhanced Viewer - Active Stream with Stats

```
╔═══════════════════════════════════════════════════════════════╗
║ 🎥 camera_front_door                      📊 📐 ⚙️            ║
╠═══════════════════════════════════════════════════════════════╣
║ ┌──────────────┐                                              ║
║ │FPS:     29.8 │   ┌────────────────────────────────┐        ║
║ │Frames: 15420 │   │                                │        ║
║ │Data:  234 MB │   │     [Live Camera Feed]         │        ║
║ │Uptime: 08:35 │   │      🏠 Front Door             │        ║
║ └──────────────┘   │      1920x1080                 │        ║
║                    │                                │        ║
║                    └────────────────────────────────┘        ║
║                                                               ║
╠═══════════════════════════════════════════════════════════════╣
║ 🟢 Connected     👁️ 5 viewers         [⏸️ Disconnect]        ║
╚═══════════════════════════════════════════════════════════════╝
```

### Enhanced Viewer - Error State

```
╔═══════════════════════════════════════════════════════════════╗
║ 🎥 camera_front_door                      📊 📐 ⚙️            ║
╠═══════════════════════════════════════════════════════════════╣
║                                                               ║
║                    ┌────────────────────┐                     ║
║                    │                    │                     ║
║                    │   [Dark Video]     │                     ║
║                    │                    │                     ║
║                    └────────────────────┘                     ║
║                                                               ║
║  ┌──────────────────────────────────────────────────────┐    ║
║  │ ⚠️ Connection error: timeout                         │    ║
║  └──────────────────────────────────────────────────────┘    ║
╠═══════════════════════════════════════════════════════════════╣
║ 🔴 Disconnected                            [▶️ Connect]        ║
╚═══════════════════════════════════════════════════════════════╝
```

## Next Steps

1. **Try the Enhanced Viewer** for the full experience
2. **Customize colors** in the code to match your brand
3. **Add notifications** for camera events
4. **Implement recording** to save video clips
5. **Build multi-camera grid** for simultaneous viewing

See [README.md](README.md) for implementation details.

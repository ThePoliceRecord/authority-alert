# OOBE Camera Registration - Implementation Summary

**Created:** 2026-01-10  
**Status:** Ready for Implementation

## Quick Overview

Add a new step to the OOBE that allows cameras to register with The Police Record platform by scanning a QR code displayed on the user's phone.

## Complete Flow

1. **User completes WiFi setup** (Step 4)
2. **OOBE shows Camera Registration step** (New Step 5)
   - Displays live camera preview from Channel 2 via WebSocket
   - Shows "Open Activation Page" and "Start Scanning" buttons
3. **User clicks "Open Activation Page"**
   - Opens `https://dev.thepolicerecord.com/activate/camera/` in new tab
   - Platform displays QR code containing: `{"api_key": "...", "user_id": 14}`
4. **User clicks "Start Scanning" in OOBE**
   - OOBE calls `POST /api/qr/scan` (supervisor API)
   - Camera activates QR scanner
5. **User holds phone with QR code in front of camera**
   - Camera scans QR code
   - OOBE polls `GET /api/qr/scan?scan_id={id}` for results
6. **When scan succeeds:**
   - OOBE parses `api_key` and `user_id` from QR code
   - OOBE collects device info (serial, MAC, location, firmware, etc.)
   - OOBE calls `POST https://dev.thepolicerecord.com/api/v1/cameras/self-register/`
     - Headers: `TPR-API-KEY: {api_key from QR}`
     - Body: Camera device information
   - Platform responds with camera data and permanent `camera_api_key`
   - OOBE saves registration to `/userdata/config/police-record.json`
7. **User continues to Complete step** (Step 6)

## Key Technical Details

### QR Code Data (from Platform)
```json
{
  "api_key": "E6ntWqJr.NMpgnvmsX7oWdqLjtXQtvAyHnqvH90n7",
  "user_id": 14
}
```

### Platform API Call
```
POST https://dev.thepolicerecord.com/api/v1/cameras/self-register/
Headers:
  Content-Type: application/json
  TPR-API-KEY: {api_key from QR code}
Body:
  {
    "camera_id": "uuid-generated-by-oobe",
    "location_name": "Front Door",
    "serial_number": "device-serial",
    "wifi_mac_address": "00:11:22:33:44:55",
    "mac_address": "66:77:88:99:AA:BB",
    "device_model": "Seeed Studio reCamera2002w",
    "firmware_version": "1.0.0",
    "latitude": 37.7749,
    "longitude": -122.4194
  }
```

### Platform Response
```json
{
  "code": 200,
  "message": "Success",
  "data": {
    "uid": "platform-generated-uid",
    "camera_id": "uuid-from-request",
    ...camera details...
  },
  "key": "permanent_camera_api_key_for_future_use"
}
```

### Saved Registration Data
**File:** `/userdata/config/police-record.json`
```json
{
  "platform_url": "https://dev.thepolicerecord.com",
  "api_key": "E6ntWqJr.NMpgnvmsX7oWdqLjtXQtvAyHnqvH90n7",
  "camera_api_key": "permanent_camera_api_key_for_future_use",
  "user_id": 14,
  "camera_id": "uuid-generated-by-oobe",
  "camera_uid": "platform-generated-uid",
  "registered_at": "2023-01-01T12:00:00Z",
  "camera_data": { ...full camera object from platform... }
}
```

## Files to Modify

### Frontend
1. **[`sscma-example-sg200x/solutions/oobe/web/index.html`](../sscma-example-sg200x/solutions/oobe/web/index.html)**
   - Add step 5 HTML (video player, scan status, buttons)
   - Rename step 5 to step 6
   - Update progress indicators
   - Add jmuxer library script tag

2. **[`sscma-example-sg200x/solutions/oobe/web/css/style.css`](../sscma-example-sg200x/solutions/oobe/web/css/style.css)**
   - Add video container styles
   - Add scan status indicator styles
   - Add registration instructions styles

3. **[`sscma-example-sg200x/solutions/oobe/web/js/oobe-app.js`](../sscma-example-sg200x/solutions/oobe/web/js/oobe-app.js)**
   - Add registration state variables
   - Implement camera streaming (WebSocket + jmuxer)
   - Implement QR scanning (call supervisor API)
   - Implement platform registration (call TPR API)
   - Implement step navigation and cleanup

### Backend
4. **[`sscma-example-sg200x/solutions/oobe/main/oobe_server.cpp`](../sscma-example-sg200x/solutions/oobe/main/oobe_server.cpp)**
   - Add `POST /oobe/api/saveRegistration` endpoint
   - Add `GET /oobe/api/registrationStatus` endpoint
   - Implement secure file storage with proper permissions

## APIs Used

### Supervisor APIs (Already Exist)
- `GET /api/deviceMgr/getCameraWebsocketUrl` - Get WebSocket URL for camera
- `WebSocket ws://device:8765?channel=2` - Stream H.264 video
- `POST /api/qr/scan` - Start QR code scan
- `GET /api/qr/scan?scan_id={id}` - Poll scan status
- `DELETE /api/qr/scan?scan_id={id}` - Cancel scan

### Platform APIs (The Police Record)
- `POST /api/v1/cameras/self-register/` - Register camera with platform

### OOBE APIs (To Be Created)
- `POST /oobe/api/saveRegistration` - Save registration data locally
- `GET /oobe/api/registrationStatus` - Check if camera is registered

## Implementation Order

1. **HTML/CSS** - Add UI elements and styling
2. **Camera Streaming** - Implement WebSocket + jmuxer video display
3. **QR Scanning** - Integrate with supervisor QR API
4. **Platform Registration** - Call TPR self-register API
5. **Backend Storage** - Save registration data securely
6. **Testing** - End-to-end testing with real QR codes

## Testing Checklist

- [ ] Video streams from Channel 2
- [ ] Activation page opens correctly
- [ ] QR scan starts and polls status
- [ ] QR code from platform is successfully scanned
- [ ] Platform API registration succeeds
- [ ] Registration data saved with correct permissions (600)
- [ ] Can skip registration
- [ ] Can retry after timeout
- [ ] Error handling works correctly
- [ ] Navigation between steps works
- [ ] Cleanup happens on step change

## Security Notes

- **Never log API keys** - Mask in console and logs
- **File permissions** - `/userdata/config/police-record.json` must be 600
- **HTTPS only** - All platform API calls use HTTPS
- **Validate QR data** - Check JSON structure and required fields
- **Rate limiting** - Prevent scan abuse

## Related Documents

1. **[oobe-camera-registration-step.md](oobe-camera-registration-step.md)** - Detailed implementation plan with code examples
2. **[oobe-camera-registration-platform-api.md](oobe-camera-registration-platform-api.md)** - Platform API integration details and data formats

## Quick Reference

**Platform QR Code Format:**
```json
{"api_key": "E6ntWqJr.NMpgnvmsX7oWdqLjtXQtvAyHnqvH90n7", "user_id": 14}
```

**Registration Endpoint:**
```
POST https://dev.thepolicerecord.com/api/v1/cameras/self-register/
Header: TPR-API-KEY: {api_key}
```

**Saved Config Location:**
```
/userdata/config/police-record.json (permissions: 600)
```

**Camera Streaming:**
```
WebSocket: ws://device:8765?channel=2
Format: H.264 with 8-byte timestamp
Decoder: jmuxer
```

**QR Scanning:**
```
POST /api/qr/scan → {scan_id}
GET /api/qr/scan?scan_id={id} → {status, result}
```

# QR Code Reader Service

External services (OOBE and supervisor web page) can now use the QR code reader through an on-demand HTTP API.

## Architecture Overview

```
┌──────────────┐         ┌────────────────┐         ┌──────────────┐
│ OOBE Web UI  │────────▶│   Supervisor   │────────▶│  QR Reader   │
│   / Client   │  HTTP   │   QR Manager   │ spawn() │   Binary     │
└──────────────┘         └────────────────┘         └──────────────┘
                              │                            │
                              │                            ▼
                              │                     ┌───────────────┐
                              │                     │ Camera Stream │
                              │                     │   Channel 2   │
                              │                     └───────────────┘
                              ▼
                         ┌──────────────┐
                         │ Result Cache │
                         │ (5min expiry)│
                         └──────────────┘
```

## Components

### 1. QR Reader Binary
**Location:** `/usr/bin/qr-reader`
**Source:** `sscma-example-sg200x/solutions/sscma-qrcode-reader/main/main.cpp`

A one-shot command-line tool that:
- Connects to camera stream (channel 2)
- Scans for QR codes with timeout
- Outputs results as JSON to stdout
- Diagnostic logging to stderr
- Proper exit codes (0=success, 1=timeout, 2=error, 3=cancelled)

#### Command-Line Interface

```bash
qr-reader [OPTIONS]

Options:
  --timeout <seconds>      Scan timeout (default: 30)
  --max-results <count>    Maximum QR codes (default: 1, 0=unlimited)
  --schema <name>          Validate against schema
                           (authority_config, wifi_config, device_pairing)
  --help                   Show help
```

#### Exit Codes

- **0**: Success - QR code(s) detected and decoded
- **1**: Timeout - No QR codes found within timeout
- **2**: Error - Initialization failure or validation failed
- **3**: Cancelled - Received SIGTERM/SIGINT

#### Output Format (stdout)

**Success:**
```json
{
  "success": true,
  "qr_codes": [
    {
      "data": "https://example.com",
      "version": 1,
      "ecc_level": "M",
      "mask": 2,
      "data_type": 4,
      "validated": true
    }
  ],
  "count": 1,
  "frames_processed": 87,
  "detection_time_ms": 2941
}
```

**Timeout:**
```json
{
  "success": false,
  "reason": "timeout",
  "frames_processed": 450,
  "scan_duration_ms": 30000
}
```

**Validation Failed:**
```json
{
  "success": false,
  "reason": "validation_failed",
  "qr_data": "invalid content",
  "schema_expected": "authority_config",
  "frames_processed": 23,
  "detection_time_ms": 782
}
```

### 2. Supervisor QR Manager
**Location:** `sscma-example-sg200x/solutions/supervisor/qr-scan-manager.js`

Node.js module that:
- Manages scan sessions with UUID tracking
- Spawns `qr-reader` processes on demand
- Prevents concurrent scans
- Stores completed results for 5 minutes
- Automatic cleanup of expired sessions
- Captures diagnostic output

#### Features Implemented

✅ **1. Concurrent scan prevention** - Only one active scan at a time  
✅ **2. Graceful cancellation** - SIGTERM handling with proper cleanup  
✅ **4. Session cleanup** - Automatic expiry after 5 minutes  
✅ **5. Diagnostic logging** - stderr captured and logged  
✅ **8. Multi-QR handling** - Configurable max_results  
✅ **9. Schema validation** - QR data validation before return  

### 3. HTTP API Routes
**Location:** `sscma-example-sg200x/solutions/supervisor/qr-routes-example.js`

Express.js routes for supervisor:

#### POST /api/qr/scan
Start a new scan session.

**Request:**
```json
{
  "timeout": 30,
  "max_results": 1,
  "schema": "authority_config"
}
```

**Response (201 Created):**
```json
{
  "code": 0,
  "msg": "Scan started",
  "data": {
    "scan_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "scanning",
    "started_at": "2026-01-07T23:40:00Z"
  }
}
```

**Response (409 Conflict):**
```json
{
  "code": 409,
  "msg": "Scan already in progress",
  "data": {
    "active_scan_id": "abc123"
  }
}
```

#### GET /api/qr/scan/:scanId
Query scan status.

**Response (scanning):**
```json
{
  "code": 0,
  "msg": "Success",
  "data": {
    "scan_id": "550e8400-...",
    "status": "scanning",
    "started_at": "2026-01-07T23:40:00Z"
  }
}
```

**Response (complete):**
```json
{
  "code": 0,
  "msg": "Success",
  "data": {
    "scan_id": "550e8400-...",
    "status": "complete",
    "started_at": "2026-01-07T23:40:00Z",
    "completed_at": "2026-01-07T23:40:03Z",
    "result": {
      "success": true,
      "qr_codes": [...]
    }
  }
}
```

#### DELETE /api/qr/scan/:scanId
Cancel active scan.

**Response:**
```json
{
  "code": 0,
  "msg": "Scan cancelled",
  "data": {
    "scan_id": "550e8400-...",
    "status": "cancelled"
  }
}
```

#### GET /api/qr/health
Check QR service health.

**Response:**
```json
{
  "code": 0,
  "msg": "Success",
  "data": {
    "status": "ready",
    "active_scan": null,
    "completed_scans_count": 2,
    "qr_reader_path": "/usr/bin/qr-reader"
  }
}
```

### 4. Client-Side API
**Location:** `sscma-example-sg200x/solutions/oobe/web/js/supervisor-api.js`

JavaScript client library added to SupervisorAPI class:

#### Methods

**startQRScan(timeout, maxResults, schema)**
Start a scan session.

**getQRScanStatus(scanId)**
Query scan status.

**cancelQRScan(scanId)**
Cancel a scan.

**scanQRCode(timeout, maxResults, schema, onProgress)**
Helper method that handles the full scan lifecycle with automatic polling.

#### Example Usage

```javascript
const api = new SupervisorAPI();

// Simple scan with automatic polling
try {
  const result = await api.scanQRCode(30, 1, 'wifi_config', (status, result) => {
    console.log('Scan status:', status);
    document.getElementById('status').textContent = `Scanning... (${status})`;
  });
  
  console.log('QR Code detected:', result.qr_codes[0].data);
  // Parse and use QR data
  const config = JSON.parse(result.qr_codes[0].data);
  
} catch (error) {
  console.error('QR scan failed:', error.message);
}

// Manual control
const startResult = await api.startQRScan(30, 1, null);
const scanId = startResult.data.scan_id;

// Poll manually
const statusInterval = setInterval(async () => {
  const status = await api.getQRScanStatus(scanId);
  if (status.data.status === 'complete') {
    clearInterval(statusInterval);
    console.log('Result:', status.data.result);
  }
}, 500);

// Cancel if needed
await api.cancelQRScan(scanId);
```

## Integration Examples

### OOBE WiFi Configuration

```javascript
// In OOBE step 3 - Network Setup
async scanWiFiConfig() {
  const button = document.getElementById('qr-scan-btn');
  const status = document.getElementById('scan-status');
  
  button.disabled = true;
  status.textContent = 'Starting camera...';
  
  try {
    const result = await this.api.scanQRCode(
      30,  // 30 second timeout
      1,   // Single QR code
      'wifi_config',  // Validate WiFi config format
      (scanStatus) => {
        status.textContent = `Scanning... Point camera at QR code`;
      }
    );
    
    // Parse WiFi config
    const config = JSON.parse(result.qr_codes[0].data);
    
    // Fill form
    document.getElementById('ssid').value = config.ssid;
    document.getElementById('password').value = config.password;
    
    status.textContent = '✓ WiFi configuration detected!';
    
  } catch (error) {
    status.textContent = `Error: ${error.message}`;
  } finally {
    button.disabled = false;
  }
}
```

### Supervisor Device Pairing

```javascript
// In supervisor web page
async pairDevice() {
  try {
    const result = await api.scanQRCode(30, 1, 'device_pairing');
    
    const pairingData = JSON.parse(result.qr_codes[0].data);
    
    // Connect to paired device
    await api.addPairedDevice(pairingData.device_id, pairingData.shared_key);
    
    alert('Device paired successfully!');
    
  } catch (error) {
    alert('Pairing failed: ' + error.message);
  }
}
```

## Schema Validation

The QR reader can validate QR code content against predefined schemas.

### authority_config
Configuration for Authority Alert system.

```json
{
  "type": "authority_alert_config",
  "version": 1,
  "network": {
    "ssid": "MyWiFi",
    "password": "secret"
  },
  "remote": {
    "police_record_url": "https://records.example.org"
  }
}
```

### wifi_config
WiFi network credentials.

```json
{
  "ssid": "NetworkName",
  "password": "password123",
  "security": "WPA2"
}
```

### device_pairing
Device pairing information.

```json
{
  "device_id": "aa-device-001",
  "shared_key": "base64_encoded_key",
  "relay_url": "https://relay.example.com"
}
```

## Deployment

### 1. Build QR Reader Binary

```bash
cd sscma-example-sg200x/solutions/sscma-qrcode-reader
make
sudo cp build/qr-reader /usr/bin/
sudo chmod +x /usr/bin/qr-reader
```

### 2. Install Supervisor Modules

```bash
cd sscma-example-sg200x/solutions/supervisor
npm install uuid  # If not already installed
```

### 3. Integrate with Supervisor

```javascript
// In supervisor main app
const { registerQRRoutes } = require('./qr-routes-example');

// After creating Express app
registerQRRoutes(app);
```

### 4. Update OOBE/Web UI

The `supervisor-api.js` is already updated with QR methods. Just deploy the updated file.

## Testing

### Test QR Reader Binary

```bash
# Basic scan
/usr/bin/qr-reader --timeout 10

# With schema validation
/usr/bin/qr-reader --timeout 15 --schema wifi_config

# Multiple QR codes
/usr/bin/qr-reader --timeout 20 --max-results 5
```

### Test Supervisor API

```bash
# Start scan
curl -X POST https://localhost/api/qr/scan \
  -H "Content-Type: application/json" \
  -d '{"timeout": 30, "max_results": 1}'

# Check status
curl https://localhost/api/qr/scan/{scan_id}

# Cancel scan
curl -X DELETE https://localhost/api/qr/scan/{scan_id}
```

### Test Client Integration

Open browser console on OOBE or supervisor page:

```javascript
const api = new SupervisorAPI();

// Test scan
const result = await api.scanQRCode(30);
console.log(result);
```

## Troubleshooting

### QR Reader Won't Start
```bash
# Check if camera-streamer is running
ps | grep camera-streamer

# Check camera stream
ls -la /dev/shm/video_stream_ch*

# Test QR reader manually
/usr/bin/qr-reader --timeout 5
```

### Scan Always Times Out
- Ensure QR code is well-lit and in focus
- Use larger QR codes (version 3+)
- Increase timeout
- Check camera positioning

### Schema Validation Fails
- Verify QR code contains valid JSON
- Check required fields match schema
- Test without schema first

### Supervisor API Errors
- Check supervisor logs for errors
- Verify qr-reader binary path
- Test binary directly from command line

## Security Considerations

1. **Rate Limiting**: Consider adding rate limits to prevent scan abuse
2. **Authentication**: QR endpoints use `requiresAuth=false` for OOBE access
3. **Process Cleanup**: Supervisor kills orphaned processes on restart
4. **Validation**: Always validate QR data before using it
5. **Timeouts**: Enforced both in binary and supervisor

## Performance

- **Scan time**: Typically 1-5 seconds for well-positioned QR codes
- **Resource usage**: Only during active scans (no background process)
- **Memory**: ~2-3 MB per scan process
- **Camera**: Shared with other consumers (no exclusive lock)

## Future Enhancements

- [ ] Live preview feed during scanning
- [ ] Multi-camera support
- [ ] QR code history/audit log
- [ ] Encrypted QR code support
- [ ] Batch scanning capabilities

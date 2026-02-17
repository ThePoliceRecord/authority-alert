# Supervisor API Reference for OOBE

This document provides a comprehensive guide to using the Supervisor's REST APIs from the OOBE (Out-of-Box Experience) application.

## Base Configuration

- **Supervisor HTTPS Port**: 443 (default) or configured via `HTTPSPort`
- **Supervisor HTTP Port**: 80 (default, redirects to HTTPS) or configured via `HTTPPort`
- **OOBE Server Port**: 8081 (HTTP only)
- **Base URL**: `https://<device-ip>` or `https://localhost` (from device itself)

## Authentication

Most API endpoints require authentication via JWT token, except for the following public endpoints:

### Public Endpoints (No Auth Required)
- `/api/version` - Get version and uptime
- `/api/channels` - Get video channel information
- `/api/userMgr/login` - User login
- `/api/deviceMgr/queryDeviceInfo` - Query device info (gets serial number)
- `/api/deviceMgr/queryServiceStatus` - Query service status
- `/api/deviceMgr/oauthCallback` - OAuth callback handler
- `/api/deviceMgr/generateCameraToken` - Generate camera token (OOBE only)

### Authentication Flow

1. **Login** (POST `/api/userMgr/login`)
   ```json
   Request:
   {
     "username": "admin",
     "password": "password123"
   }
   
   Response:
   {
     "code": 0,
     "msg": "success",
     "data": {
       "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
     }
   }
   ```

2. **Use Token in Subsequent Requests**
   ```
   Authorization: Bearer <token>
   ```

## API Categories

### 1. System Information

#### Get Version and Uptime
```
GET /api/version
```
**Auth Required**: No

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "uptime": 12345,
    "timestamp": 1704500000,
    "version": "1.0.0"
  }
}
```

**Use Case**: Check if supervisor is running and get system uptime.

---

### 2. User Management

#### Query User Info
```
GET /api/userMgr/queryUserInfo
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "userName": "recamera",
    "sshEnabled": false,
    "sshkeyList": []
  }
}
```

**Use Case**: Get user information including SSH status and keys.

#### Update Password
```
POST /api/userMgr/updatePassword
```
**Auth Required**: Yes

**Request**:
```json
{
  "oldPassword": "currentPassword",
  "newPassword": "newSecurePassword123"
}
```

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "Password updated successfully"
  }
}
```

**Use Case**: Change user password (requires authentication).

#### Set SSH Status
```
POST /api/userMgr/setSShStatus
```
**Auth Required**: Yes

**Request**:
```json
{
  "enabled": true
}
```

**Use Case**: Enable/disable SSH access during setup.

#### Add SSH Key
```
POST /api/userMgr/addSShkey
```
**Auth Required**: Yes

**Request**:
```json
{
  "key": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."
}
```

#### Delete SSH Key
```
POST /api/userMgr/deleteSShkey
```
**Auth Required**: Yes

**Request**:
```json
{
  "key": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."
}
```

---

### 3. Device Management

#### Query Device Info (Basic)
```
GET /api/deviceMgr/queryDeviceInfo
```
**Auth Required**: No

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "serialNumber": "SG2002-12345678",
    "deviceName": "reCamera"
  }
}
```

**Use Case**: Get device serial number for display during OOBE.

#### Get Device Info (Detailed)
```
GET /api/deviceMgr/getDeviceInfo
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "serialNumber": "SG2002-12345678",
    "deviceName": "reCamera",
    "model": "SG2002",
    "firmwareVersion": "1.0.0",
    "hardwareVersion": "1.0"
  }
}
```

#### Update Device Name
```
POST /api/deviceMgr/updateDeviceName
```
**Auth Required**: Yes

**Request**:
```json
{
  "deviceName": "My Authority Alert Camera"
}
```

**Use Case**: Allow user to set a friendly device name during OOBE.

#### Get System Status
```
GET /api/deviceMgr/getSystemStatus
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "cpuUsage": 45.2,
    "memoryUsage": 60.5,
    "diskUsage": 30.1,
    "temperature": 55.0
  }
}
```

#### Query Service Status
```
GET /api/deviceMgr/queryServiceStatus
```
**Auth Required**: No

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "services": [
      {"name": "supervisor", "status": "running"},
      {"name": "camera", "status": "running"}
    ]
  }
}
```

**Use Case**: Check if required services are running during OOBE.

#### Set Power (Reboot/Shutdown)
```
POST /api/deviceMgr/setPower
```
**Auth Required**: Yes

**Request**:
```json
{
  "action": "reboot"  // or "shutdown"
}
```

**Use Case**: Reboot device after OOBE completion.

---

### 4. Network Management (WiFi)

#### Get WiFi Info List
```
GET /api/wifiMgr/getWiFiInfoList
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "networks": [
      {
        "ssid": "MyNetwork",
        "signal": -45,
        "security": "WPA2",
        "connected": false
      },
      {
        "ssid": "OfficeWiFi",
        "signal": -60,
        "security": "WPA2",
        "connected": true
      }
    ]
  }
}
```

**Use Case**: Display available WiFi networks during OOBE setup.

#### Connect to WiFi
```
POST /api/wifiMgr/connectWiFi
```
**Auth Required**: Yes

**Request**:
```json
{
  "ssid": "MyNetwork",
  "password": "wifipassword123",
  "security": "WPA2"
}
```

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "connected": true,
    "ipAddress": "192.168.1.100"
  }
}
```

**Use Case**: Connect to WiFi network during OOBE.

#### Disconnect WiFi
```
POST /api/wifiMgr/disconnectWiFi
```
**Auth Required**: Yes

#### Forget WiFi
```
POST /api/wifiMgr/forgetWiFi
```
**Auth Required**: Yes

**Request**:
```json
{
  "ssid": "MyNetwork"
}
```

#### Switch WiFi
```
POST /api/wifiMgr/switchWiFi
```
**Auth Required**: Yes

**Request**:
```json
{
  "ssid": "NewNetwork",
  "password": "newpassword"
}
```

---

### 5. Time Configuration

#### Get Timestamp
```
GET /api/deviceMgr/getTimestamp
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "timestamp": 1704500000
  }
}
```

#### Set Timestamp
```
POST /api/deviceMgr/setTimestamp
```
**Auth Required**: Yes

**Request**:
```json
{
  "timestamp": 1704500000
}
```

**Use Case**: Set device time during OOBE if no NTP available.

#### Get Timezone
```
GET /api/deviceMgr/getTimezone
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "timezone": "America/Chicago"
  }
}
```

#### Set Timezone
```
POST /api/deviceMgr/setTimezone
```
**Auth Required**: Yes

**Request**:
```json
{
  "timezone": "America/Chicago"
}
```

#### Get Timezone List
```
GET /api/deviceMgr/getTimezoneList
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "timezones": [
      "America/New_York",
      "America/Chicago",
      "America/Denver",
      "America/Los_Angeles",
      "Europe/London",
      "Asia/Tokyo"
    ]
  }
}
```

**Use Case**: Display timezone picker during OOBE.

---

### 6. Platform Configuration

#### Get Platform Info
```
GET /api/deviceMgr/getPlatformInfo
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "platformUrl": "https://api.authority-alert.com",
    "apiKey": "encrypted-key",
    "deviceId": "device-123"
  }
}
```

**Use Case**: Check if device is already registered with cloud platform.

#### Save Platform Info
```
POST /api/deviceMgr/savePlatformInfo
```
**Auth Required**: Yes

**Request**:
```json
{
  "platformUrl": "https://api.authority-alert.com",
  "apiKey": "your-api-key",
  "deviceId": "device-123"
}
```

**Use Case**: Register device with Authority Alert cloud platform during OOBE.

---

### 7. Camera/Video

#### Get Channels
```
GET /api/channels
```
**Auth Required**: No

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "channels": [
      {
        "id": 0,
        "name": "Main Stream",
        "resolution": "1920x1080",
        "fps": 30
      }
    ]
  }
}
```

#### Get Camera WebSocket URL
```
GET /api/deviceMgr/getCameraWebsocketUrl
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "url": "wss://device-ip/ws/camera"
  }
}
```

**Use Case**: Get WebSocket URL for live camera preview during OOBE.

**Note**: The WebSocket endpoint `/ws/camera` bypasses authentication middleware for direct streaming.

---

### 8. Model Management

#### Get Model List
```
GET /api/deviceMgr/getModelList
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "models": [
      {
        "name": "yolo11n_detection_cv181x_int8",
        "type": "detection",
        "size": 5242880
      }
    ]
  }
}
```

#### Get Model Info
```
GET /api/deviceMgr/getModelInfo?name=yolo11n_detection_cv181x_int8
```
**Auth Required**: Yes

#### Upload Model
```
POST /api/deviceMgr/uploadModel
```
**Auth Required**: Yes
**Content-Type**: multipart/form-data

---

### 9. File Management

#### List Files
```
GET /api/fileMgr/list?path=/path/to/dir
```
**Auth Required**: Yes

#### Create Directory
```
POST /api/fileMgr/mkdir
```
**Auth Required**: Yes

**Request**:
```json
{
  "path": "/path/to/new/dir"
}
```

#### Remove File/Directory
```
POST /api/fileMgr/remove
```
**Auth Required**: Yes

**Request**:
```json
{
  "path": "/path/to/file"
}
```

#### Upload File
```
POST /api/fileMgr/upload
```
**Auth Required**: Yes
**Content-Type**: multipart/form-data

#### Download File
```
GET /api/fileMgr/download?path=/path/to/file
```
**Auth Required**: Yes

#### Rename File
```
POST /api/fileMgr/rename
```
**Auth Required**: Yes

**Request**:
```json
{
  "oldPath": "/path/to/old",
  "newPath": "/path/to/new"
}
```

#### Get File Info
```
GET /api/fileMgr/info?path=/path/to/file
```
**Auth Required**: Yes

---

### 10. LED Management

#### Get All LEDs
```
GET /api/ledMgr/getLEDs
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "leds": [
      {
        "name": "led0",
        "brightness": 255,
        "trigger": "none"
      }
    ]
  }
}
```

#### Get Single LED
```
GET /api/ledMgr/getLED?name=led0
```
**Auth Required**: Yes

#### Set LED
```
POST /api/ledMgr/setLED
```
**Auth Required**: Yes

**Request**:
```json
{
  "name": "led0",
  "brightness": 128,
  "trigger": "heartbeat"
}
```

**Use Case**: Provide visual feedback during OOBE (e.g., blink LED during WiFi connection).

#### Get LED Triggers
```
GET /api/ledMgr/getLEDTriggers
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "triggers": ["none", "heartbeat", "timer", "default-on"]
  }
}
```

---

### 11. System Update

#### Get System Update Version
```
GET /api/deviceMgr/getSystemUpdateVersion
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "currentVersion": "1.0.0",
    "latestVersion": "1.1.0",
    "updateAvailable": true
  }
}
```

#### Update System
```
POST /api/deviceMgr/updateSystem
```
**Auth Required**: Yes

**Request**:
```json
{
  "version": "1.1.0",
  "url": "https://ota-server/releases/latest/..."
}
```

#### Get Update Progress
```
GET /api/deviceMgr/getUpdateProgress
```
**Auth Required**: Yes

**Response**:
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "status": "downloading",
    "progress": 45
  }
}
```

#### Cancel Update
```
POST /api/deviceMgr/cancelUpdate
```
**Auth Required**: Yes

---

### 12. Maintenance

#### Factory Reset
```
POST /api/deviceMgr/factoryReset
```
**Auth Required**: Yes

**Warning**: This will erase all user data and settings.

#### Format SD Card
```
POST /api/deviceMgr/formatSDCard
```
**Auth Required**: Yes

---

## Typical OOBE Flow

Here's a recommended sequence of API calls for a complete OOBE experience:

### 1. Initial Setup Check
```javascript
// Check if supervisor is running
GET /api/version

// Get device serial number for display
GET /api/deviceMgr/queryDeviceInfo
```

### 2. User Login
```javascript
// Login with default or configured credentials
POST /api/userMgr/login
{
  "userName": "recamera",
  "password": "recamera"
}
// Save the returned token for subsequent requests
```

### 3. Device Configuration
```javascript
// Set device name
POST /api/deviceMgr/updateDeviceName
{
  "deviceName": "Living Room Camera"
}

// Set timezone
POST /api/deviceMgr/setTimezone
{
  "timezone": "America/Chicago"
}
```

### 4. Network Setup
```javascript
// Scan for WiFi networks
GET /api/wifiMgr/getWiFiInfoList

// Connect to selected network
POST /api/wifiMgr/connectWiFi
{
  "ssid": "HomeNetwork",
  "password": "wifipassword",
  "security": "WPA2"
}
```

### 5. Platform Registration
```javascript
// Register with Authority Alert cloud
POST /api/deviceMgr/savePlatformInfo
{
  "platformUrl": "https://api.authority-alert.com",
  "apiKey": "user-api-key",
  "deviceId": "generated-device-id"
}
```

### 6. Optional: Camera Preview
```javascript
// Get WebSocket URL for camera preview
GET /api/deviceMgr/getCameraWebsocketUrl

// Connect to WebSocket at returned URL
// wss://device-ip/ws/camera
```

### 7. Completion
```javascript
// Optional: Set LED to indicate completion
POST /api/ledMgr/setLED
{
  "name": "led0",
  "brightness": 255,
  "trigger": "heartbeat"
}

// Optional: Reboot to apply all settings
POST /api/deviceMgr/setPower
{
  "action": "reboot"
}
```

---

## Error Handling

All API responses follow this format:

**Success**:
```json
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}
```

**Error**:
```json
{
  "code": 1001,
  "msg": "Invalid credentials",
  "data": null
}
```

Common error codes:
- `1001`: Authentication failed
- `1002`: Invalid parameters
- `1003`: Resource not found
- `1004`: Permission denied
- `1005`: Internal server error

---

## CORS and Security

The supervisor includes CORS middleware that allows cross-origin requests from the OOBE server (port 8081).

For HTTPS connections, the supervisor uses self-signed certificates by default. Your OOBE application should:
1. Accept self-signed certificates (for local device communication)
2. Or provide a way to trust the device certificate

---

## Example: JavaScript Fetch from OOBE

```javascript
// Helper function to call supervisor API
async function callSupervisorAPI(endpoint, method = 'GET', body = null) {
  const baseUrl = 'https://localhost'; // or device IP
  const token = localStorage.getItem('authToken');
  
  const options = {
    method: method,
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  if (token) {
    options.headers['Authorization'] = `Bearer ${token}`;
  }
  
  if (body) {
    options.body = JSON.stringify(body);
  }
  
  try {
    const response = await fetch(`${baseUrl}${endpoint}`, options);
    const data = await response.json();
    
    if (data.code === 0) {
      return data.data;
    } else {
      throw new Error(data.msg);
    }
  } catch (error) {
    console.error('API call failed:', error);
    throw error;
  }
}

// Example usage
async function setupDevice() {
  // Login
  const loginData = await callSupervisorAPI('/api/userMgr/login', 'POST', {
    userName: 'recamera',
    password: 'recamera'
  });
  
  // Save token
  localStorage.setItem('authToken', loginData.token);
  
  // Continue with setup...
}
```

---

## Notes

1. **WebSocket Camera Proxy**: The `/ws/camera` endpoint bypasses authentication middleware for performance. Ensure your OOBE UI handles this appropriately.

2. **HTTPS Redirect**: The supervisor redirects all HTTP traffic to HTTPS. Your OOBE application should use HTTPS URLs when calling supervisor APIs.

3. **Token Expiration**: JWT tokens may expire. Implement token refresh logic or prompt for re-login.

4. **Service Dependencies**: Some APIs depend on underlying system services. Check service status with `/api/deviceMgr/queryServiceStatus` before calling dependent APIs.

5. **Platform Integration**: The platform info APIs are designed for cloud service integration. Ensure you have valid credentials before calling these endpoints.

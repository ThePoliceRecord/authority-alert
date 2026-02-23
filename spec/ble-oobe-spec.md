# BLE OOBE Protocol Specification

Complete spec for BLE-based camera discovery, WiFi provisioning, and full out-of-box setup. The iOS app performs the entire OOBE over BLE without switching WiFi networks.

**Version:** 2.1
**Date:** 2026-02-23
**Firmware ref:** `internal/ble/` package in Go supervisor

---

## 1. Overview

The camera setup wizard requires users to discover and configure the camera. iOS has no public API to scan nearby WiFi SSIDs, making auto-discovery impossible over WiFi alone. BLE solves this — CoreBluetooth can scan for nearby peripherals without prior pairing.

### What BLE Handles

Everything needed for OOBE:
- Camera discovery and identification
- Authentication (password-free during OOBE)
- Initial password setup
- Device naming
- Timezone and clock configuration
- WiFi provisioning (scan + connect)
- Platform registration (claim code flow)
- OOBE completion

### Coexistence

During OOBE, the camera runs **both** BLE advertising and the WiFi AP simultaneously. BLE is the primary path; the WiFi AP is a fallback for BLE failure. After OOBE completes, BLE advertising stops within ~5 seconds.

### Platform Requirements

- **iOS:** CoreBluetooth (iOS 17.6+). We do NOT use AccessorySetupKit (iOS 18+ only) to maintain compatibility.
- **Camera:** BlueZ 5 via D-Bus (`github.com/godbus/dbus/v5`), driven by Go supervisor

---

## 2. GATT Service & Characteristics

### Service UUID
```
AA01FC00-B5A3-F393-E0A9-E50E24DCCA9E
```

### Characteristics

| Name | UUID | Properties | Auth Required? |
|------|------|------------|----------------|
| Device Info | `AA01FC04-...` | Read | No |
| Session | `AA01FC01-...` | Write Without Response, Notify | No |
| WiFi Scan | `AA01FC02-...` | Write Without Response, Notify | Yes |
| WiFi Config | `AA01FC03-...` | Write Without Response, Notify | Yes |
| Setup | `AA01FC05-...` | Write Without Response, Notify | Mixed* |

\* Setup commands `get_timezone_list` and `complete` require no auth. All others require the session token.

### Swift UUIDs

```swift
enum CameraUUID {
    static let service    = CBUUID(string: "AA01FC00-B5A3-F393-E0A9-E50E24DCCA9E")
    static let deviceInfo = CBUUID(string: "AA01FC04-B5A3-F393-E0A9-E50E24DCCA9E")
    static let session    = CBUUID(string: "AA01FC01-B5A3-F393-E0A9-E50E24DCCA9E")
    static let wifiScan   = CBUUID(string: "AA01FC02-B5A3-F393-E0A9-E50E24DCCA9E")
    static let wifiConfig = CBUUID(string: "AA01FC03-B5A3-F393-E0A9-E50E24DCCA9E")
    static let setup      = CBUUID(string: "AA01FC05-B5A3-F393-E0A9-E50E24DCCA9E")
}
```

---

## 3. BLE Advertising

The camera advertises when OOBE is active (file `/etc/oobe/flag` exists). Once OOBE completes, advertising stops within 5 seconds.

| Field | Value |
|-------|-------|
| Local Name | `AA-XXXXXX` (last 6 hex of WiFi MAC, uppercase) |
| Service UUIDs | `AA01FC00-B5A3-F393-E0A9-E50E24DCCA9E` |
| Includes | `tx-power` |
| Type | `peripheral` |

The advertising name uses the same MAC suffix as the AP SSID (`AuthorityAlert-XXXXXX`).

**Scanning:**
```swift
centralManager.scanForPeripherals(
    withServices: [CameraUUID.service],
    options: [CBCentralManagerScanOptionAllowDuplicatesKey: false]
)
```

The service UUID filter ensures we only discover our cameras, not every BLE device in range.

---

## 4. Message Protocol

### Request (iOS → Camera)

All writes use **Write Without Response**. Wrap commands in this JSON envelope, then fragment:

```json
{
  "cmd": "command_name",
  "token": "jwt-token",
  "data": { ... }
}
```

- `cmd` — required, the command name
- `token` — omit for `auth`, `get_timezone_list`, `complete`; required for everything else
- `data` — command-specific payload, omit if none

### Response (Camera → iOS)

Responses arrive as BLE notifications. Same JSON envelope:

```json
{
  "code": 200,
  "msg": "human-readable",
  "data": { ... }
}
```

### Status Codes

| Code | Meaning | Used By |
|------|---------|---------|
| `200` | Success | All commands |
| `400` | Bad request (missing/invalid field) | All commands |
| `401` | Unauthorized (bad/expired token, wrong password) | `auth`, `scan`, `connect`, `status`, setup commands |
| `408` | Timeout | `connect`, `scan` |
| `500` | Internal error | All commands |

---

## 5. Fragmentation

BLE packets are limited by MTU. All messages (both directions) use a 1-byte framing header.

### Header Byte

```
Bit 7 (MSB): 1 = first fragment
Bit 6:       1 = last fragment
Bits 5-0:    sequence number (0-63)
```

| Scenario | Header |
|----------|--------|
| Single packet (fits in MTU) | `0xC0` (first + last, seq=0) |
| First of many | `0x80` (first, seq=0) |
| Middle | `0x01`, `0x02`, ... |
| Last | `0x43` (last, seq=3) |

### Sending (iOS → Camera)

```swift
func fragment(_ data: Data, mtu: Int) -> [Data] {
    let payloadSize = mtu - 1  // 1 byte for header
    if data.count <= payloadSize {
        return [Data([0xC0]) + data]  // single packet
    }
    var packets: [Data] = []
    var seq: UInt8 = 0
    var offset = 0
    while offset < data.count {
        let end = min(offset + payloadSize, data.count)
        var header: UInt8 = seq & 0x3F
        if offset == 0 { header |= 0x80 }          // first
        if end == data.count { header |= 0x40 }     // last
        packets.append(Data([header]) + data[offset..<end])
        offset = end
        seq += 1
    }
    return packets
}
```

Write each packet with `.withoutResponse`:
```swift
for packet in fragment(jsonData, mtu: peripheral.maximumWriteValueLength(for: .withoutResponse)) {
    peripheral.writeValue(packet, for: characteristic, type: .withoutResponse)
}
```

### Receiving (Camera → iOS)

Accumulate notification payloads. When you see bit 6 (last), concatenate and parse JSON:

```swift
class Reassembler {
    private var buffer = Data()

    func write(_ packet: Data) -> Data? {
        guard let header = packet.first else { return nil }
        let payload = packet.dropFirst()

        if header & 0x80 != 0 { buffer = Data() }  // first: reset
        buffer.append(payload)

        if header & 0x40 != 0 {  // last: return complete message
            let complete = buffer
            buffer = Data()
            return complete
        }
        return nil  // more fragments coming
    }
}
```

### MTU

- Default: 23 bytes (22 usable)
- iOS automatically negotiates higher MTU (typically 185-512)
- Use `peripheral.maximumWriteValueLength(for: .withoutResponse)` for outgoing
- Camera uses the negotiated ATT MTU for notifications
- Each characteristic needs its own `Reassembler` instance

---

## 6. Session Management

- **Idle timeout:** 60 seconds of no commands → session invalidated (checked every 10 seconds)
- **One session at a time** — new auth replaces any previous session
- **Token format:** JWT (same signing key as REST API)
- Treat any `401` response as "session expired, re-auth"

---

## 7. Complete OOBE Flow

```
Step  Char   Command              What Happens
────  ────   ───────              ────────────
 1    --     BLE scan             Discover camera advertising AA-XXXXXX
 2    --     Connect              Establish BLE connection
 3    FC04   Read                 Get device info (serial, firmware, oobe_state)
 4    FC01   auth                 Get JWT (no password needed during OOBE)
 5    FC05   set_password         User sets initial camera password
 6    FC05   set_device_name      User names the camera
 7    FC05   get_timezone_list    Get available timezones (for picker)
 8    FC05   set_timezone         User picks timezone
 9    FC05   set_timestamp        Sync clock from phone
10    FC02   scan                 Get nearby WiFi networks
11    FC03   connect              Camera joins home WiFi
12    FC03   status               (optional) Poll until wifi_state=connected
13    FC05   start_registration   Begin platform registration → get claim code
14    FC05   registration_status  Poll until status=claimed
15    FC05   complete             Mark OOBE done → BLE stops advertising
16    --     Disconnect           Done
```

### Notes on Flow

- **Step 4 (auth):** During OOBE, the camera skips password verification entirely. The `data` field is ignored — you can omit it or send `{"password":""}`. The camera issues a JWT regardless. This is safe because BLE only runs during OOBE. After OOBE, normal password auth applies.
- **Steps 5-9 can be reordered** as needed by the iOS UI.
- **Step 5 (set_password):** Creates `/etc/oobe/started` marker. This is the point of no return — if the device reboots before OOBE completes, it factory resets.
- **Step 11 (connect)** triggers async WiFi connection. You get an immediate `"connecting"` ack, then a `"connected"` or `"connection timeout"` notification. The camera sends **two** notifications on the same characteristic.
- **Step 13 (start_registration):** Camera registers with the platform and generates a claim code. The iOS app displays this code; the user enters it on the platform website/app to claim the camera.
- **Step 14:** Poll every 3-5 seconds. Status progresses: `generating` → `active` → `claimed`. The `claimed` status means registration is complete.
- **Step 15 (complete):** Removes the OOBE flag. The camera stops BLE advertising within 5 seconds and becomes a normal operational camera.

---

## 8. Command Reference

### 8.1 Device Info (FC04) — Read

No write needed. Just read the characteristic value.

**Response:**
```json
{
  "serial": "RC200X-A1B2C3",
  "firmware": "0.2.9",
  "ap_ssid": "AuthorityAlert-1A2B3C",
  "ap_ip": "192.168.16.1",
  "oobe_state": "pending",
  "proto_ver": 1,
  "model": "reCamera 200x"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `serial` | string | Camera serial number (from `system.GetSerialNumber()`) |
| `firmware` | string | OS version (from `system.GetOSVersion()`) |
| `ap_ssid` | string | WiFi AP SSID (for fallback path) |
| `ap_ip` | string | WiFi AP IP (`192.168.16.1`) |
| `oobe_state` | string | `"pending"` or `"complete"` |
| `proto_ver` | int | Protocol version. Current: `1` |
| `model` | string | Hardware model (`"reCamera 200x"`) |

**Important:** Check `oobe_state`. If `"complete"`, the camera is already set up — BLE OOBE flow is not needed.

---

### 8.2 `auth` (FC01) — Session

**Write to:** Session characteristic

**Request:**
```json
{
  "cmd": "auth",
  "data": {
    "password": "anything"
  }
}
```

During OOBE, the password is ignored — the camera returns a JWT without verification. The `data` field can be omitted entirely. Send `{"password": "..."}` for forward compatibility with post-OOBE auth.

**Success (200):**
```json
{
  "code": 200,
  "msg": "authenticated",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "ap_ip": "192.168.16.1"
  }
}
```

Save the `token` — include it in all subsequent commands.

**Errors:**
| Code | Msg | When |
|------|-----|------|
| `400` | `"password required"` | Missing `data.password` (post-OOBE only) |
| `401` | `"invalid password"` | Wrong password (post-OOBE only) |
| `500` | `"token generation failed"` | Internal error |

---

### 8.3 `scan` (FC02) — WiFi Scan

**Write to:** WiFi Scan characteristic

**Request:**
```json
{
  "cmd": "scan",
  "token": "jwt-token"
}
```

**Success (200):**
```json
{
  "code": 200,
  "msg": "scan complete",
  "data": {
    "networks": [
      {
        "ssid": "MyHomeWiFi",
        "signal": -45,
        "security": "WPA2",
        "channel": 6
      }
    ]
  }
}
```

- Sorted by signal strength (strongest first)
- `signal`: RSSI in dBm
- `security`: `"Open"`, `"WEP"`, `"WPA"`, `"WPA2"`, `"WPA3"`
- `channel`: WiFi channel number (derived from frequency)
- Hidden networks (empty SSID) are filtered out
- Response may span multiple BLE packets (use reassembler)

**Errors:**
| Code | Msg | When |
|------|-----|------|
| `401` | `"unauthorized"` | Bad/expired token |
| `408` | `"scan timeout"` | Scan took >15 seconds |
| `500` | `"scan failed: ..."` | wpa_supplicant error |

---

### 8.4 `connect` (FC03) — WiFi Connect

**Write to:** WiFi Config characteristic

**Request:**
```json
{
  "cmd": "connect",
  "token": "jwt-token",
  "data": {
    "ssid": "MyHomeWiFi",
    "password": "wifi-password",
    "security": "WPA2"
  }
}
```

**Response sequence (two notifications):**

1. **Immediate ack:**
```json
{ "code": 200, "msg": "connecting" }
```

2. **Final result** (up to 30 seconds later):
```json
{
  "code": 200,
  "msg": "connected",
  "data": { "ip": "192.168.1.42", "ssid": "MyHomeWiFi" }
}
```
or
```json
{ "code": 408, "msg": "connection timeout" }
```
or
```json
{ "code": 500, "msg": "connect failed: ..." }
```

**Important:** The camera does NOT drop its WiFi AP during this process. BLE remains operational throughout.

**Errors:**
| Code | Msg | When |
|------|-----|------|
| `400` | `"ssid required"` | Missing `data.ssid` |
| `401` | `"unauthorized"` | Bad/expired token |

---

### 8.5 `status` (FC03) — WiFi Status

**Write to:** WiFi Config characteristic

**Request:**
```json
{
  "cmd": "status",
  "token": "jwt-token"
}
```

**Response:**
```json
{
  "code": 200,
  "msg": "ok",
  "data": {
    "wifi_state": "connected",
    "ssid": "MyHomeWiFi",
    "ip": "192.168.1.42"
  }
}
```

`wifi_state` values: `"disconnected"`, `"connecting"`, `"connected"`

The `"connecting"` state maps to wpa_supplicant states: SCANNING, ASSOCIATING, ASSOCIATED, 4WAY_HANDSHAKE, GROUP_HANDSHAKE.

---

### 8.6 `set_password` (FC05) — Set Initial Password

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "set_password",
  "token": "jwt-token",
  "data": {
    "password": "user-chosen-password"
  }
}
```

**Validation:** Minimum 8 characters.

**Success (200):**
```json
{
  "code": 200,
  "msg": "password set",
  "data": { "message": "password set" }
}
```

**Side effects:**
- Sets the Linux system password for the `recamera` user (via `passwd`)
- Creates `/etc/oobe/started` marker (point of no return — if device reboots before OOBE completes, it factory resets)

**Errors:**
| Code | Msg |
|------|-----|
| `400` | `"password required"` |
| `400` | `"password must be at least 8 characters"` |
| `401` | `"unauthorized"` |
| `500` | `"failed to set password"` |

---

### 8.7 `set_device_name` (FC05) — Name the Camera

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "set_device_name",
  "token": "jwt-token",
  "data": {
    "name": "Front Porch Camera"
  }
}
```

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": { "name": "Front Porch Camera" }
}
```

**Errors:**
| Code | Msg |
|------|-----|
| `400` | `"name required"` |
| `401` | `"unauthorized"` |
| `500` | `"failed to set device name"` |

---

### 8.8 `get_timezone_list` (FC05) — Get Timezones

**Write to:** Setup characteristic. **No auth required.**

**Request:**
```json
{
  "cmd": "get_timezone_list"
}
```

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": {
    "timezones": [
      "UTC",
      "America/New_York",
      "America/Los_Angeles",
      "America/Chicago",
      "Europe/London",
      "Europe/Paris",
      "Europe/Berlin",
      "Asia/Tokyo",
      "Asia/Shanghai",
      "Asia/Singapore",
      "Australia/Sydney",
      "Pacific/Auckland"
    ]
  }
}
```

This is a fixed list of common timezones. Only timezones with a valid zoneinfo file on the camera are included.

---

### 8.9 `set_timezone` (FC05) — Set Timezone

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "set_timezone",
  "token": "jwt-token",
  "data": {
    "timezone": "America/New_York"
  }
}
```

**Validation:** Must be a valid IANA timezone name (letters, digits, `/`, `_`, `-`, `+` only; no `..`, no leading `/`, max 100 chars) with a corresponding file in `/usr/share/zoneinfo/`.

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": { "timezone": "America/New_York" }
}
```

**Side effect:** Symlinks `/etc/localtime` to the zoneinfo file.

**Errors:**
| Code | Msg |
|------|-----|
| `400` | `"timezone required"` |
| `400` | `"invalid timezone format"` |
| `400` | `"invalid timezone"` |
| `401` | `"unauthorized"` |
| `500` | `"failed to set timezone"` |

---

### 8.10 `set_timestamp` (FC05) — Sync Clock

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "set_timestamp",
  "token": "jwt-token",
  "data": {
    "timestamp": 1740268800
  }
}
```

`timestamp` is Unix epoch seconds. Use `Date().timeIntervalSince1970` (truncated to integer).

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": { "timestamp": 1740268800 }
}
```

**Side effects:** Sets system clock via `date -s` and persists to hardware clock via `hwclock -w`.

**Errors:**
| Code | Msg |
|------|-----|
| `400` | `"timestamp required"` |
| `401` | `"unauthorized"` |
| `500` | `"failed to set timestamp"` |

---

### 8.11 `start_registration` (FC05) — Begin Platform Registration

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "start_registration",
  "token": "jwt-token",
  "data": {
    "location_name": "Front Porch",
    "lat": 37.7749,
    "lon": -122.4194
  }
}
```

- `location_name` — required, user-friendly name for the camera location
- `lat`, `lon` — optional, GPS coordinates from phone

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": {
    "status": "generating",
    "message": "Registering code with platform...",
    "claim_code": "ABK7N3",
    "claim_code_formatted": "ABK 7N3",
    "expires_at": "2026-02-22T12:10:00Z",
    "started_at": "2026-02-22T12:00:00Z"
  }
}
```

**Key fields:**
- `claim_code` — 6-character code (letters + digits, no ambiguous chars like 0/O/1/I/L)
- `claim_code_formatted` — display-friendly version with space in middle ("ABK 7N3")
- `expires_at` — code expires after 10 minutes
- `status` — starts as `"generating"`, transitions to `"active"` once registered with platform

**If registration is already in progress**, the camera returns the current state instead of starting a new one.

**Errors:**
| Code | Msg |
|------|-----|
| `400` | `"location_name required"` |
| `401` | `"unauthorized"` |
| `500` | varies (registration failure details) |

---

### 8.12 `registration_status` (FC05) — Poll Registration

**Write to:** Setup characteristic

**Request:**
```json
{
  "cmd": "registration_status",
  "token": "jwt-token"
}
```

**Response (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": {
    "status": "active",
    "message": "Code ready. Waiting for user to claim...",
    "claim_code": "ABK7N3",
    "claim_code_formatted": "ABK 7N3",
    "expires_at": "2026-02-22T12:10:00Z",
    "started_at": "2026-02-22T12:00:00Z",
    "internet_available": true
  }
}
```

### Status State Machine

```
idle → generating → active → claimed
                  ↘ error
                  ↘ expired
```

| Status | Meaning | iOS Action |
|--------|---------|------------|
| `"idle"` | No registration in progress | Call `start_registration` |
| `"generating"` | Registering code with platform | Show spinner, keep polling |
| `"active"` | Code registered, waiting for user to claim on platform | Display claim code, keep polling |
| `"claimed"` | User claimed the camera on the platform | Proceed to `complete` |
| `"expired"` | Code expired (10 min) | Call `start_registration` again |
| `"error"` | Registration failed | Show error, offer retry |

**Additional fields on `"active"` / `"generating"`:**
- `internet_available` — `false` if camera lost internet (WiFi disconnected)
- `last_error` — present if there was a transient error
- `retry_count` — number of consecutive retries (resets on success)

**Additional field on `"claimed"`:**
- `result` — registration data (platform URL, camera UID, etc.)

**Poll interval:** Every 3-5 seconds.

---

### 8.13 `complete` (FC05) — Finish OOBE

**Write to:** Setup characteristic. **No auth required.**

**Request:**
```json
{
  "cmd": "complete"
}
```

**Success (200):**
```json
{
  "code": 200,
  "msg": "ok",
  "data": { "message": "setup complete" }
}
```

**Side effects:**
- Removes `/etc/oobe/flag`
- Camera stops BLE advertising within ~5 seconds (via `oobeLoop`)
- Camera becomes fully operational (web UI accessible, REST API requires password auth)

**Errors:**
| Code | Msg |
|------|-----|
| `500` | `"failed to complete OOBE"` |

---

## 9. Security

### OOBE Auth Bypass

During OOBE, BLE auth skips password verification and issues a JWT directly. This is intentional:

1. No password exists yet on a fresh device
2. BLE only runs during OOBE (`/etc/oobe/flag` must exist)
3. BLE advertising stops as soon as OOBE completes
4. Password is set via `set_password` before any sensitive operations

The auth bypass is self-gating: `Service.Start()` returns immediately if `isOOBEActive()` is false, and `oobeLoop()` stops the service within 5 seconds of the flag being removed.

### Threat Model

BLE "Just Works" pairing provides encrypted transport but no MITM protection. This is acceptable because:
- The camera is headless (no display or keyboard) — Passkey/Numeric Comparison is impossible
- Application-level auth (JWT) provides access control on all commands except Device Info read
- Provisioning only runs during OOBE, reducing the attack window
- The camera is mains-powered and in physical proximity during setup

### Session Limits

| Limit | Value | Behavior |
|-------|-------|----------|
| Concurrent sessions | 1 | New auth invalidates previous session |
| Idle timeout | 60 seconds | JWT invalidated, checked every 10s |
| Token scope | BLE session only | JWT validated against `bleSession.token`, not the REST API session |

### Data in Transit

- Camera password sent once during `set_password` (over BLE encrypted link)
- WiFi password sent once during `connect` (over BLE encrypted link)
- No sensitive data stored in BLE characteristics persistently

---

## 10. Error Handling

### Session Expired

Any command returning `401` means the session is gone. Re-authenticate:

```swift
if response.code == 401 {
    // Re-auth, then retry the failed command
    authenticate { token in
        retryCommand(with: token)
    }
}
```

### BLE Disconnect Mid-Flow

If `didDisconnectPeripheral` fires during OOBE:
1. Try reconnecting (the camera is still advertising if OOBE isn't complete)
2. On reconnect, re-auth and check state:
   - Read Device Info — is `oobe_state` still `"pending"`?
   - If registration was in progress, poll `registration_status` to resume
3. If reconnection fails after 3 attempts, offer WiFi AP fallback

### WiFi Connection During Registration

After `connect` succeeds, the camera has internet access. The `start_registration` command depends on this. If WiFi connect fails, registration will fail too.

Suggested order: WiFi connect → confirm connected → start registration.

### Registration Polling Resilience

The camera handles network flakiness internally:
- If internet drops during registration, the camera retries with exponential backoff
- The `internet_available` field tells iOS whether to show a connectivity warning
- The claim code remains valid for its full 10-minute window regardless of transient failures

### Camera-Side Timeouts

| Timeout | Duration | Action |
|---------|----------|--------|
| Idle session | 60 seconds | Invalidate JWT |
| WiFi connection attempt | 30 seconds | Send `code: 408` notify |
| WiFi scan | 15 seconds | Send `code: 408` notify |

### iOS-Side Error Handling

| Condition | Detection | Response |
|-----------|-----------|----------|
| Bluetooth off | `centralManagerDidUpdateState` → `.poweredOff` | Show alert + fallback to WiFi AP |
| Bluetooth unauthorized | `.unauthorized` state | Show alert linking to Settings + fallback |
| No cameras found | 10s scan timeout | Show "No cameras found" + retry + WiFi AP fallback |
| Connection failed | `didFailToConnect` | Retry once, then show error + WiFi AP fallback |
| Unexpected disconnect | `didDisconnectPeripheral` | If mid-flow: reconnect or WiFi AP fallback |
| Auth failed | `code: 401` on auth response | Show "Wrong password" + re-prompt (post-OOBE only) |
| WiFi connect timeout | `code: 408` on connect response | Show error + retry with different network |

---

## 11. Sequence Diagram

```
 iOS App                                     Camera
    │                                           │
    ├── BLE scan (service UUID filter) ────────>│  Advertising AA-XXXXXX
    │                                           │
    ├── Connect ───────────────────────────────>│
    ├── Discover services + characteristics ───>│
    ├── Subscribe to FC01, FC02, FC03, FC05 ───>│  (enable notifications)
    │                                           │
    ├── Read FC04 (Device Info) ───────────────>│
    │<── { serial, firmware, oobe_state } ──────│
    │                                           │
    │   [oobe_state == "pending"]               │
    │                                           │
    ├── FC01: { cmd: "auth" } ─────────────────>│
    │<── { code: 200, token: "..." } ───────────│  (OOBE bypass, no password check)
    │                                           │
    │   [User enters password]                  │
    ├── FC05: { cmd: "set_password" } ─────────>│
    │<── { code: 200 } ────────────────────────│
    │                                           │
    │   [User names camera]                     │
    ├── FC05: { cmd: "set_device_name" } ──────>│
    │<── { code: 200 } ────────────────────────│
    │                                           │
    ├── FC05: { cmd: "get_timezone_list" } ────>│
    │<── { timezones: [...] } ─────────────────│
    │                                           │
    │   [User picks timezone]                   │
    ├── FC05: { cmd: "set_timezone" } ─────────>│
    │<── { code: 200 } ────────────────────────│
    │                                           │
    ├── FC05: { cmd: "set_timestamp" } ────────>│
    │<── { code: 200 } ────────────────────────│
    │                                           │
    ├── FC02: { cmd: "scan" } ─────────────────>│
    │<── { networks: [...] } ──────────────────│  (may be multi-fragment)
    │                                           │
    │   [User picks WiFi + enters password]     │
    ├── FC03: { cmd: "connect" } ──────────────>│
    │<── { code: 200, msg: "connecting" } ──────│  (ack)
    │       ...                                 │  (camera connecting to WiFi)
    │<── { code: 200, msg: "connected" } ───────│  (camera has internet now)
    │                                           │
    │   [User enters location name]             │
    ├── FC05: { cmd: "start_registration" } ───>│
    │<── { claim_code: "ABK7N3" } ─────────────│
    │                                           │
    │   [Display claim code to user]            │
    │   [User enters code on platform]          │
    │                                           │
    ├── FC05: { cmd: "registration_status" } ──>│  (poll every 3-5s)
    │<── { status: "active" } ─────────────────│
    │       ...                                 │
    ├── FC05: { cmd: "registration_status" } ──>│
    │<── { status: "claimed" } ────────────────│
    │                                           │
    ├── FC05: { cmd: "complete" } ─────────────>│
    │<── { code: 200, message: "setup complete"}│
    │                                           │
    ├── Disconnect BLE ────────────────────────>│  (camera stops advertising)
    │                                           │
```

---

## 12. Firmware Implementation Reference

### Source Files

```
supervisor/internal/ble/
├── service.go     # BLE lifecycle, GATT callbacks, device info, idle/OOBE loops
├── protocol.go    # GATT UUIDs, Request/Response types, Fragment(), Reassembler
├── wifi.go        # auth, scan, connect, status command handlers
├── setup.go       # FC05 handlers: password, name, timezone, timestamp, registration, complete
└── bluez.go       # D-Bus types, GATT/advertisement export, BlueZ registration
```

### Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/godbus/dbus/v5` | D-Bus client for BlueZ interaction |
| `supervisor/internal/auth` | `AuthManager.GenerateToken()` (OOBE), `AuthManager.Authenticate()` (post-OOBE) |
| `supervisor/internal/network` | `WiFiManager.Scan()`, `.Connect()`, `.GetStatus()`, `.GetAPConfig()` |
| `supervisor/internal/device` | `UpdateDeviceName()` |
| `supervisor/internal/system` | `GetSerialNumber()`, `GetOSVersion()`, `GetMAC()`, `IsProcessRunning()` |

### Constructor

```go
ble.NewService(authMgr *auth.AuthManager, wifiMgr *network.WiFiManager, regSvc RegistrationService) *Service
```

`RegistrationService` is an interface implemented by `DeviceHandler`:
```go
type RegistrationService interface {
    StartRegistrationDirect(locationName string, lat, lon float64) (map[string]interface{}, error)
    GetRegistrationStatusDirect() map[string]interface{}
}
```

### BlueZ Requirements

- `bluetoothd` running (started automatically by `ensureBluetoothd()` if not present)
- D-Bus system bus available (`/var/run/dbus/system_bus_socket`)
- BLE adapter present (`/sys/class/bluetooth/hci0` — waits up to 60 seconds)
- Adapter powered on (set automatically via D-Bus)

---

## 13. iOS Implementation Checklist

### CoreBluetooth Setup

- [ ] Add `NSBluetoothAlwaysUsageDescription` to Info.plist
- [ ] Do NOT add `bluetooth-central` background mode (OOBE is foreground-only)
- [ ] Minimum iOS 17.6 (CoreBluetooth, no AccessorySetupKit)

### Connection Lifecycle

- [ ] Scan with service UUID filter
- [ ] On connect: discover services → discover characteristics → subscribe to notifications on FC01, FC02, FC03, FC05
- [ ] Read FC04 immediately after discovery
- [ ] Handle `didDisconnectPeripheral` — attempt reconnect if OOBE not complete
- [ ] Handle `centralManagerDidUpdateState` — `.poweredOff`, `.unauthorized`

### Fragmentation

- [ ] Implement `fragment()` for outgoing writes
- [ ] Implement `Reassembler` for incoming notifications
- [ ] Use `peripheral.maximumWriteValueLength(for: .withoutResponse)` as MTU
- [ ] Each characteristic needs its own `Reassembler` instance

### State Machine

- [ ] Track OOBE progress: `discovered` → `connected` → `authenticated` → `passwordSet` → `named` → `timeConfigured` → `wifiConnected` → `registering` → `claimed` → `complete`
- [ ] Allow going back to previous steps (re-enter password, re-pick WiFi, etc.)
- [ ] On reconnect, determine which step to resume from

### Registration UI

- [ ] Display `claim_code_formatted` (e.g., "ABK 7N3") prominently
- [ ] Show countdown timer based on `expires_at`
- [ ] Show internet status warning if `internet_available` is `false`
- [ ] Auto-advance when `status` becomes `"claimed"`
- [ ] Handle `"expired"` — offer to generate a new code

---

## Appendix A: UUID Summary

| Component | Full UUID | Short |
|-----------|-----------|-------|
| Service | `AA01FC00-B5A3-F393-E0A9-E50E24DCCA9E` | `FC00` |
| Session | `AA01FC01-B5A3-F393-E0A9-E50E24DCCA9E` | `FC01` |
| WiFi Scan | `AA01FC02-B5A3-F393-E0A9-E50E24DCCA9E` | `FC02` |
| WiFi Config | `AA01FC03-B5A3-F393-E0A9-E50E24DCCA9E` | `FC03` |
| Device Info | `AA01FC04-B5A3-F393-E0A9-E50E24DCCA9E` | `FC04` |
| Setup | `AA01FC05-B5A3-F393-E0A9-E50E24DCCA9E` | `FC05` |

All UUIDs share a common base (`AA01xxxx-B5A3-F393-E0A9-E50E24DCCA9E`) with the 16-bit short UUID in positions 5-8.

---

## Appendix B: Fragmentation Example

Sending a WiFi scan result of ~600 bytes over a 185-byte MTU (184 bytes usable after 1-byte header):

```
Packet 1: [0x80] + 184 bytes of JSON    (first, seq=0)
Packet 2: [0x01] + 184 bytes of JSON    (middle, seq=1)
Packet 3: [0x02] + 184 bytes of JSON    (middle, seq=2)
Packet 4: [0x43] +  48 bytes of JSON    (last, seq=3)
```

Receiver reassembles: strips 1-byte header from each packet, concatenates payloads, parses JSON.

---

## Appendix C: Codable Models

```swift
struct BLERequest: Encodable {
    let cmd: String
    var token: String?
    var data: AnyCodable?
}

struct BLEResponse: Decodable {
    let code: Int
    let msg: String
    var data: AnyCodable?
}

struct DeviceInfo: Decodable {
    let serial: String
    let firmware: String
    let apSsid: String
    let apIp: String
    let oobeState: String
    let protoVer: Int
    let model: String

    enum CodingKeys: String, CodingKey {
        case serial, firmware, model
        case apSsid = "ap_ssid"
        case apIp = "ap_ip"
        case oobeState = "oobe_state"
        case protoVer = "proto_ver"
    }
}

struct WiFiNetwork: Decodable {
    let ssid: String
    let signal: Int
    let security: String
    let channel: Int
}

struct RegistrationStatus: Decodable {
    let status: String
    let message: String
    var claimCode: String?
    var claimCodeFormatted: String?
    var expiresAt: String?
    var startedAt: String?
    var internetAvailable: Bool?
    var lastError: String?
    var retryCount: Int?
    var result: [String: AnyCodable]?

    enum CodingKeys: String, CodingKey {
        case status, message, result
        case claimCode = "claim_code"
        case claimCodeFormatted = "claim_code_formatted"
        case expiresAt = "expires_at"
        case startedAt = "started_at"
        case internetAvailable = "internet_available"
        case lastError = "last_error"
        case retryCount = "retry_count"
    }
}
```

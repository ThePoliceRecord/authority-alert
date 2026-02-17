# Authority Alert OOBE (Out-of-Box Experience)

A lightweight web-based setup wizard for Authority Alert devices, designed to guide users through initial device configuration.

## Overview

The OOBE service provides a user-friendly setup interface that runs on port 8081 and communicates with the Supervisor service (port 443) to configure the device.

## Features

- **Welcome Screen**: Displays device information and serial number
- **Password Setup**: Secure first-time password configuration
- **Device Configuration**: Set device name and timezone
- **WiFi Setup**: Scan and connect to wireless networks
- **Completion**: Summary and redirect to main interface

## Architecture

```
┌─────────────────┐         ┌──────────────────┐
│  OOBE Server    │         │   Supervisor     │
│  (Port 8081)    │────────▶│   (Port 443)     │
│  HTTPS          │  HTTPS  │   HTTPS          │
└─────────────────┘         └──────────────────┘
        │                           │
        └───────────────────────────┘
         Shared TLS Certificates
         (/etc/supervisor/certs/)
        │
        │ Serves
        ▼
┌─────────────────┐
│  Web Interface  │
│  - HTML/CSS/JS  │
│  - API Client   │
└─────────────────┘
```

## Components

### Backend (C++)
- **oobe_server.cpp**: Mongoose-based HTTPS server
  - Serves static web files from `/usr/share/oobe/www`
  - Provides health check endpoint at `/api/health`
  - Runs on port 8081 with HTTPS
  - Uses supervisor's TLS certificates for secure communication

### Frontend (Web)
- **index.html**: Main setup wizard interface
- **alerts.html**: Placeholder for alert management
- **css/style.css**: Modern, responsive styling
- **js/supervisor-api.js**: API client for supervisor communication
- **js/oobe-app.js**: Setup wizard logic and flow control

## API Integration

The OOBE application uses the Supervisor's REST APIs for all device configuration. See [`SUPERVISOR_API_REFERENCE.md`](SUPERVISOR_API_REFERENCE.md) for complete API documentation.

### Key APIs Used

1. **System Information**
   - `GET /api/version` - Check supervisor status
   - `GET /api/deviceMgr/queryDeviceInfo` - Get device serial number

2. **User Management**
   - `GET /api/userMgr/queryUserInfo` - Check first login status
   - `POST /api/userMgr/updatePassword` - Set new password
   - `POST /api/userMgr/login` - Authenticate user

3. **Device Configuration**
   - `POST /api/deviceMgr/updateDeviceName` - Set device name
   - `POST /api/deviceMgr/setTimezone` - Configure timezone
   - `GET /api/deviceMgr/getTimezoneList` - Get available timezones

4. **WiFi Management**
   - `GET /api/wifiMgr/getWiFiInfoList` - Scan networks
   - `POST /api/wifiMgr/connectWiFi` - Connect to network

5. **LED Control**
   - `POST /api/ledMgr/setLED` - Visual feedback during setup

## Setup Flow

```
1. Welcome
   ├─ Display device serial number
   └─ Check supervisor connectivity

2. Password Setup
   ├─ Check if first login
   ├─ Set new password (if first time)
   └─ Login and obtain auth token

3. Device Configuration
   ├─ Set device name
   └─ Select timezone

4. WiFi Setup
   ├─ Scan available networks
   ├─ Select network
   ├─ Enter password
   └─ Connect

5. Completion
   ├─ Display summary
   ├─ Set LED indicator
   └─ Redirect to supervisor UI
```

## Building

### Prerequisites
- RISC-V cross-compilation toolchain
- CMake 3.10+
- Mongoose library (included in dependencies)

### Build Commands

```bash
# Build the OOBE server binary
make build-riscv

# Create opkg package for deployment
make opkg

# Clean build artifacts
make clean
```

### Build Output

- **Binary**: `build/oobe`
- **Package**: `build/oobe_0.1.0_riscv64.ipk`

## Installation

The OOBE service is packaged as an opkg package and installed to:

- **Binary**: `/usr/local/bin/oobe`
- **Web Files**: `/usr/share/oobe/www/`
- **Init Script**: `/etc/init.d/S94oobe`

### Manual Installation

```bash
# Install the package
opkg install oobe_0.1.0_riscv64.ipk

# Start the service
/etc/init.d/S94oobe start

# Check status
ps | grep oobe
```

## Usage

### Accessing OOBE

1. Connect to the device's network (WiFi AP or Ethernet)
2. Open a web browser
3. Navigate to: `https://<device-ip>:8081`

**Note**: You may see a certificate warning since the device uses a self-signed certificate. This is normal - click "Advanced" and "Proceed" to continue.

### Default Credentials

- **Username**: `admin`
- **Default Password**: `admin` (must be changed on first login)

## Development

### Testing Locally

You can test the web interface locally by serving the `web/` directory:

```bash
# Using Python with HTTPS (requires certificate)
cd web
python3 -m http.server 8081

# Using Node.js
cd web
npx http-server -p 8081 --ssl
```

**Note**: Local testing requires:
1. A running Supervisor instance to connect to
2. Valid TLS certificates (or disable certificate validation for testing)

### API Client

The `SupervisorAPI` class in `supervisor-api.js` provides a clean interface:

```javascript
const api = new SupervisorAPI('https://localhost');

// Check version
const version = await api.getVersion();

// Login
const result = await api.login('admin', 'password');

// Configure device
await api.updateDeviceName('My Camera');
await api.setTimezone('America/Chicago');

// Connect WiFi
await api.connectWiFi('MyNetwork', 'password123', 'WPA2');
```

## Configuration

### Server Configuration

The OOBE server accepts command-line arguments:

```bash
oobe --listen https://0.0.0.0:8081 \
     --root /usr/share/oobe/www \
     --cert /etc/supervisor/certs/cert.pem \
     --key /etc/supervisor/certs/key.pem
```

Options:
- `--listen`: Listen address (default: `https://0.0.0.0:8081`)
- `--root`: Web root directory (default: `/usr/share/oobe/www`)
- `--cert`: TLS certificate file (default: `/etc/supervisor/certs/cert.pem`)
- `--key`: TLS key file (default: `/etc/supervisor/certs/key.pem`)
- `-h, --help`: Show help message

**Note**: The OOBE server uses the same TLS certificates as the supervisor to avoid mixed content issues and provide a seamless secure experience.

### Init Script

The service is managed by the init script at `/etc/init.d/S94oobe`:

```bash
# Start service
/etc/init.d/S94oobe start

# Stop service
/etc/init.d/S94oobe stop

# Restart service
/etc/init.d/S94oobe restart
```

## Security Considerations

1. **HTTPS Everywhere**: Both OOBE and supervisor use HTTPS with shared certificates
2. **Self-Signed Certificates**: The device uses self-signed certificates by default
3. **Token-Based Auth**: JWT tokens are used for authenticated API calls
4. **Password Requirements**: Minimum 8 characters enforced
5. **First Login**: Default password must be changed on first use
6. **Same-Origin Security**: OOBE and supervisor share the same certificate, eliminating mixed content issues

## Troubleshooting

### OOBE Won't Start

```bash
# Check if service is running
ps | grep oobe

# Check logs
logread | grep oobe

# Restart service
/etc/init.d/S94oobe restart
```

### Cannot Connect to Supervisor

1. Verify supervisor is running: `ps | grep supervisor`
2. Check supervisor port: `netstat -tlnp | grep 443`
3. Test connectivity: `curl -k https://localhost/api/version`
4. Verify certificates exist: `ls -l /etc/supervisor/certs/`

### WiFi Connection Fails

1. Check WiFi adapter: `ifconfig`
2. Verify network credentials
3. Check signal strength
4. Review supervisor logs: `logread | grep wifi`

### Browser Shows Certificate Warning

This is normal for self-signed certificates. Click "Advanced" and "Proceed" to accept the certificate. You only need to do this once per browser session.

**Why this happens:**
- The device generates its own TLS certificate on first boot
- This certificate is not signed by a trusted Certificate Authority
- Both OOBE and supervisor use the same certificate for consistency

## File Structure

```
oobe/
├── CMakeLists.txt              # Build configuration
├── Makefile                    # Build targets
├── README.md                   # This file
├── SUPERVISOR_API_REFERENCE.md # API documentation
├── main/
│   ├── CMakeLists.txt
│   └── oobe_server.cpp         # Server implementation
├── web/
│   ├── index.html              # Main setup wizard
│   ├── alerts.html             # Alerts page
│   ├── css/
│   │   └── style.css           # Styles
│   └── js/
│       ├── supervisor-api.js   # API client
│       └── oobe-app.js         # Application logic
├── rootfs/
│   └── etc/
│       └── init.d/
│           └── S94oobe         # Init script
└── opkg/
    └── CONTROL/
        ├── control             # Package metadata
        └── postinst            # Post-install script
```

## Future Enhancements

- [ ] Platform registration (cloud service integration)
- [ ] Camera preview during setup
- [ ] Network diagnostics
- [ ] Firmware update check
- [ ] Multi-language support
- [ ] Accessibility improvements
- [ ] Mobile app integration

## License

See the main project license.

## Support

For issues or questions, please refer to the main Authority Alert documentation.

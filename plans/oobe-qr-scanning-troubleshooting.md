# OOBE QR Code Scanning Troubleshooting Guide

**Created:** 2026-01-10

## Issue: QR Code Not Being Detected

The QR reader is processing frames but not detecting the QR code. This is a common issue with QR code scanning from screens.

## Possible Causes

### 1. QR Code Size
- **Problem:** QR code too small or too large in camera view
- **Solution:** 
  - QR code should fill 30-50% of the camera frame
  - User should adjust distance from camera
  - Platform should generate larger QR codes

### 2. Screen Brightness
- **Problem:** Phone/device screen too dim or too bright
- **Solution:**
  - Increase screen brightness to maximum
  - Disable auto-brightness
  - Avoid screen glare

### 3. Focus Issues
- **Problem:** Camera not focusing on QR code
- **Solution:**
  - Hold phone steady for 2-3 seconds
  - Ensure adequate distance (15-30cm typically)
  - Check if camera has auto-focus enabled

### 4. Lighting Conditions
- **Problem:** Too dark or too bright
- **Solution:**
  - Ensure good ambient lighting
  - Avoid direct sunlight on screen
  - Avoid shadows on QR code

### 5. Screen Refresh Rate
- **Problem:** Screen flickering interferes with scanning
- **Solution:**
  - Use static QR code image (not animated)
  - Disable screen animations
  - Use white background for QR code

### 6. QR Code Format
- **Problem:** QR code encoding or error correction level
- **Solution:**
  - Use QR code with high error correction (Level H)
  - Ensure QR code is properly formatted
  - Test with simple text QR code first

## Recommended Changes

### 1. Increase Scan Timeout

Current: 60 seconds  
Recommended: 90-120 seconds for better success rate

```javascript
body: JSON.stringify({
  timeout: 120,  // Increase to 120 seconds
  max_results: 1
})
```

### 2. Add User Instructions

Update instructions to be more specific:

```html
<div class="registration-instructions">
  <h3>QR Code Scanning Tips:</h3>
  <ol>
    <li>Set your phone screen brightness to maximum</li>
    <li>Hold your phone 20-30cm (8-12 inches) from the camera</li>
    <li>Keep the phone steady and ensure the QR code fills about half the screen</li>
    <li>Make sure there's good lighting (not too dark or too bright)</li>
    <li>Wait patiently - scanning may take 10-30 seconds</li>
  </ol>
</div>
```

### 3. Add Visual Feedback

Show frames processed count to user:

```javascript
pollScanStatus() {
  this.scanPollInterval = setInterval(async () => {
    // ... existing code ...
    
    if (status === 'scanning' && result.data.frames_processed) {
      this.updateScanStatus(
        `Scanning... ${result.data.frames_processed} frames processed`, 
        'scanning'
      );
    }
  }, 500);
}
```

### 4. Test with Simple QR Code

Before testing with platform QR code, test with a simple text QR code:
- Generate QR code with just text: "TEST123"
- If this works, the issue is with the JSON QR code format
- If this doesn't work, the issue is with camera/lighting/distance

### 5. Check QR Reader Binary

Verify the QR reader binary is working:

```bash
# SSH into device
ssh recamera@device-ip

# Test QR reader manually
/usr/bin/qr-reader --timeout 30 --max-results 1

# Check if it's processing frames
# Hold QR code in front of camera
```

### 6. Platform QR Code Requirements

The platform should generate QR codes with:
- **Error Correction:** Level H (highest)
- **Size:** At least 200x200 pixels on screen
- **Background:** White
- **Foreground:** Black
- **Quiet Zone:** At least 4 modules
- **Format:** Plain JSON string (no special encoding)

Example QR code generation (platform side):
```javascript
QRCode.toCanvas(canvas, JSON.stringify({
  api_key: "E6ntWqJr.NMpgnvmsX7oWdqLjtXQtvAyHnqvH90n7",
  user_id: 14
}), {
  errorCorrectionLevel: 'H',  // Highest error correction
  width: 300,  // Larger size
  margin: 4,  // Adequate quiet zone
  color: {
    dark: '#000000',
    light: '#FFFFFF'
  }
});
```

## Debugging Steps

### 1. Check QR Reader Logs

```bash
# View supervisor logs
journalctl -u supervisor -f | grep -i qr

# Or check system logs
tail -f /var/log/messages | grep -i qr
```

### 2. Test QR Reader Directly

```bash
# Create test QR code
echo '{"api_key":"test123","user_id":1}' | qrencode -o /tmp/test.png

# Display on screen and test
/usr/bin/qr-reader --timeout 30 --max-results 1
```

### 3. Check Camera Feed

Verify Channel 2 is working:
- Open supervisor overview page
- Switch to Channel 2
- Verify video is clear and focused

### 4. Increase Timeout in Code

If detection is slow, increase timeout:

```javascript
body: JSON.stringify({
  timeout: 120,  // 2 minutes
  max_results: 1
})
```

## Alternative Approach

If QR scanning continues to fail, consider alternative registration methods:

### 1. Manual Code Entry

Add a text input for manual entry of registration code:

```html
<div class="form-group">
  <label>Or enter code manually:</label>
  <input type="text" id="manual-code" placeholder="Paste registration code here" />
  <button onclick="oobeApp.handleManualRegistration()">Register with Code</button>
</div>
```

### 2. NFC Pairing

For future enhancement, support NFC tap-to-pair.

### 3. Web-based Pairing

Use WebRTC or similar to establish direct connection between phone and camera.

## Recommended Next Steps

1. **Test with simple QR code** - Verify basic QR detection works
2. **Increase timeout** - Give more time for detection
3. **Improve instructions** - Add specific guidance on distance/lighting
4. **Check platform QR code** - Ensure it's generated with proper settings
5. **Add manual fallback** - Allow code entry if scanning fails

## Contact Information

If issues persist, check:
- QR reader binary version and capabilities
- Camera focus and resolution settings
- Platform QR code generation settings

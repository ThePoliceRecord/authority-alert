/**
 * Supervisor API Client
 * Provides a clean interface to interact with the supervisor's REST APIs
 */

class SupervisorAPI {
  constructor(baseUrl = null, tokenManager = null) {
    // Auto-detect supervisor URL
    // Use same protocol as current page (HTTP for AP hotspot, HTTPS for Ethernet)
    if (!baseUrl) {
      const hostname = window.location.hostname;
      const protocol = window.location.protocol; // 'http:' or 'https:'
      // If accessing via IP or hostname, use that for supervisor
      if (hostname && hostname !== 'localhost' && hostname !== '127.0.0.1') {
        this.baseUrl = `${protocol}//${hostname}`;
      } else {
        this.baseUrl = `${protocol}//localhost`;
      }
    } else {
      this.baseUrl = baseUrl;
    }
    
    // Use provided token manager or create a fallback
    this.tokenManager = tokenManager || {
      getToken: (key) => localStorage.getItem(key),
      setToken: (key, value) => localStorage.setItem(key, value),
      removeToken: (key) => localStorage.removeItem(key)
    };
    
    this.token = this.tokenManager.getToken('authToken');
    
    // Callback for handling auth failures (401 errors)
    // Set this to handle re-login flow
    this.onAuthFailure = null;
  }

  /**
   * Make an API request to the supervisor
   */
  async request(endpoint, method = 'GET', body = null, requiresAuth = true) {
    const options = {
      method: method,
      headers: {
        'Content-Type': 'application/json',
      },
    };

    // Always get the latest token from secure storage before making a request
    // This ensures we use the token even if it was set after the API instance was created
    if (requiresAuth) {
      const currentToken = this.tokenManager.getToken('authToken');
      if (currentToken) {
        options.headers['Authorization'] = `Bearer ${currentToken}`;
        console.log(`[API] ${method} ${endpoint} with auth token (${currentToken.substring(0, 20)}...)`);
      } else {
        console.warn(`[API] ${method} ${endpoint} requires auth but no token found!`);
      }
    } else {
      console.log(`[API] ${method} ${endpoint} (no auth required)`);
    }

    // Add body for POST requests
    if (body) {
      options.body = JSON.stringify(body);
    }

    try {
      const response = await fetch(`${this.baseUrl}${endpoint}`, options);

      // Check if response is OK before parsing
      if (!response.ok) {
        // Handle 401 Unauthorized - token is invalid or expired
        if (response.status === 401) {
          console.warn(`[API] Auth failure (401) for ${method} ${endpoint} - token may be invalid or expired`);
          // Clear the invalid token
          this.clearToken();
          // Trigger auth failure callback if set
          if (this.onAuthFailure) {
            this.onAuthFailure('Token expired or invalid. Please login again.');
          }
          return { success: false, error: 'Authentication required', code: 401, authFailure: true };
        }
        
        // Try to get error message from response body
        let errorMsg = `HTTP ${response.status}: ${response.statusText}`;
        try {
          const text = await response.text();
          // Try to parse as JSON for error details
          const errorData = JSON.parse(text);
          if (errorData.msg) errorMsg = errorData.msg;
          else if (errorData.error) errorMsg = errorData.error;
        } catch (e) {
          // Response wasn't JSON, use status text
        }
        console.error(`[API] Request failed: ${method} ${endpoint} - ${errorMsg}`);
        return { success: false, error: errorMsg, code: response.status };
      }

      // Parse JSON response
      const text = await response.text();
      if (!text) {
        return { success: false, error: 'Empty response from server' };
      }

      let data;
      try {
        data = JSON.parse(text);
      } catch (parseError) {
        console.error('Failed to parse response as JSON:', text.substring(0, 200));
        return { success: false, error: 'Invalid response from server' };
      }

      if (data.code === 0) {
        console.log(`[API] Success: ${method} ${endpoint}`);
        return { success: true, data: data.data };
      } else {
        console.error(`[API] Failed: ${method} ${endpoint} - Code: ${data.code}, Message: ${data.msg}`);
        return { success: false, error: data.msg || 'Unknown error', code: data.code, msg: data.msg };
      }
    } catch (error) {
      console.error('API request failed:', error);
      return { success: false, error: error.message || 'Network error' };
    }
  }

  /**
   * Save authentication token
   */
  setToken(token) {
    this.token = token;
    this.tokenManager.setToken('authToken', token);
  }

  /**
   * Clear authentication token
   */
  clearToken() {
    this.token = null;
    this.tokenManager.removeToken('authToken');
  }

  // ========== System Information ==========

  async getVersion() {
    return this.request('/api/version');
  }

  // ========== User Management ==========

  async queryUserInfo() {
    return this.request('/api/userMgr/queryUserInfo');
  }

  async login(username, password) {
    console.log(`[API] Attempting login for user: ${username}`);
    const result = await this.request('/api/userMgr/login', 'POST', {
      userName: username,
      password
    }, false);

    if (result.success && result.data.token) {
      const newToken = result.data.token;
      const oldToken = this.tokenManager.getToken('authToken');
      console.log(`[API] Login successful for ${username}`);
      console.log(`[API] Old token: ${oldToken ? oldToken.substring(0, 20) + '...' : 'none'}`);
      console.log(`[API] New token: ${newToken.substring(0, 20)}...`);
      this.setToken(newToken);
    } else {
      console.warn(`[API] Login failed for ${username}:`, result.error || result.msg);
    }

    return result;
  }

  async updatePassword(oldPassword, newPassword) {
    return this.request('/api/userMgr/updatePassword', 'POST', {
      oldPassword,
      newPassword
    });
  }

  async setSSHStatus(enabled) {
    return this.request('/api/userMgr/setSShStatus', 'POST', {
      enabled
    });
  }

  // ========== Device Management ==========

  async queryDeviceInfo() {
    return this.request('/api/deviceMgr/queryDeviceInfo');
  }

  async getDeviceInfo() {
    return this.request('/api/deviceMgr/getDeviceInfo');
  }

  async updateDeviceName(deviceName) {
    return this.request('/api/deviceMgr/updateDeviceName', 'POST', {
      deviceName
    });
  }

  async getSystemStatus() {
    return this.request('/api/deviceMgr/getSystemStatus');
  }

  async queryServiceStatus() {
    return this.request('/api/deviceMgr/queryServiceStatus');
  }

  async setPower(action) {
    return this.request('/api/deviceMgr/setPower', 'POST', {
      action
    });
  }

  // ========== Time Configuration ==========

  async getTimestamp() {
    return this.request('/api/deviceMgr/getTimestamp');
  }

  async setTimestamp(timestamp) {
    return this.request('/api/deviceMgr/setTimestamp', 'POST', {
      timestamp
    });
  }

  async getTimezone() {
    return this.request('/api/deviceMgr/getTimezone');
  }

  async setTimezone(timezone) {
    return this.request('/api/deviceMgr/setTimezone', 'POST', {
      timezone
    });
  }

  async getTimezoneList() {
    return this.request('/api/deviceMgr/getTimezoneList');
  }

  // ========== WiFi Management ==========

  async getWiFiInfoList() {
    return this.request('/api/wifiMgr/getWiFiInfoList');
  }

  async getConnectionStatus() {
    return this.request('/api/wifiMgr/getConnectionStatus');
  }

  async connectWiFi(ssid, password, security = 'WPA2') {
    return this.request('/api/wifiMgr/connectWiFi', 'POST', {
      ssid,
      password,
      security
    });
  }

  async disconnectWiFi() {
    return this.request('/api/wifiMgr/disconnectWiFi', 'POST');
  }

  async forgetWiFi(ssid) {
    return this.request('/api/wifiMgr/forgetWiFi', 'POST', {
      ssid
    });
  }

  // ========== Platform Configuration ==========

  async getPlatformInfo() {
    return this.request('/api/deviceMgr/getPlatformInfo');
  }

  async savePlatformInfo(platformUrl, apiKey, deviceId) {
    return this.request('/api/deviceMgr/savePlatformInfo', 'POST', {
      platformUrl,
      apiKey,
      deviceId
    });
  }

  // ========== Camera ==========

  async getChannels() {
    return this.request('/api/channels');
  }

  async getCameraWebsocketUrl() {
    return this.request('/api/deviceMgr/getCameraWebsocketUrl');
  }

  // ========== LED Management ==========

  async getLEDs() {
    return this.request('/api/ledMgr/getLEDs');
  }

  async setLED(name, brightness, trigger = 'none') {
    return this.request('/api/ledMgr/setLED', 'POST', {
      name,
      brightness,
      trigger
    });
  }

  // ========== QR Code Scanner ==========

  /**
   * Start a QR code scan session
   * @param {number} timeout - Scan timeout in seconds (default: 30)
   * @param {number} maxResults - Maximum QR codes to detect (default: 1, 0=unlimited)
   * @param {string} schema - Optional schema validation (authority_config, wifi_config, device_pairing)
   * @returns {Promise} {success: boolean, data: {scan_id: string, status: string, started_at: string}}
   */
  async startQRScan(timeout = 30, maxResults = 1, schema = null) {
    const body = { timeout, max_results: maxResults };
    if (schema) {
      body.schema = schema;
    }
    return this.request('/api/qr/scan', 'POST', body);
  }

  /**
   * Get status of a QR scan session
   * @param {string} scanId - Scan session ID
   * @returns {Promise} {success: boolean, data: {scan_id: string, status: string, result?: object}}
   */
  async getQRScanStatus(scanId) {
    return this.request(`/api/qr/scan/${scanId}`);
  }

  /**
   * Cancel an active QR scan session
   * @param {string} scanId - Scan session ID
   * @returns {Promise} {success: boolean, data: {scan_id: string, status: string}}
   */
  async cancelQRScan(scanId) {
    return this.request(`/api/qr/scan/${scanId}`, 'DELETE');
  }

  /**
   * Helper method to scan and wait for result (polls automatically)
   * @param {number} timeout - Scan timeout in seconds
   * @param {number} maxResults - Maximum QR codes to detect
   * @param {string} schema - Optional schema validation
   * @param {function} onProgress - Optional callback for progress updates
   * @returns {Promise} Result object with QR code data
   */
  async scanQRCode(timeout = 30, maxResults = 1, schema = null, onProgress = null) {
    // Start scan
    const startResult = await this.startQRScan(timeout, maxResults, schema);
    if (!startResult.success) {
      throw new Error(startResult.error || 'Failed to start QR scan');
    }

    const scanId = startResult.data.scan_id;
    const pollInterval = 500; // Poll every 500ms

    // Poll for results
    return new Promise((resolve, reject) => {
      const intervalId = setInterval(async () => {
        try {
          const statusResult = await this.getQRScanStatus(scanId);
          
          if (!statusResult.success) {
            clearInterval(intervalId);
            reject(new Error(statusResult.error || 'Failed to get scan status'));
            return;
          }

          const { status, result } = statusResult.data;

          // Call progress callback if provided
          if (onProgress) {
            onProgress(status, result);
          }

          // Check if scan is complete
          if (status === 'complete') {
            clearInterval(intervalId);
            resolve(result);
          } else if (status === 'timeout') {
            clearInterval(intervalId);
            reject(new Error('QR scan timed out - no QR code detected'));
          } else if (status === 'cancelled') {
            clearInterval(intervalId);
            reject(new Error('QR scan was cancelled'));
          } else if (status === 'error') {
            clearInterval(intervalId);
            reject(new Error(result?.reason || 'QR scan failed'));
          }
          // Otherwise status === 'scanning', keep polling
        } catch (error) {
          clearInterval(intervalId);
          reject(error);
        }
      }, pollInterval);

      // Cleanup on timeout (scan timeout + 5 seconds grace period)
      setTimeout(() => {
        clearInterval(intervalId);
        this.cancelQRScan(scanId).catch(() => {}); // Best effort cancel
        reject(new Error('QR scan polling timeout'));
      }, (timeout + 5) * 1000);
    });
  }

  // ========== Code Registration ==========

  /**
   * Start code-based camera registration
   * @param {string} locationName - Camera location name
   * @param {number} [lat] - Optional latitude
   * @param {number} [lng] - Optional longitude
   * @returns {Promise} {success: boolean, data: {status, claim_code, claim_code_formatted, expires_at}}
   */
  async startCodeRegistration(locationName, lat = null, lng = null) {
    const body = { location_name: locationName };
    if (lat !== null) body.latitude = lat;
    if (lng !== null) body.longitude = lng;
    return this.request('/api/deviceMgr/startCodeRegistration', 'POST', body);
  }

  /**
   * Get current code registration status
   * @returns {Promise} {success: boolean, data: {status, claim_code, claim_code_formatted, expires_at, result}}
   */
  async getCodeRegistrationStatus() {
    return this.request('/api/deviceMgr/codeRegistrationStatus');
  }

  /**
   * Cancel active code registration
   * @returns {Promise} {success: boolean, data: {status, message}}
   */
  async cancelCodeRegistration() {
    return this.request('/api/deviceMgr/cancelCodeRegistration', 'POST');
  }
}

// Export for use in other scripts
window.SupervisorAPI = SupervisorAPI;

/**
 * Authority Alert OOBE Application
 * Handles the out-of-box experience setup flow
 */

/**
 * Secure Token Manager
 * Stores tokens in memory with sessionStorage fallback
 * Provides better XSS protection than localStorage
 */
class SecureTokenManager {
  constructor() {
    // Store tokens in memory (cleared on page reload)
    this.tokens = new Map();
    
    // Use sessionStorage as fallback (cleared when tab closes)
    // This is more secure than localStorage which persists indefinitely
    this.storage = window.sessionStorage;
    
    // Token expiration tracking
    this.expirations = new Map();
    
    // Migrate any existing localStorage tokens to secure storage
    this.migrateFromLocalStorage();
  }
  
  /**
   * Migrate tokens from localStorage to secure storage
   */
  migrateFromLocalStorage() {
    const tokensToMigrate = [
      'authToken',
      'platformAccessToken',
      'platformRefreshToken',
      'platformIdToken'
    ];
    
    tokensToMigrate.forEach(key => {
      const value = localStorage.getItem(key);
      if (value) {
        console.log(`Migrating ${key} from localStorage to secure storage`);
        this.setToken(key, value);
        localStorage.removeItem(key);
      }
    });
  }
  
  /**
   * Set a token with optional expiration
   */
  setToken(key, value, expiresInSeconds = null) {
    if (!key || !value) return;
    
    // Store in memory
    this.tokens.set(key, value);
    
    // Store in sessionStorage as backup
    try {
      this.storage.setItem(key, value);
    } catch (e) {
      console.warn('Failed to store token in sessionStorage:', e);
    }
    
    // Set expiration if provided
    if (expiresInSeconds) {
      const expiresAt = Date.now() + (expiresInSeconds * 1000);
      this.expirations.set(key, expiresAt);
    }
  }
  
  /**
   * Get a token, checking expiration
   */
  getToken(key) {
    // Check if token is expired
    if (this.expirations.has(key)) {
      const expiresAt = this.expirations.get(key);
      if (Date.now() > expiresAt) {
        console.log(`Token ${key} has expired`);
        this.removeToken(key);
        return null;
      }
    }
    
    // Try memory first
    let token = this.tokens.get(key);
    
    // Fallback to sessionStorage
    if (!token) {
      try {
        token = this.storage.getItem(key);
        if (token) {
          // Restore to memory
          this.tokens.set(key, token);
        }
      } catch (e) {
        console.warn('Failed to retrieve token from sessionStorage:', e);
      }
    }
    
    return token;
  }
  
  /**
   * Remove a token
   */
  removeToken(key) {
    this.tokens.delete(key);
    this.expirations.delete(key);
    
    try {
      this.storage.removeItem(key);
    } catch (e) {
      console.warn('Failed to remove token from sessionStorage:', e);
    }
    
    // Also clean up from localStorage if it exists
    try {
      localStorage.removeItem(key);
    } catch (e) {
      // Ignore
    }
  }
  
  /**
   * Clear all tokens
   */
  clearAll() {
    this.tokens.clear();
    this.expirations.clear();
    
    try {
      this.storage.clear();
    } catch (e) {
      console.warn('Failed to clear sessionStorage:', e);
    }
  }
  
  /**
   * Check if a token exists and is valid
   */
  hasToken(key) {
    return this.getToken(key) !== null;
  }
}

class OOBEApp {
  // WiFi scan configuration
  static WIFI_SCAN_MAX_RETRIES = 3;
  static WIFI_SCAN_RETRY_DELAY = 2000; // milliseconds

  // Platform API base URL
  // Default to dev environment for testing
  // Override at runtime by setting window.TPR_PLATFORM_URL before this script loads.
  static PLATFORM_API_URL = window.TPR_PLATFORM_URL || 'https://dev.thepolicerecord.com';

  constructor() {
    // Initialize secure token manager
    this.tokenManager = new SecureTokenManager();
    
    // Pass token manager to API for secure token handling
    this.api = new SupervisorAPI(null, this.tokenManager);
    this.currentStep = 1;
    this.setupData = {
      deviceInfo: null,
      userInfo: null,
      deviceName: '',
      timezone: '',
      wifiSSID: '',
      wifiPassword: '',
      wifiSecurity: 'WPA2'
    };
    
    // WiFi scan abort controller
    this.wifiScanController = null;

    // Set up auth failure handler
    this.api.onAuthFailure = (message) => this.handleAuthFailure(message);
    
    this.init();
  }

  async init() {
    console.log('Initializing OOBE...');
    this.hideLoading();

    // Check if we have a stored token and validate it
    const storedToken = this.tokenManager.getToken('authToken');
    if (storedToken) {
      console.log('Found stored auth token, validating...');
      // Try a simple API call to validate the token
      const result = await this.api.queryUserInfo();
      if (result.success) {
        console.log('Stored token is valid');
        // Token is valid, but for OOBE we still start at step 1
        // The user can proceed through the flow
      } else if (result.authFailure) {
        console.log('Stored token is invalid, clearing...');
        // Token is invalid, already cleared by API
      }
    }
    
    // Start at step 1 (Welcome)
    // We'll login when the user provides the old password in step 2
    this.showStep(1);
  }

  /**
   * Handle authentication failures (401 errors)
   * This is called when the API detects an invalid/expired token
   */
  handleAuthFailure(message) {
    console.warn('Auth failure detected:', message);
    this.hideLoading();
    
    // If we're past the password step, we need to go back
    if (this.currentStep > 2) {
      this.showError('Session expired. Please enter your password again.');
      // Go back to password step
      this.showStep(2);
    }
  }

  /**
   * Make an authenticated fetch request with proper error handling
   * Use this instead of direct fetch() calls for API endpoints
   */
  async authenticatedFetch(url, options = {}) {
    const token = this.tokenManager.getToken('authToken');
    
    // Set up headers with auth token
    const headers = {
      ...options.headers,
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    
    const response = await fetch(url, { ...options, headers });
    
    // Handle 401 errors
    if (response.status === 401) {
      console.warn(`[OOBE] Auth failure (401) for ${options.method || 'GET'} ${url}`);
      this.tokenManager.removeToken('authToken');
      this.handleAuthFailure('Token expired or invalid');
      throw new Error('Authentication required');
    }
    
    return response;
  }

  showStep(step) {
    this.currentStep = step;
    
    // Hide all steps
    document.querySelectorAll('.setup-step').forEach(el => {
      el.classList.add('hidden');
    });

    // Show current step
    const stepElement = document.getElementById(`step-${step}`);
    if (stepElement) {
      stepElement.classList.remove('hidden');
    }

    // Update step indicators
    document.querySelectorAll('.step').forEach((el, index) => {
      el.classList.remove('active', 'completed');
      if (index + 1 < step) {
        el.classList.add('completed');
      } else if (index + 1 === step) {
        el.classList.add('active');
      }
    });

    // Load step content
    switch (step) {
      case 1:
        this.loadWelcomeStep();
        break;
      case 2:
        this.loadPasswordStep();
        break;
      case 3:
        this.loadDeviceConfigStep();
        break;
      case 4:
        this.loadWiFiStep();
        break;
      case 5:
        this.loadRegistrationStep();
        break;
      case 6:
        this.loadCompletionStep();
        break;
    }
  }

  loadWelcomeStep() {
    // Welcome step - no additional setup needed
  }

  loadPasswordStep() {
    const messageEl = document.getElementById('password-message');
    messageEl.textContent = 'Please enter the current password and set a new password for your device.';
  }

  async loadDeviceConfigStep() {
    // Load timezone list
    const timezoneResult = await this.api.getTimezoneList();
    if (timezoneResult.success && timezoneResult.data.timezones) {
      const select = document.getElementById('timezone-select');
      select.innerHTML = '<option value="">Select timezone...</option>';
      
      timezoneResult.data.timezones.forEach(tz => {
        const option = document.createElement('option');
        option.value = tz;
        option.textContent = tz;
        select.appendChild(option);
      });

      // Try to detect user's timezone
      try {
        const userTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
        if (userTimezone) {
          select.value = userTimezone;
          this.setupData.timezone = userTimezone;
        }
      } catch (e) {
        console.warn('Could not detect timezone:', e.message);
        // User will need to select manually
      }
    }

    // Set current date and time from browser
    const now = new Date();
    const dateInput = document.getElementById('device-date');
    const timeInput = document.getElementById('device-time');
    
    if (dateInput) {
      // Format: YYYY-MM-DD
      const year = now.getFullYear();
      const month = String(now.getMonth() + 1).padStart(2, '0');
      const day = String(now.getDate()).padStart(2, '0');
      dateInput.value = `${year}-${month}-${day}`;
    }
    
    if (timeInput) {
      // Format: HH:MM
      const hours = String(now.getHours()).padStart(2, '0');
      const minutes = String(now.getMinutes()).padStart(2, '0');
      timeInput.value = `${hours}:${minutes}`;
    }
  }

  async requestGeolocation() {
    // Check if geolocation is available
    if (!navigator.geolocation) {
      this.showError('Geolocation is not supported by your browser. Please enter the location manually.');
      return;
    }

    // Check if we're in a secure context (required for geolocation)
    // Note: localhost and 127.0.0.1 are considered secure, but other IPs with self-signed certs may not be
    if (typeof window.isSecureContext !== 'undefined' && !window.isSecureContext) {
      this.showError('Geolocation requires a secure connection. Please enter the location manually.');
      return;
    }

    this.showLoading('Getting your location...');

    // Helper to get human-readable error message
    const getGeolocationErrorMessage = (error) => {
      // Handle the case where error might not have standard properties
      if (!error) {
        return 'Unable to get location. Please enter it manually.';
      }

      // Standard GeolocationPositionError codes
      const PERMISSION_DENIED = 1;
      const POSITION_UNAVAILABLE = 2;
      const TIMEOUT = 3;

      switch (error.code) {
        case PERMISSION_DENIED:
          return 'Location permission denied. Please allow location access in your browser settings and try again.';
        case POSITION_UNAVAILABLE:
          return 'Location information unavailable. Your device may not have location services, or they may be disabled.';
        case TIMEOUT:
          return 'Location request timed out. Please try again or enter the location manually.';
        default:
          // Include the error message if available for debugging
          const msg = error.message || 'Unknown error';
          console.log('Geolocation error:', error);
          return `Unable to get location (${msg}). Please enter it manually.`;
      }
    };

    // Try to get position with promise wrapper
    const tryGetPosition = (highAccuracy, timeoutMs) => {
      return new Promise((resolve, reject) => {
        const options = {
          enableHighAccuracy: highAccuracy,
          timeout: timeoutMs,
          maximumAge: 300000 // Accept cached position up to 5 minutes old
        };

        navigator.geolocation.getCurrentPosition(resolve, reject, options);
      });
    };

    try {
      let position = null;
      let lastError = null;

      // Strategy 1: Try with low accuracy first (faster, works with WiFi/IP)
      try {
        console.log('Trying low accuracy geolocation...');
        position = await tryGetPosition(false, 20000);
      } catch (lowAccuracyError) {
        console.log('Low accuracy failed:', lowAccuracyError.code, lowAccuracyError.message);
        lastError = lowAccuracyError;
      }

      // Strategy 2: If low accuracy failed with timeout, try high accuracy
      if (!position && lastError && lastError.code === 3) { // TIMEOUT
        try {
          console.log('Trying high accuracy geolocation...');
          position = await tryGetPosition(true, 30000);
        } catch (highAccuracyError) {
          console.log('High accuracy failed:', highAccuracyError.code, highAccuracyError.message);
          lastError = highAccuracyError;
        }
      }

      // If we still don't have a position, throw the last error
      if (!position) {
        throw lastError || new Error('Unable to get location');
      }

      const lat = position.coords.latitude;
      const lon = position.coords.longitude;

      console.log(`Got location: ${lat}, ${lon} (accuracy: ${position.coords.accuracy}m)`);

      // Store coordinates for device info
      this.setupData.geolocation = {
        latitude: lat,
        longitude: lon,
        accuracy: position.coords.accuracy
      };

      // Try to reverse geocode using OpenStreetMap Nominatim API
      let locationSet = false;
      try {
        const response = await fetch(
          `https://nominatim.openstreetmap.org/reverse?format=json&lat=${lat}&lon=${lon}&zoom=18&addressdetails=1`,
          {
            headers: {
              'Accept': 'application/json'
            }
          }
        );
        if (response.ok) {
          const data = await response.json();
          const address = data.address || {};

          // Build location string from address components
          let location = '';
          // Try different address fields in order of preference
          const road = address.road || address.street || address.pedestrian || address.footway || '';
          const houseNumber = address.house_number || '';
          const city = address.city || address.town || address.village || address.hamlet || address.municipality || '';
          const state = address.state || address.region || address.county || '';

          if (houseNumber && road) {
            location = `${houseNumber} ${road}`;
          } else if (road) {
            location = road;
          }
          if (city) location += (location ? ', ' : '') + city;
          if (state) location += (location ? ', ' : '') + state;

          if (location) {
            document.getElementById('device-name').value = location;
            locationSet = true;
          }
        }
      } catch (geocodeError) {
        console.log('Geocoding failed:', geocodeError);
      }

      // Fallback to coordinates if geocoding didn't work
      if (!locationSet) {
        document.getElementById('device-name').value = `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
      }

      this.hideLoading();
    } catch (error) {
      this.hideLoading();
      this.showError(getGeolocationErrorMessage(error));
    }
  }

  async loadWiFiStep() {
    await this.scanWiFiNetworks();
  }

  async scanWiFiNetworks(retryCount = 0) {
    const maxRetries = OOBEApp.WIFI_SCAN_MAX_RETRIES;
    const retryDelay = OOBEApp.WIFI_SCAN_RETRY_DELAY;
    
    // Cancel any existing scan
    if (this.wifiScanController) {
      this.wifiScanController.abort();
    }
    
    // Create new abort controller for this scan
    this.wifiScanController = new AbortController();
    const signal = this.wifiScanController.signal;
    
    this.showLoading('Scanning for WiFi networks...');
    
    try {
      // Check if we have a valid token before attempting WiFi scan
      const token = this.tokenManager.getToken('authToken');
      if (!token) {
        this.hideLoading();
        this.showError('Authentication required. Please go back and login again.');
        console.error('No auth token found in secure storage');
        return;
      }
      
      // Check if scan was aborted
      if (signal.aborted) {
        console.log('WiFi scan was cancelled');
        return;
      }
      
      console.log(`Scanning WiFi (attempt ${retryCount + 1}/${maxRetries + 1}) with token:`, token.substring(0, 20) + '...');
      const result = await this.api.getWiFiInfoList();
      
      // Check if scan was aborted after API call
      if (signal.aborted) {
        console.log('WiFi scan was cancelled');
        return;
      }
      
      if (result.success && result.data) {
        // API returns two lists:
        // - connectedWifiInfoList: saved/previously connected networks
        // - wifiInfoList: scanned available networks
        const savedNetworks = result.data.connectedWifiInfoList || [];
        const scannedNetworks = result.data.wifiInfoList || [];

        console.log('Saved networks:', savedNetworks.length, 'Scanned networks:', scannedNetworks.length);
        console.log('Raw saved networks:', JSON.stringify(savedNetworks, null, 2));
        console.log('Raw scanned networks:', JSON.stringify(scannedNetworks, null, 2));

        // Merge both lists, avoiding duplicates (prefer saved network info)
        const ssidSet = new Set();
        const allNetworks = [];

        // Add saved networks first (they have connection status)
        savedNetworks.forEach(network => {
          if (network.ssid && !ssidSet.has(network.ssid)) {
            ssidSet.add(network.ssid);
            allNetworks.push(network);
          }
        });

        // Add scanned networks that aren't already in saved list
        scannedNetworks.forEach(network => {
          if (network.ssid && !ssidSet.has(network.ssid)) {
            ssidSet.add(network.ssid);
            allNetworks.push(network);
          }
        });

        console.log('Found', allNetworks.length, 'total WiFi networks');

        // If no networks found and we haven't exhausted retries, wait and try again
        if (allNetworks.length === 0 && retryCount < maxRetries) {
          console.log(`No networks found, retrying in ${retryDelay}ms...`);
          this.showLoading(`Scanning for WiFi networks... (attempt ${retryCount + 2}/${maxRetries + 1})`);

          // Use abortable timeout
          await new Promise((resolve, reject) => {
            const timeout = setTimeout(resolve, retryDelay);
            signal.addEventListener('abort', () => {
              clearTimeout(timeout);
              reject(new Error('Scan cancelled'));
            });
          }).catch(() => {
            console.log('Retry cancelled');
            return;
          });

          return this.scanWiFiNetworks(retryCount + 1);
        }

        if (allNetworks.length === 0) {
          // Show message but don't error - user can rescan
          this.displayWiFiNetworks([]);
          this.hideLoading();
          this.showError('No WiFi networks found after multiple scans. Click "Rescan Networks" to try again, or check your WiFi adapter.');
          return;
        }

        // Transform the data to match our expected format
        // status field: 1 = Disconnected, 2 = Connecting, 3 = Connected
        const transformedNetworks = allNetworks.map(network => ({
          ssid: network.ssid,
          signal: network.signal,
          security: network.auth === 0 ? 'Open' : 'WPA2',
          connected: network.status === 3, // 3 = NetworkStatusConnected
          bssid: network.bssid,
          frequency: network.frequency
        }));

        console.log('WiFi networks after transform:', transformedNetworks.map(n => ({ ssid: n.ssid, connected: n.connected })));

        // Sort networks: connected first, then by signal strength
        transformedNetworks.sort((a, b) => {
          if (a.connected && !b.connected) return -1;
          if (!a.connected && b.connected) return 1;
          return b.signal - a.signal; // Higher signal (less negative) first
        });

        this.displayWiFiNetworks(transformedNetworks);
      } else {
        // Standardized error handling
        this.handleAPIError(result, 'WiFi scan');
      }
    } catch (error) {
      if (error.name === 'AbortError' || error.message === 'Scan cancelled') {
        console.log('WiFi scan cancelled by user');
        return;
      }
      console.error('WiFi scan error:', error);
      this.showError('Failed to scan WiFi networks: ' + error.message);
    } finally {
      this.hideLoading();
      this.wifiScanController = null;
    }
  }

  async rescanWiFi() {
    console.log('User requested WiFi rescan');
    await this.scanWiFiNetworks();
  }
  
  /**
   * Standardized API error handler
   */
  handleAPIError(result, operation) {
    if (result.code === 401) {
      this.showError(`Authentication failed during ${operation}. Please go back and login again.`);
      console.error(`${operation} auth error:`, result);
    } else if (result.code === 403) {
      this.showError(`Permission denied for ${operation}.`);
      console.error(`${operation} permission error:`, result);
    } else if (result.code === 404) {
      this.showError(`${operation} endpoint not found.`);
      console.error(`${operation} not found:`, result);
    } else if (result.code >= 500) {
      this.showError(`Server error during ${operation}. Please try again later.`);
      console.error(`${operation} server error:`, result);
    } else {
      this.showError(`Failed to ${operation}: ${result.error || result.msg || 'Unknown error'}`);
      console.error(`${operation} error:`, result);
    }
  }

  displayWiFiNetworks(networks) {
    const listEl = document.getElementById('wifi-list');
    listEl.innerHTML = '';

    if (networks.length === 0) {
      listEl.innerHTML = '<div class="alert alert-info">No WiFi networks found. Please check your WiFi adapter.</div>';
      return;
    }

    // Find if there's an already connected network
    const connectedNetwork = networks.find(n => n.connected);

    networks.forEach(network => {
      const item = document.createElement('div');
      item.className = 'wifi-item';

      // Add connected class for visual highlighting
      if (network.connected) {
        item.className += ' connected';
      }

      item.onclick = (event) => this.selectWiFiNetwork(network.ssid, network.security, event);

      const signalStrength = this.getSignalStrength(network.signal);

      item.innerHTML = `
        <div class="wifi-info">
          <div class="wifi-ssid">${this.escapeHtml(network.ssid)}</div>
          <div class="wifi-details">
            ${network.security} • Signal: ${signalStrength}
          </div>
        </div>
        <div class="wifi-signal">${this.getSignalIcon(network.signal)}</div>
      `;

      listEl.appendChild(item);

      // Auto-select the connected network
      if (network.connected) {
        this.selectWiFiNetwork(network.ssid, network.security, { currentTarget: item });
      }
    });

    // Update button text based on connection status
    const connectBtn = document.querySelector('#step-4 .btn-primary');
    if (connectBtn) {
      if (connectedNetwork) {
        connectBtn.textContent = 'Continue →';
        // Reset onclick to just continue since we're already connected
        connectBtn.onclick = () => this.showStep(5);
      } else {
        connectBtn.textContent = 'Connect →';
        connectBtn.onclick = () => this.handleWiFiNext();
      }
    }
  }

  selectWiFiNetwork(ssid, security, event) {
    this.setupData.wifiSSID = ssid;
    this.setupData.wifiSecurity = security;

    // Update UI
    document.querySelectorAll('.wifi-item').forEach(el => {
      el.classList.remove('selected');
    });
    if (event && event.currentTarget) {
      event.currentTarget.classList.add('selected');
    }

    // Check if selected network is already connected
    const isConnected = event?.currentTarget?.classList.contains('connected');

    // Show password input if secured and not already connected
    const passwordGroup = document.getElementById('wifi-password-group');
    const passwordInput = document.getElementById('wifi-password');
    if (security !== 'Open' && !isConnected) {
      passwordGroup.classList.remove('hidden');
      passwordInput.focus();
    } else {
      passwordGroup.classList.add('hidden');
      this.setupData.wifiPassword = '';
    }

    // Update button text and handler based on connection status
    const connectBtn = document.querySelector('#step-4 .btn-primary');
    if (connectBtn) {
      if (isConnected) {
        connectBtn.textContent = 'Continue →';
        connectBtn.onclick = () => this.showStep(5);
      } else {
        connectBtn.textContent = 'Connect →';
        connectBtn.onclick = () => this.handleWiFiNext();
      }
    }
  }

  togglePasswordVisibility(inputId, button) {
    const input = document.getElementById(inputId);
    if (input.type === 'password') {
      input.type = 'text';
      button.classList.add('visible');
      button.title = 'Hide password';
    } else {
      input.type = 'password';
      button.classList.remove('visible');
      button.title = 'Show password';
    }
  }

  getSignalStrength(signal) {
    if (signal >= -50) return 'Excellent';
    if (signal >= -60) return 'Good';
    if (signal >= -70) return 'Fair';
    return 'Weak';
  }

  getSignalIcon(signal) {
    // Return different signal strength indicators
    if (signal >= -50) return '📶📶📶'; // Excellent
    if (signal >= -60) return '📶📶';   // Good
    if (signal >= -70) return '📶';     // Fair
    return '📡';                        // Weak
  }

  loadCompletionStep() {
    console.log('Loading completion step with data:', this.setupData);
    const summaryEl = document.getElementById('setup-summary');
    
    // Ensure we have values, provide defaults if missing
    const deviceName = this.setupData.deviceName || 'Not set';
    const timezone = this.setupData.timezone || 'Not set';
    const wifiSSID = this.setupData.wifiSSID || 'Not configured';
    
    summaryEl.innerHTML = `
      <div class="device-info">
        <div class="device-info-item">
          <span class="device-info-label">Device Name:</span>
          <span class="device-info-value">${this.escapeHtml(deviceName)}</span>
        </div>
        <div class="device-info-item">
          <span class="device-info-label">Timezone:</span>
          <span class="device-info-value">${this.escapeHtml(timezone)}</span>
        </div>
        <div class="device-info-item">
          <span class="device-info-label">WiFi Network:</span>
          <span class="device-info-value">${this.escapeHtml(wifiSSID)}</span>
        </div>
      </div>
    `;
  }

  // Step Actions

  async handleWelcomeNext() {
    this.showStep(2);
  }

  /**
   * Validate password strength
   */
  validatePassword(password) {
    if (!password || password.length < 8) {
      return { valid: false, message: 'Password must be at least 8 characters long' };
    }
    
    if (password.length > 128) {
      return { valid: false, message: 'Password must be less than 128 characters' };
    }
    
    // Check for at least one uppercase letter
    if (!/[A-Z]/.test(password)) {
      return { valid: false, message: 'Password must contain at least one uppercase letter' };
    }
    
    // Check for at least one lowercase letter
    if (!/[a-z]/.test(password)) {
      return { valid: false, message: 'Password must contain at least one lowercase letter' };
    }
    
    // Check for at least one number
    if (!/\d/.test(password)) {
      return { valid: false, message: 'Password must contain at least one number' };
    }
    
    // Check for at least one special character
    if (!/[@$!%*?&#^()_+\-=\[\]{};':"\\|,.<>\/]/.test(password)) {
      return { valid: false, message: 'Password must contain at least one special character (@$!%*?&#, etc.)' };
    }
    
    // Check for common weak passwords
    const weakPasswords = ['Password1!', 'Admin123!', 'Welcome1!', 'Qwerty123!'];
    if (weakPasswords.some(weak => password.toLowerCase().includes(weak.toLowerCase()))) {
      return { valid: false, message: 'Password is too common. Please choose a more unique password' };
    }
    
    return { valid: true };
  }

  async handlePasswordNext() {
    const oldPassword = document.getElementById('old-password').value;
    const newPassword = document.getElementById('new-password').value;
    const confirmPassword = document.getElementById('confirm-password').value;

    // Validation
    if (!oldPassword) {
      this.showError('Please enter the current password');
      return;
    }

    // Comprehensive password validation
    const validation = this.validatePassword(newPassword);
    if (!validation.valid) {
      this.showError(validation.message);
      return;
    }

    if (newPassword !== confirmPassword) {
      this.showError('Passwords do not match');
      return;
    }

    this.showLoading('Logging in...');
    
    // First, login with the old password to get a token
    let loginResult = await this.api.login('recamera', oldPassword);
    if (!loginResult.success) {
      loginResult = await this.api.login('admin', oldPassword);
    }

    if (!loginResult.success) {
      this.hideLoading();
      this.showError('Invalid current password');
      return;
    }

    // Now we have a token, use it to change the password
    this.showLoading('Setting new password...');
    const result = await this.api.updatePassword(oldPassword, newPassword);
    
    console.log('Password update result:', result);
    
    if (!result.success) {
      this.hideLoading();
      const errorMsg = result.error || result.msg || 'Unknown error';
      this.showError('Failed to set password: ' + errorMsg);
      console.error('Password update failed:', result);
      return;
    }

    // Login with new password to get a fresh token
    this.showLoading('Verifying new password...');
    loginResult = await this.api.login('recamera', newPassword);
    if (!loginResult.success) {
      loginResult = await this.api.login('admin', newPassword);
    }

    if (!loginResult.success) {
      this.hideLoading();
      this.showError('Password set but login failed. Please refresh and try again.');
      console.error('Login with new password failed:', loginResult);
      return;
    }
    
    console.log('Successfully logged in with new password');
    
    // Store password temporarily in a secure way for re-authentication
    // Use a closure to avoid storing in setupData
    const securePassword = newPassword;
    
    // Define re-auth function that uses the closure
    this.reAuthenticateAfterTimeChange = async () => {
      let loginResult = await this.api.login('recamera', securePassword);
      if (!loginResult.success) {
        loginResult = await this.api.login('admin', securePassword);
      }
      return loginResult;
    };

    // Fetch device and user info now that we're logged in
    const deviceResult = await this.api.queryDeviceInfo();
    if (deviceResult.success) {
      this.setupData.deviceInfo = deviceResult.data;
    }

    const userResult = await this.api.queryUserInfo();
    if (userResult.success) {
      this.setupData.userInfo = userResult.data;
    }

    this.hideLoading();
    this.showStep(3);
  }

  async handleDeviceConfigNext() {
    const deviceName = document.getElementById('device-name').value.trim();
    const timezone = document.getElementById('timezone-select').value;
    const dateValue = document.getElementById('device-date').value;
    const timeValue = document.getElementById('device-time').value;

    if (!deviceName) {
      this.showError('Please enter a location');
      return;
    }

    if (!timezone) {
      this.showError('Please select a timezone');
      return;
    }

    if (!dateValue || !timeValue) {
      this.showError('Please set the date and time');
      return;
    }

    this.setupData.deviceName = deviceName;
    this.setupData.timezone = timezone;

    this.showLoading('Saving device configuration...');

    // Update device name
    const nameResult = await this.api.updateDeviceName(deviceName);
    if (!nameResult.success) {
      this.hideLoading();
      this.showError('Failed to set device name: ' + (nameResult.error || 'Unknown error'));
      return;
    }

    // Update timezone
    const tzResult = await this.api.setTimezone(timezone);
    if (!tzResult.success) {
      this.hideLoading();
      this.showError('Failed to set timezone: ' + (tzResult.error || 'Unknown error'));
      return;
    }

    // Set system time from user input
    // Note: Changing system time can invalidate JWT tokens if the new time
    // is before the token's NotBefore claim. We need to re-login after.
    const dateTimeString = `${dateValue}T${timeValue}:00`;
    const timestamp = Math.floor(new Date(dateTimeString).getTime() / 1000);
    const timeResult = await this.api.setTimestamp(timestamp);
    if (!timeResult.success) {
      console.warn('Failed to set timestamp:', timeResult.error);
      // Don't fail the setup if time sync fails
    } else {
      // System time changed - re-login to get a fresh token with correct timestamps
      // This is necessary because the old token's NotBefore claim may now be in the "future"
      console.log('System time changed, re-authenticating to get fresh token...');
      if (this.reAuthenticateAfterTimeChange) {
        const loginResult = await this.reAuthenticateAfterTimeChange();
        if (loginResult.success) {
          console.log('Re-authentication successful after time change');
        } else {
          console.warn('Re-authentication failed after time change:', loginResult.error);
        }
      } else {
        console.warn('No re-authentication function available after time change');
      }
    }

    this.hideLoading();
    this.showStep(4);
  }

  async handleWiFiNext() {
    if (!this.setupData.wifiSSID) {
      this.showError('Please select a WiFi network');
      return;
    }

    const password = document.getElementById('wifi-password').value;

    if (this.setupData.wifiSecurity !== 'Open' && !password) {
      this.showError('Please enter the WiFi password');
      return;
    }

    this.setupData.wifiPassword = password;

    this.showLoading('Connecting to WiFi...');

    const result = await this.api.connectWiFi(
      this.setupData.wifiSSID,
      this.setupData.wifiPassword,
      this.setupData.wifiSecurity
    );

    if (!result.success) {
      this.hideLoading();
      this.showError('Failed to connect to WiFi: ' + (result.error || 'Unknown error'));
      return;
    }

    // Connection is async - wait and check if it actually connected
    this.showLoading('Verifying connection...');
    await this.waitForWiFiConnection(this.setupData.wifiSSID);
  }

  async waitForWiFiConnection(ssid, maxAttempts = 15, intervalMs = 1000) {
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      this.showLoading(`Verifying connection... (${attempt}/${maxAttempts})`);

      // Wait before checking (give time for connection to establish)
      await new Promise(resolve => setTimeout(resolve, intervalMs));

      // Use fast connection status endpoint (no caching, real-time status)
      const result = await this.api.getConnectionStatus();
      if (result.success && result.data) {
        console.log(`[WiFi Check ${attempt}]`, result.data);

        // Check if connected to our network
        if (result.data.connected && result.data.ssid === ssid) {
          console.log(`[WiFi] Connected to ${ssid} with IP: ${result.data.ip}`);
          this.hideLoading();
          this.showWiFiConnectionSuccess(ssid, result.data.ip || 'Obtained via DHCP');
          return;
        }

        // status 2 = connecting, keep polling
        if (result.data.status === 2) {
          console.log(`[WiFi] Still connecting to ${ssid}...`);
          continue;
        }

        // status 1 = disconnected, but give more time before failing
        if (result.data.status === 1 && attempt > 10) {
          console.log(`[WiFi] Connection failed for ${ssid} after ${attempt} attempts`);
          this.hideLoading();
          this.showError('WiFi connection failed. Please check the password and try again.');
          return;
        }
      }
    }

    // Timeout - connection took too long
    this.hideLoading();
    this.showError('WiFi connection timed out. Please try again.');
  }

  showWiFiConnectionSuccess(ssid, ipAddress) {
    const listEl = document.getElementById('wifi-list');
    listEl.innerHTML = `
      <div class="wifi-connection-success">
        <div class="success-icon">✓</div>
        <h3>Connected Successfully!</h3>
        <div class="connection-details">
          <div class="detail-row">
            <span class="detail-label">Network:</span>
            <span class="detail-value">${this.escapeHtml(ssid)}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">IP Address:</span>
            <span class="detail-value">${this.escapeHtml(ipAddress)}</span>
          </div>
        </div>
        <p class="success-message">Your device is now connected to the internet.</p>
      </div>
    `;

    // Hide password group since we're connected
    document.getElementById('wifi-password-group').classList.add('hidden');

    // Update button to show "Continue" instead of "Connect"
    const connectBtn = document.querySelector('#step-4 .btn-primary');
    if (connectBtn) {
      connectBtn.textContent = 'Continue →';
      connectBtn.onclick = () => this.showStep(5);
    }
  }

  async handleWiFiSkip() {
    if (confirm('Are you sure you want to skip WiFi setup? You can configure it later.')) {
      this.setupData.wifiSSID = 'Skipped';
      this.showStep(5);
    }
  }

  // Camera Registration Step (OAuth-based)

  async loadRegistrationStep() {
    console.log('Loading camera registration step');

    // Always reset UI first so stale state doesn't bleed between steps/runs
    this.showRegistrationPrompt();

    // IMPORTANT:
    // Do NOT infer "camera registered" from a browser-stored OAuth token.
    // Users often run OOBE from the same phone/laptop for multiple cameras;
    // localStorage persists across devices and would incorrectly skip login.
    //
    // Instead, ask the device (OOBE server) whether the camera has been
    // registered previously by checking the on-device registration file.
    try {
      const resp = await fetch('/oobe/api/registrationStatus', {
        headers: { 'Accept': 'application/json' }
      });

      if (!resp.ok) {
        console.warn('Failed to check registration status:', resp.status);
        return;
      }

      const status = await resp.json();
      if (status?.ok && status?.registered) {
        this.showRegistrationSuccess();
      }
    } catch (e) {
      console.warn('Error checking registration status:', e);
      // If status check fails (e.g., offline), default to prompting sign-in.
    }
  }

  /**
   * Reset registration UI to the default "please sign in" state.
   */
  showRegistrationPrompt() {
    // Show sign-in section
    const signinSection = document.getElementById('signin-section');
    if (signinSection) {
      signinSection.classList.remove('hidden');
    }

    // Hide success section
    const successSection = document.getElementById('registration-success');
    if (successSection) {
      successSection.classList.add('hidden');
    }

    // Hide continue button until registration succeeds
    const continueBtn = document.getElementById('continue-after-registration-btn');
    if (continueBtn) {
      continueBtn.classList.add('hidden');
    }

    // Hide any prior status banner
    const statusEl = document.getElementById('registration-status');
    if (statusEl) {
      statusEl.classList.add('hidden');
    }
  }

  /**
   * Show registration options, hide code registration section
   */
  showRegistrationOptions() {
    document.getElementById('code-registration-section')?.classList.add('hidden');
    document.getElementById('registration-options').classList.remove('hidden');
  }

  // ======================
  // Code Registration Methods
  // ======================

  /**
   * Start code-based registration flow
   */
  async startCodeRegistration() {
    // Get location name from step 3 data
    const locationName = this.setupData.deviceName || 'Unknown Location';

    // Show code registration section, hide registration options
    document.getElementById('registration-options').classList.add('hidden');
    document.getElementById('code-registration-section').classList.remove('hidden');

    // Update UI to show generating state
    this.updateCodeRegistrationUI({
      status: 'generating',
      message: 'Generating registration code...'
    });

    // Start code registration via supervisor
    const result = await this.api.startCodeRegistration(
      locationName,
      this.setupData.geolocation?.latitude,
      this.setupData.geolocation?.longitude
    );

    if (!result.success) {
      // Check if this is an auth error - try to re-authenticate
      if (result.authFailure || result.code === 401) {
        console.log('Auth failure on code registration, attempting re-authentication...');
        if (this.reAuthenticateAfterTimeChange) {
          const reAuthResult = await this.reAuthenticateAfterTimeChange();
          if (reAuthResult.success) {
            console.log('Re-authentication successful, retrying code registration...');
            // Retry the registration
            const retryResult = await this.api.startCodeRegistration(
              locationName,
              this.setupData.geolocation?.latitude,
              this.setupData.geolocation?.longitude
            );
            if (retryResult.success) {
              console.log('Code registration started after re-auth:', retryResult.data);
              this.displayRegistrationCode(retryResult.data.claim_code_formatted, retryResult.data.expires_at);
              this.codeStatusPollInterval = setInterval(() => this.pollCodeStatus(), 2000);
              return;
            }
          }
        }
        this.showError('Session expired. Please go back to the password step and try again.');
        this.showRegistrationOptions();
        return;
      }
      // Check if this is a network error
      if (result.error && (result.error.includes('network') || result.error.includes('connect') || result.error.includes('Failed to fetch'))) {
        this.showError('No internet connection. Please check your network and try again.');
        this.showInternetWarning(true);
      } else {
        this.showError('Failed to start code registration: ' + (result.error || 'Unknown error'));
      }
      this.showRegistrationOptions();
      return;
    }

    console.log('Code registration started:', result.data);

    // Display the code and start countdown
    this.displayRegistrationCode(result.data.claim_code_formatted, result.data.expires_at);

    // Start polling for status updates (every 2 seconds)
    this.codeStatusPollInterval = setInterval(() => this.pollCodeStatus(), 2000);
  }

  /**
   * Display the registration code with countdown timer
   */
  displayRegistrationCode(formattedCode, expiresAt) {
    const codeDisplay = document.getElementById('registration-code-display');
    const timerDisplay = document.getElementById('code-expiry-timer');
    const statusText = document.getElementById('code-status-text');

    if (codeDisplay) {
      codeDisplay.textContent = formattedCode;
    }

    if (statusText) {
      statusText.textContent = 'Enter this code in the Authority Alert mobile app';
    }

    // Start countdown timer
    this.startCodeExpiryTimer(expiresAt);
  }

  /**
   * Start the countdown timer for code expiry
   */
  startCodeExpiryTimer(expiresAtStr) {
    const expiresAt = new Date(expiresAtStr);
    const timerDisplay = document.getElementById('code-expiry-timer');

    // Clear any existing timer
    if (this.codeExpiryInterval) {
      clearInterval(this.codeExpiryInterval);
    }

    const updateTimer = () => {
      const now = new Date();
      const remaining = Math.max(0, Math.floor((expiresAt - now) / 1000));

      if (remaining <= 0) {
        if (timerDisplay) {
          timerDisplay.textContent = 'Expired';
          timerDisplay.classList.add('expired');
        }
        clearInterval(this.codeExpiryInterval);
        return;
      }

      const minutes = Math.floor(remaining / 60);
      const seconds = remaining % 60;
      const timeStr = `${minutes}:${seconds.toString().padStart(2, '0')}`;

      if (timerDisplay) {
        timerDisplay.textContent = `Expires in ${timeStr}`;
        // Add warning class when less than 2 minutes
        if (remaining < 120) {
          timerDisplay.classList.add('warning');
        } else {
          timerDisplay.classList.remove('warning');
        }
      }
    };

    // Update immediately and then every second
    updateTimer();
    this.codeExpiryInterval = setInterval(updateTimer, 1000);
  }

  /**
   * Poll for code registration status updates
   */
  async pollCodeStatus() {
    const result = await this.api.getCodeRegistrationStatus();
    if (!result.success) {
      // Network error during polling - show warning but keep trying
      console.warn('Failed to poll code status:', result.error);
      this.showInternetWarning(true);
      return;
    }

    const status = result.data;
    this.updateCodeRegistrationUI(status);

    // Handle internet availability status
    if (status.internet_available === false) {
      this.showInternetWarning(true);
      // Show "Continue in Background" button
      const bgBtn = document.getElementById('continue-background-btn');
      if (bgBtn) bgBtn.classList.remove('hidden');
    } else {
      this.showInternetWarning(false);
      // Hide background button if internet is back
      const bgBtn = document.getElementById('continue-background-btn');
      if (bgBtn && status.status === 'active') bgBtn.classList.add('hidden');
    }

    // Handle terminal states
    if (status.status === 'claimed') {
      this.stopCodePolling();
      this.showInternetWarning(false);
      // Show success and continue button
      document.getElementById('code-registration-section').classList.add('hidden');
      document.getElementById('registration-success').classList.remove('hidden');
      document.getElementById('continue-after-registration-btn').classList.remove('hidden');
      console.log('Code registration completed:', status.result);
    } else if (status.status === 'error' || status.status === 'expired') {
      this.stopCodePolling();
      this.showInternetWarning(false);
      this.showError(status.message || 'Code registration failed');
      this.showRegistrationOptions();
    }
  }

  /**
   * Show or hide the internet warning banner
   */
  showInternetWarning(show) {
    const warningEl = document.getElementById('internet-warning');
    if (warningEl) {
      if (show) {
        warningEl.classList.remove('hidden');
      } else {
        warningEl.classList.add('hidden');
      }
    }
  }

  /**
   * Exit OOBE and navigate to supervisor UI
   * Registration will continue in background
   */
  exitToSupervisorUI() {
    // Stop polling in OOBE (backend continues independently)
    this.stopCodePolling();

    // Complete OOBE (remove flag file)
    fetch('/oobe/api/complete', { method: 'POST' })
      .then(() => console.log('OOBE completed, navigating to supervisor UI'))
      .catch(e => console.warn('Error completing OOBE:', e));

    // Navigate to main supervisor UI
    // Note: Registration continues in background on the device
    window.location.href = '/';
  }

  /**
   * Update the code registration UI based on status
   */
  updateCodeRegistrationUI(status) {
    const statusText = document.getElementById('code-status-text');
    const statusIcon = document.getElementById('code-status-icon');

    // Default messages for each status
    const defaultMessages = {
      'idle': 'Ready',
      'generating': 'Generating registration code...',
      'active': 'Enter this code in the Authority Alert mobile app',
      'claimed': 'Camera registered successfully!',
      'expired': 'Code expired. Please try again.',
      'error': status.message || 'An error occurred'
    };

    const icons = {
      'idle': '',
      'generating': '⏳',
      'active': '📱',
      'claimed': '✓',
      'expired': '⏰',
      'error': '❌'
    };

    // Use server-provided message if available (may include retry info)
    let displayMessage = status.message || defaultMessages[status.status] || status.status;
    let displayIcon = icons[status.status] || '';

    // Special handling for "Setting up..." state (confirming with platform)
    if (status.message === 'Setting up...' || status.status === 'confirming') {
      displayIcon = '⏳';
      displayMessage = 'Setting up...';
    }

    // Special handling for internet issues
    if (status.internet_available === false) {
      displayIcon = '⚠️';
      if (status.retry_count > 0) {
        displayMessage = `No internet. Retrying... (attempt ${status.retry_count})`;
      } else {
        displayMessage = 'No internet. We\'ll keep trying in the background.';
      }
    }

    if (statusText) {
      statusText.textContent = displayMessage;
    }
    if (statusIcon) {
      statusIcon.textContent = displayIcon;
    }
  }

  /**
   * Cancel code registration and return to options
   */
  cancelCodeRegistration() {
    this.stopCodePolling();
    // Tell supervisor to cancel (fire and forget)
    this.api.cancelCodeRegistration();
    this.showRegistrationOptions();
  }

  /**
   * Stop code registration polling and timers
   */
  stopCodePolling() {
    if (this.codeStatusPollInterval) {
      clearInterval(this.codeStatusPollInterval);
      this.codeStatusPollInterval = null;
    }
    if (this.codeExpiryInterval) {
      clearInterval(this.codeExpiryInterval);
      this.codeExpiryInterval = null;
    }
  }

  /**
   * Show registration success UI
   */
  showRegistrationSuccess() {
    // Hide sign-in section
    const signinSection = document.getElementById('signin-section');
    if (signinSection) {
      signinSection.classList.add('hidden');
    }
    
    // Show success section
    const successSection = document.getElementById('registration-success');
    if (successSection) {
      successSection.classList.remove('hidden');
    }
    
    // Show continue button
    const continueBtn = document.getElementById('continue-after-registration-btn');
    if (continueBtn) {
      continueBtn.classList.remove('hidden');
    }
    
    // Update status
    this.updateRegistrationStatus('Camera registered successfully!', 'success', '✓');
  }

  /**
   * Update registration status display
   */
  updateRegistrationStatus(message, type, icon = '') {
    const statusEl = document.getElementById('registration-status');
    const textEl = document.getElementById('registration-status-text');
    const iconEl = document.getElementById('registration-status-icon');

    if (statusEl && textEl) {
      textEl.textContent = message;
      statusEl.className = 'registration-status ' + type;
      statusEl.classList.remove('hidden');

      if (iconEl && icon) {
        iconEl.textContent = icon;
      }
    }
  }

  handleRegistrationSkip() {
    if (confirm('Are you sure you want to skip camera registration? You can register later from the settings page.')) {
      this.showStep(6);
    }
  }

  handleRegistrationComplete() {
    this.showStep(6);
  }

  showInfo(message) {
    // Similar to showError but with info styling
    const alertEl = document.getElementById('error-alert');
    const messageEl = document.getElementById('error-message');
    
    if (alertEl && messageEl) {
      // Change to info styling
      const alertDiv = alertEl.querySelector('.alert');
      if (alertDiv) {
        alertDiv.className = 'alert alert-info';
      }
      
      messageEl.innerHTML = '<strong>Info:</strong> ' + message;
      alertEl.classList.remove('hidden');
      
      // Auto-hide after 8 seconds
      setTimeout(() => {
        alertEl.classList.add('hidden');
      }, 8000);
    }
  }

  async handleComplete() {
    this.showLoading('Finalizing setup...');

    // Optional: Set LED to indicate completion
    try {
      await this.api.setLED('led0', 255, 'heartbeat');
    } catch (e) {
      console.log('Could not set LED');
    }

    // Signal OOBE completion - removes /etc/oobe/flag
    try {
      await fetch('/oobe/api/complete', { method: 'POST' });
      console.log('OOBE flag removed');
    } catch (e) {
      console.error('Error completing OOBE:', e);
    }

    this.hideLoading();

    // Show success message
    document.getElementById('completion-message').innerHTML = `
      <div class="alert alert-success">
        <strong>Setup Complete!</strong><br>
        Your Authority Alert device is ready to use.
      </div>
    `;

    // Redirect to main dashboard after a delay
    setTimeout(() => {
      window.location.href = '/';
    }, 2000);
  }

  // UI Helpers

  showLoading(message = 'Loading...') {
    const loadingEl = document.getElementById('loading-overlay');
    const messageEl = document.getElementById('loading-message');
    
    if (loadingEl && messageEl) {
      messageEl.textContent = message;
      loadingEl.classList.remove('hidden');
    }
  }

  hideLoading() {
    const loadingEl = document.getElementById('loading-overlay');
    if (loadingEl) {
      loadingEl.classList.add('hidden');
    }
  }

  showError(message) {
    const alertEl = document.getElementById('error-alert');
    const messageEl = document.getElementById('error-message');
    
    if (alertEl && messageEl) {
      messageEl.textContent = message;
      alertEl.classList.remove('hidden');
      
      // Auto-hide after 5 seconds
      setTimeout(() => {
        alertEl.classList.add('hidden');
      }, 5000);
    } else {
      alert(message);
    }
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  window.oobeApp = new OOBEApp();
});

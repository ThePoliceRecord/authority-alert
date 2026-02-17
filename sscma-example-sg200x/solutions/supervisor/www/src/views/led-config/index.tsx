import { useState, useEffect } from "react";
import { Switch, message, Spin, Select, Slider, Segmented, Tooltip } from "antd";
import { BulbOutlined, SaveOutlined, SettingOutlined, ThunderboltOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { getLEDsApi, setLEDApi, getLEDTriggersApi, LEDInfo } from "@/api/led";

// LED configuration state with extended properties
interface LEDConfigState {
  brightness: number;
  maxBrightness: number;
  trigger: string;
  availableTriggers: string[];
  mode: 'system' | 'manual'; // 'system' uses trigger, 'manual' uses direct brightness control
}

interface LEDConfigs {
  [key: string]: LEDConfigState;
}

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

// LED display configuration
interface LEDDisplayInfo {
  label: string;
  description: string;
  color: string;
  systemDescription: string;
}

const ledDisplayConfig: { [key: string]: LEDDisplayInfo } = {
  white: {
    label: "White Light LEDs",
    description: "Illumination LEDs for night vision",
    color: "#ffffff",
    systemDescription: "System controls brightness based on ambient light or triggers",
  },
  blue: {
    label: "Blue LED",
    description: "Status indicator - typically shows connectivity",
    color: "#3b82f6",
    systemDescription: "System uses trigger patterns for network/activity status",
  },
  red: {
    label: "Red LED",
    description: "Status indicator - typically shows recording",
    color: "#ef4444",
    systemDescription: "System uses trigger patterns for recording/error states",
  },
};

// Common trigger descriptions for better UX
const triggerDescriptions: { [key: string]: string } = {
  none: "LED stays at set brightness (manual control)",
  heartbeat: "Pulsing pattern indicating system is alive",
  timer: "Blink at regular intervals",
  default_on: "LED on by default",
  "default-on": "LED on by default",
  mmc0: "Activity indicator for storage (eMMC/SD)",
  mmc1: "Activity indicator for storage (SD)",
  mmc2: "Activity indicator for storage (WiFi-SDIO)",
  cpu: "CPU activity indicator",
  ir: "Infrared sensor activity",
  network: "Network activity indicator",
  gpio: "GPIO pin state indicator",
};

// Consumer-friendly trigger names shown in the UI
const triggerDisplayNames: { [key: string]: string } = {
  none: "Manual (no trigger)",
  heartbeat: "Heartbeat (system alive)",
  timer: "Blink (fixed interval)",
  default_on: "Always on",
  "default-on": "Always on",
  mmc0: "Storage activity",
  mmc1: "Storage activity",
  mmc2: "Storage activity",
};

// Triggers we consider "safe" / meaningful for regular users per LED
const consumerAllowedTriggers: Record<string, string[]> = {
  red: ["heartbeat", "timer", "default_on", "default-on", "mmc0", "none"],
  blue: ["heartbeat", "timer", "default_on", "default-on", "mmc0", "none"],
  white: ["timer", "default_on", "default-on", "none"],
};

function LEDConfig() {
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [configs, setConfigs] = useState<LEDConfigs>({});
  const [originalConfigs, setOriginalConfigs] = useState<LEDConfigs>({});
  const [loadError, setLoadError] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Dark-theme overrides for AntD Select inside this page
  const antdDarkCss = `
    .led-select .ant-select-selector {
      background: rgba(255, 255, 255, 0.08) !important;
      border-color: rgba(255, 255, 255, 0.18) !important;
      color: #e0e0e0 !important;
    }
    .led-select .ant-select-selection-item,
    .led-select .ant-select-selection-placeholder {
      color: rgba(224, 224, 224, 0.85) !important;
    }
    .led-select .ant-select-arrow {
      color: rgba(224, 224, 224, 0.65) !important;
    }
    .led-select-dropdown {
      background: #1f1f1b !important;
      border: 1px solid rgba(255, 255, 255, 0.12) !important;
    }
    .led-select-dropdown .ant-select-item {
      color: rgba(224, 224, 224, 0.92) !important;
    }
    .led-select-dropdown .ant-select-item-option-active:not(.ant-select-item-option-disabled),
    .led-select-dropdown .ant-select-item-option-selected:not(.ant-select-item-option-disabled) {
      background: rgba(255, 255, 255, 0.08) !important;
    }
  `;

  const isConsumerLed = (name: string) => {
    // Only show the 3 user-facing LEDs by default.
    // Everything else (e.g. mmc0::, mmc1::, mmc2::) is treated as system/advanced.
    return name === "red" || name === "blue" || name === "white";
  };

  const getConsumerTriggersForLed = (ledName: string, triggers: string[], current: string) => {
    if (showAdvanced) return triggers;

    const allowed = consumerAllowedTriggers[ledName];
    if (!allowed) {
      // Unknown LED: only show 'none' + current trigger so the UI remains usable.
      const base = new Set<string>(["none", current]);
      return triggers.filter((t) => base.has(t));
    }

    // Always include current (so we can display whatever the system is currently using)
    // and include 'none' (manual override).
    const allowedSet = new Set<string>([...allowed, current, "none"]);
    return triggers.filter((t) => allowedSet.has(t));
  };

  const getTriggerLabel = (trigger: string) => triggerDisplayNames[trigger] || trigger;
  const getTriggerDescription = (trigger: string): string => {
    return triggerDescriptions[trigger] || `${trigger} trigger`;
  };

  // Load LED states and triggers on mount
  useEffect(() => {
    const loadLEDs = async () => {
      setLoading(true);
      setLoadError(null);
      try {
        console.log("Calling getLEDsApi...");
        const response = await getLEDsApi();
        console.log("getLEDsApi raw response:", response);
        
        // The response structure is: { code: 0, msg: "OK", data: { leds: LEDInfo[] } }
        // Check if API returned an error
        if (response.code !== 0) {
          console.error("API returned error:", response);
          setLoadError(response.msg || "Failed to load LED configuration");
          return;
        }
        
        // Access the leds array from response.data
        const leds = response.data?.leds;
        console.log("Parsed leds array:", leds);
        
        if (!leds || !Array.isArray(leds)) {
          console.error("LEDs is not an array or undefined:", leds);
          setLoadError("Invalid LED data format received from server");
          return;
        }
        
        if (leds.length === 0) {
          console.warn("No LEDs found in response - this device may not have controllable LEDs");
          setLoadError("No controllable LEDs found on this device");
          return;
        }
        
        const newConfigs: LEDConfigs = {};
        
        // Load each LED's data including triggers
        for (const led of leds as LEDInfo[]) {
          console.log("Processing LED:", led);
          let availableTriggers: string[] = ["none"];
          try {
            const triggersResponse = await getLEDTriggersApi(led.name);
            console.log(`Triggers for ${led.name}:`, triggersResponse);
            // Response structure: { code: 0, data: { triggers: string[], current: string } }
            if (triggersResponse.code === 0 && triggersResponse.data?.triggers) {
              availableTriggers = triggersResponse.data.triggers;
            }
          } catch (err) {
            console.warn(`Could not load triggers for ${led.name}:`, err);
          }
          
          // Determine if LED is in manual mode (trigger is "none" or no trigger)
          const isManual = !led.trigger || led.trigger === "none";
          
          newConfigs[led.name] = {
            brightness: led.brightness || 0,
            maxBrightness: led.max_brightness || 255,
            trigger: led.trigger || "none",
            availableTriggers,
            mode: isManual ? 'manual' : 'system',
          };
        }
        
        console.log("Final configs:", newConfigs);
        setConfigs(newConfigs);
        setOriginalConfigs(JSON.parse(JSON.stringify(newConfigs)));
      } catch (error: any) {
        console.error("Failed to load LED configuration:", error);
        if (error?.response?.status === 401 || error?.status === 401) {
          setLoadError("Session expired. Please log in again.");
        } else {
          setLoadError(error?.msg || error?.message || "Failed to load LED configuration");
          messageApi.error("Failed to load LED configuration");
        }
      } finally {
        setLoading(false);
      }
    };
    loadLEDs();
  }, [messageApi]);

  const handleModeChange = (ledName: string, mode: 'system' | 'manual') => {
    setConfigs(prev => ({
      ...prev,
      [ledName]: {
        ...prev[ledName],
        mode,
        // When switching to manual, set trigger to "none"
        // When switching to system, set a default trigger if available
        trigger: mode === 'manual' ? 'none' : (prev[ledName].availableTriggers.find(t => t !== 'none') || 'none'),
      },
    }));
  };

  const handleTriggerChange = (ledName: string, trigger: string) => {
    setConfigs(prev => ({
      ...prev,
      [ledName]: {
        ...prev[ledName],
        trigger,
      },
    }));
  };

  const handleBrightnessChange = (ledName: string, brightness: number) => {
    setConfigs(prev => ({
      ...prev,
      [ledName]: {
        ...prev[ledName],
        brightness,
      },
    }));
  };

  const handleToggle = (ledName: string, checked: boolean) => {
    const config = configs[ledName];
    // Toggle between off (0) and max brightness
    const newBrightness = checked ? config.maxBrightness : 0;
    handleBrightnessChange(ledName, newBrightness);
  };

  const hasChanges = () => {
    return JSON.stringify(configs) !== JSON.stringify(originalConfigs);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      // Save each LED state
      for (const [name, config] of Object.entries(configs)) {
        // If mode is manual, first set trigger to "none", then set brightness
        // If mode is system, set the selected trigger
        if (config.mode === 'manual') {
          await setLEDApi({
            name,
            trigger: 'none',
            brightness: config.brightness,
          });
        } else {
          await setLEDApi({
            name,
            trigger: config.trigger,
          });
        }
      }
      setOriginalConfigs(JSON.parse(JSON.stringify(configs)));
      messageApi.success("LED settings saved successfully");
    } catch (error) {
      console.error("Failed to save LED configuration:", error);
      messageApi.error("Failed to save LED settings");
    } finally {
      setSaving(false);
    }
  };

  const getLedDisplayInfo = (name: string): LEDDisplayInfo => {
    return ledDisplayConfig[name] || {
      label: name.charAt(0).toUpperCase() + name.slice(1) + " LED",
      description: "LED indicator",
      color: "#888888",
      systemDescription: "System controlled LED",
    };
  };

  return (
    <div className="p-16">
      <style>{antdDarkCss}</style>
      {contextHolder}
      
      {loading ? (
        <div className="flex justify-center items-center py-64">
          <Spin size="large" />
        </div>
      ) : (
        <>
          {/* Page Header */}
          <div className="mb-24">
            <div className="flex items-center justify-between gap-12 mb-8">
              <div className="flex items-center gap-12">
                <BulbOutlined style={{ fontSize: 28, color: '#9be564' }} />
                <h1 className="text-28 font-bold text-platinum m-0">LED Configuration</h1>
              </div>

              <div className="flex items-center gap-8">
                <span className="text-12 text-platinum/60">Advanced</span>
                <Switch checked={showAdvanced} onChange={setShowAdvanced} />
              </div>
            </div>
            <p className="text-14 text-platinum/60 mt-8">
              Control the LED indicators on your camera. Choose between system-controlled triggers or manual brightness control.
            </p>
            {!showAdvanced && (
              <p className="text-12 text-platinum/40 mt-6">
                Showing consumer LEDs only (red/blue/white). Enable Advanced to see system LEDs like mmc activity LEDs.
              </p>
            )}
          </div>

          {/* LED Controls */}
          <div className="mb-24">
            <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
              LED Controls
            </div>
            <div className="space-y-16">
              {Object.entries(configs)
                .filter(([ledName]) => showAdvanced || isConsumerLed(ledName))
                .map(([ledName, config]) => {
                const displayInfo = getLedDisplayInfo(ledName);
                const isOn = config.brightness > 0;

                const shownTriggers = getConsumerTriggersForLed(
                  ledName,
                  config.availableTriggers,
                  config.trigger
                );
                
                return (
                  <div
                    key={ledName}
                    className="p-20"
                    style={translucentCardStyle}
                  >
                    {/* LED Header with visual indicator */}
                    <div className="flex items-start justify-between mb-16">
                      <div className="flex items-center gap-16">
                        <div
                          className="w-48 h-48 rounded-full flex items-center justify-center"
                          style={{
                            backgroundColor: isOn
                              ? `${displayInfo.color}30`
                              : "rgba(224, 224, 224, 0.1)",
                            boxShadow: isOn
                              ? `0 0 20px ${displayInfo.color}60`
                              : "none",
                            transition: "all 0.3s ease",
                          }}
                        >
                          <div
                            className="w-20 h-20 rounded-full"
                            style={{
                              backgroundColor: isOn ? displayInfo.color : "#666666",
                              boxShadow: isOn
                                ? `0 0 12px ${displayInfo.color}`
                                : "none",
                              transition: "all 0.3s ease",
                            }}
                          />
                        </div>
                        <div>
                          <div className="text-18 font-semibold text-platinum">
                            {displayInfo.label}
                          </div>
                          <div className="text-13 text-platinum/50 mt-2">
                            {displayInfo.description}
                          </div>
                        </div>
                      </div>
                      
                      {/* Quick toggle for manual mode */}
                      {config.mode === 'manual' && (
                        <Switch
                          checked={isOn}
                          onChange={(checked) => handleToggle(ledName, checked)}
                        />
                      )}
                    </div>

                    {/* Mode Selector */}
                    <div className="mb-16">
                      <div className="flex items-center gap-8 mb-8">
                        <span className="text-13 text-platinum/70">Control Mode</span>
                        <Tooltip title="System mode uses predefined triggers that let the OS control the LED based on events. Manual mode gives you direct control over brightness.">
                          <InfoCircleOutlined className="text-platinum/40 cursor-help" />
                        </Tooltip>
                      </div>
                      <Segmented
                        value={config.mode}
                        onChange={(value) => handleModeChange(ledName, value as 'system' | 'manual')}
                        options={[
                          {
                            value: 'system',
                            label: (
                              <div className="flex items-center gap-8 px-8 py-4">
                                <SettingOutlined />
                                <span>System Default</span>
                              </div>
                            ),
                          },
                          {
                            value: 'manual',
                            label: (
                              <div className="flex items-center gap-8 px-8 py-4">
                                <ThunderboltOutlined />
                                <span>Manual Control</span>
                              </div>
                            ),
                          },
                        ]}
                        style={{
                          backgroundColor: 'rgba(255, 255, 255, 0.1)',
                        }}
                      />
                    </div>

                    {/* System Mode - Trigger Selection */}
                    {config.mode === 'system' && (
                      <div className="mt-16 p-16 rounded-lg" style={{ backgroundColor: 'rgba(0, 0, 0, 0.2)' }}>
                        <div className="flex items-center gap-8 mb-12">
                          <SettingOutlined className="text-platinum/60" />
                          <span className="text-14 font-medium text-platinum/80">System Trigger</span>
                        </div>
                        <Select
                          className="led-select"
                          popupClassName="led-select-dropdown"
                          value={config.trigger}
                          onChange={(value) => handleTriggerChange(ledName, value)}
                          style={{ width: '100%' }}
                          options={shownTriggers.map(t => ({
                            value: t,
                            label: getTriggerLabel(t),
                            description: getTriggerDescription(t),
                          })) as any}
                          optionRender={(option: any) => (
                            <div className="py-4">
                              <div className="font-medium">{option.label}</div>
                              <div className="text-12 text-gray-400">{option.data?.description}</div>
                            </div>
                          )}
                        />
                        <div className="mt-12 text-12 text-platinum/50 italic">
                          {displayInfo.systemDescription}
                        </div>
                      </div>
                    )}

                    {/* Manual Mode - Brightness Control */}
                    {config.mode === 'manual' && (
                      <div className="mt-16 p-16 rounded-lg" style={{ backgroundColor: 'rgba(0, 0, 0, 0.2)' }}>
                        <div className="flex items-center gap-8 mb-12">
                          <ThunderboltOutlined className="text-platinum/60" />
                          <span className="text-14 font-medium text-platinum/80">Brightness Level</span>
                        </div>
                        <div className="flex items-center gap-16">
                          <Slider
                            value={config.brightness}
                            min={0}
                            max={config.maxBrightness}
                            onChange={(value) => handleBrightnessChange(ledName, value)}
                            style={{ flex: 1 }}
                            trackStyle={{ backgroundColor: displayInfo.color }}
                            handleStyle={{ borderColor: displayInfo.color }}
                          />
                          <div 
                            className="w-48 text-center text-14 font-mono"
                            style={{ color: displayInfo.color }}
                          >
                            {Math.round((config.brightness / config.maxBrightness) * 100)}%
                          </div>
                        </div>
                        <div className="mt-8 text-12 text-platinum/50">
                          Raw value: {config.brightness} / {config.maxBrightness}
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          {/* Status Summary */}
          <div className="mb-24">
            <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
              Status Summary
            </div>
            <div className="p-20" style={translucentCardStyle}>
              {loadError ? (
                <div className="text-14 text-red-400 text-center py-8">
                  {loadError}
                </div>
              ) : Object.keys(configs).length === 0 ? (
                <div className="text-14 text-platinum/50 text-center py-8">
                  No LEDs detected
                </div>
              ) : (
                <div className="flex flex-wrap gap-12">
                  {Object.entries(configs).map(([ledName, config]) => {
                    const displayInfo = getLedDisplayInfo(ledName);
                    const isOn = config.brightness > 0 || config.mode === 'system';
                    const statusText = config.mode === 'system' 
                      ? `${config.trigger}` 
                      : (config.brightness > 0 ? `${Math.round((config.brightness / config.maxBrightness) * 100)}%` : "OFF");
                    
                    return (
                      <div
                        key={ledName}
                        className="flex items-center gap-8 px-12 py-8 rounded-lg"
                        style={{
                          backgroundColor: isOn
                            ? `${displayInfo.color}20`
                            : "rgba(224, 224, 224, 0.1)",
                        }}
                      >
                        <div
                          className="w-8 h-8 rounded-full"
                          style={{
                            backgroundColor: isOn ? displayInfo.color : "#666666",
                          }}
                        />
                        <span
                          className="text-12"
                          style={{
                            color: isOn ? displayInfo.color : "#999999",
                          }}
                        >
                          {displayInfo.label.split(" ")[0]}: {statusText}
                        </span>
                        {config.mode === 'system' && (
                          <span className="text-10 text-platinum/40 ml-4">(auto)</span>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          {/* Save Button */}
          <div className="mt-32">
            <button
              onClick={handleSave}
              disabled={saving || !hasChanges()}
              className="w-full py-14 px-24 rounded-lg font-semibold text-16 text-white flex items-center justify-center gap-8 transition-all duration-200 hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                backgroundColor: hasChanges() ? '#2328bb' : '#444',
                boxShadow: hasChanges() ? '0 4px 12px rgba(35, 40, 187, 0.4)' : 'none',
              }}
            >
              {saving ? <Spin size="small" /> : <SaveOutlined />}
              {saving ? "Saving..." : hasChanges() ? "Save LED Settings" : "No Changes"}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

export default LEDConfig;

import { useState, useEffect } from "react";
import { Radio, Checkbox, message, Spin } from "antd";
import { VideoCameraOutlined, SaveOutlined } from "@ant-design/icons";
import type { RadioChangeEvent, CheckboxProps } from "antd";
import { getRecordingConfigApi, setRecordingConfigApi, RecordingConfig as APIRecordingConfig } from "@/api/recording";

// Recording location options
type RecordingLocation = "sd_card" | "local_storage";

// Recording mode options
type RecordingMode = "motion" | "constant" | "scheduled";

// Days of week
interface DaysOfWeek {
  sunday: boolean;
  monday: boolean;
  tuesday: boolean;
  wednesday: boolean;
  thursday: boolean;
  friday: boolean;
  saturday: boolean;
}

// Recording configuration state
interface RecordingConfig {
  location: RecordingLocation;
  mode: RecordingMode;
  schedule: {
    days: DaysOfWeek;
    startTime: string;
    endTime: string;
  };
}

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

const dayLabels: { key: keyof DaysOfWeek; label: string; short: string }[] = [
  { key: "sunday", label: "Sunday", short: "Sun" },
  { key: "monday", label: "Monday", short: "Mon" },
  { key: "tuesday", label: "Tuesday", short: "Tue" },
  { key: "wednesday", label: "Wednesday", short: "Wed" },
  { key: "thursday", label: "Thursday", short: "Thu" },
  { key: "friday", label: "Friday", short: "Fri" },
  { key: "saturday", label: "Saturday", short: "Sat" },
];

function Recording() {
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  
  const [config, setConfig] = useState<RecordingConfig>({
    location: "local_storage",
    mode: "motion",
    schedule: {
      days: {
        sunday: false,
        monday: true,
        tuesday: true,
        wednesday: true,
        thursday: true,
        friday: true,
        saturday: false,
      },
      startTime: "00:00",
      endTime: "00:00",
    },
  });

  // Load configuration on mount
  useEffect(() => {
    const loadConfig = async () => {
      setLoading(true);
      try {
        const response = await getRecordingConfigApi();
        // Map API response to local state
        setConfig({
          location: response.data.location,
          mode: response.data.mode,
          schedule: {
            days: response.data.schedule.days,
            startTime: response.data.schedule.start_time,
            endTime: response.data.schedule.end_time,
          },
        });
      } catch (error) {
        console.error("Failed to load recording configuration:", error);
        messageApi.error("Failed to load recording configuration");
      } finally {
        setLoading(false);
      }
    };
    loadConfig();
  }, [messageApi]);

  const handleLocationChange = (e: RadioChangeEvent) => {
    setConfig((prev) => ({
      ...prev,
      location: e.target.value as RecordingLocation,
    }));
  };

  const handleModeChange = (e: RadioChangeEvent) => {
    setConfig((prev) => ({
      ...prev,
      mode: e.target.value as RecordingMode,
    }));
  };

  const handleDayChange = (day: keyof DaysOfWeek): CheckboxProps["onChange"] => {
    return (e) => {
      setConfig((prev) => ({
        ...prev,
        schedule: {
          ...prev.schedule,
          days: {
            ...prev.schedule.days,
            [day]: e.target.checked,
          },
        },
      }));
    };
  };

  const handleStartTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const time = e.target.value;
    if (time) {
      setConfig((prev) => ({
        ...prev,
        schedule: {
          ...prev.schedule,
          startTime: time,
        },
      }));
    }
  };

  const handleEndTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const time = e.target.value;
    if (time) {
      setConfig((prev) => ({
        ...prev,
        schedule: {
          ...prev.schedule,
          endTime: time,
        },
      }));
    }
  };

  const is24HourRecording = config.schedule.startTime === "00:00" && config.schedule.endTime === "00:00";

  const handleSave = async () => {
    setSaving(true);
    try {
      // Map local state to API format
      const apiConfig: APIRecordingConfig = {
        location: config.location,
        mode: config.mode,
        schedule: {
          days: config.schedule.days,
          start_time: config.schedule.startTime,
          end_time: config.schedule.endTime,
        },
      };
      
      await setRecordingConfigApi(apiConfig);
      messageApi.success("Recording settings saved successfully");
    } catch (error) {
      console.error("Failed to save recording configuration:", error);
      messageApi.error("Failed to save recording settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-16">
      {contextHolder}
      
      {loading ? (
        <div className="flex justify-center items-center py-64">
          <Spin size="large" />
        </div>
      ) : (
        <>
      {/* Page Header */}
      <div className="mb-24">
        <div className="flex items-center gap-12 mb-8">
          <VideoCameraOutlined style={{ fontSize: 28, color: '#9be564' }} />
          <h1 className="text-28 font-bold text-platinum m-0">Recording</h1>
        </div>
        <p className="text-14 text-platinum/60 mt-8">
          Configure video recording settings for your camera
        </p>
      </div>

      {/* Recording Location */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
          Recording Location
        </div>
        <div className="p-20" style={translucentCardStyle}>
          <Radio.Group
            value={config.location}
            onChange={handleLocationChange}
            className="w-full"
          >
            <div className="flex flex-col gap-16">
              <Radio value="local_storage" className="text-platinum">
                <div className="ml-8">
                  <div className="text-16 font-medium text-platinum">Local Storage</div>
                  <div className="text-12 text-platinum/50 mt-2">
                    Save recordings to device internal storage
                  </div>
                </div>
              </Radio>
              <Radio value="sd_card" className="text-platinum">
                <div className="ml-8">
                  <div className="text-16 font-medium text-platinum">SD Card</div>
                  <div className="text-12 text-platinum/50 mt-2">
                    Save recordings to external SD card
                  </div>
                </div>
              </Radio>
            </div>
          </Radio.Group>
        </div>
      </div>

      {/* Recording Mode */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
          Recording Mode
        </div>
        <div className="p-20" style={translucentCardStyle}>
          <Radio.Group
            value={config.mode}
            onChange={handleModeChange}
            className="w-full"
          >
            <div className="flex flex-col gap-16">
              <Radio value="motion" className="text-platinum">
                <div className="ml-8">
                  <div className="text-16 font-medium text-platinum">Motion Based Recording</div>
                  <div className="text-12 text-platinum/50 mt-2">
                    Record only when motion is detected
                  </div>
                </div>
              </Radio>
              <Radio value="constant" className="text-platinum">
                <div className="ml-8">
                  <div className="text-16 font-medium text-platinum">Constant Recording</div>
                  <div className="text-12 text-platinum/50 mt-2">
                    Record continuously 24/7
                  </div>
                </div>
              </Radio>
              <Radio value="scheduled" className="text-platinum">
                <div className="ml-8">
                  <div className="text-16 font-medium text-platinum">Scheduled Recording</div>
                  <div className="text-12 text-platinum/50 mt-2">
                    Record during selected times only
                  </div>
                </div>
              </Radio>
            </div>
          </Radio.Group>
        </div>
      </div>

      {/* Time-Based Recording Configuration */}
      {config.mode === "scheduled" && (
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Schedule Configuration
          </div>
          <div className="p-20" style={translucentCardStyle}>
            {/* Days of Week */}
            <div className="mb-20">
              <div className="text-14 font-medium text-platinum mb-12">Days of Week</div>
              <div className="flex flex-wrap gap-8">
                {dayLabels.map(({ key, short }) => (
                  <Checkbox
                    key={key}
                    checked={config.schedule.days[key]}
                    onChange={handleDayChange(key)}
                    className="text-platinum"
                  >
                    <span className="text-platinum">{short}</span>
                  </Checkbox>
                ))}
              </div>
            </div>

            {/* Time Range */}
            <div className="border-t border-white/10 pt-16">
              <div className="text-14 font-medium text-platinum mb-12">Recording Time (24-hour format)</div>
              <div className="flex items-center gap-16 flex-wrap">
                <div className="flex items-center gap-8">
                  <span className="text-14 text-platinum/70">Start:</span>
                  <input
                    type="time"
                    value={config.schedule.startTime}
                    onChange={handleStartTimeChange}
                    className="px-12 py-8 rounded-lg text-14"
                    style={{
                      backgroundColor: 'rgba(255, 255, 255, 0.1)',
                      borderColor: 'rgba(255, 255, 255, 0.2)',
                      border: '1px solid rgba(255, 255, 255, 0.2)',
                      color: '#e0e0e0',
                      width: 120,
                    }}
                  />
                </div>
                <div className="flex items-center gap-8">
                  <span className="text-14 text-platinum/70">End:</span>
                  <input
                    type="time"
                    value={config.schedule.endTime}
                    onChange={handleEndTimeChange}
                    className="px-12 py-8 rounded-lg text-14"
                    style={{
                      backgroundColor: 'rgba(255, 255, 255, 0.1)',
                      borderColor: 'rgba(255, 255, 255, 0.2)',
                      border: '1px solid rgba(255, 255, 255, 0.2)',
                      color: '#e0e0e0',
                      width: 120,
                    }}
                  />
                </div>
              </div>
              {is24HourRecording && (
                <div className="mt-12 px-12 py-8 rounded-lg" style={{ backgroundColor: 'rgba(35, 40, 187, 0.2)' }}>
                  <span className="text-12 text-platinum/80">
                    ℹ️ When both start and end are set to 00:00, recording runs for the full 24 hours on selected days
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Save Button */}
      <div className="mt-32">
        <button
          onClick={handleSave}
          disabled={saving}
          className="w-full py-14 px-24 rounded-lg font-semibold text-16 text-white flex items-center justify-center gap-8 transition-all duration-200 hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          style={{
            backgroundColor: '#2328bb',
            boxShadow: '0 4px 12px rgba(35, 40, 187, 0.4)',
          }}
        >
          {saving ? <Spin size="small" /> : <SaveOutlined />}
          {saving ? "Saving..." : "Save Recording Settings"}
        </button>
      </div>
      </>
      )}
    </div>
  );
}

export default Recording;

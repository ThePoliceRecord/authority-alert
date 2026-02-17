import { supervisorRequest } from "@/utils/request";

// Recording configuration interfaces
export interface RecordingSchedule {
  days: {
    sunday: boolean;
    monday: boolean;
    tuesday: boolean;
    wednesday: boolean;
    thursday: boolean;
    friday: boolean;
    saturday: boolean;
  };
  start_time: string; // HH:MM format
  end_time: string;   // HH:MM format
}

export interface RecordingConfig {
  location: "sd_card" | "local_storage";
  mode: "motion" | "constant" | "scheduled";
  schedule: RecordingSchedule;
}

// Get recording configuration
export const getRecordingConfigApi = async () =>
  supervisorRequest<RecordingConfig>({
    url: "api/recordingMgr/getConfig",
    method: "get",
  });

// Set recording configuration
export const setRecordingConfigApi = async (data: RecordingConfig) =>
  supervisorRequest<{ status: string; message: string }>({
    url: "api/recordingMgr/setConfig",
    method: "post",
    data,
  });

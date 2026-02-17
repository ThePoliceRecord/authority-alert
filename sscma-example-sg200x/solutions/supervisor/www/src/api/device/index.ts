import { supervisorRequest } from "@/utils/request";
import { PowerMode, DeviceChannleMode, SystemUpdateStatus } from "@/enum";
import {
  IDeviceInfo,
  IChannelParams,
  IServiceStatus,
  IIPDevice,
} from "./device";

// 获取设备信息
export const queryDeviceInfoApi = async () =>
  supervisorRequest<IDeviceInfo>(
    {
      url: "api/deviceMgr/queryDeviceInfo",
      method: "get",
    },
    {
      catchs: true,
    }
  );

export const getDeviceListApi = async () =>
  supervisorRequest<{
    deviceList: IIPDevice[];
  }>(
    {
      url: "api/deviceMgr/getDeviceList",
      method: "get",
    },
    {
      catchs: true,
    }
  );

// 获取设备运行状态
export const queryServiceStatusApi = async () =>
  supervisorRequest<IServiceStatus>(
    {
      url: "api/deviceMgr/queryServiceStatus",
      method: "get",
      timeout: 5000,
    },
    {
      catchs: true,
    }
  );

// 修改设备信息
export const updateDeviceInfoApi = async (data: { deviceName: string }) =>
  supervisorRequest<IDeviceInfo>({
    url: "api/deviceMgr/updateDeviceName",
    method: "post",
    data,
  });

// 修改渠道信息
export const changeChannleApi = async (data: IChannelParams) =>
  supervisorRequest({
    url: "api/deviceMgr/updateChannel",
    method: "post",
    data,
  });
// 设备重启与关机
export const setDevicePowerApi = async (data: { mode: PowerMode }) =>
  supervisorRequest({
    url: "api/deviceMgr/setPower",
    method: "post",
    data,
  });
// 更新设备系统
export const updateSystemApi = async () =>
  supervisorRequest({
    url: "api/deviceMgr/updateSystem",
    method: "post",
  });
// 获取设备更新进度
export const getUpdateSystemProgressApi = async () =>
  supervisorRequest<{
    progress: number;
    status?: string;
  }>({
    url: "api/deviceMgr/getUpdateProgress",
    method: "get",
  });
// 更新设备系统
export const cancelUpdateApi = async () =>
  supervisorRequest({
    url: "api/deviceMgr/cancelUpdate",
    method: "post",
  });

// Custom OTA package upload (staged local update)
export interface UploadedUpdatePackageInfo {
  exists: boolean;
  fileName?: string;
  checksum?: string;
  osName?: string;
  version?: string;
  size?: number;
  packageType?: "ota_zip" | "swu" | "upgrade_zip" | string;
}

export const uploadUpdatePackageApi = async (data: FormData, onProgress?: (percent: number) => void) =>
  supervisorRequest<{
    fileName: string;
    checksum: string;
    size: number;
    osName: string;
    version: string;
    packageType?: "ota_zip" | "swu" | "upgrade_zip" | string;
  }>({
    url: "api/deviceMgr/uploadUpdatePackage",
    method: "post",
    data,
    onUploadProgress: (evt) => {
      if (!onProgress) return;
      const total = evt.total ?? 0;
      const loaded = evt.loaded ?? 0;
      if (total > 0) {
        onProgress(Math.min(100, Math.max(0, Math.round((loaded / total) * 100))));
      }
    },
  });

export const getUploadedUpdatePackageApi = async () =>
  supervisorRequest<UploadedUpdatePackageInfo>({
    url: "api/deviceMgr/getUploadedUpdatePackage",
    method: "get",
  });

export const applyUploadedUpdatePackageApi = async () =>
  supervisorRequest<{ status: string }>({
    url: "api/deviceMgr/applyUploadedUpdatePackage",
    method: "post",
  });

// 获取设备更新版本信息
export const getSystemUpdateVesionInfoApi = async (data: {
  url: string;
  channel?: DeviceChannleMode;
  force?: boolean;
}) =>
  supervisorRequest<{
    osName: string;
    osVersion: string;
    status: SystemUpdateStatus;
    checkedAt?: number;
    error?: string;
  }>(
    {
      url: "api/deviceMgr/getSystemUpdateVersion",
      method: "post",
      data,
    },
    {
      catchs: true,
    }
  );

// Lightweight variant used by the Updates page; backend ignores body fields.
export const getSystemUpdateVersionApi = async (force = false) =>
  supervisorRequest<{
    osName: string;
    osVersion: string;
    status: SystemUpdateStatus;
    checkedAt?: number;
    error?: string;
  }>({
    url: "api/deviceMgr/getSystemUpdateVersion",
    method: "post",
    data: { force },
  });

// Update-check progress (used to show feedback while checking for updates)
export const getUpdateCheckProgressApi = async () =>
  supervisorRequest<{
    progress: number;
    status?: string;
  }>({
    url: "api/deviceMgr/getUpdateCheckProgress",
    method: "get",
  });

// 获取模型信息
export const getModelInfoApi = async () =>
  supervisorRequest<Record<string, unknown>>(
    {
      url: "api/deviceMgr/getModelInfo",
      method: "get",
    },
    {
      catchs: true,
    }
  );

export const uploadModelApi = async (data: FormData) =>
  supervisorRequest<IDeviceInfo>({
    url: "api/deviceMgr/uploadModel",
    method: "post",
    data,
  });

// 保存平台信息
export const savePlatformInfoApi = async (data: { platform_info: string }) =>
  supervisorRequest({
    url: "api/deviceMgr/savePlatformInfo",
    method: "post",
    data,
  });

// 获取平台信息
export const getPlatformInfoApi = async () =>
  supervisorRequest<{
    platform_info: string;
  }>(
    {
      url: "api/deviceMgr/getPlatformInfo",
      method: "get",
    },
    {
      catchs: true,
    }
  );

// Factory reset - sets the factory reset flag
export const factoryResetApi = async () =>
  supervisorRequest<{
    status: string;
    message: string;
  }>({
    url: "api/deviceMgr/factoryReset",
    method: "post",
  });

// Format SD card - formats the SD card with ext4
export const formatSDCardApi = async () =>
  supervisorRequest<{
    status: string;
    message: string;
  }>({
    url: "api/deviceMgr/formatSDCard",
    method: "post",
  });

// Analytics configuration
export const getAnalyticsConfigApi = async () =>
  supervisorRequest<{
    enabled: boolean;
  }>({
    url: "api/deviceMgr/getAnalyticsConfig",
    method: "get",
  });

export const setAnalyticsConfigApi = async (data: { enabled: boolean }) =>
  supervisorRequest<{
    status: string;
    message: string;
    enabled: boolean;
  }>({
    url: "api/deviceMgr/setAnalyticsConfig",
    method: "post",
    data,
  });

// Camera re-registration
export const reRegisterCameraApi = async () =>
  supervisorRequest<{
    status: string;
    message: string;
  }>({
    url: "api/deviceMgr/reRegisterCamera",
    method: "post",
  });

// Model update status
export interface ModelUpdateStatus {
  last_check: string;
  last_update: string;
  current_version: number;
  latest_version: number;
  update_available: boolean;
  is_downloading: boolean;
  download_progress: number;
  last_error: string;
  model_file: string;
}

export const getModelUpdateStatusApi = async () =>
  supervisorRequest<ModelUpdateStatus>({
    url: "api/deviceMgr/getModelUpdateStatus",
    method: "get",
  });

export const checkModelUpdatesApi = async () =>
  supervisorRequest<{
    status: string;
    message: string;
  }>({
    url: "api/deviceMgr/checkModelUpdates",
    method: "post",
  });

// Update configuration
export interface UpdateConfig {
  os_source: "tpr_official" | "self_hosted";
  model_source: "tpr_official" | "self_hosted";
  self_hosted_os_url: string;
  self_hosted_model_url: string;
  check_frequency: "1min" | "30min" | "daily" | "weekly" | "manual";
  weekly_day: "sunday" | "monday" | "tuesday" | "wednesday" | "thursday" | "friday" | "saturday";
}

export const getUpdateConfigApi = async () =>
  supervisorRequest<UpdateConfig>({
    url: "api/updateMgr/getConfig",
    method: "get",
  });

export const setUpdateConfigApi = async (data: UpdateConfig) =>
  supervisorRequest<{
    status: string;
    message: string;
  }>({
    url: "api/updateMgr/setConfig",
    method: "post",
    data,
  });

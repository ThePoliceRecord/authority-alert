import { useState, useEffect, useRef } from "react";
import { Radio, Select, Button, Input, message, Spin, Progress, Modal } from "antd";
import { CloudDownloadOutlined, SyncOutlined, CheckCircleOutlined, LinkOutlined, UploadOutlined, CloseCircleOutlined } from "@ant-design/icons";
import type { RadioChangeEvent } from "antd";
import React from "react";
import {
  queryDeviceInfoApi,
  getUpdateConfigApi,
  setUpdateConfigApi,
  UpdateConfig as APIUpdateConfig,
  updateSystemApi,
  uploadUpdatePackageApi,
  getUploadedUpdatePackageApi,
  applyUploadedUpdatePackageApi,
  getUpdateSystemProgressApi,
  cancelUpdateApi,
  getSystemUpdateVersionApi,
  getUpdateCheckProgressApi,
  getModelUpdateStatusApi,
  checkModelUpdatesApi,
  type UploadedUpdatePackageInfo,
} from "@/api/device";
import { SystemUpdateStatus } from "@/enum";

// Update source options
type UpdateSource = "tpr_official" | "self_hosted";

// Update check frequency options
type UpdateFrequency = "30min" | "daily" | "weekly" | "manual";

// Day of week for weekly updates
type DayOfWeek = "sunday" | "monday" | "tuesday" | "wednesday" | "thursday" | "friday" | "saturday";

// Update configuration state
interface UpdateConfig {
  osSource: UpdateSource;
  modelSource: UpdateSource;
  selfHostedOSUrl: string;
  selfHostedModelUrl: string;
  checkFrequency: UpdateFrequency;
  weeklyDay: DayOfWeek;
}

// Update info
interface UpdateInfo {
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  lastChecked: string;
  // Raw versions for comparison/decision-making.
  currentVersionRaw?: string;
  latestVersionRaw?: string;
  isDowngrade?: boolean;
}

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

// Best-effort version parsing + comparison (handles "v1.2.3", "1.2.3-rc1", etc.).
const parseVersionParts = (v?: string): number[] => {
  if (!v) return [];
  const s = v.trim().replace(/^v\s*/i, "");
  // Split on dots and dashes; take the first number from each segment.
  const segs = s.split(/[.\-]/g);
  const out: number[] = [];
  for (const seg of segs) {
    const m = seg.match(/\d+/);
    if (!m) {
      out.push(0);
      continue;
    }
    const n = Number.parseInt(m[0], 10);
    out.push(Number.isFinite(n) ? n : 0);
  }
  // Trim trailing zeros to avoid false “downgrade” comparisons like 1.2 == 1.2.0
  while (out.length > 0 && out[out.length - 1] === 0) out.pop();
  return out;
};

const compareVersions = (a?: string, b?: string): number => {
  const ap = parseVersionParts(a);
  const bp = parseVersionParts(b);
  const n = Math.max(ap.length, bp.length);
  for (let i = 0; i < n; i++) {
    const ai = ap[i] ?? 0;
    const bi = bp[i] ?? 0;
    if (ai > bi) return 1;
    if (ai < bi) return -1;
  }
  return 0;
};

const dayOptions = [
  { value: "sunday", label: "Sunday" },
  { value: "monday", label: "Monday" },
  { value: "tuesday", label: "Tuesday" },
  { value: "wednesday", label: "Wednesday" },
  { value: "thursday", label: "Thursday" },
  { value: "friday", label: "Friday" },
  { value: "saturday", label: "Saturday" },
];

const frequencyOptions = [
  { value: "30min", label: "Every 30 minutes" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "manual", label: "Manual only" },
];

function Updates() {
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);

  const formatPackageType = (t?: string) => {
    switch ((t || "").toLowerCase()) {
      case "ota_zip":
      case "ota":
        return "ota_zip (A/B)";
      case "swu":
        return "swu (SWUpdate)";
      case "upgrade_zip":
        return "upgrade_zip";
      default:
        return t || "(unknown)";
    }
  };

  const progressTimerRef = useRef<number | null>(null);
  const progressPollStartedAtRef = useRef<number | null>(null);
  const progressSeenActiveRef = useRef<boolean>(false);
  const progressIdleCountRef = useRef<number>(0);
  const osCheckTimerRef = useRef<number | null>(null);
  const [selectedPackage, setSelectedPackage] = useState<File | null>(null);
  const [uploadingPackage, setUploadingPackage] = useState(false);
  const [uploadProgressPct, setUploadProgressPct] = useState<number>(0);
  const [stagedPackage, setStagedPackage] = useState<UploadedUpdatePackageInfo | null>(null);
  const [applyingStagedPackage, setApplyingStagedPackage] = useState(false);
  const [updateProgress, setUpdateProgress] = useState<{ progress: number; status?: string }>({
    progress: 0,
    status: "idle",
  });
  // Treat any non-idle/non-cancelled/non-failed status as an active install.
  // (Backend uses statuses like "download", "upgrade: ...", "failed: ...", "idle", "cancelled".)
  const isSystemUpdateRunning = (() => {
    const st = (updateProgress.status || "idle").toLowerCase();
    if (!st || st === "idle" || st === "cancelled") return false;
    if (st.startsWith("failed")) return false;
    return true;
  })();
  const [osCheckProgress, setOsCheckProgress] = useState<{ progress: number; status?: string }>({
    progress: 0,
    status: "idle",
  });

  const [config, setConfig] = useState<UpdateConfig>({
    osSource: "tpr_official",
    modelSource: "tpr_official",
    selfHostedOSUrl: "",
    selfHostedModelUrl: "",
    checkFrequency: "daily",
    weeklyDay: "sunday",
  });

  const [osUpdateInfo, setOsUpdateInfo] = useState<UpdateInfo>({
    currentVersion: "(unknown)",
    latestVersion: "(unknown)",
    hasUpdate: false,
    lastChecked: "Never",
    currentVersionRaw: "",
    latestVersionRaw: "",
    isDowngrade: false,
  });

  const [modelUpdateInfo, setModelUpdateInfo] = useState<UpdateInfo>({
    currentVersion: "(loading...)",
    latestVersion: "(loading...)",
    hasUpdate: false,
    lastChecked: "Never",
    currentVersionRaw: "",
    latestVersionRaw: "",
    isDowngrade: false,
  });
  const [modelDownloadProgress, setModelDownloadProgress] = useState<{ progress: number; status?: string }>({
    progress: 0,
    status: "idle",
  });
  const modelPollTimerRef = useRef<number | null>(null);

  const formatAge = (unixSeconds?: number) => {
    if (!unixSeconds) return "Never";
    const diffMs = Date.now() - unixSeconds * 1000;
    if (diffMs < 0) return "Just now";
    const mins = Math.floor(diffMs / 60000);
    if (mins < 1) return "Just now";
    if (mins < 60) return `${mins} min ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours} hour${hours === 1 ? "" : "s"} ago`;
    const days = Math.floor(hours / 24);
    return `${days} day${days === 1 ? "" : "s"} ago`;
  };

  const formatAgeFromISO = (isoString?: string) => {
    if (!isoString) return "Never";
    const date = new Date(isoString);
    if (isNaN(date.getTime())) return "Never";
    const unixSeconds = Math.floor(date.getTime() / 1000);
    return formatAge(unixSeconds);
  };

  const stopModelPolling = () => {
    if (modelPollTimerRef.current != null) {
      window.clearInterval(modelPollTimerRef.current);
      modelPollTimerRef.current = null;
    }
  };

  const startModelPolling = () => {
    if (modelPollTimerRef.current != null) return;
    modelPollTimerRef.current = window.setInterval(async () => {
      try {
        const status = await getModelUpdateStatusApi();
        setModelDownloadProgress({
          progress: status.data.download_progress,
          status: status.data.is_downloading ? "downloading" : "idle",
        });

        // Update model info
        setModelUpdateInfo({
          currentVersion: formatModelVersion(status.data.current_version),
          latestVersion: formatModelVersion(status.data.latest_version),
          hasUpdate: status.data.update_available,
          lastChecked: formatAgeFromISO(status.data.last_check),
          currentVersionRaw: status.data.current_version?.toString() || "",
          latestVersionRaw: status.data.latest_version?.toString() || "",
          isDowngrade: false,
        });

        // Stop polling if not downloading (either download completed or no download started)
        if (!status.data.is_downloading) {
          stopModelPolling();
          setModelDownloadProgress({ progress: 0, status: "idle" });
        }
      } catch {
        stopModelPolling();
      }
    }, 1000);
  };

  const formatModelVersion = (v: number | undefined | null): string => {
    if (v === undefined || v === null) return "(unknown)";
    if (v === 0) return "(not installed)";
    return `v${v}`;
  };

  const refreshModelUpdateInfo = async () => {
    try {
      const status = await getModelUpdateStatusApi();
      setModelUpdateInfo({
        currentVersion: formatModelVersion(status.data.current_version),
        latestVersion: formatModelVersion(status.data.latest_version),
        hasUpdate: status.data.update_available,
        lastChecked: status.data.last_error
          ? `Error: ${status.data.last_error}`
          : formatAgeFromISO(status.data.last_check),
        currentVersionRaw: status.data.current_version?.toString() || "",
        latestVersionRaw: status.data.latest_version?.toString() || "",
        isDowngrade: false,
      });

      if (status.data.is_downloading) {
        setModelDownloadProgress({
          progress: status.data.download_progress,
          status: "downloading",
        });
        startModelPolling();
      }
    } catch (err) {
      console.error("Failed to refresh model update info:", err);
    }
  };

  const stopOSCheckProgressPolling = () => {
    if (osCheckTimerRef.current != null) {
      window.clearInterval(osCheckTimerRef.current);
      osCheckTimerRef.current = null;
    }
  };

  const startOSCheckProgressPolling = () => {
    if (osCheckTimerRef.current != null) return;
    // Fetch immediately so the UI shows feedback even before the first tick.
    (async () => {
      try {
        const p = await getUpdateCheckProgressApi();
        setOsCheckProgress({ progress: p.data.progress, status: p.data.status });
      } catch {
        // Ignore
      }
    })();
    osCheckTimerRef.current = window.setInterval(async () => {
      try {
        const p = await getUpdateCheckProgressApi();
        setOsCheckProgress({ progress: p.data.progress, status: p.data.status });
        const st = (p.data.status || "").toLowerCase();
        if ((p.data.progress ?? 0) >= 100 || st.startsWith("failed") || st.includes("done")) {
          stopOSCheckProgressPolling();
        }
      } catch {
        stopOSCheckProgressPolling();
      }
    }, 500);
  };

  const refreshOSUpdateInfo = async (forceCheck: boolean, effectiveConfig?: UpdateConfig) => {
    const cfg = effectiveConfig ?? config;

    // Ensure self-hosted URL is set if selected.
    if (cfg.osSource === "self_hosted" && !cfg.selfHostedOSUrl.trim()) {
      messageApi.error("Please enter a self-hosted server URL");
      return;
    }

    setCheckingOS(true);
    if (forceCheck) {
      startOSCheckProgressPolling();
    }
    try {
      const dev = await queryDeviceInfoApi();
      const currentOSName = dev.data.osName || "";
      const currentOSVersion = dev.data.osVersion || "(unknown)";

      // Ask supervisor for latest version info. This may return "Checking" initially.
      let latest = await getSystemUpdateVersionApi(forceCheck);
      if (forceCheck && latest.data.status === SystemUpdateStatus.Checking) {
        // Poll a few times while the backend fetches version.json.
        for (let i = 0; i < 8; i++) {
          await new Promise((r) => setTimeout(r, 1500));
          // Subsequent polls should NOT re-force; just read the latest status.
          latest = await getSystemUpdateVersionApi(false);
          if (latest.data.status !== SystemUpdateStatus.Checking) break;
        }
      }

      const latestOSName = latest.data.osName || currentOSName;
      const latestOSVersion = latest.data.osVersion || currentOSVersion;
      const checkError = latest.data.error;

      const cmp = compareVersions(latestOSVersion, currentOSVersion);
      const isDowngrade = !checkError && latest.data.status === SystemUpdateStatus.Normal && cmp < 0;
      const hasUpdate =
        !checkError &&
        latest.data.status === SystemUpdateStatus.Normal &&
        (!!latest.data.osVersion && latestOSVersion !== currentOSVersion);

      setOsUpdateInfo({
        currentVersion: currentOSName ? `${currentOSName} ${currentOSVersion}` : currentOSVersion,
        latestVersion: latestOSName ? `${latestOSName} ${latestOSVersion}` : latestOSVersion,
        hasUpdate,
        currentVersionRaw: currentOSVersion,
        latestVersionRaw: latestOSVersion,
        isDowngrade,
        lastChecked:
          latest.data.status === SystemUpdateStatus.Checking
            ? "Checking…"
            : checkError
              ? `Failed: ${checkError}`
              : formatAge(latest.data.checkedAt),
      });
    } catch (err) {
      console.error("Failed to refresh OS update info:", err);
      messageApi.error("Failed to check OS updates");
    } finally {
      setCheckingOS(false);
      stopOSCheckProgressPolling();
    }
  };

  // Load configuration on mount
  useEffect(() => {
    const loadConfig = async () => {
      setLoading(true);
      try {
        const response = await getUpdateConfigApi();
        // Hide legacy/testing 1-minute option from UI; if encountered, coerce to a supported value.
        const coercedFrequency: UpdateFrequency =
          response.data.check_frequency === "1min" ? "30min" : (response.data.check_frequency as UpdateFrequency);
        const nextConfig: UpdateConfig = {
          osSource: response.data.os_source,
          modelSource: response.data.model_source,
          selfHostedOSUrl: response.data.self_hosted_os_url,
          selfHostedModelUrl: response.data.self_hosted_model_url,
          checkFrequency: coercedFrequency,
          weeklyDay: response.data.weekly_day,
        };
        setConfig(nextConfig);

        // Load real OS version + latest version cache.
        // (The backend may still be querying; the UI will show "Checking…".)
        await refreshOSUpdateInfo(false, nextConfig);

        // Load model update status
        await refreshModelUpdateInfo();

        try {
          const staged = await getUploadedUpdatePackageApi();
          setStagedPackage(staged.data);
        } catch {
          // Non-fatal
        }

        try {
          const prog = await getUpdateSystemProgressApi();
          setUpdateProgress({ progress: prog.data.progress, status: prog.data.status });
          if ((prog.data.status && prog.data.status !== "idle") || prog.data.progress > 0) {
            startProgressPolling();
          }
        } catch {
          // Non-fatal
        }
      } catch (error) {
        console.error("Failed to load update configuration:", error);
        messageApi.error("Failed to load update configuration");
      } finally {
        setLoading(false);
      }
    };
    loadConfig();

    return () => {
      stopProgressPolling();
      stopOSCheckProgressPolling();
      stopModelPolling();
    };
  }, [messageApi]);

  const stopProgressPolling = () => {
    if (progressTimerRef.current != null) {
      window.clearInterval(progressTimerRef.current);
      progressTimerRef.current = null;
    }
    progressPollStartedAtRef.current = null;
    progressSeenActiveRef.current = false;
    progressIdleCountRef.current = 0;
  };

  const startProgressPolling = () => {
    if (progressTimerRef.current != null) return;

    // Reset per-run polling state.
    progressPollStartedAtRef.current = Date.now();
    progressSeenActiveRef.current = false;
    progressIdleCountRef.current = 0;

    // Fetch immediately so we don't miss the early phase (e.g. 0–20%) before the first 1s tick.
    (async () => {
      try {
        const prog = await getUpdateSystemProgressApi();
        setUpdateProgress({ progress: prog.data.progress, status: prog.data.status });
      } catch {
        // Ignore; interval will handle failures.
      }
    })();
    progressTimerRef.current = window.setInterval(async () => {
      try {
        const prog = await getUpdateSystemProgressApi();
        setUpdateProgress({ progress: prog.data.progress, status: prog.data.status });

        const p = prog.data.progress ?? 0;
        const stRaw = prog.data.status || "idle";
        const st = stRaw.toLowerCase();

        // Mark that we've seen an actual running state.
        if (st !== "idle" && st !== "cancelled") {
          progressSeenActiveRef.current = true;
          progressIdleCountRef.current = 0;
        }

        // Terminal conditions.
        if (p >= 100 || st === "cancelled" || st.startsWith("failed")) {
          stopProgressPolling();
          return;
        }

        // The backend may briefly report "idle" right after an update is triggered
        // (before the progress file is written). Don’t stop polling immediately.
        if (st === "idle") {
          progressIdleCountRef.current += 1;
          const startedAt = progressPollStartedAtRef.current ?? Date.now();
          const ageMs = Date.now() - startedAt;

          // If we've never seen an active status, keep polling for a grace window.
          if (!progressSeenActiveRef.current) {
            if (ageMs > 15000) {
              stopProgressPolling();
            }
            return;
          }

          // If we *have* seen activity, stop after a few consecutive idle polls.
          if (progressIdleCountRef.current >= 3) {
            stopProgressPolling();
          }
        }
      } catch {
        // If polling fails, stop to avoid spamming.
        stopProgressPolling();
      }
    }, 1000);
  };

  // Save configuration helper
  const saveConfig = async (newConfig: UpdateConfig) => {
    try {
      const apiConfig: APIUpdateConfig = {
        os_source: newConfig.osSource,
        model_source: newConfig.modelSource,
        self_hosted_os_url: newConfig.selfHostedOSUrl,
        self_hosted_model_url: newConfig.selfHostedModelUrl,
        check_frequency: newConfig.checkFrequency,
        weekly_day: newConfig.weeklyDay,
      };
      await setUpdateConfigApi(apiConfig);
    } catch (error) {
      console.error("Failed to save update configuration:", error);
      throw error;
    }
  };

  const [checkingOS, setCheckingOS] = useState(false);
  const [checkingModel, setCheckingModel] = useState(false);

  const handleOSSourceChange = async (e: RadioChangeEvent) => {
    const newConfig = {
      ...config,
      osSource: e.target.value as UpdateSource,
    };
    setConfig(newConfig);
    try {
      await saveConfig(newConfig);
      messageApi.success("Update source saved");
    } catch (error) {
      messageApi.error("Failed to save update source");
    }
  };

  const handleModelSourceChange = async (e: RadioChangeEvent) => {
    const newConfig = {
      ...config,
      modelSource: e.target.value as UpdateSource,
    };
    setConfig(newConfig);
    try {
      await saveConfig(newConfig);
      messageApi.success("Update source saved");
    } catch (error) {
      messageApi.error("Failed to save update source");
    }
  };

  const handleSelfHostedOSUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setConfig((prev) => ({
      ...prev,
      selfHostedOSUrl: e.target.value,
    }));
  };

  const handleSelfHostedModelUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setConfig((prev) => ({
      ...prev,
      selfHostedModelUrl: e.target.value,
    }));
  };

  const handleFrequencyChange = async (value: UpdateFrequency) => {
    const newConfig = {
      ...config,
      checkFrequency: value,
    };
    setConfig(newConfig);
    try {
      await saveConfig(newConfig);
      messageApi.success("Update frequency saved");
    } catch (error) {
      messageApi.error("Failed to save update frequency");
    }
  };

  const handleWeeklyDayChange = async (value: DayOfWeek) => {
    const newConfig = {
      ...config,
      weeklyDay: value,
    };
    setConfig(newConfig);
    try {
      await saveConfig(newConfig);
      messageApi.success("Weekly day saved");
    } catch (error) {
      messageApi.error("Failed to save weekly day");
    }
  };

  const handleCheckOSUpdates = async () => {
    await refreshOSUpdateInfo(true);
  };

  const handleCheckModelUpdates = async () => {
    if (config.modelSource === "self_hosted" && !config.selfHostedModelUrl.trim()) {
      messageApi.error("Please enter a self-hosted server URL");
      return;
    }
    setCheckingModel(true);
    try {
      await checkModelUpdatesApi();
      messageApi.info("Checking for model updates...");

      // Poll for status updates while checking
      startModelPolling();

      // Wait and poll for the check to complete (up to 30 seconds)
      let latestStatus = await getModelUpdateStatusApi();
      for (let i = 0; i < 10; i++) {
        await new Promise((resolve) => setTimeout(resolve, 3000));
        latestStatus = await getModelUpdateStatusApi();

        // If we got a recent check (within 10 seconds), we're done
        if (latestStatus.data.last_check) {
          const checkTime = new Date(latestStatus.data.last_check).getTime();
          const now = Date.now();
          if (now - checkTime < 10000) break;
        }
      }

      stopModelPolling();
      await refreshModelUpdateInfo();

      // Check the latest status for proper feedback
      if (latestStatus.data.last_error) {
        messageApi.warning(`Check completed with error: ${latestStatus.data.last_error}`);
      } else if (latestStatus.data.update_available) {
        messageApi.success("New model update available!");
      } else if (latestStatus.data.current_version > 0) {
        messageApi.success("Model is up to date");
      } else {
        messageApi.info("Check completed. Camera may need to be registered first.");
      }
    } catch (err) {
      console.error("Failed to check model updates:", err);
      messageApi.error("Failed to check for model updates");
      stopModelPolling();
    } finally {
      setCheckingModel(false);
    }
  };

  const handleInstallUpdate = async (type: "os" | "model") => {
    if (uploadingPackage || applyingStagedPackage || isSystemUpdateRunning) {
      messageApi.warning("An update is already in progress. Please wait for it to finish before starting another update.");
      return;
    }

    const startInstall = async () => {
      await updateSystemApi();
      messageApi.info(`Installing ${type === "os" ? "Camera OS" : "Camera Model"} update...`);

      // Ensure the UI immediately shows status/progress (self-hosted installs previously looked idle).
      setUpdateProgress({ progress: 0, status: "download" });
      startProgressPolling();
    };

    try {
      if (type === "os" && osUpdateInfo.isDowngrade && osUpdateInfo.currentVersionRaw && osUpdateInfo.latestVersionRaw) {
        Modal.confirm({
          title: "Downgrade Camera OS?",
          content: `You are about to downgrade from ${osUpdateInfo.currentVersionRaw} to ${osUpdateInfo.latestVersionRaw}. This is not recommended unless you know you need an older version.`,
          okText: "Downgrade",
          okButtonProps: { danger: true },
          cancelText: "Cancel",
          centered: true,
          onOk: startInstall,
        });
        return;
      }

      await startInstall();
    } catch (error) {
      console.error("Failed to start update:", error);
      messageApi.error("Failed to start update");
    }
  };

  const handleSelectPackageFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null;
    setSelectedPackage(file);
  };

  const handleUploadPackage = async () => {
    if (!selectedPackage) {
      messageApi.error("Please choose an update package file");
      return;
    }

    const name = selectedPackage.name;
    const isOTA = name.endsWith("ota.zip");
    const isSWU = name.endsWith(".swu") || name.endsWith("_swu.zip");
    const isUpgradeZip = name === "upgrade.zip" || name.endsWith("_emmc.zip");
    if (!(isOTA || isSWU || isUpgradeZip)) {
      messageApi.error("Invalid file: expected *_ota.zip, *_swu.zip, *.swu, *_emmc.zip, or upgrade.zip");
      return;
    }

    setUploadingPackage(true);
    setUploadProgressPct(0);
    try {
      const form = new FormData();
      form.append("file", selectedPackage);
      const res = await uploadUpdatePackageApi(form, (pct) => setUploadProgressPct(pct));
      messageApi.success("Update package uploaded and staged");

      // Refresh staged info from server response
      setStagedPackage({
        exists: true,
        fileName: res.data.fileName,
        checksum: res.data.checksum,
        osName: res.data.osName,
        version: res.data.version,
        size: res.data.size,
        packageType: res.data.packageType,
      });
      setSelectedPackage(null);
    } catch (error) {
      console.error("Failed to upload update package:", error);
      messageApi.error("Failed to upload update package");
    } finally {
      setUploadingPackage(false);
      // Leave the final pct visible briefly when successful.
      window.setTimeout(() => setUploadProgressPct(0), 1500);
    }
  };

  const handleApplyStagedPackage = async () => {
    if (!stagedPackage?.exists) {
      messageApi.error("No staged update package found");
      return;
    }

    // Avoid starting a staged install while an OTA upload or any other update install is running.
    if (uploadingPackage || applyingStagedPackage || isSystemUpdateRunning) {
      messageApi.warning("An update is already in progress. Please wait for it to finish before installing the staged package.");
      return;
    }

    Modal.confirm({
      title: "Install staged update package?",
      content: `This will install the uploaded update package (${formatPackageType(stagedPackage.packageType)}). The device will need to reboot after the update completes.`,
      okText: "Install",
      cancelText: "Cancel",
      centered: true,
      onOk: async () => {
        setApplyingStagedPackage(true);
        try {
          await applyUploadedUpdatePackageApi();
          messageApi.info("Installing uploaded update package...");
          startProgressPolling();
        } catch (error) {
          console.error("Failed to start staged update:", error);
          messageApi.error("Failed to start staged update");
        } finally {
          setApplyingStagedPackage(false);
        }
      },
    });
  };

  const handleCancelUpdate = async () => {
    try {
      await cancelUpdateApi();
      messageApi.info("Update cancelled");
      setUpdateProgress({ progress: 0, status: "cancelled" });
      stopProgressPolling();
    } catch (error) {
      console.error("Failed to cancel update:", error);
      messageApi.error("Failed to cancel update");
    }
  };

  // Render update card
  const renderUpdateCard = (
    title: string,
    info: UpdateInfo,
    source: UpdateSource,
    selfHostedUrl: string,
    onSourceChange: (e: RadioChangeEvent) => void,
    onUrlChange: (e: React.ChangeEvent<HTMLInputElement>) => void,
    checking: boolean,
    onCheck: () => void,
    onInstall: () => void,
    checkProgress?: { progress: number; status?: string },
    installProgress?: { progress: number; status?: string },
    onCancelInstall?: () => void
  ) => (
    <div className="p-20" style={translucentCardStyle}>
      <div className="flex justify-between items-start mb-16">
        <div>
          <h3 className="text-18 font-semibold text-platinum m-0">{title}</h3>
          <div className="text-12 text-platinum/50 mt-4">
            Last checked: {info.lastChecked}
          </div>
        </div>
        {info.hasUpdate ? (
          <div className="px-12 py-4 rounded-full bg-cta/20 text-cta text-12 font-medium">
            {info.isDowngrade ? "Downgrade Available" : "Update Available"}
          </div>
        ) : info.currentVersion === "(not installed)" && info.latestVersion !== "(unknown)" ? (
          <div className="px-12 py-4 rounded-full bg-cta/20 text-cta text-12 font-medium">
            Awaiting download
          </div>
        ) : (
          <div className="flex items-center gap-4 text-12 text-platinum/60">
            <CheckCircleOutlined style={{ color: '#9be564' }} />
            Up to date
          </div>
        )}
      </div>

      {info.isDowngrade && (
        <div className="mb-16 p-12 rounded-lg" style={{ backgroundColor: 'rgba(255, 77, 79, 0.12)', border: '1px solid rgba(255, 77, 79, 0.35)' }}>
          <div className="text-12" style={{ color: 'rgba(255, 77, 79, 0.95)' }}>
            Warning: installing this will downgrade your Camera OS.
          </div>
        </div>
      )}

      {/* Version Info */}
      <div className="mb-16 p-12 rounded-lg" style={{ backgroundColor: 'rgba(255, 255, 255, 0.05)' }}>
        <div className="flex justify-between items-center mb-8">
          <span className="text-14 text-platinum/70">Current Version</span>
          <span className="text-14 text-platinum font-mono">{info.currentVersion}</span>
        </div>
        <div className="flex justify-between items-center">
          <span className="text-14 text-platinum/70">Latest Version</span>
          <span className={`text-14 font-mono ${info.hasUpdate ? 'text-cta' : 'text-platinum'}`}>
            {info.latestVersion}
          </span>
        </div>
      </div>

      {/* Source Selection */}
      <div className="mb-16">
        <div className="text-14 font-medium text-platinum mb-8">Update Source</div>
        <Radio.Group
          value={source}
          onChange={onSourceChange}
          optionType="button"
          buttonStyle="solid"
          className="w-full"
        >
          <Radio.Button value="tpr_official" className="w-1/2 text-center">
            TPR Official
          </Radio.Button>
          <Radio.Button value="self_hosted" className="w-1/2 text-center">
            Self Hosted
          </Radio.Button>
        </Radio.Group>
        <div className="text-11 text-platinum/40 mt-8">
          {source === "tpr_official" 
            ? "Official updates from The Police Record servers. Recommended for most users."
            : "Use your own update server for custom deployments."}
        </div>
      </div>

      {/* Self-Hosted URL Input */}
      {source === "self_hosted" && (
        <div className="mb-16">
          <div className="text-14 font-medium text-platinum mb-8">Server URL</div>
          <Input
            prefix={<LinkOutlined style={{ color: 'rgba(255, 255, 255, 0.5)' }} />}
            placeholder="https://your-update-server.com/api"
            value={selfHostedUrl}
            onChange={onUrlChange}
            style={{
              backgroundColor: 'rgba(255, 255, 255, 0.1)',
              borderColor: 'rgba(255, 255, 255, 0.2)',
              color: '#e0e0e0',
            }}
          />
          <div className="text-11 text-platinum/40 mt-4">
            Enter the full URL to your self-hosted update server endpoint
          </div>
        </div>
      )}

      {/* Action Buttons */}
      <div className="flex gap-12 flex-wrap">
        <Button
          icon={checking ? <Spin size="small" /> : <SyncOutlined />}
          onClick={onCheck}
          disabled={checking}
          className="flex-1"
        >
          {checking ? "Checking..." : "Check for Updates"}
        </Button>
        {info.hasUpdate && (
          <Button
            type="primary"
            icon={<CloudDownloadOutlined />}
            onClick={onInstall}
            className="flex-1"
            disabled={isSystemUpdateRunning}
          >
            Install Update
          </Button>
        )}
        {onCancelInstall && (
          <Button
            icon={<CloseCircleOutlined />}
            onClick={onCancelInstall}
            disabled={(installProgress?.status || "idle") === "idle" || (installProgress?.status || "") === "cancelled"}
            className="flex-1"
          >
            Cancel Update
          </Button>
        )}
      </div>

      {/* Install progress (OS updates) */}
      {installProgress?.status && installProgress.status !== "idle" && (
        <div className="mt-12">
          <div className="text-12 text-platinum/60 mb-8">
            Status: <span className="font-mono">{installProgress.status}</span>
          </div>
          <Progress percent={Math.min(100, Math.max(0, installProgress.progress || 0))} />
        </div>
      )}

      {/* Checking progress */}
      {checking && (
        <div className="mt-12">
          <div className="text-12 text-platinum/60 mb-8">
            Status: <span className="font-mono">{checkProgress?.status || "checking"}</span>
          </div>
          <Progress percent={Math.min(100, Math.max(0, checkProgress?.progress ?? 0))} size="small" />
        </div>
      )}
    </div>
  );

  return (
    <div className="p-16">
      {contextHolder}
      <Spin spinning={loading} tip="Loading…">
        {/* Page Header */}
        <div className="mb-24">
          <div className="flex items-center gap-12 mb-8">
            <CloudDownloadOutlined style={{ fontSize: 28, color: '#9be564' }} />
            <h1 className="text-28 font-bold text-platinum m-0">Updates</h1>
          </div>
          <p className="text-14 text-platinum/60 mt-8">
            Keep your camera software and models up to date
          </p>
        </div>

        {/* Camera OS Updates */}
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Camera OS Updates
          </div>
          {renderUpdateCard(
            "Camera OS",
            osUpdateInfo,
            config.osSource,
            config.selfHostedOSUrl,
            handleOSSourceChange,
            handleSelfHostedOSUrlChange,
            checkingOS,
            handleCheckOSUpdates,
            () => handleInstallUpdate("os"),
            osCheckProgress,
            updateProgress,
            handleCancelUpdate
          )}
        </div>

        {/* Custom Update Package */}
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Custom Update Package
          </div>
          <div className="p-20" style={translucentCardStyle}>
            <div className="text-14 font-medium text-platinum mb-8">Upload an update package</div>
            <div className="text-12 text-platinum/60 mb-12">
              Supported package types:
              <ul className="mt-6 mb-0 pl-18">
                <li>
                  <span className="font-mono">*_ota.zip</span> (A/B OTA) — recommended for normal upgrades. Keeps the inactive slot as a rollback path.
                </li>
                <li>
                  <span className="font-mono">*.swu</span> or <span className="font-mono">*_swu.zip</span> (SWUpdate) — use if you specifically build and deploy SWUpdate bundles.
                </li>
                <li>
                  <span className="font-mono">*_emmc.zip</span> or <span className="font-mono">upgrade.zip</span> (upgrade zip) — applied here as a rootfs-only A/B slot update. Higher risk than OTA/SWU; use only for troubleshooting.
                </li>
              </ul>
            </div>

            <div className="flex gap-12 items-center flex-wrap">
              <input
                type="file"
                accept=".zip,.swu"
                onChange={handleSelectPackageFile}
                style={{ color: "rgba(255,255,255,0.7)" }}
              />
              <Button
                icon={<UploadOutlined />}
                onClick={handleUploadPackage}
                loading={uploadingPackage}
                disabled={!selectedPackage || uploadingPackage}
              >
                Upload Package
              </Button>
            </div>

            {/* Upload progress */}
            {(uploadingPackage || uploadProgressPct > 0) && (
              <div className="mt-12">
                <div className="text-12 text-platinum/60 mb-8">
                  Uploading: <span className="font-mono">{uploadProgressPct}%</span>
                </div>
                <Progress percent={Math.min(100, Math.max(0, uploadProgressPct || 0))} />
              </div>
            )}

            {/* Staged info */}
            <div className="mt-16 p-12 rounded-lg" style={{ backgroundColor: 'rgba(255, 255, 255, 0.05)' }}>
              <div className="text-14 font-medium text-platinum mb-8">Staged Package</div>
              {stagedPackage?.exists ? (
                <div className="space-y-6 text-12 text-platinum/70">
                  <div className="flex justify-between gap-12">
                    <span>Type</span>
                    <span className="font-mono">{formatPackageType(stagedPackage.packageType)}</span>
                  </div>
                  <div className="flex justify-between gap-12">
                    <span>File</span>
                    <span className="font-mono truncate">{stagedPackage.fileName}</span>
                  </div>
                  <div className="flex justify-between gap-12">
                    <span>OS</span>
                    <span className="font-mono">{stagedPackage.osName || "(unknown)"}</span>
                  </div>
                  <div className="flex justify-between gap-12">
                    <span>Version</span>
                    <span className="font-mono">{stagedPackage.version || "(unknown)"}</span>
                  </div>
                  <div className="flex justify-between gap-12">
                    <span>SHA256</span>
                    <span className="font-mono truncate">{stagedPackage.checksum}</span>
                  </div>
                </div>
              ) : (
                <div className="text-12 text-platinum/50">No staged update package</div>
              )}
            </div>

            {/* Install staged package */}
            <div className="mt-16">
              <Button
                type="primary"
                onClick={handleApplyStagedPackage}
                loading={applyingStagedPackage}
                disabled={!stagedPackage?.exists || uploadingPackage || isSystemUpdateRunning}
              >
                Install Staged Package
              </Button>
            </div>
          </div>
        </div>

        {/* Camera Model Updates */}
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Camera Model Updates
          </div>
          {renderUpdateCard(
            "Camera Model",
            modelUpdateInfo,
            config.modelSource,
            config.selfHostedModelUrl,
            handleModelSourceChange,
            handleSelfHostedModelUrlChange,
            checkingModel,
            handleCheckModelUpdates,
            () => handleInstallUpdate("model"),
            undefined, // checkProgress - model updates auto-download
            modelDownloadProgress.status !== "idle" ? modelDownloadProgress : undefined
          )}
        </div>

        {/* Update Check Frequency */}
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Update Check Frequency
          </div>
          <div className="p-20" style={translucentCardStyle}>
            <div className="mb-16">
              <div className="text-14 font-medium text-platinum mb-8">Check for updates</div>
              <Select
                value={config.checkFrequency}
                onChange={handleFrequencyChange}
                options={frequencyOptions}
                className="w-full"
                style={{ backgroundColor: 'rgba(255, 255, 255, 0.1)' }}
              />
            </div>

            {config.checkFrequency === "weekly" && (
              <div>
                <div className="text-14 font-medium text-platinum mb-8">Day of week</div>
                <Select
                  value={config.weeklyDay}
                  onChange={handleWeeklyDayChange}
                  options={dayOptions}
                  className="w-full"
                  style={{ backgroundColor: 'rgba(255, 255, 255, 0.1)' }}
                />
              </div>
            )}

            <div className="mt-12 text-12 text-platinum/50">
              {config.checkFrequency === "manual" 
                ? "Updates will only be checked when you manually click 'Check for Updates'"
                : `Updates will be automatically checked ${
                    config.checkFrequency === "30min" ? "every 30 minutes" :
                    config.checkFrequency === "daily" ? "once per day" :
                    `every ${dayOptions.find(d => d.value === config.weeklyDay)?.label}`
                  }`
              }
            </div>
          </div>
        </div>
      </Spin>
    </div>
  );
}

export default Updates;

import { Modal } from "antd";
import { useEffect, useMemo, useState } from "react";
import {
  InfoCircleOutlined,
  FileTextOutlined,
  SafetyOutlined,
  HeartOutlined,
} from "@ant-design/icons";
import useConfigStore from "@/store/config";
import { queryDeviceInfoApi, getModelInfoApi } from "@/api/device/index";

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

// Modal styles
const modalContentStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.95)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
};

const modalStyles = {
  content: modalContentStyle,
  header: {
    backgroundColor: 'transparent',
    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
  },
  body: {
    backgroundColor: 'transparent',
  },
  footer: {
    backgroundColor: 'transparent',
    borderTop: '1px solid rgba(255, 255, 255, 0.1)',
  },
};

// Sample open source licenses
const openSourceLicenses = [
  { name: "React", version: "18.2.0", license: "MIT" },
  { name: "Ant Design", version: "5.x", license: "MIT" },
  { name: "Tailwind CSS", version: "3.x", license: "MIT" },
  { name: "TypeScript", version: "5.x", license: "Apache-2.0" },
  { name: "Vite", version: "5.x", license: "MIT" },
  { name: "Day.js", version: "1.x", license: "MIT" },
  { name: "React Router", version: "6.x", license: "MIT" },
  { name: "Zustand", version: "4.x", license: "MIT" },
];

function About() {
  const [licensesModalVisible, setLicensesModalVisible] = useState(false);
  const [privacyModalVisible, setPrivacyModalVisible] = useState(false);

  const { deviceInfo, updateDeviceInfo } = useConfigStore();
  const [modelVersion, setModelVersion] = useState<string>("");

  useEffect(() => {
    // Refresh device info when opening About so the page reflects the current system state.
    const fetchDeviceInfo = async () => {
      try {
        const res = await queryDeviceInfoApi();
        updateDeviceInfo(res.data);
      } catch {
        // Non-fatal; About can still render with cached store data.
      }
    };
    fetchDeviceInfo();
  }, [updateDeviceInfo]);

  useEffect(() => {
    // Fetch model metadata (model.json). This endpoint used to return raw JSON; backend now returns the standard envelope.
    (async () => {
      try {
        const res = await getModelInfoApi();
        const info: unknown = res?.data;

        const asString = (v: unknown): string | undefined => {
          if (typeof v === "string") return v;
          if (typeof v === "number" || typeof v === "boolean") return String(v);
          return undefined;
        };

        const pick = (obj: unknown, keys: string[]): string | undefined => {
          if (!obj || typeof obj !== "object") return undefined;
          const rec = obj as Record<string, unknown>;
          for (const k of keys) {
            const v = asString(rec[k]);
            if (v) return v;
          }
          return undefined;
        };

        const rawFallback = (() => {
          if (!info || typeof info !== "object") return "";
          const raw = (info as Record<string, unknown>).raw;
          return typeof raw === "string" ? raw : "";
        })();

        // Try common fields first; if model.json uses a different schema, fall back to showing a stable identifier.
        const version =
          pick(info, [
            "version",
            "modelVersion",
            "model_version",
            "ver",
            "sha",
            "git_sha",
            "checksum",
          ]) || rawFallback;

        setModelVersion((version || "").trim());
      } catch {
        setModelVersion("");
      }
    })();
  }, []);

  const softwareVersion = useMemo(() => {
    const osName = (deviceInfo?.osName || "").toString().trim();
    const osVersion = (deviceInfo?.osVersion || "").toString().trim();
    if (osName && osVersion) return `${osName} ${osVersion}`;
    return osVersion || osName || "—";
  }, [deviceInfo?.osName, deviceInfo?.osVersion]);

  const hardwareModel = useMemo(() => {
    return (
      (deviceInfo?.type || "").toString().trim() ||
      (deviceInfo?.cpu || "").toString().trim() ||
      "—"
    );
  }, [deviceInfo?.type, deviceInfo?.cpu]);

  const serialNumber = useMemo(() => {
    return (deviceInfo?.sn || "").toString().trim() || "—";
  }, [deviceInfo?.sn]);

  const modelVersionDisplay = useMemo(() => {
    return modelVersion || "—";
  }, [modelVersion]);

  return (
    <div className="p-16">
      {/* Page Header */}
      <div className="mb-24">
        <div className="flex items-center gap-12 mb-8">
          <InfoCircleOutlined style={{ fontSize: 28, color: "#9be564" }} />
          <h1 className="text-28 font-bold text-platinum m-0">About</h1>
        </div>
        <p className="text-14 text-platinum/60 mt-8">
          Information about your camera and software
        </p>
      </div>

      {/* Device Information */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
          Device Information
        </div>
        <div className="p-20" style={translucentCardStyle}>
          <div className="flex justify-between py-8 border-b border-white/10">
            <span className="text-14 text-platinum/70">Software Version</span>
            <span className="text-14 text-platinum font-mono">{softwareVersion}</span>
          </div>
          <div className="flex justify-between py-8 border-b border-white/10">
            <span className="text-14 text-platinum/70">Model Version</span>
            <span className="text-14 text-platinum font-mono">{modelVersionDisplay}</span>
          </div>
          <div className="flex justify-between py-8 border-b border-white/10">
            <span className="text-14 text-platinum/70">Hardware Model</span>
            <span className="text-14 text-platinum">{hardwareModel}</span>
          </div>
          <div className="flex justify-between py-8">
            <span className="text-14 text-platinum/70">Serial Number</span>
            <span className="text-14 text-platinum font-mono">{serialNumber}</span>
          </div>
        </div>
      </div>

      {/* Links Section */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
          Legal & Information
        </div>
        <div className="p-20" style={translucentCardStyle}>
          {/* Open Source Licenses */}
          <div
            className="flex justify-between items-center py-16 cursor-pointer hover:bg-white/5 -mx-12 px-12 rounded-lg transition-colors"
            onClick={() => setLicensesModalVisible(true)}
          >
            <div className="flex items-center">
              <div className="w-40 h-40 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                <FileTextOutlined style={{ fontSize: 18, color: '#9be564' }} />
              </div>
              <div>
                <div className="text-16 font-medium text-platinum">Open Source Licenses</div>
                <div className="text-12 text-platinum/50 mt-2">
                  View third-party software licenses
                </div>
              </div>
            </div>
            <span className="text-platinum/40">→</span>
          </div>

          <div className="border-t border-white/10" />

          {/* Data Collection Policy */}
          <div
            className="flex justify-between items-center py-16 cursor-pointer hover:bg-white/5 -mx-12 px-12 rounded-lg transition-colors"
            onClick={() => setPrivacyModalVisible(true)}
          >
            <div className="flex items-center">
              <div className="w-40 h-40 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                <SafetyOutlined style={{ fontSize: 18, color: '#9be564' }} />
              </div>
              <div>
                <div className="text-16 font-medium text-platinum">Data Collection Policy</div>
                <div className="text-12 text-platinum/50 mt-2">
                  Learn how we handle your data
                </div>
              </div>
            </div>
            <span className="text-platinum/40">→</span>
          </div>
        </div>
      </div>

      {/* Footer Message - Centered container */}
      <div className="mt-32 flex justify-center">
        <div className="w-full max-w-lg mx-auto text-center">
          <div className="p-32 rounded-xl" style={{ backgroundColor: 'rgba(35, 40, 187, 0.2)' }}>
            <div className="flex justify-center items-center mb-20">
              <HeartOutlined style={{ fontSize: 40, color: '#9be564' }} />
            </div>
            <h3 className="text-20 text-white font-bold mb-12 text-center leading-relaxed">
              Enhancing existing hardware through better software.
            </h3>
            <p className="text-16 text-platinum/80 mb-20 text-center leading-relaxed">
              Designed with the user in mind, prioritizing security, clarity, and reliable operation.
            </p>
            <div className="mt-20 pt-20 border-t border-white/20">
              <p className="text-18 text-white font-bold mb-6 text-center">
                Authority Alert
              </p>
              <p className="text-14 text-platinum/70 text-center">
                Powered by The Police Record
              </p>
              <p className="text-13 text-platinum/50 mt-12 text-center">
                © 2024 The Police Record. All rights reserved.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Open Source Licenses Modal */}
      <Modal
        title={<span className="text-platinum">Open Source Licenses</span>}
        open={licensesModalVisible}
        onCancel={() => setLicensesModalVisible(false)}
        footer={null}
        centered
        width={600}
        styles={modalStyles}
      >
        <div className="max-h-400 overflow-y-auto">
          <p className="text-14 text-platinum/70 mb-16">
            This software includes the following open source components:
          </p>
          {openSourceLicenses.map((lib, index) => (
            <div
              key={lib.name}
              className={`flex justify-between items-center py-12 ${
                index > 0 ? "border-t border-white/10" : ""
              }`}
            >
              <div>
                <div className="text-15 font-medium text-platinum">{lib.name}</div>
                <div className="text-12 text-platinum/50">Version {lib.version}</div>
              </div>
              <div className="px-12 py-4 rounded-full text-12" style={{ backgroundColor: 'rgba(155, 229, 100, 0.2)', color: '#9be564' }}>
                {lib.license}
              </div>
            </div>
          ))}
          <div className="mt-16 pt-16 border-t border-white/10">
            <p className="text-12 text-platinum/50">
              Full license texts are available in the software distribution package.
            </p>
          </div>
        </div>
      </Modal>

      {/* Data Collection Policy Modal */}
      <Modal
        title={<span className="text-platinum">Data Collection Policy</span>}
        open={privacyModalVisible}
        onCancel={() => setPrivacyModalVisible(false)}
        footer={null}
        centered
        width={600}
        styles={modalStyles}
      >
        <div className="max-h-400 overflow-y-auto">
          <div className="space-y-16">
            <div>
              <h4 className="text-16 font-semibold text-platinum mb-8">Your Privacy Matters</h4>
              <p className="text-14 text-platinum/70">
                We are committed to protecting your privacy and ensuring the security of your data.
              </p>
            </div>

            <div>
              <h4 className="text-16 font-semibold text-platinum mb-8">What We Collect</h4>
              <ul className="text-14 text-platinum/70 list-disc pl-20 space-y-4">
                <li>Device configuration settings (stored locally)</li>
                <li>Network connection information (for connectivity only)</li>
                <li>Error logs (optional, for troubleshooting)</li>
              </ul>
            </div>

            <div>
              <h4 className="text-16 font-semibold text-platinum mb-8">What We Don't Collect</h4>
              <ul className="text-14 text-platinum/70 list-disc pl-20 space-y-4">
                <li>Video or image content from your camera</li>
                <li>Personal identification information</li>
                <li>Location data beyond network configuration</li>
                <li>Usage analytics or tracking data</li>
              </ul>
            </div>

            <div>
              <h4 className="text-16 font-semibold text-platinum mb-8">Data Storage</h4>
              <p className="text-14 text-platinum/70">
                All recordings and images are stored locally on your device or SD card. 
                No data is transmitted to external servers without your explicit consent.
              </p>
            </div>

            <div className="p-16 rounded-lg" style={{ backgroundColor: 'rgba(155, 229, 100, 0.1)' }}>
              <p className="text-14 text-platinum/80">
                <strong>Our Commitment:</strong> Your camera, your data, your control. 
                We believe in transparency and user empowerment.
              </p>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
}

export default About;

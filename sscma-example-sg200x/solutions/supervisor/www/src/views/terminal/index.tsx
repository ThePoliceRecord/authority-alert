import { useEffect, useState } from "react";
import { CodeOutlined } from "@ant-design/icons";
import useConfigStore from "@/store/config";

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

function WebShell() {
  const { deviceInfo } = useConfigStore();
  const [iframeUrl, setIframeUrl] = useState("");
  
  useEffect(() => {
    if (deviceInfo) {
      const url =
        import.meta.env.MODE === "development"
          ? "http://192.168.120.99"
          : window.location.origin;
      setIframeUrl(`${url}:${deviceInfo.terminalPort}`);
    }
  }, [deviceInfo]);

  return (
    <div className="h-full p-16 flex flex-col">
      {/* Page Header */}
      <div className="mb-16 flex-shrink-0">
        <div className="flex items-center gap-12">
          <CodeOutlined style={{ fontSize: 28, color: '#9be564' }} />
          <h1 className="text-28 font-bold text-white m-0">Terminal</h1>
        </div>
      </div>

      {/* Terminal Card */}
      <div 
        className="flex-1 p-4 overflow-hidden" 
        style={{
          ...translucentCardStyle,
          minHeight: '400px',
        }}
      >
        <iframe
          src={iframeUrl}
          style={{ 
            width: "100%", 
            height: "100%",
            border: "none",
            borderRadius: "8px",
            backgroundColor: "#1a1a1a",
          }}
        />
      </div>
    </div>
  );
}

export default WebShell;

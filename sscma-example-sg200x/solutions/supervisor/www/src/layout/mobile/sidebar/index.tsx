import { useState } from "react";
import { Popup } from "antd-mobile";
import OverviewImg from "@/assets/images/svg/overview.svg";
import SecurityImg from "@/assets/images/svg/security.svg";
import NetworkImg from "@/assets/images/svg/network.svg";
import TerminalImg from "@/assets/images/svg/terminal.svg";
import SystemImg from "@/assets/images/svg/system.svg";
import FilesImg from "@/assets/images/svg/files.svg";
import { useLocation, useNavigate } from "react-router-dom";
import useConfigStore from "@/store/config";
import { clearCurrentUser } from "@/store/user";
import {
  MenuOutlined,
  LogoutOutlined,
  VideoCameraOutlined,
  BulbOutlined,
  CloudDownloadOutlined,
  InfoCircleOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";

function Sidebar() {
  const location = useLocation();
  const currentRoute = location.pathname;
  const [visible, setVisible] = useState(false);
  const navigate = useNavigate();
  const { deviceInfo } = useConfigStore();

  const handleLogout = async () => {
    await clearCurrentUser();
    setVisible(false);
    // Reset URL to root - App.tsx will show Login component when token is cleared
    window.location.hash = "#/";
  };

  // Menu items with support for both SVG images and Ant Design icons
  type MenuItem = {
    label: string;
    icon?: string;
    antIcon?: React.ReactNode;
    route: string;
    judgeApp?: boolean;
  };

  const menuList: MenuItem[][] = [
    [
      {
        label: "Overview",
        icon: OverviewImg,
        route: "/overview",
        judgeApp: true,
      },
      { label: "Files", icon: FilesImg, route: "/files" },
      { label: "Recording", antIcon: <VideoCameraOutlined />, route: "/recording" },
      { label: "Detections", antIcon: <CloudUploadOutlined />, route: "/detections" },
      { label: "Security", icon: SecurityImg, route: "/security" },
      { label: "Network", icon: NetworkImg, route: "/network" },
    ],
    [
      { label: "LED Config", antIcon: <BulbOutlined />, route: "/led-config" },
      { label: "Updates", antIcon: <CloudDownloadOutlined />, route: "/updates" },
      { label: "Terminal", icon: TerminalImg, route: "/terminal" },
      { label: "System", icon: SystemImg, route: "/system" },
    ],
    [
      { label: "About", antIcon: <InfoCircleOutlined />, route: "/about" },
    ],
  ];

  return (
    <div className="inline-block">
      <div
        className="flex p-16 cursor-pointer text-white hover:opacity-80 transition-opacity"
        onClick={() => {
          setVisible(true);
        }}
      >
        <MenuOutlined className="text-20" />
      </div>

      <Popup
        visible={visible}
        onMaskClick={() => {
          setVisible(false);
        }}
        position="left"
        bodyStyle={{ 
          width: "290px",
          backgroundColor: 'rgba(31, 31, 27, 0.95)',
          boxShadow: '4px 0 16px rgba(0, 0, 0, 0.5)',
        }}
      >
        <div className="pt-60 flex flex-col h-full">
          {/* Header Section */}
          <div 
            className="px-24 pb-20"
            style={{
              backgroundColor: 'rgba(35, 40, 187, 0.9)',
              marginTop: '-60px',
              paddingTop: '60px',
            }}
          >
            <div className="font-bold text-22 text-white truncate pr-20">
              {deviceInfo.deviceName}
            </div>
            <div className="text-14 text-white opacity-70 mt-4">
              {deviceInfo.ip}
            </div>
          </div>
          
          {/* Menu Section - Translucent Dark */}
          <div className="flex-1 pt-16 overflow-y-auto">
            {menuList.map((item, index) => {
              return (
                <div key={index}>
                  {index > 0 && (
                    <div className="border-t border-white/10 mx-20 my-8"></div>
                  )}
                  <div className="py-8">
                    {item.map((citem, cindex) => {
                      const isActive = currentRoute === citem.route;
                      return (
                        (deviceInfo.isReCamera || !citem.judgeApp) && (
                          <div
                            className={`mx-12 px-20 py-12 text-16 flex items-center rounded-lg cursor-pointer transition-all duration-fast ${
                              isActive 
                                ? "text-white" 
                                : "text-platinum hover:bg-white/10"
                            }`}
                            style={isActive ? {
                              backgroundColor: 'rgba(3, 68, 255, 0.6)',
                              boxShadow: '0 2px 8px rgba(3, 68, 255, 0.4)',
                            } : undefined}
                            key={`${index}${cindex}`}
                            onClick={() => {
                              navigate(citem.route);
                              setVisible(false);
                            }}
                          >
                            {citem.icon ? (
                              <img
                                className={`w-22 h-22 mr-12 ${isActive ? "invert brightness-200" : "opacity-80"}`}
                                src={citem.icon}
                                alt=""
                                style={!isActive ? { filter: 'invert(0.9)' } : undefined}
                              />
                            ) : citem.antIcon ? (
                              <span
                                className={`mr-12 text-22 ${isActive ? "text-white" : "text-platinum/80"}`}
                              >
                                {citem.antIcon}
                              </span>
                            ) : null}
                            <span className="font-medium">{citem.label}</span>
                          </div>
                        )
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
          
          {/* Logout Section */}
          <div className="border-t border-white/10">
            <div className="py-12 px-12">
              <div
                className="px-20 py-12 text-16 flex items-center cursor-pointer text-red-400 hover:bg-red-500/20 rounded-lg transition-all duration-fast"
                onClick={handleLogout}
              >
                <LogoutOutlined className="w-22 h-22 mr-12 text-20" />
                <span className="font-medium">Logout</span>
              </div>
            </div>
          </div>
        </div>
      </Popup>
    </div>
  );
}

export default Sidebar;

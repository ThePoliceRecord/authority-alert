import React, { useState } from "react";
import { Form, Input, Modal } from "antd";
import {
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  VideoCameraOutlined,
  BulbOutlined,
  CloudDownloadOutlined,
  InfoCircleOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";
import useConfigStore from "@/store/config";
import { clearCurrentUser } from "@/store/user";
import EditImg from "@/assets/images/svg/edit.svg";
import OverviewImg from "@/assets/images/svg/overview.svg";
import SecurityImg from "@/assets/images/svg/security.svg";
import NetworkImg from "@/assets/images/svg/network.svg";
import TerminalImg from "@/assets/images/svg/terminal.svg";
import SystemImg from "@/assets/images/svg/system.svg";
import FilesImg from "@/assets/images/svg/files.svg";
import { updateDeviceInfoApi, queryDeviceInfoApi } from "@/api/device/index";
import { hostnameValidate } from "@/utils/validate";
import { useLocation, useNavigate } from "react-router-dom";

interface Props {
  children: React.ReactNode;
}

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

// Translucent card style (matching TPR.css .translucent-card-grey)
const translucentSidebarStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.9)',
  boxShadow: '2px 0 8px rgba(0, 0, 0, 0.5)',
};

// Modal styles (matching TPR.css .translucent-card-grey-1)
const modalContentStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.95)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
};

const PCLayout: React.FC<Props> = ({ children }) => {
  const { deviceInfo, updateDeviceInfo } = useConfigStore();
  const [isEditNameModalOpen, setIsEditNameModalOpen] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const location = useLocation();
  const [form] = Form.useForm();
  const currentRoute = location.pathname;
  const navigate = useNavigate();

  const onQueryDeviceInfo = async () => {
    const res = await queryDeviceInfoApi();
    updateDeviceInfo(res.data);
  };

  const handleLogout = async () => {
    await clearCurrentUser();
    // Reset URL to root - App.tsx will show Login component when token is cleared
    window.location.hash = "#/";
  };

  const handleEditNameOk = async () => {
    try {
      const values = await form.validateFields();
      const deviceName = (values.deviceName || "").trim();
      setConfirmLoading(true);
      await updateDeviceInfoApi({ deviceName });
      setIsEditNameModalOpen(false);
      form.resetFields();
      await onQueryDeviceInfo();
    } catch (error) {
      console.log(error);
    } finally {
      setConfirmLoading(false);
    }
  };

  const handleEditNameCancel = () => {
    setIsEditNameModalOpen(false);
    form.resetFields();
  };

  const toggleSidebar = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  return (
    <>
      {/* Top Header Bar */}
      <div 
        className="text-center py-12 border-b border-white/10"
        style={{
          backgroundColor: 'rgba(35, 40, 187, 0.95)',
          boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
        }}
      >
        <div className="text-white text-18 font-semibold relative flex justify-center items-center px-40">
          {/* Sidebar Toggle Button */}
          <button
            onClick={toggleSidebar}
            className="absolute left-16 p-8 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-all duration-200"
            title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {sidebarCollapsed ? (
              <MenuUnfoldOutlined style={{ fontSize: 20 }} />
            ) : (
              <MenuFoldOutlined style={{ fontSize: 20 }} />
            )}
          </button>
          
          <div className="truncate">{deviceInfo?.deviceName}</div>
          <img
            className="w-20 h-20 ml-2 self-center cursor-pointer opacity-80 hover:opacity-100 transition-opacity invert"
            onClick={() => {
              setIsEditNameModalOpen(true);
            }}
            src={EditImg}
            alt="Edit"
          />
        </div>
        <div className="mt-2 text-white opacity-70 text-14">{deviceInfo?.ip}</div>
      </div>

      <div className="flex-1 relative">
        {/* Left Sidebar Navigation - Absolute positioned to overlay content */}
        <div
          className={`absolute top-0 left-0 h-full flex flex-col transition-all duration-300 z-50 ${
            sidebarCollapsed ? 'w-70' : 'w-220'
          }`}
          style={translucentSidebarStyle}
        >
          <div className="flex-1 pt-16 overflow-y-auto overflow-x-hidden">
            {menuList.map((item, index) => {
              return (
                <div key={index}>
                  {index > 0 && (
                    <div className={`border-t border-white/10 my-8 ${sidebarCollapsed ? 'mx-8' : 'mx-16'}`}></div>
                  )}
                  <div className="py-8">
                    {item.map((citem, cindex) => {
                      const isActive = currentRoute === citem.route;
                      return (
                        (deviceInfo.isReCamera || !citem.judgeApp) && (
                          <div
                            className={`${sidebarCollapsed ? 'mx-8 px-12 justify-center' : 'mx-12 px-20'} py-12 text-15 flex items-center rounded-lg cursor-pointer transition-all duration-fast ${
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
                            }}
                            title={sidebarCollapsed ? citem.label : undefined}
                          >
                            {citem.icon ? (
                              <img
                                className={`w-22 h-22 ${sidebarCollapsed ? '' : 'mr-12'} ${isActive ? "invert brightness-200" : "opacity-80"}`}
                                src={citem.icon}
                                alt=""
                                style={!isActive ? { filter: 'invert(0.9)' } : undefined}
                              />
                            ) : citem.antIcon ? (
                              <span
                                className={`${sidebarCollapsed ? '' : 'mr-12'} text-22 ${isActive ? "text-white" : "text-platinum/80"}`}
                              >
                                {citem.antIcon}
                              </span>
                            ) : null}
                            {!sidebarCollapsed && (
                              <span className="font-medium whitespace-nowrap">{citem.label}</span>
                            )}
                          </div>
                        )
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
          
          {/* Logout Button */}
          <div className={`border-t border-white/10 ${sidebarCollapsed ? 'mx-8' : 'mx-16'}`}></div>
          <div className={`py-12 ${sidebarCollapsed ? 'px-8' : 'px-12'}`}>
            <div
              className={`${sidebarCollapsed ? 'px-12 justify-center' : 'px-20'} py-12 text-15 flex items-center cursor-pointer text-red-400 hover:bg-red-500/20 rounded-lg transition-all duration-fast`}
              onClick={handleLogout}
              title={sidebarCollapsed ? "Logout" : undefined}
            >
              <LogoutOutlined className={`${sidebarCollapsed ? '' : 'mr-12'} text-20`} />
              {!sidebarCollapsed && (
                <span className="font-medium whitespace-nowrap">Logout</span>
              )}
            </div>
          </div>
        </div>
        
        {/* Main Content Area - With Background Image - Full width, content centered */}
        <div
          className="w-full h-full overflow-y-auto"
          style={{
            backgroundImage: `linear-gradient(rgba(0, 0, 0, 0.6), rgba(0, 0, 0, 0.6)), url('/brand/blueback.jpeg')`,
            backgroundSize: 'cover',
            backgroundPosition: 'center',
            backgroundRepeat: 'no-repeat',
            backgroundAttachment: 'fixed',
          }}
        >
          <div className="flex justify-center w-full h-full">
            <div style={{ maxWidth: "700px", width: "100%" }} className="px-24 py-24">
              {children}
            </div>
          </div>
        </div>
      </div>

      {/* Edit Device Name Modal */}
      <Modal
        title={<span className="text-platinum">Edit Device Name</span>}
        open={isEditNameModalOpen}
        confirmLoading={confirmLoading}
        onOk={handleEditNameOk}
        onCancel={handleEditNameCancel}
        okButtonProps={{ 
          style: { 
            backgroundColor: '#2328bb', 
            borderColor: '#0065a3' 
          } 
        }}
        styles={{
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
        }}
      >
        <Form
          form={form}
          name="dependencies"
          autoComplete="off"
          style={{ maxWidth: 600 }}
          layout="vertical"
        >
          <Form.Item
            name="deviceName"
            label={<span className="text-platinum">Name</span>}
            rules={[hostnameValidate(32)]}
          >
            <Input 
              placeholder="recamera-132456" 
              maxLength={32} 
              allowClear
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default PCLayout;

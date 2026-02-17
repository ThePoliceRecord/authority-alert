import { useEffect, useMemo, useState } from "react";
import EditBlackImg from "@/assets/images/svg/editBlack.svg";
import ArrowImg from "@/assets/images/svg/downArrow.svg";
import CommonPopup from "@/components/common-popup";
import { Form, Input, Picker, ProgressBar, Mask } from "antd-mobile";
import {
  PickerValue,
  PickerValueExtend,
} from "antd-mobile/es/components/picker";
import { Button, Modal, message, Switch } from "antd";
import { ExclamationCircleOutlined, ReloadOutlined, SyncOutlined, PoweroffOutlined } from "@ant-design/icons";
import moment from "moment";
import { useData } from "./hook";
import { DeviceChannleMode, UpdateStatus, PowerMode } from "@/enum";
import { requiredTrimValidate } from "@/utils/validate";
import { parseUrlParam } from "@/utils";
import useConfigStore from "@/store/config";
import { factoryResetApi, setDevicePowerApi, getAnalyticsConfigApi, setAnalyticsConfigApi, reRegisterCameraApi } from "@/api/device/index";

const channelList = [
  { label: "Self Hosted", value: DeviceChannleMode.Self },
  { label: "TPR Official", value: DeviceChannleMode.Official },
];
const infoList = [
  { label: "Serial Number", key: "sn" },
  { label: "CPU", key: "cpu" },
  { label: "RAM", key: "ram" },
  { label: "NPU", key: "npu" },
  { label: "OS", key: "osVersion" },
  { label: "Device Info", key: "type" },
];

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

function System() {
  const {
    deviceInfo,
    addressFormRef,
    onEditServerAddress,
    onCancel,
    onFinish,
    onUpdateCancel,
    onUpdateApply,
    onConfirm,
    onChannelChange,
    onUpdateRestart,
    onUpdateCheck,
  } = useData();

  const { systemUpdateState, setSystemUpdateState } = useConfigStore();

  const [isDashboard, setIsDashboard] = useState(false);
  const [factoryResetLoading, setFactoryResetLoading] = useState(false);
  const [shareAnalytics, setShareAnalytics] = useState(true);
  const [reRegisterLoading, setReRegisterLoading] = useState(false);
  
  useEffect(() => {
    const param = parseUrlParam(window.location.href);
    const dashboard = param.dashboard || param.disablelayout;
    setIsDashboard(dashboard == 1);
  }, []);

  // Load analytics configuration on mount
  useEffect(() => {
    const loadAnalyticsConfig = async () => {
      try {
        const response = await getAnalyticsConfigApi();
        setShareAnalytics(response.data.enabled);
      } catch (error) {
        console.error("Failed to load analytics config:", error);
      }
    };
    loadAnalyticsConfig();
  }, []);

  const channelLable = useMemo(() => {
    const index = channelList.findIndex(
      (item) => item.value === systemUpdateState.channel
    );
    return index > -1 && channelList[index].label;
  }, [systemUpdateState.channel]);

  const handleFactoryReset = () => {
    Modal.confirm({
      title: "Factory Reset",
      icon: <ExclamationCircleOutlined style={{ color: "#ff4d4f" }} />,
      content: (
        <div>
          <p>This will reset the device to factory defaults.</p>
          <p className="text-red-500 font-bold mt-8">
            All data and settings will be lost!
          </p>
          <p className="mt-8">The device will reboot to complete the reset.</p>
        </div>
      ),
      okText: "Reset",
      okType: "danger",
      cancelText: "Cancel",
      centered: true,
      onOk: async () => {
        setFactoryResetLoading(true);
        try {
          const response = await factoryResetApi();
          if (response.code === 0) {
            message.success("Factory reset scheduled. Rebooting device...");
            // Reboot after a short delay
            setTimeout(async () => {
              await setDevicePowerApi({ mode: PowerMode.Restart });
              Modal.info({
                title: "Device rebooting...",
                icon: <ReloadOutlined spin style={{ color: "#8fc31f" }} />,
                content: "Please wait for the device to restart and then refresh the page.",
                centered: true,
                footer: null,
              });
            }, 1000);
          } else {
            message.error(response.msg || "Failed to initiate factory reset");
          }
        } catch (error) {
          message.error("Failed to initiate factory reset");
        } finally {
          setFactoryResetLoading(false);
        }
      },
    });
  };

  const handleShareAnalyticsChange = async (checked: boolean) => {
    // Optimistic update
    setShareAnalytics(checked);
    
    try {
      await setAnalyticsConfigApi({ enabled: checked });
      message.success(checked ? "Analytics sharing enabled" : "Analytics sharing disabled");
    } catch (error) {
      console.error("Failed to save analytics preference:", error);
      message.error("Failed to save analytics preference");
      // Revert on error
      setShareAnalytics(!checked);
    }
  };

  const handleReRegisterCamera = () => {
    Modal.confirm({
      title: "Re-register Camera",
      icon: <SyncOutlined style={{ color: "#2328bb" }} />,
      content: (
        <div>
          <p>This will re-register the camera with the Authority Alert service.</p>
          <p className="mt-8">Use this if you need to update the camera's registration or if you're experiencing connection issues.</p>
        </div>
      ),
      okText: "Re-register",
      cancelText: "Cancel",
      centered: true,
      onOk: async () => {
        setReRegisterLoading(true);
        try {
          await reRegisterCameraApi();
          message.success("Camera re-registered successfully");
        } catch (error) {
          console.error("Failed to re-register camera:", error);
          message.error("Failed to re-register camera");
        } finally {
          setReRegisterLoading(false);
        }
      },
    });
  };

  const handlePowerAction = (mode: PowerMode) => {
    const isReboot = mode === PowerMode.Restart;
    
    Modal.confirm({
      title: <span className="text-platinum">{isReboot ? 'Reboot Device' : 'Shutdown Device'}</span>,
      icon: <ExclamationCircleOutlined style={{ color: isReboot ? '#2328bb' : '#ff4d4f' }} />,
      content: (
        <div className="text-platinum/80">
          {isReboot
            ? 'The device will restart. This may take a few minutes. Are you sure you want to continue?'
            : 'The device will shut down completely. You will need physical access to turn it back on. Are you sure you want to continue?'
          }
        </div>
      ),
      okText: isReboot ? 'Reboot' : 'Shutdown',
      okType: isReboot ? 'primary' : 'primary',
      okButtonProps: isReboot ? {} : { danger: true },
      cancelText: 'Cancel',
      centered: true,
      onOk: async () => {
        await setDevicePowerApi({ mode });
        message.success(isReboot ? "Device is rebooting..." : "Device is shutting down...");
      },
    });
  };

  return (
    <div className="my-8 p-16">
      {!isDashboard && (
        <div>
          <div className="font-bold text-18 mb-14 my-24 text-platinum">System Info</div>
          <div className="px-24" style={translucentCardStyle}>
            {infoList.map((item, index) => {
              return (
                <div
                  key={item.key}
                  className={`flex justify-between py-24 ${
                    index && "border-t border-white/10"
                  }`}
                >
                  <span className="text-platinum/70 mr-20">
                    {item.label}
                  </span>
                  <div className="flex-1 truncate text-right text-platinum">
                    {item.key == "osVersion"
                      ? `${deviceInfo.osName} ${deviceInfo[item.key]}`
                      : deviceInfo[item.key]}
                  </div>
                </div>
              );
            })}
          </div>

          <div className="font-bold text-18 mb-14 my-24 text-platinum">Camera Registration</div>
          <div className="px-24 py-24" style={translucentCardStyle}>
            <div className="flex justify-between items-center">
              <div className="flex-1 mr-20">
                <span className="text-platinum/70">Re-register Camera</span>
                <p className="text-12 text-platinum/50 mt-4">
                  Re-register this camera with the Authority Alert service. Use this if you need to update registration or fix connection issues.
                </p>
              </div>
              <Button
                type="primary"
                onClick={handleReRegisterCamera}
                loading={reRegisterLoading}
                icon={<SyncOutlined />}
              >
                Re-register Camera
              </Button>
            </div>
          </div>

          <div className="font-bold text-18 mb-14 my-24 text-platinum">Privacy</div>
          <div className="px-24 py-24" style={translucentCardStyle}>
            <div className="flex justify-between items-center">
              <div className="flex-1 mr-20">
                <span className="text-platinum/70">Share Analytics with Us</span>
                <p className="text-12 text-platinum/50 mt-4">
                  Help improve Authority Alert by sharing anonymous usage data and analytics.
                </p>
              </div>
              <Switch
                checked={shareAnalytics}
                onChange={handleShareAnalyticsChange}
              />
            </div>
          </div>

          <div className="font-bold text-18 mb-14 my-24 text-platinum">Power</div>
          <div className="p-24" style={translucentCardStyle}>
            <div className="space-y-16">
              {/* Reboot Option */}
              <div
                className="flex items-center justify-between p-16 rounded-12 border border-white/10 hover:border-primary/30 transition-colors cursor-pointer"
                style={{ backgroundColor: 'rgba(255, 255, 255, 0.03)' }}
                onClick={() => handlePowerAction(PowerMode.Restart)}
              >
                <div className="flex items-center">
                  <div className="w-40 h-40 rounded-full flex items-center justify-center mr-12" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                    <ReloadOutlined style={{ fontSize: 20, color: '#9be564' }} />
                  </div>
                  <div>
                    <div className="text-16 font-medium text-platinum">Reboot</div>
                    <div className="text-12 text-platinum/50 mt-2">
                      Restart the device. Services will be temporarily unavailable.
                    </div>
                  </div>
                </div>
                <Button
                  type="primary"
                  icon={<ReloadOutlined />}
                  onClick={(e) => {
                    e.stopPropagation();
                    handlePowerAction(PowerMode.Restart);
                  }}
                >
                  Reboot
                </Button>
              </div>

              {/* Shutdown Option */}
              <div
                className="flex items-center justify-between p-16 rounded-12 border border-white/10 hover:border-red-500/30 transition-colors cursor-pointer"
                style={{ backgroundColor: 'rgba(255, 255, 255, 0.03)' }}
                onClick={() => handlePowerAction(PowerMode.Shutdown)}
              >
                <div className="flex items-center">
                  <div className="w-40 h-40 rounded-full flex items-center justify-center mr-12" style={{ backgroundColor: 'rgba(255, 77, 79, 0.2)' }}>
                    <PoweroffOutlined style={{ fontSize: 20, color: '#ff4d4f' }} />
                  </div>
                  <div>
                    <div className="text-16 font-medium text-platinum">Shutdown</div>
                    <div className="text-12 text-platinum/50 mt-2">
                      Power off the device completely. Requires physical access to restart.
                    </div>
                  </div>
                </div>
                <Button
                  danger
                  icon={<PoweroffOutlined />}
                  onClick={(e) => {
                    e.stopPropagation();
                    handlePowerAction(PowerMode.Shutdown);
                  }}
                >
                  Shutdown
                </Button>
              </div>
            </div>
          </div>

          {/* Power Warning */}
          <div className="mt-12 px-16 py-12 rounded-lg" style={{ backgroundColor: 'rgba(255, 77, 79, 0.15)' }}>
            <div className="flex items-start">
              <ExclamationCircleOutlined style={{ color: '#ff6b6b', fontSize: 16, marginTop: 2, marginRight: 12 }} />
              <div className="text-14 text-white">
                Shutdown requires a power cycle (disconnect and reconnect power) to restart the device.
              </div>
            </div>
          </div>

          <div className="font-bold text-18 mb-14 my-24 text-platinum">Factory Reset</div>
          <div className="px-24 py-24" style={translucentCardStyle}>
            <div className="flex justify-between items-center">
              <div className="flex-1 mr-20">
                <span className="text-platinum/70">Reset Device</span>
                <p className="text-12 text-platinum/50 mt-4">
                  Reset the device to factory defaults. All data and settings will be lost.
                </p>
              </div>
              <Button
                danger
                onClick={handleFactoryReset}
                loading={factoryResetLoading}
              >
                Factory Reset
              </Button>
            </div>
          </div>
        </div>
      )}

      <CommonPopup
        visible={systemUpdateState.visible}
        title={"Server Address"}
        onCancel={onCancel}
      >
        <Form
          ref={addressFormRef}
          className="border-b-0"
          requiredMarkStyle="none"
          onFinish={onFinish}
          initialValues={{
            serverUrl: systemUpdateState.address,
          }}
          footer={
            <Button block htmlType="submit" type="primary">
              Confirm
            </Button>
          }
        >
          <Form.Item name="serverUrl" label="" rules={[requiredTrimValidate()]}>
            <Input className="border rounded-6 p-10" placeholder="" clearable />
          </Form.Item>
        </Form>
      </CommonPopup>
      <Mask
        visible={systemUpdateState.updateInfoVisible}
        onMaskClick={() =>
          setSystemUpdateState({
            updateInfoVisible: false,
          })
        }
      >
        <div className="px-30 pt-100 pb-100 h-full" style={{ height: "100vh" }}>
          <div className="rounded-16 p-20 h-full flex-1 flex flex-col justify-between" style={translucentCardStyle}>
            <div className="font-bold text-16 text-platinum">Authority Alert OS Update</div>
            <div className="flex justify-between text-platinum/60 font-bold mt-6 mb-10">
              <span>Version 15.4</span>
              <span>24/06/2024</span>
            </div>
            <div className="flex-1 overflow-y-auto">
              <div className="text-12 text-platinum/70">
                channel, and included all the changes have been tested in
                preview
              </div>
            </div>
            <div className="flex mt-20">
              <Button className="flex-1 mr-28" onClick={onUpdateCancel}>
                Cancel
              </Button>
              <Button type="primary" className="flex-1" onClick={onUpdateApply}>
                Apply
              </Button>
            </div>
          </div>
        </div>
      </Mask>
    </div>
  );
}

export default System;

import { message, Button, Modal } from "antd";
import { PoweroffOutlined, ReloadOutlined, ExclamationCircleOutlined } from "@ant-design/icons";
import { setDevicePowerApi } from "@/api/device/index";
import { PowerMode } from "@/enum";

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

// Modal styles
const modalStyles = {
  content: {
    backgroundColor: 'rgba(31, 31, 27, 0.95)',
    boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  },
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

function Power() {
  const onOperateDevice = async (mode: PowerMode) => {
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
      styles: modalStyles,
      onOk: async () => {
        await setDevicePowerApi({ mode });
        message.success(isReboot ? "Device is rebooting..." : "Device is shutting down...");
      },
    });
  };

  return (
    <div className="h-full p-16">
      {/* Page Header */}
      <div className="mb-24">
        <div className="flex items-center gap-12">
          <PoweroffOutlined style={{ fontSize: 28, color: '#9be564' }} />
          <h1 className="text-28 font-bold text-white m-0">Power</h1>
        </div>
      </div>

      {/* Power Options Card */}
      <div className="p-24" style={translucentCardStyle}>
        <div className="space-y-16">
          {/* Reboot Option */}
          <div 
            className="flex items-center justify-between p-20 rounded-12 border border-white/10 hover:border-primary/30 transition-colors cursor-pointer"
            style={{ backgroundColor: 'rgba(255, 255, 255, 0.03)' }}
            onClick={() => onOperateDevice(PowerMode.Restart)}
          >
            <div className="flex items-center">
              <div className="w-48 h-48 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                <ReloadOutlined style={{ fontSize: 24, color: '#9be564' }} />
              </div>
              <div>
                <div className="text-18 font-medium text-platinum">Reboot</div>
                <div className="text-13 text-platinum/50 mt-4">
                  Restart the device. Services will be temporarily unavailable.
                </div>
              </div>
            </div>
            <Button 
              type="primary"
              size="large"
              icon={<ReloadOutlined />}
              onClick={(e) => {
                e.stopPropagation();
                onOperateDevice(PowerMode.Restart);
              }}
            >
              Reboot
            </Button>
          </div>

          {/* Shutdown Option */}
          <div 
            className="flex items-center justify-between p-20 rounded-12 border border-white/10 hover:border-red-500/30 transition-colors cursor-pointer"
            style={{ backgroundColor: 'rgba(255, 255, 255, 0.03)' }}
            onClick={() => onOperateDevice(PowerMode.Shutdown)}
          >
            <div className="flex items-center">
              <div className="w-48 h-48 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(255, 77, 79, 0.2)' }}>
                <PoweroffOutlined style={{ fontSize: 24, color: '#ff4d4f' }} />
              </div>
              <div>
                <div className="text-18 font-medium text-platinum">Shutdown</div>
                <div className="text-13 text-platinum/50 mt-4">
                  Power off the device completely. Requires physical access to restart.
                </div>
              </div>
            </div>
            <Button 
              danger
              size="large"
              icon={<PoweroffOutlined />}
              onClick={(e) => {
                e.stopPropagation();
                onOperateDevice(PowerMode.Shutdown);
              }}
            >
              Shutdown
            </Button>
          </div>
        </div>
      </div>

      {/* Info Note */}
      <div className="mt-16 px-16 py-12 rounded-lg" style={{ backgroundColor: 'rgba(255, 77, 79, 0.15)' }}>
        <div className="flex items-start">
          <ExclamationCircleOutlined style={{ color: '#ff6b6b', fontSize: 16, marginTop: 2, marginRight: 12 }} />
          <div className="text-14 text-white">
            Shutdown requires a power cycle (disconnect and reconnect power) to restart the device.
          </div>
        </div>
      </div>
    </div>
  );
}

export default Power;

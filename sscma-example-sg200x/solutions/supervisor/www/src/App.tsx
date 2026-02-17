import { useEffect, useMemo } from "react";
import { ConfigProvider } from "antd";
import { ConfigProvider as MobileConfigProvider } from "antd-mobile";
import enUS from "antd-mobile/es/locales/en-US";
import { createHashRouter, RouterProvider } from "react-router-dom";
import Routes from "@/router";
import Login from "@/views/login";
import { queryDeviceInfoApi } from "@/api/device/index";
import useUserStore from "@/store/user";
import useConfigStore from "@/store/config";
import { Version } from "@/utils";

const router = createHashRouter(Routes);

// Brand color tokens from TPR theme
const brandColors = {
  primary: "#2328bb",
  primaryHover: "#0065a3",
  primaryLight: "#1fa9ff",
  primaryDark: "#1a237e",
  success: "#9be564",
  warning: "#f3b61f",
  error: "#e3170a",
  errorDark: "#730001",
  text: "#2a2a27",
  textSecondary: "#5e747f",
  textMuted: "#7f7f76",
  border: "#e0e0e0",
  background: "#f1f3f5",
  surface: "#fdfffc",
};

const App = () => {
  const {
    currentSn,
    usersBySn,
    setCurrentSn,
    clearCurrentUserInfo,
  } = useUserStore();

  const { updateDeviceInfo } = useConfigStore();

  useEffect(() => {
    console.log(`%cVersion: ${Version}`, "font-weight: bold");
    initUserData();
  }, []);

  const token = useMemo(() => {
    return currentSn ? usersBySn[currentSn]?.token : null;
  }, [usersBySn, currentSn]);

  const initUserData = async () => {
    try {
      const response = await queryDeviceInfoApi();
      const deviceInfo = response.data;
      // Query device info, get sn
      updateDeviceInfo(deviceInfo);
      const sn = deviceInfo.sn;
      setCurrentSn(sn);
    } catch (error) {
      // Don't clear user info, likely service not started timeout
    }
  };

  // Clear hash when redirecting to login to prevent redirect loops
  useEffect(() => {
    if (!token && window.location.hash && window.location.hash !== '#/') {
      window.location.hash = '/';
    }
  }, [token]);

  return (
    <ConfigProvider
      theme={{
        token: {
          // Primary colors
          colorPrimary: brandColors.primary,
          colorPrimaryHover: brandColors.primaryHover,
          colorPrimaryActive: brandColors.primaryDark,
          colorPrimaryBg: `${brandColors.primary}10`,
          colorPrimaryBgHover: `${brandColors.primary}20`,
          colorPrimaryBorder: brandColors.primary,
          colorPrimaryBorderHover: brandColors.primaryHover,
          colorPrimaryText: brandColors.primary,
          colorPrimaryTextHover: brandColors.primaryHover,
          colorPrimaryTextActive: brandColors.primaryDark,
          
          // Success colors
          colorSuccess: brandColors.success,
          colorSuccessBg: `${brandColors.success}20`,
          colorSuccessBorder: brandColors.success,
          colorSuccessText: brandColors.success,
          
          // Warning colors
          colorWarning: brandColors.warning,
          colorWarningBg: `${brandColors.warning}20`,
          colorWarningBorder: brandColors.warning,
          colorWarningText: brandColors.warning,
          
          // Error colors
          colorError: brandColors.errorDark,
          colorErrorBg: `${brandColors.error}10`,
          colorErrorBorder: brandColors.errorDark,
          colorErrorText: brandColors.errorDark,
          colorErrorHover: brandColors.error,
          
          // Info colors (using primary)
          colorInfo: brandColors.primaryLight,
          colorInfoBg: `${brandColors.primaryLight}10`,
          colorInfoBorder: brandColors.primaryLight,
          colorInfoText: brandColors.primaryLight,
          
          // Text colors
          colorText: brandColors.text,
          colorTextSecondary: brandColors.textSecondary,
          colorTextTertiary: brandColors.textMuted,
          colorTextQuaternary: brandColors.textMuted,
          
          // Background colors
          colorBgContainer: brandColors.surface,
          colorBgElevated: brandColors.surface,
          colorBgLayout: brandColors.background,
          colorBgSpotlight: brandColors.primaryDark,
          
          // Border colors
          colorBorder: brandColors.border,
          colorBorderSecondary: brandColors.border,
          
          // Link colors
          colorLink: brandColors.primary,
          colorLinkHover: brandColors.primaryHover,
          colorLinkActive: brandColors.primaryDark,
          
          // Border radius
          borderRadius: 6,
          borderRadiusLG: 8,
          borderRadiusSM: 4,
          borderRadiusXS: 2,
          
          // Font
          fontFamily: "'Inter', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          fontSize: 14,
          fontSizeLG: 16,
          fontSizeSM: 12,
          fontSizeXL: 20,
          
          // Control heights
          controlHeight: 36,
          controlHeightLG: 44,
          controlHeightSM: 28,
          
          // Motion
          motionDurationFast: "0.15s",
          motionDurationMid: "0.25s",
          motionDurationSlow: "0.35s",
        },
        components: {
          Button: {
            primaryShadow: "0 2px 4px rgba(35, 40, 187, 0.2)",
            defaultBorderColor: brandColors.border,
            defaultColor: brandColors.text,
            fontWeight: 500,
          },
          Input: {
            activeBorderColor: brandColors.primary,
            hoverBorderColor: brandColors.primaryHover,
            activeShadow: `0 0 0 2px ${brandColors.primary}20`,
          },
          Select: {
            optionSelectedBg: `${brandColors.primary}10`,
            optionActiveBg: `${brandColors.primary}05`,
          },
          Modal: {
            titleFontSize: 18,
            headerBg: brandColors.surface,
            contentBg: brandColors.surface,
          },
          Card: {
            headerBg: brandColors.surface,
            actionsBg: brandColors.surface,
          },
          Menu: {
            itemSelectedBg: `${brandColors.primary}10`,
            itemSelectedColor: brandColors.primary,
            itemHoverBg: `${brandColors.primary}05`,
          },
          Table: {
            headerBg: brandColors.background,
            rowHoverBg: `${brandColors.primary}05`,
            headerSortActiveBg: `${brandColors.primary}10`,
          },
          Tabs: {
            inkBarColor: brandColors.primary,
            itemSelectedColor: brandColors.primary,
            itemHoverColor: brandColors.primaryHover,
          },
          Switch: {
            colorPrimary: brandColors.primary,
            colorPrimaryHover: brandColors.primaryHover,
          },
          Checkbox: {
            colorPrimary: brandColors.primary,
            colorPrimaryHover: brandColors.primaryHover,
          },
          Radio: {
            colorPrimary: brandColors.primary,
            colorPrimaryHover: brandColors.primaryHover,
          },
          Progress: {
            defaultColor: brandColors.primary,
          },
          Slider: {
            trackBg: brandColors.primary,
            trackHoverBg: brandColors.primaryHover,
            handleColor: brandColors.primary,
            handleActiveColor: brandColors.primaryHover,
          },
          Tag: {
            defaultBg: `${brandColors.primary}10`,
            defaultColor: brandColors.primary,
          },
          Badge: {
            colorError: brandColors.errorDark,
          },
          Alert: {
            colorInfoBg: `${brandColors.primaryLight}10`,
            colorInfoBorder: brandColors.primaryLight,
            colorSuccessBg: `${brandColors.success}10`,
            colorSuccessBorder: brandColors.success,
            colorWarningBg: `${brandColors.warning}10`,
            colorWarningBorder: brandColors.warning,
            colorErrorBg: `${brandColors.error}10`,
            colorErrorBorder: brandColors.errorDark,
          },
          Message: {
            contentBg: brandColors.surface,
          },
          Notification: {
            colorBgElevated: brandColors.surface,
          },
        },
      }}
    >
      <MobileConfigProvider locale={enUS}>
        <div className="w-full h-full">
          {token ? <RouterProvider router={router} /> : <Login />}
        </div>
      </MobileConfigProvider>
    </ConfigProvider>
  );
};

export default App;

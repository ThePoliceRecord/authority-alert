import React from "react";
import { useMediaQuery } from "react-responsive";
import { Popup, Button } from "antd-mobile";
import { Button as AntdButton, Modal } from "antd";
import { PopupProps } from "antd-mobile/es/components/popup";
import { CloseOutline } from "antd-mobile-icons";

export interface MyPopupProps extends PopupProps {
  onCancel: () => void;
  title?: string;
  onConfirm?: () => void;
  okText?: string;
}

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.95)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
};

const CommonPopup: React.FC<MyPopupProps> = ({
  title = "",
  onConfirm,
  onCancel,
  okText = "Save",
  children,
  position = "bottom",
  ...restProps
}) => {
  const isMobile = useMediaQuery({ maxWidth: 767 });

  return isMobile ? (
    <Popup
      bodyStyle={{ 
        borderRadius: "16px 16px 0 0",
        ...translucentCardStyle,
      }}
      position={position}
      {...restProps}
    >
      <div className="p-24 pb-30">
        {/* Header */}
        <div className="flex items-center justify-between mb-20">
          <span className="text-18 font-semibold text-platinum">{title}</span>
          <div 
            onClick={onCancel} 
            className="w-32 h-32 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 cursor-pointer transition-colors"
          >
            <CloseOutline className="text-platinum text-16" />
          </div>
        </div>
        
        {/* Content */}
        <div className="text-platinum">
          {children}
        </div>
        
        {/* Footer */}
        {onConfirm && (
          <div className="mt-24">
            <Button 
              className="w-full font-medium" 
              color="primary"
              onClick={onConfirm}
              style={{
                backgroundColor: '#2328bb',
                borderColor: '#0065a3',
              }}
            >
              {okText}
            </Button>
          </div>
        )}
      </div>
    </Popup>
  ) : (
    <Modal
      title={
        <span className="text-18 font-semibold text-platinum">{title}</span>
      }
      centered
      open={restProps.visible}
      onCancel={onCancel}
      footer={
        onConfirm ? (
          <AntdButton 
            type="primary" 
            onClick={onConfirm}
            className="font-medium"
            style={{
              backgroundColor: '#2328bb',
              borderColor: '#0065a3',
            }}
          >
            {okText}
          </AntdButton>
        ) : null
      }
      styles={{
        content: translucentCardStyle,
        header: {
          backgroundColor: 'transparent',
          borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
          paddingBottom: '16px',
        },
        body: {
          backgroundColor: 'transparent',
          padding: '20px 24px',
        },
        footer: {
          backgroundColor: 'transparent',
          borderTop: '1px solid rgba(255, 255, 255, 0.1)',
          paddingTop: '16px',
        },
      }}
    >
      <div className="text-platinum">
        {children}
      </div>
    </Modal>
  );
};

export default CommonPopup;

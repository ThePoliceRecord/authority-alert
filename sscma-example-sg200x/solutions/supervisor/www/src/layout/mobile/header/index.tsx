import { useState, useRef } from "react";
import Sidebar from "../sidebar/index";
import CommonPopup from "@/components/common-popup";
import EditImg from "@/assets/images/svg/edit.svg";
import { Form, Input, Button } from "antd-mobile";
import { FormInstance } from "antd-mobile/es/components/form";
import useConfigStore from "@/store/config";
import { updateDeviceInfoApi, queryDeviceInfoApi } from "@/api/device/index";
import { hostnameValidate } from "@/utils/validate";

interface FormParams {
  deviceName: string;
}

function Header() {
  const [visible, setVisible] = useState(false);
  const formRef = useRef<FormInstance>(null);
  const { deviceInfo, updateDeviceInfo } = useConfigStore();

  const onQueryDeviceInfo = async () => {
    const res = await queryDeviceInfoApi();
    updateDeviceInfo(res.data);
  };
  const onFinish = async (values: FormParams) => {
    const deviceName = (values.deviceName || "").trim();
    await updateDeviceInfoApi({ deviceName });
    onCancel();
    await onQueryDeviceInfo();
  };
  const onCancel = () => {
    setVisible(false);
    resetFields();
  };
  const resetFields = () => {
    formRef.current?.resetFields();
  };

  return (
    <div
      className="text-center py-12"
      style={{
        backgroundColor: 'rgba(35, 40, 187, 0.95)',
        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
      }}
    >
      <div className="text-white text-18 font-semibold relative flex justify-center items-center px-40 pl-50">
        <div className="absolute left-0 -mt-4">
          <Sidebar />
        </div>
        <div className="truncate">
          <span className="text-white/70 text-14 font-normal">Camera Name: </span>
          {deviceInfo?.deviceName}
        </div>
        <div
          className="ml-6 p-4 rounded-full cursor-pointer hover:bg-white/30 transition-all duration-200 flex items-center justify-center"
          onClick={() => {
            setVisible(true);
          }}
          title="Edit Camera Name"
          style={{
            backgroundColor: 'rgba(255, 255, 255, 0.15)',
            border: '1px solid rgba(255, 255, 255, 0.3)',
          }}
        >
          <img
            className="w-20 h-20"
            src={EditImg}
            alt="Edit"
            style={{
              filter: 'brightness(0) invert(1)',
            }}
          />
        </div>
        <CommonPopup
          visible={visible}
          title={"Edit Camera Name"}
          onCancel={onCancel}
        >
          <Form
            requiredMarkStyle="none"
            onFinish={onFinish}
            initialValues={{ deviceName: deviceInfo?.deviceName }}
            footer={
              <Button
                block
                type="submit"
                color="primary"
                className="font-medium"
                style={{
                  backgroundColor: '#2328bb',
                  borderColor: '#0065a3',
                }}
              >
                Save
              </Button>
            }
          >
            <Form.Item
              name="deviceName"
              label={<span className="text-platinum">Name</span>}
              rules={[hostnameValidate(32)]}
            >
              <Input
                placeholder="recamera-132456"
                maxLength={32}
                clearable
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                }}
              />
            </Form.Item>
          </Form>
        </CommonPopup>
      </div>
    </div>
  );
}

export default Header;

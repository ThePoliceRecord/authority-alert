import {
  LockOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Button, Form, Input, Modal, message } from "antd";
import { useState } from "react";
import useUserStore from "@/store/user";
import { requiredTrimValidate, passwordRules } from "@/utils/validate";
import { loginApi, updateUserPasswordApi } from "@/api/user";

const Login = () => {
  const { updateUserInfo } = useUserStore();

  const [form] = Form.useForm();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [passwordErrorMsg, setPasswordErrorMsg] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleChangePassword = async () => {
    try {
      // Validate form first
      const fieldsValue = await form.validateFields();
      const oldpassword = fieldsValue.oldpassword;
      const newpassword = fieldsValue.newpassword;
      const confirmpassword = fieldsValue.confirmpassword;

      if (newpassword !== confirmpassword) {
        messageApi.error("New passwords do not match");
        return;
      }

      setLoading(true);
      const response = await updateUserPasswordApi({
        oldPassword: oldpassword,
        newPassword: newpassword,
      });

      if (response.code == 0) {
        messageApi.success("Password changed successfully");
        form.resetFields();
      } else {
        messageApi.error(response.msg || "Password change failed");
      }
    } catch (error: unknown) {
      // Form validation failed - error will have fields that failed
      if (error && typeof error === 'object' && 'errorFields' in error) {
        // This is a validation error from antd
        console.log("Form validation failed:", error);
      } else {
        messageApi.error("An error occurred");
      }
    } finally {
      setLoading(false);
    }
  };

  const loginAction = async (userName: string, password: string) => {
    try {
      const response = await loginApi({
        userName,
        password,
      });
      const code = response.code;
      const data = response.data;
      if (code === 0) {
        updateUserInfo({
          userName,
          password,
          token: data.token,
        });
        return { success: true };
      }
      // Unified error message
      let errorMsg = response.msg || "Login failed";
      if (code === -1 && data && typeof data.retryCount !== "undefined") {
        errorMsg =
          data.retryCount > 0
            ? `${data.retryCount} attempts remaining before temporary lock`
            : "Account locked - Please retry later";
        return { success: false, errorMsg, usePasswordErrorMsg: true };
      }
      return { success: false, errorMsg, usePasswordErrorMsg: false };
    } catch (error) {
      return {
        success: false,
        errorMsg: "Login failed",
        usePasswordErrorMsg: false,
      };
    }
  };

  const onFinish = async (values: { username: string; password: string }) => {
    const userName = values.username;
    const password = values.password;
    const result = await loginAction(userName, password);
    if (!result.success) {
      if (result.usePasswordErrorMsg) {
        setPasswordErrorMsg(result.errorMsg);
      } else {
        setPasswordErrorMsg(null);
        messageApi.error(result.errorMsg);
      }
    } else {
      setPasswordErrorMsg(null);
    }
  };

  return (
    <div 
      className="h-full flex flex-col justify-center items-center relative"
      style={{
        backgroundImage: `linear-gradient(rgba(0, 0, 0, 0.5), rgba(0, 0, 0, 0.5)), url('/brand/blueback.jpeg')`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
      }}
    >
      {/* Card Container - Translucent Grey Card */}
      <div
        className="rounded-lg p-32 w-full mx-16"
        style={{
          backgroundColor: 'rgba(31, 31, 27, 0.85)',
          boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
          maxWidth: '380px',
        }}
      >
        {/* Logo/Title */}
        <div className="flex justify-center mb-16">
          <h1 className="text-24 font-bold text-white tracking-wide">Authority Alert</h1>
        </div>
        
        {/* Welcome Text */}
        <div className="text-center mb-24">
          <h2 className="text-16 font-semibold text-white mb-4">Welcome Back</h2>
          <p className="text-12 text-platinum leading-relaxed opacity-80">
            Sign in to access your dashboard.
          </p>
        </div>
        
        {/* Login Form */}
        <Form
          className="w-full"
          name="login"
          layout="vertical"
          initialValues={{ username: "recamera" }}
          onFinish={onFinish}
        >
          <Form.Item
            name="username"
            label={<span className="text-platinum font-medium">Username</span>}
            rules={[
              {
                required: true,
                message: "Please input Username",
                whitespace: true,
              },
            ]}
          >
            <Input 
              prefix={<UserOutlined className="text-platinum opacity-60" />} 
              placeholder="Username"
              size="large"
              className="rounded-lg"
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>
          <Form.Item
            name="password"
            label={<span className="text-platinum font-medium">Password</span>}
            rules={[requiredTrimValidate()]}
            validateStatus={passwordErrorMsg ? "error" : undefined}
            help={passwordErrorMsg}
            extra={
              !passwordErrorMsg && (
                <span className="text-platinum opacity-60 text-12">
                  * First time login password is&nbsp;
                  <span className="font-bold text-cobalt">"recamera"</span>
                </span>
              )
            }
          >
            <Input.Password
              prefix={<LockOutlined className="text-platinum opacity-60" />}
              placeholder="Password"
              visibilityToggle
              size="large"
              className="rounded-lg"
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>
          <Form.Item className="w-full mb-0 mt-16" noStyle>
            <Button
              block
              type="primary"
              htmlType="submit"
              className="h-40 text-14 font-semibold rounded-lg"
              style={{
                backgroundColor: '#2328bb',
                borderColor: '#0065a3',
                boxShadow: '0 4px 6px rgba(3, 68, 255, 0.3)',
              }}
            >
              Login
            </Button>
          </Form.Item>
        </Form>
      </div>
      
      {/* Footer */}
      <div className="mt-24 text-platinum opacity-60 text-12">
        Powered by The Police Record
      </div>

      {/* Change Password Modal */}
      <Modal
        title={
          <span className="text-18 font-semibold text-white">Change Password</span>
        }
        open={false}
        closable={false}
        footer={
          <Button
            className="w-1/2 m-auto block h-44 text-15 font-medium"
            type="primary"
            loading={loading}
            onClick={handleChangePassword}
            style={{
              backgroundColor: '#2328bb',
              borderColor: '#0065a3',
            }}
          >
            Confirm
          </Button>
        }
        styles={{
          content: {
            backgroundColor: 'rgba(31, 31, 27, 0.95)',
            boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
          },
          header: {
            backgroundColor: 'transparent',
            borderBottom: '1px solid rgba(224, 224, 224, 0.2)',
          },
          body: {
            backgroundColor: 'transparent',
          },
          footer: {
            backgroundColor: 'transparent',
            borderTop: '1px solid rgba(224, 224, 224, 0.2)',
          },
        }}
      >
        <div className="mb-16 text-platinum opacity-80 text-14">
          For security reasons, please change your password on first login.
        </div>
        <Form
          form={form}
          name="dependencies"
          autoComplete="off"
          style={{ maxWidth: 600 }}
          layout="vertical"
        >
          <Form.Item
            name="oldpassword"
            label={<span className="text-platinum font-medium">Old Password</span>}
            rules={[requiredTrimValidate()]}
            extra={
              <span className="text-platinum opacity-60 text-12">
                For first time login, default password is&nbsp;
                <span className="font-bold text-cobalt">"recamera"</span>
              </span>
            }
          >
            <Input.Password 
              placeholder="recamera" 
              visibilityToggle 
              size="large"
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>
          <Form.Item
            name="newpassword"
            label={<span className="text-platinum font-medium">New Password</span>}
            rules={passwordRules}
          >
            <Input.Password
              placeholder="Enter new password here"
              visibilityToggle
              size="large"
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>

          <Form.Item
            name="confirmpassword"
            label={<span className="text-platinum font-medium">Confirm New Password</span>}
            dependencies={["newpassword"]}
            rules={[
              {
                required: true,
              },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue("newpassword") === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(
                    new Error("The new password that you entered do not match!")
                  );
                },
              }),
            ]}
          >
            <Input.Password
              placeholder="Confirm new password"
              visibilityToggle
              size="large"
              style={{
                backgroundColor: 'rgba(255, 255, 255, 0.1)',
                borderColor: 'rgba(224, 224, 224, 0.3)',
                color: '#e0e0e0',
              }}
            />
          </Form.Item>
        </Form>
      </Modal>
      {messageContextHolder}
    </div>
  );
};

export default Login;

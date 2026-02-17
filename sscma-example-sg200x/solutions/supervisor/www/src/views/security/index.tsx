import CommonPopup from "@/components/common-popup";
import { Button, Form, Input, Switch, Empty, Alert, Modal } from "antd";
import KeyImg from "@/assets/images/svg/key.svg";
import { DeleteOutlined, UserOutlined, LockOutlined, SafetyCertificateOutlined, KeyOutlined, PlusOutlined, CloseOutlined, WifiOutlined, SyncOutlined, CheckCircleOutlined, ExclamationCircleOutlined } from "@ant-design/icons";
import { useData, IFormTypeEnum } from "./hook";
import moment from "moment";
import {
  requiredTrimValidate,
  publicKeyValidate,
  passwordRules,
} from "@/utils/validate";
import { useEffect, useState } from "react";

const titleObj = {
  [IFormTypeEnum.Key]: "Add new SSH Key",
  [IFormTypeEnum.Username]: "Edit Username",
  [IFormTypeEnum.Password]: "Confirm Password",
  [IFormTypeEnum.DelKey]: "Remove SSH Key",
};

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

const Security = () => {
  const {
    state,
    formRef,
    passwordFormRef,
    usernameFormRef,
    onCancel,
    onEdit,
    addSshKey,
    onPasswordFinish,
    onUsernameFinish,
    onDelete,
    onAddSshFinish,
    onDeleteFinish,
    setSShStatus,
    cancelCodeRegistration,
    handleReRegister,
  } = useData();

  const handleSShStatusChange = (checked: boolean) => {
    setSShStatus(checked);
  };

  // Calculate countdown for code expiry
  const [countdown, setCountdown] = useState<string>('');
  useEffect(() => {
    if (!state.codeRegStatus?.expires_at) {
      setCountdown('');
      return;
    }

    const updateCountdown = () => {
      const expiresAt = new Date(state.codeRegStatus!.expires_at!);
      const now = new Date();
      const remaining = Math.max(0, Math.floor((expiresAt.getTime() - now.getTime()) / 1000));

      if (remaining <= 0) {
        setCountdown('Expired');
        return;
      }

      const minutes = Math.floor(remaining / 60);
      const seconds = remaining % 60;
      setCountdown(`${minutes}:${seconds.toString().padStart(2, '0')}`);
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);
    return () => clearInterval(interval);
  }, [state.codeRegStatus?.expires_at]);

  // Check if we have an active registration
  const hasActiveRegistration = state.codeRegStatus?.status === 'active' || state.codeRegStatus?.status === 'generating';

  return (
    <div className="p-16">
      {/* Page Header */}
      <div className="mb-24">
        <div className="flex items-center gap-12 mb-8">
          <SafetyCertificateOutlined style={{ fontSize: 28, color: '#9be564' }} />
          <h1 className="text-28 font-bold text-platinum m-0">Security</h1>
        </div>
        <p className="text-14 text-platinum/60 mt-8">
          Manage your account credentials and SSH access
        </p>
      </div>

      {/* Pending Registration Card */}
      {hasActiveRegistration && (
        <div className="mb-24">
          <div className="p-20" style={{
            ...translucentCardStyle,
            borderLeft: '4px solid #9be564',
          }}>
            <div className="flex items-start justify-between">
              <div className="flex items-center">
                <div className="w-48 h-48 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(155, 229, 100, 0.2)' }}>
                  <SyncOutlined spin style={{ fontSize: 24, color: '#9be564' }} />
                </div>
                <div>
                  <div className="text-18 font-bold text-platinum mb-4">Pending Registration</div>
                  <div className="text-14 text-platinum/70">
                    {state.codeRegStatus?.message || 'Waiting for claim...'}
                  </div>
                </div>
              </div>
              <Button
                type="text"
                danger
                icon={<CloseOutlined />}
                onClick={cancelCodeRegistration}
                loading={state.codeRegLoading}
              >
                Cancel
              </Button>
            </div>

            {/* Claim Code Display */}
            {state.codeRegStatus?.claim_code_formatted && (
              <div className="mt-16 p-16 rounded-12 text-center" style={{ backgroundColor: 'rgba(0, 0, 0, 0.3)' }}>
                <div className="text-12 text-platinum/50 uppercase tracking-wide mb-8">Registration Code</div>
                <div className="text-32 font-mono font-bold text-platinum tracking-widest">
                  {state.codeRegStatus.claim_code_formatted}
                </div>
                {countdown && (
                  <div className={`text-14 mt-8 ${countdown === 'Expired' ? 'text-red-400' : 'text-platinum/60'}`}>
                    {countdown === 'Expired' ? 'Code expired' : `Expires in ${countdown}`}
                  </div>
                )}
              </div>
            )}

            {/* Internet Warning */}
            {state.codeRegStatus?.internet_available === false && (
              <Alert
                type="warning"
                showIcon
                icon={<WifiOutlined />}
                message="No internet connection"
                description="Registration will continue automatically when connection is restored."
                className="mt-16"
                style={{ backgroundColor: 'rgba(255, 193, 7, 0.1)', border: '1px solid rgba(255, 193, 7, 0.3)' }}
              />
            )}

            {/* Retry Count */}
            {state.codeRegStatus?.retry_count && state.codeRegStatus.retry_count > 0 && (
              <div className="text-12 text-platinum/50 mt-12">
                Retry attempt: {state.codeRegStatus.retry_count}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Platform Registration Section */}
      {!hasActiveRegistration && (
        <div className="mb-24">
          <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
            Platform Registration
          </div>
          <div className="p-20" style={translucentCardStyle}>
            {/* Loading state */}
            {state.platformInfoLoading && (
              <div className="flex items-center justify-center py-8">
                <SyncOutlined spin style={{ fontSize: 24, color: '#9be564' }} />
                <span className="ml-12 text-platinum/60">Loading registration status...</span>
              </div>
            )}

            {/* Registered state */}
            {!state.platformInfoLoading && state.platformInfo?.tpr_camera_id && (
              <div>
                <div className="flex items-center mb-16">
                  <div className="w-48 h-48 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(155, 229, 100, 0.2)' }}>
                    <CheckCircleOutlined style={{ fontSize: 24, color: '#9be564' }} />
                  </div>
                  <div>
                    <div className="text-18 font-bold text-platinum">Registered</div>
                    <div className="text-14 text-platinum/60">Camera is connected to the platform</div>
                  </div>
                </div>

                <div className="space-y-12 mb-20 pl-4">
                  {state.platformInfo.platform_url && (
                    <div className="flex items-start">
                      <div className="text-12 text-platinum/50 uppercase tracking-wide w-100 flex-shrink-0">Platform</div>
                      <a
                        href={state.platformInfo.platform_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-14 text-primary hover:underline break-all"
                      >
                        {state.platformInfo.platform_url}
                      </a>
                    </div>
                  )}
                  <div className="flex items-start">
                    <div className="text-12 text-platinum/50 uppercase tracking-wide w-100 flex-shrink-0">Camera ID</div>
                    <div className="text-14 text-platinum font-mono">{state.platformInfo.tpr_camera_id}</div>
                  </div>
                  {state.platformInfo.location_name && (
                    <div className="flex items-start">
                      <div className="text-12 text-platinum/50 uppercase tracking-wide w-100 flex-shrink-0">Location</div>
                      <div className="text-14 text-platinum">{state.platformInfo.location_name}</div>
                    </div>
                  )}
                  {state.platformInfo.registered_at && (
                    <div className="flex items-start">
                      <div className="text-12 text-platinum/50 uppercase tracking-wide w-100 flex-shrink-0">Registered</div>
                      <div className="text-14 text-platinum">
                        {moment(state.platformInfo.registered_at).format("MMMM DD, YYYY [at] h:mm A")}
                      </div>
                    </div>
                  )}
                </div>

                <Button
                  type="default"
                  loading={state.reRegisterLoading}
                  onClick={() => {
                    Modal.confirm({
                      title: 'Re-register Camera',
                      content: (
                        <div>
                          <p>This will re-register the camera with the platform. Use this if:</p>
                          <ul className="list-disc pl-20 mt-8">
                            <li>The camera was moved to a different location</li>
                            <li>You need to transfer ownership to a different user</li>
                            <li>There are connection issues with the platform</li>
                          </ul>
                          <p className="mt-12 text-amber-500">The camera will generate a new claim code that must be entered on the platform.</p>
                        </div>
                      ),
                      okText: 'Re-register',
                      cancelText: 'Cancel',
                      onOk: handleReRegister,
                    });
                  }}
                >
                  Re-register Camera
                </Button>
              </div>
            )}

            {/* Not registered state */}
            {!state.platformInfoLoading && !state.platformInfo?.tpr_camera_id && (
              <div className="flex items-center">
                <div className="w-48 h-48 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(255, 193, 7, 0.2)' }}>
                  <ExclamationCircleOutlined style={{ fontSize: 24, color: '#ffc107' }} />
                </div>
                <div>
                  <div className="text-18 font-bold text-platinum">Not Registered</div>
                  <div className="text-14 text-platinum/60">
                    Complete the initial setup to register this camera with the platform
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* User Account Section */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">Account</div>
        <div className="p-20" style={translucentCardStyle}>
          {/* Username Row */}
          <div className="flex items-center justify-between pb-16 border-b border-white/10">
            <div className="flex items-center">
              <div className="w-40 h-40 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                <UserOutlined style={{ fontSize: 18, color: '#9be564' }} />
              </div>
              <div>
                <div className="text-12 text-platinum/50 uppercase tracking-wide">Username</div>
                <div className="text-16 font-medium text-platinum mt-2">{state.username}</div>
              </div>
            </div>
          </div>
          
          {/* Password Row */}
          <div className="flex items-center justify-between pt-16">
            <div className="flex items-center">
              <div className="w-40 h-40 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)' }}>
                <LockOutlined style={{ fontSize: 18, color: '#9be564' }} />
              </div>
              <div>
                <div className="text-12 text-platinum/50 uppercase tracking-wide">Password</div>
                <div className="text-14 text-platinum/60 mt-2">••••••••</div>
              </div>
            </div>
            <Button
              type="primary"
              onClick={() => onEdit(IFormTypeEnum.Password)}
            >
              Change Password
            </Button>
          </div>
        </div>
      </div>

      {/* SSH Section */}
      <div className="mb-24">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">SSH Access</div>
        
        {/* SSH Enable Toggle Card */}
        <div className="p-20 mb-16" style={translucentCardStyle}>
          <div className="flex justify-between items-center">
            <div className="flex items-center">
              <div className="w-40 h-40 rounded-full flex items-center justify-center mr-16" style={{ backgroundColor: state.sshEnabled ? 'rgba(35, 40, 187, 0.3)' : 'rgba(224, 224, 224, 0.1)' }}>
                <KeyOutlined style={{ fontSize: 18, color: state.sshEnabled ? '#9be564' : '#e0e0e0' }} />
              </div>
              <div>
                <div className="text-16 font-medium text-platinum">SSH Server</div>
                <div className="text-12 text-platinum/50 mt-2">
                  {state.sshEnabled ? 'Remote access enabled' : 'Remote access disabled'}
                </div>
              </div>
            </div>
            <Switch checked={state.sshEnabled} onChange={handleSShStatusChange} />
          </div>
        </div>

        {/* SSH Keys List */}
        {state.sshEnabled && (
          <div className="p-20" style={translucentCardStyle}>
            <div className="flex justify-between items-center mb-16">
              <div className="text-14 text-platinum/70">Authorized Keys</div>
              <Button 
                type="primary" 
                size="small" 
                icon={<PlusOutlined />}
                onClick={addSshKey}
              >
                Add Key
              </Button>
            </div>
            
            {state.sshkeyList?.length ? (
              <div className="space-y-12">
                {state.sshkeyList.map((item, index) => (
                  <div
                    className="rounded-12 p-16 border border-white/10 hover:border-primary/30 transition-colors"
                    style={{ backgroundColor: 'rgba(255, 255, 255, 0.03)' }}
                    key={item.id || index}
                  >
                    <div className="flex items-start">
                      <div className="w-44 h-44 rounded-lg flex items-center justify-center mr-16 flex-shrink-0" style={{ backgroundColor: 'rgba(35, 40, 187, 0.2)' }}>
                        <img className="w-24 h-24 invert opacity-80" src={KeyImg} alt="" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex justify-between items-start">
                          <div>
                            <div className="text-16 font-medium text-platinum">{item.name}</div>
                            <div className="inline-block mt-4 px-8 py-2 rounded text-10 uppercase tracking-wide" style={{ backgroundColor: 'rgba(35, 40, 187, 0.3)', color: '#9be564' }}>
                              SSH Key
                            </div>
                          </div>
                          <Button
                            type="text"
                            danger
                            size="small"
                            icon={<DeleteOutlined />}
                            onClick={() => onDelete(item)}
                          >
                            Remove
                          </Button>
                        </div>
                        <div className="text-12 text-platinum/40 mt-12 font-mono break-all line-clamp-2">
                          {item.value}
                        </div>
                        <div className="text-11 text-platinum/40 mt-8">
                          Added {item.addTime && moment(item.addTime).format("MMMM DD, YYYY")}
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="py-16">
                <Empty 
                  description={
                    <div className="text-center">
                      <div className="text-platinum/50 mb-8">No SSH keys configured</div>
                      <div className="text-12 text-platinum/40">Add a public key to enable secure remote access</div>
                    </div>
                  }
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              </div>
            )}
          </div>
        )}
      </div>

      {/* Modals */}
      <CommonPopup
        visible={state.visible}
        title={titleObj[state.formType]}
        onCancel={onCancel}
      >
        {state.formType == IFormTypeEnum.Key && (
          <Form
            form={formRef}
            className="border-b-0"
            onFinish={onAddSshFinish}
            layout="vertical"
          >
            <Form.Item
              name="sshName"
              label={<span className="text-platinum">Key Name</span>}
              rules={[requiredTrimValidate()]}
            >
              <Input 
                placeholder="e.g., My MacBook Pro" 
                allowClear 
                maxLength={32}
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                }}
              />
            </Form.Item>
            <Form.Item
              name="sshKey"
              label={<span className="text-platinum">Public Key</span>}
              trigger="onChange"
              rules={[publicKeyValidate()]}
              extra={<span className="text-platinum/40 text-12">Begins with 'ssh-rsa', 'ssh-ed25519', or 'ssh-dss'</span>}
            >
              <Input.TextArea
                rows={6}
                placeholder="Paste your public key here..."
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                  fontFamily: 'monospace',
                }}
              />
            </Form.Item>
            <Form.Item className="mb-0 mt-24">
              <Button type="primary" block htmlType="submit">
                Add SSH Key
              </Button>
            </Form.Item>
          </Form>
        )}
        {state.formType == IFormTypeEnum.Password && (
          <Form
            form={passwordFormRef}
            className="border-b-0"
            onFinish={onPasswordFinish}
            layout="vertical"
          >
            <Form.Item
              name="oldPassword"
              label={<span className="text-platinum">Current Password</span>}
              rules={[requiredTrimValidate()]}
            >
              <Input.Password 
                placeholder="Enter current password" 
                allowClear 
                maxLength={16}
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                }}
              />
            </Form.Item>
            <Form.Item
              name="newPassword"
              label={<span className="text-platinum">New Password</span>}
              rules={passwordRules}
              extra={<span className="text-platinum/40 text-12">Must be at least 8 characters</span>}
            >
              <Input.Password 
                placeholder="Enter new password" 
                allowClear 
                maxLength={16}
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                }}
              />
            </Form.Item>
            <Form.Item className="mb-0 mt-24">
              <Button type="primary" block htmlType="submit">
                Update Password
              </Button>
            </Form.Item>
          </Form>
        )}
        {state.formType == IFormTypeEnum.Username && (
          <Form
            form={usernameFormRef}
            onFinish={onUsernameFinish}
            layout="vertical"
            initialValues={{
              username: state.username,
            }}
          >
            <Form.Item
              name="username"
              label={<span className="text-platinum">Username</span>}
              rules={[requiredTrimValidate()]}
            >
              <Input 
                placeholder="Enter username" 
                allowClear 
                maxLength={32}
                style={{
                  backgroundColor: 'rgba(255, 255, 255, 0.1)',
                  borderColor: 'rgba(224, 224, 224, 0.3)',
                  color: '#e0e0e0',
                }}
              />
            </Form.Item>
            <Form.Item className="mb-0 mt-24">
              <Button type="primary" block htmlType="submit">
                Update Username
              </Button>
            </Form.Item>
          </Form>
        )}
        {state.formType == IFormTypeEnum.DelKey && (
          <div>
            <div className="text-platinum/80 text-16 mb-8">
              Are you sure you want to remove this SSH key?
            </div>
            <div className="text-platinum/50 text-14 mb-24">
              This action cannot be undone. You will need to add the key again to restore access.
            </div>
            <Button type="primary" block danger onClick={onDeleteFinish}>
              Remove Key
            </Button>
          </div>
        )}
      </CommonPopup>
    </div>
  );
};

export default Security;

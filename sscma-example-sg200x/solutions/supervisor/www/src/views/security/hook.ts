import { useEffect, Reducer, useReducer, useRef, useCallback } from "react";
import { Form, message } from "antd";
import useConfigStore from "@/store/config";
import { encryptPassword } from "@/utils";
import moment from "moment";
import {
  getUserInfoApi,
  updateUserInfoApi,
  updateUserPasswordApi,
  setSShStatusApi,
  delSshKeyApi,
  addSshKeyApi,
} from "@/api/user";
import { getPlatformInfoApi, reRegisterCameraApi } from "@/api/device";
import useUserStore from "@/store/user";
import { supervisorRequest } from "@/utils/request";

interface FormParams {
  username: string;
  oldPassword: string;
  newPassword: string;
  sshName: string;
  sshKey: string;
}

interface PlatformInfo {
  platform_url?: string;
  secret_key?: string;
  camera_uid?: string;
  tpr_camera_id?: string;
  user_id?: number;
  registered_at?: string;
  location_name?: string;
}

// Code registration status from supervisor
interface CodeRegistrationStatus {
  status: string; // idle | generating | active | claimed | expired | error
  message?: string;
  claim_code?: string;
  claim_code_formatted?: string;
  expires_at?: string;
  started_at?: string;
  internet_available?: boolean;
  last_error?: string;
  retry_count?: number;
}

export enum IFormTypeEnum {
  Key = "Key",
  Username = "Username",
  Password = "Password",
  DelKey = "DelKey",
}
interface IInitialState {
  visible: boolean;
  formType: IFormTypeEnum;
  password: string;
  form: Partial<FormParams>;
  username: string;
  sshkeyList: ISshItem[];
  curSshInfo?: ISshItem;
  sshEnabled: boolean;
  // Code registration
  codeRegStatus: CodeRegistrationStatus | null;
  codeRegLoading: boolean;
  // Platform info
  platformInfo: PlatformInfo | null;
  platformInfoLoading: boolean;
  reRegisterLoading: boolean;
}
type ActionType = { type: "setState"; payload: Partial<IInitialState> };
const initialState: IInitialState = {
  visible: false,
  formType: IFormTypeEnum.Key,
  password: "",
  form: {},
  username: "",
  sshkeyList: [],
  sshEnabled: false,
  codeRegStatus: null,
  codeRegLoading: false,
  platformInfo: null,
  platformInfoLoading: false,
  reRegisterLoading: false,
};
function reducer(state: IInitialState, action: ActionType): IInitialState {
  switch (action.type) {
    case "setState":
      return {
        ...state,
        ...action.payload,
      };
    default:
      throw new Error();
  }
}
export function useData() {
  // ---状态管理
  const [state, stateDispatch] = useReducer<Reducer<IInitialState, ActionType>>(
    reducer,
    initialState
  );
  const { deviceInfo } = useConfigStore();
  const { updatePassword } = useUserStore();
  const setStates = (payload: Partial<IInitialState>) => {
    stateDispatch({ type: "setState", payload: payload });
  };
  const [passwordFormRef] = Form.useForm();
  const [usernameFormRef] = Form.useForm();
  const [formRef] = Form.useForm();

  const onQueryUserInfo = async () => {
    try {
      const { data } = await getUserInfoApi();
      setStates({
        username: data.userName,
        sshkeyList: data.sshkeyList,
        sshEnabled: data.sshEnabled || false,
      });
    } catch (err) {
      // 错误处理
      console.error("Failed to fetch user info:", err);
    }
  };
  const onCancel = () => {
    setStates({
      visible: false,
    });
    switch (state.formType) {
      case IFormTypeEnum.Key:
        formRef.resetFields();
        break;
      case IFormTypeEnum.Password:
        passwordFormRef.resetFields();
        break;
      case IFormTypeEnum.Username:
        usernameFormRef.resetFields();
        break;
    }
  };
  const onEdit = (type: IFormTypeEnum) => {
    setStates({
      visible: true,
      formType: type,
    });
  };

  const addSshKey = () => {
    setStates({
      visible: true,
      formType: IFormTypeEnum.Key,
    });
  };

  const onPasswordFinish = async (values: FormParams) => {
    const encryptedOldpassword = encryptPassword(values.oldPassword);
    const encryptedNewPassword = encryptPassword(values.newPassword || "");
    const response = await updateUserPasswordApi({
      oldPassword: encryptedOldpassword,
      newPassword: encryptedNewPassword,
    });
    if (response.code == 0) {
      updatePassword(encryptedNewPassword);
    }
    refresh();
  };
  const onAddSshFinish = async (values: FormParams) => {
    await addSshKeyApi({
      name: values.sshName,
      value: values.sshKey || "",
      time: moment().format("YYYY-MM-DD"),
    });
    refresh();
  };
  const refresh = () => {
    onCancel();
    onQueryUserInfo();
  };
  const onUsernameFinish = async (values: FormParams) => {
    await updateUserInfoApi({
      userName: values.username || "",
    });
    refresh();
  };

  const onDelete = (item?: ISshItem) => {
    setStates({
      visible: true,
      formType: IFormTypeEnum.DelKey,
      curSshInfo: item,
    });
  };
  const onDeleteFinish = async () => {
    await delSshKeyApi({ id: state.curSshInfo?.id || "" });
    onQueryUserInfo();
    setStates({
      visible: false,
    });
  };
  const setSShStatus = async (enabled: boolean) => {
    try {
      const response = await setSShStatusApi({ enabled });
      if (response.code === 0) {
        setStates({
          sshEnabled: enabled,
        });
      }
    } catch (error) {
      console.error("Failed to set SSH status:", error);
    }
  };

  // Code registration status polling
  const pollIntervalRef = useRef<NodeJS.Timeout | null>(null);

  const fetchCodeRegistrationStatus = useCallback(async () => {
    try {
      const response = await supervisorRequest<CodeRegistrationStatus>({
        url: '/api/deviceMgr/codeRegistrationStatus',
        method: 'GET',
      }, { catchs: true });

      if (response.code === 0 && response.data) {
        setStates({ codeRegStatus: response.data });

        // If claimed, show success and stop polling
        if (response.data.status === 'claimed') {
          message.success('Camera registered successfully!');
          stopCodeRegPolling();
          // Refresh page after a delay
          setTimeout(() => window.location.reload(), 2000);
        }
      }
    } catch (error) {
      console.error("Failed to fetch code registration status:", error);
    }
  }, []);

  const startCodeRegPolling = useCallback(() => {
    // Fetch immediately
    fetchCodeRegistrationStatus();
    // Then poll every 3 seconds
    if (!pollIntervalRef.current) {
      pollIntervalRef.current = setInterval(fetchCodeRegistrationStatus, 3000);
    }
  }, [fetchCodeRegistrationStatus]);

  const stopCodeRegPolling = useCallback(() => {
    if (pollIntervalRef.current) {
      clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = null;
    }
  }, []);

  const cancelCodeRegistration = async () => {
    try {
      setStates({ codeRegLoading: true });
      await supervisorRequest({
        url: '/api/deviceMgr/cancelCodeRegistration',
        method: 'POST',
      });
      stopCodeRegPolling();
      setStates({ codeRegStatus: null, codeRegLoading: false });
      message.success('Registration cancelled');
    } catch (error) {
      console.error("Failed to cancel code registration:", error);
      setStates({ codeRegLoading: false });
    }
  };

  // Platform info functions
  const fetchPlatformInfo = useCallback(async () => {
    try {
      setStates({ platformInfoLoading: true });
      const response = await getPlatformInfoApi();
      if (response.code === 0 && response.data?.platform_info) {
        const parsed = JSON.parse(response.data.platform_info) as PlatformInfo;
        setStates({ platformInfo: parsed, platformInfoLoading: false });
      } else {
        setStates({ platformInfo: null, platformInfoLoading: false });
      }
    } catch (error) {
      console.error("Failed to fetch platform info:", error);
      setStates({ platformInfo: null, platformInfoLoading: false });
    }
  }, []);

  const handleReRegister = async () => {
    try {
      setStates({ reRegisterLoading: true });
      const response = await reRegisterCameraApi();
      if (response.code === 0) {
        message.success('Re-registration initiated successfully');
        // Refresh platform info after a delay
        setTimeout(() => {
          fetchPlatformInfo();
        }, 2000);
      } else {
        message.error(response.message || 'Failed to re-register camera');
      }
    } catch (error) {
      console.error("Failed to re-register camera:", error);
      message.error('Failed to re-register camera');
    } finally {
      setStates({ reRegisterLoading: false });
    }
  };

  useEffect(() => {
    onQueryUserInfo();
    // Check for active code registration on mount
    fetchCodeRegistrationStatus();
    // Fetch platform info on mount
    fetchPlatformInfo();

    return () => {
      stopCodeRegPolling();
    };
  }, []);

  // Start polling if there's an active registration
  useEffect(() => {
    if (state.codeRegStatus?.status === 'active' || state.codeRegStatus?.status === 'generating') {
      startCodeRegPolling();
    }
    return () => {
      stopCodeRegPolling();
    };
  }, [state.codeRegStatus?.status]);
  return {
    state,
    setStates,
    formRef,
    deviceInfo,
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
    fetchPlatformInfo,
    handleReRegister,
  };
}

import axios, { AxiosRequestConfig, AxiosError } from "axios";
import { message } from "antd";
import { getToken, clearCurrentUser } from "@/store/user";
import { isDev } from "@/utils";

// Set baseURL based on environment
// In production, use empty string for relative URLs (uses same protocol as current page)
// In dev, use explicit HTTP URL
export const baseIP = isDev ? "http://192.168.42.1" : "";

// Device communication service
const supervisorService = axios.create({
  baseURL: baseIP,
});

// Set token
supervisorService.interceptors.request.use((config) => {
  config.headers.Authorization = getToken();
  return config;
});

interface BaseResponse<T> {
  code: number | string;
  data: T;
  msg?: string;
  errorcode?: number;
  message?: string;
  timestamp?: string;
}
interface OtherRequestConfig {
  catchs?: boolean;
}

const createSupervisorRequest = () => {
  return async <T>(
    config: AxiosRequestConfig,
    otherConfig: OtherRequestConfig = {
      catchs: false,
    }
  ): Promise<BaseResponse<T>> => {
    const { catchs } = otherConfig;

    return await new Promise((resolve, reject) => {
      console.log("supervisorRequest: Making request with config:", config);
      supervisorService.request<BaseResponse<T>>(config).then(
        (res) => {
          console.log("supervisorRequest: Axios response received:", res);
          console.log("supervisorRequest: res.status:", res.status);
          console.log("supervisorRequest: res.data:", res.data);
          console.log("supervisorRequest: res.data.code:", res.data?.code);
          console.log("supervisorRequest: res.data.data:", res.data?.data);
          
          if (catchs) {
            resolve(res.data);
          } else {
            if (res.data.code !== 0 && res.data.code !== "0") {
              message.error(res.data.msg || "request error");
              reject(res.data);
              return;
            }
            resolve(res.data);
          }
        },
        (err: AxiosError) => {
          console.error("supervisorRequest: Axios error:", err);
          console.error("supervisorRequest: err.response:", err.response);
          console.error("supervisorRequest: err.response?.data:", err.response?.data);
          
          // Authentication failed, re-login
          if (err.response?.status == 401) {
            clearCurrentUser();
            // Force redirect to login by resetting hash
            if (window.location.hash !== '#/') {
              window.location.hash = '#/';
            }
            message.error("Session expired. Please log in again.");
          } else {
            if (!catchs) {
              message.error(err.message || "request error");
            }
          }
          reject(err);
        }
      );
    });
  };
};

const supervisorRequest = createSupervisorRequest();

export default supervisorRequest;

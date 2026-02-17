import { supervisorRequest } from "@/utils/request";

// LED information interface
export interface LEDInfo {
  name: string;
  brightness: number;
  max_brightness: number;
  trigger: string;
}

// Get all LEDs
export const getLEDsApi = async () => {
  console.log("getLEDsApi: Making request to api/ledMgr/getLEDs");
  const result = await supervisorRequest<{ leds: LEDInfo[] }>({
    url: "api/ledMgr/getLEDs",
    method: "get",
  });
  console.log("getLEDsApi: supervisorRequest returned:", result);
  console.log("getLEDsApi: result.data:", result?.data);
  console.log("getLEDsApi: result.data?.leds:", (result?.data as any)?.leds);
  return result;
};

// Get specific LED
export const getLEDApi = async (name: string) =>
  supervisorRequest<LEDInfo>({
    url: `api/ledMgr/getLED?name=${encodeURIComponent(name)}`,
    method: "get",
  });

// Set LED brightness and/or trigger
export const setLEDApi = async (data: {
  name: string;
  brightness?: number;
  trigger?: string;
}) =>
  supervisorRequest<{ name: string; status: string }>({
    url: "api/ledMgr/setLED",
    method: "post",
    data,
  });

// Get available LED triggers
export const getLEDTriggersApi = async (name: string) =>
  supervisorRequest<{ triggers: string[]; current: string }>({
    url: `api/ledMgr/getLEDTriggers?name=${encodeURIComponent(name)}`,
    method: "get",
  });

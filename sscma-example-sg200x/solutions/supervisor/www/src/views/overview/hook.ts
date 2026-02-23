import { useState, useEffect, useRef } from "react";
import useWebSocket, { ReadyState } from "react-use-websocket";
import { getWebSocketUrlApi } from "@/api";
import { getToken } from "@/store/user";
import jmuxer from "jmuxer";
import { detectPlayerMode, WebCodecsPlayer, type PlayerMode } from "./WebCodecsPlayer";

let jmuxerIns: any;

interface ChannelInfo {
  id: number;
  name: string;
  resolution: string;
  fps: number;
  bitrate: string;
  codec: string;
  use_case: string;
  websocket_url: string;
  shm_path: string;
  active: boolean;
}

interface ChannelsResponse {
  channels: ChannelInfo[];
  default_channel: number;
}

export default function usehookData() {
  const [channels, setChannels] = useState<ChannelInfo[]>([]);
  const [selectedChannel, setSelectedChannel] = useState<number>(1); // Default to CH1
  const [channelsLoaded, setChannelsLoaded] = useState<boolean>(false);
  const [socketUrl, setSocketUrl] = useState<string | null>(null);
  const [timeObj, setTimeObj] = useState({
    time: 0,
    delay: 0,
  });
  const [playerMode, setPlayerMode] = useState<PlayerMode>("mse");

  const reconnectAttempts = useRef(0);
  const maxReconnectAttempts = 10;
  const reconnectTimeoutRef = useRef<any>(null);
  const videoElementRef = useRef<HTMLVideoElement | null>(null);
  const bufferCheckIntervalRef = useRef<any>(null);
  const playerModeRef = useRef<PlayerMode>("mse");
  const webCodecsPlayerRef = useRef<WebCodecsPlayer | null>(null);

  const { getWebSocket, readyState } = useWebSocket(socketUrl, {
    onMessage,
    shouldReconnect: () => true,
    reconnectAttempts: maxReconnectAttempts,
    reconnectInterval: (attemptNumber) => {
      // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
      return Math.min(1000 * Math.pow(2, attemptNumber), 30000);
    },
    onOpen: () => {
      console.log('WebSocket connected');
      reconnectAttempts.current = 0;
    },
    onClose: () => {
      console.log('WebSocket disconnected');
    },
    onError: (error) => {
      console.error('WebSocket error:', error);
    },
  });

  // Detect player mode once on mount
  useEffect(() => {
    const mode = detectPlayerMode();
    console.log("Detected player mode:", mode);
    setPlayerMode(mode);
    playerModeRef.current = mode;
  }, []);

  // Fetch channel list from API
  useEffect(() => {
    const token = getToken();
    fetch('/api/channels', {
      headers: {
        'Authorization': token || '',
      }
    })
      .then(res => res.json())
      .then((response: any) => {
        // Handle standard API response format: {code, msg, data}
        if (response.code === 0 && response.data) {
          const data: ChannelsResponse = response.data;
          setChannels(data.channels || []);
          setSelectedChannel(data.default_channel || 1);
          setChannelsLoaded(true);
        } else {
          console.error('API error:', response.msg);
          setSelectedChannel(1);
          setChannelsLoaded(true);
        }
      })
      .catch(err => {
        console.error('Failed to fetch channels:', err);
        // Fallback to default if API fails
        setSelectedChannel(1);
        setChannelsLoaded(true);
      });
  }, []);

  // Get WebSocket URL with channel parameter
  const getWebsocketUrl = async () => {
    try {
      const { data } = await getWebSocketUrlApi({
        time: Date.now(),
      });

      // Supervisor's camera WebSocket URL is already configured
      // Append channel parameter to the URL
      let wsUrl = data.websocketUrl;

      // Check if URL already has query parameters
      const separator = wsUrl.includes('?') ? '&' : '?';
      wsUrl = wsUrl + separator + 'channel=' + selectedChannel;

      setSocketUrl(wsUrl);

      // Set binary type after connection
      setTimeout(() => {
        const obj: any = getWebSocket();
        if (obj) {
          obj.binaryType = "arraybuffer";
        }
      }, 0);
    } catch (err) {
      console.log("err:", err);
      // Retry connection after delay
      if (reconnectAttempts.current < maxReconnectAttempts) {
        reconnectAttempts.current++;
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current), 30000);
        console.log(`Retrying connection in ${delay}ms (attempt ${reconnectAttempts.current})`);
        reconnectTimeoutRef.current = setTimeout(() => {
          getWebsocketUrl();
        }, delay);
      }
    }
  };

  // Monitor video buffer and keep at live edge (MSE only)
  useEffect(() => {
    if (playerModeRef.current !== "mse") return;

    const setupBufferMonitoring = () => {
      const video = document.getElementById('player') as HTMLVideoElement;
      videoElementRef.current = video;

      if (video) {
        // Clear any existing interval
        if (bufferCheckIntervalRef.current) {
          clearInterval(bufferCheckIntervalRef.current);
        }

        // Check buffer every 1 second
        bufferCheckIntervalRef.current = setInterval(() => {
          const videoEl = videoElementRef.current;
          if (!videoEl) return;

          // If video has buffered data
          if (videoEl.buffered.length > 0) {
            const bufferEnd = videoEl.buffered.end(videoEl.buffered.length - 1);
            const currentTime = videoEl.currentTime;
            const bufferDiff = bufferEnd - currentTime;

            // If we're more than 2 seconds behind the live edge, jump to latest
            if (bufferDiff > 2) {
              console.log(`Buffer lag detected: ${bufferDiff.toFixed(2)}s, jumping to live edge`);
              videoEl.currentTime = bufferEnd - 0.5; // Stay slightly before the end
            }

            // If buffer is too large (more than 5 seconds), clear old data
            if (bufferDiff > 5 && jmuxerIns) {
              console.log('Large buffer detected, clearing old frames');
              // The jmuxer clearBuffer option should handle this, but we can also seek
              videoEl.currentTime = bufferEnd - 0.5;
            }
          }
        }, 1000);

        // Handle waiting/stalling events
        video.addEventListener('waiting', () => {
          console.log('Video waiting for data...');
        });

        video.addEventListener('playing', () => {
          console.log('Video playing');
        });

        // Keep video muted and playing
        video.muted = true;
        if (video.paused) {
          video.play().catch((err: any) => console.log('Auto-play prevented:', err));
        }
      }
    };

    // Setup monitoring after a short delay to ensure video element exists
    const setupTimeout = setTimeout(setupBufferMonitoring, 500);

    return () => {
      clearTimeout(setupTimeout);
      if (bufferCheckIntervalRef.current) {
        clearInterval(bufferCheckIntervalRef.current);
      }
    };
  }, [selectedChannel, playerMode]);

  // Reconnect when selected channel changes (only after channels are loaded)
  useEffect(() => {
    if (channelsLoaded && selectedChannel !== null) {
      const mode = playerModeRef.current;

      // Destroy existing players
      if (jmuxerIns) {
        jmuxerIns.destroy();
        jmuxerIns = undefined;
      }
      if (webCodecsPlayerRef.current) {
        webCodecsPlayerRef.current.destroy();
        webCodecsPlayerRef.current = null;
      }

      // Find channel info for selected channel
      const channelInfo = channels.find(ch => ch.id === selectedChannel);
      const fps = channelInfo ? channelInfo.fps : 30;

      if (mode === "mse") {
        try {
          jmuxerIns = new jmuxer({
            debug: false,
            node: "player",
            mode: "video",
            flushingTime: 0,
            fps: fps,
            clearBuffer: true,
          });
        } catch (err) {
          console.error("jmuxer creation failed, falling back to unsupported:", err);
          setPlayerMode("unsupported");
          playerModeRef.current = "unsupported";
        }
      } else if (mode === "webcodecs") {
        // Defer canvas lookup so React has rendered it, and connect WS after player is ready
        setTimeout(() => {
          const canvas = document.getElementById("player-canvas") as HTMLCanvasElement;
          if (canvas) {
            try {
              webCodecsPlayerRef.current = new WebCodecsPlayer(canvas);
            } catch (err) {
              console.error("WebCodecsPlayer creation failed:", err);
              setPlayerMode("unsupported");
              playerModeRef.current = "unsupported";
              return;
            }
          }
          getWebsocketUrl();
        }, 0);
        return; // Don't call getWebsocketUrl() below — it's called inside setTimeout
      }
      // "unsupported" → no player created

      // Reconnect WebSocket with new channel (mse and unsupported paths)
      getWebsocketUrl();
    }
  }, [selectedChannel, channelsLoaded]);

  function onMessage(evt: MessageEvent) {
    const buffer = new Uint8Array(evt.data);

    // Extract channel ID (first byte)
    const channelId = buffer[0];

    // Extract timestamp (last 8 bytes)
    const lastEight = buffer.slice(-8);
    const dataView = new DataView(lastEight.buffer);
    const high = dataView.getUint32(4, true);
    const low = dataView.getUint32(0, true);
    const int64Value = (BigInt(high) << BigInt(32)) | BigInt(low);
    const time = Number(int64Value);

    setTimeObj({
      time: time,
      delay: Date.now() - time,
    });

    // Feed frame data (skip channel ID byte, exclude timestamp bytes)
    const frameData = buffer.subarray(1, buffer.length - 8);
    const mode = playerModeRef.current;

    if (mode === "mse" && jmuxerIns) {
      jmuxerIns.feed({
        video: frameData,
      });
    } else if (mode === "webcodecs" && webCodecsPlayerRef.current) {
      webCodecsPlayerRef.current.feed(frameData);
    }
  }

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (jmuxerIns) {
        jmuxerIns.destroy();
        jmuxerIns = undefined;
      }
      if (webCodecsPlayerRef.current) {
        webCodecsPlayerRef.current.destroy();
        webCodecsPlayerRef.current = null;
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (bufferCheckIntervalRef.current) {
        clearInterval(bufferCheckIntervalRef.current);
      }
    };
  }, []);

  // Function to switch channels
  const switchChannel = (channelId: number) => {
    setSelectedChannel(channelId);
  };

  return {
    timeObj,
    channels,
    selectedChannel,
    switchChannel,
    connectionState: readyState,
    playerMode,
  };
}

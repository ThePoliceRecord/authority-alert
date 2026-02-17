import { supervisorRequest } from "@/utils/request";

// Detection queue item
export interface DetectionQueueItem {
  id: string;
  class_label: string;
  confidence_score: number;
  timestamp: number;
  retry_count: number;
  tsa_status: string;
  image_path: string;
}

// Detection stats
export interface DetectionStats {
  queue_size: number;
  total_uploaded: number;
  total_failed: number;
  last_upload_time: number;
}

// Get detection statistics
export const getDetectionStatsApi = async () =>
  supervisorRequest<DetectionStats>({
    url: "api/detection/stats",
    method: "get",
  });

// Get detection queue items
export const getDetectionQueueApi = async () =>
  supervisorRequest<{
    items: DetectionQueueItem[];
    count: number;
  }>({
    url: "api/detection/queue",
    method: "get",
  });

// Get image URL for a detection
export const getDetectionImageUrl = (id: string) => {
  return `/api/detection/image?id=${encodeURIComponent(id)}`;
};

import { useEffect, useState, useCallback } from "react";
import { Button, Empty, Tag, Progress, Modal, Image } from "antd";
import {
  ReloadOutlined,
  CloudUploadOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
} from "@ant-design/icons";
import {
  getDetectionStatsApi,
  getDetectionQueueApi,
  getDetectionImageUrl,
  DetectionQueueItem,
  DetectionStats,
} from "@/api/detection";
import moment from "moment";

// Translucent card style
const translucentCardStyle = {
  backgroundColor: "rgba(31, 31, 27, 0.85)",
  boxShadow:
    "2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)",
  borderRadius: "12px",
};

const Detections = () => {
  const [stats, setStats] = useState<DetectionStats | null>(null);
  const [items, setItems] = useState<DetectionQueueItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [previewImage, setPreviewImage] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [statsRes, queueRes] = await Promise.all([
        getDetectionStatsApi(),
        getDetectionQueueApi(),
      ]);
      if (statsRes.code === 0) {
        setStats(statsRes.data);
      }
      if (queueRes.code === 0) {
        setItems(queueRes.data.items || []);
      }
    } catch (error) {
      console.error("Failed to fetch detection data:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    // Auto-refresh every 5 seconds
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const getTsaStatusTag = (status: string) => {
    switch (status) {
      case "signed":
        return (
          <Tag color="success" icon={<CheckCircleOutlined />}>
            Signed
          </Tag>
        );
      case "pending":
        return (
          <Tag color="processing" icon={<ClockCircleOutlined />}>
            Pending
          </Tag>
        );
      case "failed":
        return (
          <Tag color="error" icon={<ExclamationCircleOutlined />}>
            Failed
          </Tag>
        );
      case "skipped":
      default:
        return <Tag color="default">Skipped</Tag>;
    }
  };

  const extractImageId = (imagePath: string) => {
    // Extract filename without extension from path like /userdata/detection_queue/det_123_456.jpg
    const parts = imagePath.split("/");
    const filename = parts[parts.length - 1];
    return filename.replace(".jpg", "");
  };

  return (
    <div className="p-16">
      {/* Page Header */}
      <div className="mb-24">
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-12">
            <CloudUploadOutlined style={{ fontSize: 28, color: "#9be564" }} />
            <h1 className="text-28 font-bold text-platinum m-0">
              Detection Upload Queue
            </h1>
          </div>
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchData}
            loading={loading}
          >
            Refresh
          </Button>
        </div>
        <p className="text-14 text-platinum/60 mt-8">
          Monitor and manage detections waiting to be uploaded to the platform
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-16 mb-24">
        <div className="p-16" style={translucentCardStyle}>
          <div className="text-12 text-platinum/50 uppercase tracking-wide mb-4">
            Queue Size
          </div>
          <div className="text-28 font-bold text-platinum">
            {stats?.queue_size ?? 0}
          </div>
        </div>
        <div className="p-16" style={translucentCardStyle}>
          <div className="text-12 text-platinum/50 uppercase tracking-wide mb-4">
            Total Uploaded
          </div>
          <div className="text-28 font-bold text-primary">
            {stats?.total_uploaded ?? 0}
          </div>
        </div>
        <div className="p-16" style={translucentCardStyle}>
          <div className="text-12 text-platinum/50 uppercase tracking-wide mb-4">
            Total Failed
          </div>
          <div className="text-28 font-bold text-red-400">
            {stats?.total_failed ?? 0}
          </div>
        </div>
        <div className="p-16" style={translucentCardStyle}>
          <div className="text-12 text-platinum/50 uppercase tracking-wide mb-4">
            Last Upload
          </div>
          <div className="text-16 font-medium text-platinum">
            {stats?.last_upload_time
              ? moment.unix(stats.last_upload_time).fromNow()
              : "Never"}
          </div>
        </div>
      </div>

      {/* Queue Items */}
      <div className="mb-16">
        <div className="font-bold text-16 mb-12 text-platinum/70 uppercase tracking-wide">
          Pending Uploads ({items.length})
        </div>

        {items.length === 0 ? (
          <div className="p-40" style={translucentCardStyle}>
            <Empty
              description={
                <div className="text-center">
                  <div className="text-platinum/50 mb-8">
                    No detections in queue
                  </div>
                  <div className="text-12 text-platinum/40">
                    Detections will appear here when they are waiting to be
                    uploaded
                  </div>
                </div>
              }
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          </div>
        ) : (
          <div className="space-y-12">
            {items.map((item, index) => (
              <div
                key={item.id || index}
                className="p-16"
                style={translucentCardStyle}
              >
                <div className="flex items-start gap-16">
                  {/* Thumbnail */}
                  <div
                    className="w-80 h-60 rounded-8 overflow-hidden flex-shrink-0 cursor-pointer hover:opacity-80 transition-opacity"
                    style={{ backgroundColor: "rgba(0, 0, 0, 0.3)" }}
                    onClick={() =>
                      setPreviewImage(
                        getDetectionImageUrl(extractImageId(item.image_path))
                      )
                    }
                  >
                    <img
                      src={getDetectionImageUrl(extractImageId(item.image_path))}
                      alt={item.class_label}
                      className="w-full h-full object-cover"
                      onError={(e) => {
                        (e.target as HTMLImageElement).style.display = "none";
                      }}
                    />
                  </div>

                  {/* Details */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-8 mb-4">
                      <span className="text-16 font-medium text-platinum">
                        {item.class_label || "Unknown"}
                      </span>
                      <Tag color="blue">
                        {(item.confidence_score * 100).toFixed(1)}%
                      </Tag>
                      {getTsaStatusTag(item.tsa_status)}
                    </div>
                    <div className="text-12 text-platinum/50 mb-4">
                      Captured{" "}
                      {item.timestamp
                        ? moment.unix(item.timestamp).format("MMM DD, h:mm:ss A")
                        : "Unknown time"}
                    </div>
                    <div className="flex items-center gap-12">
                      <span className="text-12 text-platinum/40">
                        ID: {item.id}
                      </span>
                      {item.retry_count > 0 && (
                        <Tag color="warning" className="text-11">
                          Retry #{item.retry_count}
                        </Tag>
                      )}
                    </div>
                  </div>

                  {/* Preview Button */}
                  <Button
                    type="text"
                    icon={<EyeOutlined />}
                    onClick={() =>
                      setPreviewImage(
                        getDetectionImageUrl(extractImageId(item.image_path))
                      )
                    }
                  />
                </div>

                {/* Retry progress indicator */}
                {item.retry_count > 0 && (
                  <div className="mt-12">
                    <Progress
                      percent={Math.min(item.retry_count * 10, 100)}
                      status="exception"
                      size="small"
                      format={() => `${item.retry_count} retries`}
                    />
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Image Preview Modal */}
      <Modal
        open={!!previewImage}
        footer={null}
        onCancel={() => setPreviewImage(null)}
        width={800}
        centered
      >
        {previewImage && (
          <Image
            src={previewImage}
            alt="Detection preview"
            style={{ width: "100%" }}
            preview={false}
          />
        )}
      </Modal>
    </div>
  );
};

export default Detections;

import { useEffect, useState, useRef } from "react";
import { ServiceStatus } from "@/enum";
import { Progress, Alert } from "antd";
import { queryServiceStatusApi } from "@/api/device";
import gif from "@/assets/gif/loading.gif";

const totalDuration = 100 * 1000; // 100 seconds

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.9)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
};

const Loading = ({
  onServiceStatusChange,
}: {
  onServiceStatusChange?: (serviceStatus: ServiceStatus) => void;
}) => {
  const [serviceStatus, setServiceStatus] = useState<ServiceStatus>(
    ServiceStatus.STARTING
  );
  const serviceStatusRef = useRef(ServiceStatus.STARTING);
  const [progress, setProgress] = useState(0);
  const tryCount = useRef<number>(0);

  useEffect(() => {
    onServiceStatusChange?.(serviceStatus);
  }, [serviceStatus]);

  useEffect(() => {
    queryServiceStatus();
  }, []);

  const queryServiceStatus = async () => {
    tryCount.current = 0;
    setServiceStatus(ServiceStatus.STARTING);
    serviceStatusRef.current = ServiceStatus.STARTING;

    while (tryCount.current < 30) {
      try {
        const response = await queryServiceStatusApi();
        if (response.code === 0 && response.data) {
          const { sscmaNode, system, uptime = 0 } = response.data;
          if (
            sscmaNode === ServiceStatus.RUNNING &&
            system === ServiceStatus.RUNNING
          ) {
            setProgress(100);
            if (tryCount.current == 0) {
              setServiceStatus(ServiceStatus.RUNNING);
              serviceStatusRef.current = ServiceStatus.RUNNING;
            } else {
              setTimeout(() => {
                setServiceStatus(ServiceStatus.RUNNING);
                serviceStatusRef.current = ServiceStatus.RUNNING;
              }, 1000);
            }
            return;
          } else {
            let percentage = (uptime / totalDuration) * 100;
            percentage = percentage >= 100 ? 99.99 : percentage;
            setProgress(parseFloat(percentage.toFixed(2)));
          }
        }
        tryCount.current++;
        setServiceStatus(ServiceStatus.STARTING);
        serviceStatusRef.current = ServiceStatus.STARTING;
        await new Promise((resolve) => setTimeout(resolve, 5000));
      } catch (error) {
        tryCount.current++;
        console.error("Error querying service status:", error);
      }
    }
    setServiceStatus(ServiceStatus.FAILED);
    serviceStatusRef.current = ServiceStatus.FAILED;
  };

  return (
    <div 
      className="flex justify-center items-center absolute left-0 top-0 right-0 bottom-0 z-100"
      style={{
        backgroundImage: `linear-gradient(rgba(0, 0, 0, 0.5), rgba(0, 0, 0, 0.5)), url('/brand/blueback.jpeg')`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
      }}
    >
      {serviceStatus == ServiceStatus.STARTING && (
        <div className="w-full max-w-400 px-24">
          <div 
            className="rounded-2xl p-32"
            style={translucentCardStyle}
          >
            <img className="w-full rounded-lg" src={gif} alt="Loading" />
            {progress > 0 && (
              <Progress
                className="mt-24"
                percent={progress}
                strokeColor={{
                  '0%': '#2328bb',
                  '100%': '#9be564',
                }}
                trailColor="rgba(224, 224, 224, 0.3)"
              />
            )}
            <div className="text-15 text-platinum/80 text-center mt-16">
              Please wait for services to start, this may take a moment.
            </div>
          </div>
        </div>
      )}
      {serviceStatus == ServiceStatus.FAILED && (
        <div className="w-full max-w-500 px-24">
          <div 
            className="rounded-2xl p-32"
            style={translucentCardStyle}
          >
            <Alert
              message={
                <span className="text-18 font-semibold text-white">System Error</span>
              }
              description={
                <span className="text-platinum/80">
                  Looks like something went wrong with the system. Please check the system
                  and restart, or contact{" "}
                  <a
                    href="mailto:techsupport@seeed.io"
                    className="text-cobalt hover:text-blue-400 font-medium"
                  >
                    techsupport@seeed.io
                  </a>{" "}
                  for support.
                </span>
              }
              type="error"
              showIcon
              className="rounded-lg"
              style={{
                backgroundColor: 'rgba(115, 0, 1, 0.5)',
                borderColor: 'rgba(115, 0, 1, 0.8)',
              }}
            />
          </div>
        </div>
      )}
    </div>
  );
};

export default Loading;

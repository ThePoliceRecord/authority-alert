import { Outlet } from "react-router-dom";
import Network from "@/views/network";
import Init from "@/views/init";
import Overview from "@/views/overview";
import Security from "@/views/security";
import WebShell from "@/views/terminal";
import System from "@/views/system";
import Power from "@/views/power";
import Dashboard from "@/views/dashboard";
import Files from "@/views/files";
import Recording from "@/views/recording";
import LEDConfig from "@/views/led-config";
import Updates from "@/views/updates";
import About from "@/views/about";
import Detections from "@/views/detections";
import ConfigLayout from "@/layout/config";
import MainLayout from "@/layout/main";

const Routes = [
  {
    path: "/",
    element: (
      <MainLayout>
        <Outlet />
      </MainLayout>
    ),
    children: [
      {
        path: "",
        element: <Dashboard />,
      },
      {
        path: "/dashboard",
        element: <Dashboard />,
      },
    ],
  },
  {
    path: "/",
    element: (
      <ConfigLayout>
        <Outlet />
      </ConfigLayout>
    ),
    children: [
      {
        path: "init",
        element: <Init />,
      },
      {
        path: "files",
        element: <Files />,
      },
      {
        path: "network",
        element: <Network />,
      },
      {
        path: "overview",
        element: <Overview />,
      },
      {
        path: "security",
        element: <Security />,
      },
      {
        path: "terminal",
        element: <WebShell />,
      },
      {
        path: "system",
        element: <System />,
      },
      {
        path: "power",
        element: <Power />,
      },
      {
        path: "recording",
        element: <Recording />,
      },
      {
        path: "detections",
        element: <Detections />,
      },
      {
        path: "led-config",
        element: <LEDConfig />,
      },
      {
        path: "updates",
        element: <Updates />,
      },
      {
        path: "about",
        element: <About />,
      },
    ],
  },
];

export default Routes;

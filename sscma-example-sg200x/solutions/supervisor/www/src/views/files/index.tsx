import { useState, useEffect } from "react";
import {
  Button,
  Input,
  Upload,
  Modal,
  message,
  Spin,
  Breadcrumb,
  Dropdown,
  Empty,
  Radio,
  Progress,
} from "antd";
import { ExclamationCircleOutlined } from "@ant-design/icons";
import type {
  BreadcrumbProps,
  MenuProps,
  DropdownProps,
  RadioChangeEvent,
} from "antd";
import type { UploadFile, RcFile } from "antd/es/upload/interface";
import {
  FolderFilled,
  FileOutlined,
  VideoCameraOutlined,
  PictureOutlined,
  ExperimentOutlined,
  UploadOutlined,
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  DownloadOutlined,
  RightOutlined,
  DatabaseOutlined,
} from "@ant-design/icons";
import {
  listFiles,
  checkSdAvailable,
  makeDirectory,
  removeEntry,
  renameEntry,
  downloadAsBlob,
  uploadFiles,
  getStorageInfo,
  StorageType,
  FileListData,
  DirectoryEntry,
  FileEntry,
  UploadProgressInfo,
} from "@/api/files";
import { formatSDCardApi } from "@/api/device/index";
import { getToken } from "@/store/user";
import { baseIP } from "@/utils/supervisorRequest";

type SortField = "name" | "size" | "type" | "modified";
type SortDirection = "asc" | "desc";

interface SortConfig {
  field: SortField;
  direction: SortDirection;
}

const PROTECTED_ROOT_DIRS = ["Models", "Videos", "Images"];

const normalizePath = (path: string) => path.replace(/^\/+/, "");
const isProtectedRootDir = (name: string) =>
  PROTECTED_ROOT_DIRS.includes(normalizePath(name));

// Translucent card style (matching TPR.css .translucent-card-grey-1)
const translucentCardStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.85)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
  borderRadius: '12px',
};

// Modal styles (matching TPR.css .translucent-card-grey-1)
const modalContentStyle = {
  backgroundColor: 'rgba(31, 31, 27, 0.95)',
  boxShadow: '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
};

const modalStyles = {
  content: modalContentStyle,
  header: {
    backgroundColor: 'transparent',
    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
  },
  body: {
    backgroundColor: 'transparent',
  },
  footer: {
    backgroundColor: 'transparent',
    borderTop: '1px solid rgba(255, 255, 255, 0.1)',
  },
};

// Upload modal styles - improved readability with solid light background
const uploadModalContentStyle = {
  backgroundColor: '#ffffff',
  boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)',
  borderRadius: '12px',
};

const uploadModalStyles = {
  content: uploadModalContentStyle,
  header: {
    backgroundColor: '#ffffff',
    borderBottom: '1px solid #e0e0e0',
    color: '#333333',
  },
  body: {
    backgroundColor: '#ffffff',
  },
  footer: {
    backgroundColor: '#ffffff',
    borderTop: '1px solid #e0e0e0',
  },
};

// Format file size
const formatFileSize = (bytes: number | undefined): string => {
  if (!bytes) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
};

// Storage info interface
interface StorageInfo {
  total: number;
  used: number;
  available: number;
}

const Files = () => {
  // State management
  const [currentStorage, setCurrentStorage] = useState<StorageType>("local");
  const [currentPath, setCurrentPath] = useState<string>("");
  const [fileListData, setFileListData] = useState<FileListData | null>(null);
  const [loading, setLoading] = useState(false);
  const [sdCardAvailable, setSdCardAvailable] = useState(false);
  const [formatSDLoading, setFormatSDLoading] = useState(false);
  
  // Storage usage state
  const [storageInfo, setStorageInfo] = useState<StorageInfo>({
    total: 0,
    used: 0,
    available: 0,
  });

  // File operation state
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const sortConfig: SortConfig = {
    field: "name",
    direction: "asc",
  };
  const [uploadModalVisible, setUploadModalVisible] = useState(false);
  const [newFolderModalVisible, setNewFolderModalVisible] = useState(false);
  const [renameModalVisible, setRenameModalVisible] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [newFileName, setNewFileName] = useState("");
  const [uploadFileList, setUploadFileList] = useState<UploadFile[]>([]);

  // Upload progress state
  const [uploadProgress, setUploadProgress] = useState<{
    visible: boolean;
    progress: number;
    currentFile?: string;
    currentFileProgress?: number;
    totalFiles?: number;
    completedFiles?: number;
  }>({
    visible: false,
    progress: 0,
  });

  // Image preview state
  const [previewModalVisible, setPreviewModalVisible] = useState(false);
  const [previewImageUrl, setPreviewImageUrl] = useState("");
  const [previewFileName, setPreviewFileName] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);

  // Navigation history
  const [navigationHistory, setNavigationHistory] = useState<
    Array<{ storage: StorageType; path: string }>
  >([]);
  const [historyIndex, setHistoryIndex] = useState(-1);

  // Initialize
  useEffect(() => {
    checkSdCardAvailability();
    // Load local storage root directory by default
    loadFileList("local", "");
    // Load storage info
    loadStorageInfo("local");
  }, []);

  // Load storage info
  const loadStorageInfo = async (storage: StorageType) => {
    try {
      const info = await getStorageInfo(storage);
      setStorageInfo({
        total: info.total,
        used: info.used,
        available: info.available,
      });
    } catch (error) {
      console.error("Failed to load storage info:", error);
      // Keep existing values on error
    }
  };

  // Check SD card availability
  const checkSdCardAvailability = async () => {
    try {
      const available = await checkSdAvailable();
      setSdCardAvailable(available);
    } catch (error) {
      console.error("Failed to check SD card availability:", error);
      setSdCardAvailable(false);
    }
  };

  // Format SD Card
  const handleFormatSDCard = () => {
    Modal.confirm({
      title: <span className="text-platinum">Format SD Card</span>,
      icon: <ExclamationCircleOutlined style={{ color: "#ff4d4f" }} />,
      content: (
        <div>
          <p className="text-platinum/80">This will format the SD card with exFAT filesystem.</p>
          <p className="text-red-500 font-bold mt-8">
            All data on the SD card will be permanently deleted!
          </p>
        </div>
      ),
      okText: "Format",
      okType: "danger",
      cancelText: "Cancel",
      centered: true,
      styles: modalStyles,
      onOk: async () => {
        setFormatSDLoading(true);
        try {
          const response = await formatSDCardApi();
          if (response.code === 0) {
            messageApi.success("SD card formatted successfully");
            // Refresh file list after formatting
            loadFileList(currentStorage, "");
          } else {
            messageApi.error(response.msg || "Failed to format SD card");
          }
        } catch (error) {
          messageApi.error("Failed to format SD card");
        } finally {
          setFormatSDLoading(false);
        }
      },
    });
  };

  // Message instance
  const [messageApi, messageContextHolder] = message.useMessage();

  // Load file list
  const loadFileList = async (storage: StorageType, path: string = "") => {
    setLoading(true);
    try {
      const data = await listFiles(storage, path);
      setFileListData(data);
      setCurrentStorage(storage);
      setCurrentPath(path);

      // Update navigation history
      const newHistory = [
        ...navigationHistory.slice(0, historyIndex + 1),
        { storage, path },
      ];
      setNavigationHistory(newHistory);
      setHistoryIndex(newHistory.length - 1);
    } catch (error) {
      console.error("Failed to load file list:", error);
      messageApi.error("Failed to load file list");
    } finally {
      setLoading(false);
    }
  };

  // Navigate to directory
  const navigateToDirectory = (dirName: string) => {
    const newPath = currentPath ? `${currentPath}/${dirName}` : dirName;
    loadFileList(currentStorage, newPath);
  };

  // Get sorted file list
  const getSortedItems = () => {
    if (!fileListData) return { directories: [], files: [] };

    const { directories, files } = fileListData;
    const allItems = [
      ...directories.map((dir) => ({ ...dir, type: "directory" as const })),
      ...files.map((file) => ({ ...file, type: "file" as const })),
    ];

    allItems.sort((a, b) => {
      const multiplier = sortConfig.direction === "asc" ? 1 : -1;
      switch (sortConfig.field) {
        case "name":
          return multiplier * a.name.localeCompare(b.name);
        case "size":
          return multiplier * ((a.size || 0) - (b.size || 0));
        case "type":
          return multiplier * a.type.localeCompare(b.type);
        case "modified":
          return multiplier * ((a.modified || 0) - (b.modified || 0));
        default:
          return 0;
      }
    });

    return {
      directories: allItems.filter((item) => item.type === "directory"),
      files: allItems.filter((item) => item.type === "file"),
    };
  };

  // Get file icon
  const getFileIcon = (filename: string, isDirectory: boolean) => {
    if (isDirectory) {
      return <FolderFilled style={{ color: "#9be564", fontSize: 24 }} />;
    }
    const ext = filename.split(".").pop()?.toLowerCase();
    switch (ext) {
      case "mp4":
      case "avi":
      case "mov":
        return <VideoCameraOutlined style={{ color: "#60a5fa", fontSize: 24 }} />;
      case "jpg":
      case "jpeg":
      case "png":
      case "gif":
        return <PictureOutlined style={{ color: "#f472b6", fontSize: 24 }} />;
      case "onnx":
      case "pth":
      case "pt":
        return <ExperimentOutlined style={{ color: "#fbbf24", fontSize: 24 }} />;
      default:
        return <FileOutlined style={{ color: "#e0e0e0", fontSize: 24 }} />;
    }
  };

  // Check if image file
  const isImageFile = (filename: string): boolean => {
    const ext = filename.split(".").pop()?.toLowerCase();
    return ["jpg", "jpeg", "png", "gif", "bmp", "webp"].includes(ext || "");
  };

  // Check if video file
  const isVideoFile = (filename: string): boolean => {
    const ext = filename.split(".").pop()?.toLowerCase();
    return ["mp4", "avi", "mov", "mkv", "wmv", "flv", "webm"].includes(
      ext || ""
    );
  };

  // Preview image
  const handleImagePreview = async (filename: string) => {
    if (!isImageFile(filename)) {
      messageApi.warning("This file is not an image");
      return;
    }

    const fullPath = currentPath ? `${currentPath}/${filename}` : filename;
    const token = getToken();
    const url = `/api/fileMgr/download?path=${encodeURIComponent(
      fullPath
    )}&storage=${encodeURIComponent(currentStorage)}&authorization=${token}`;
    const imageUrl = `${baseIP}${url}`;

    // Show loading state
    setPreviewLoading(true);
    setPreviewImageUrl("");
    setPreviewFileName(filename);
    setPreviewModalVisible(true);

    // Preload image
    const img = new Image();
    img.onload = () => {
      setPreviewImageUrl(imageUrl);
      setPreviewLoading(false);
    };
    img.onerror = () => {
      messageApi.error("Failed to load image");
      setPreviewLoading(false);
      setPreviewModalVisible(false);
    };
    img.src = imageUrl;
  };

  // Preview video
  const handleVideoPreview = async (filename: string) => {
    if (!isVideoFile(filename)) {
      messageApi.warning("This file is not a video");
      return;
    }

    const fullPath = currentPath ? `${currentPath}/${filename}` : filename;
    const token = getToken();
    
    // Check if file is being actively recorded
    try {
      const headUrl = `/api/fileMgr/download?path=${encodeURIComponent(
        fullPath
      )}&storage=${encodeURIComponent(currentStorage)}&authorization=${token}`;
      const response = await fetch(`${baseIP}${headUrl}`, { method: 'HEAD' });
      const fileStatus = response.headers.get('X-File-Status');
      
      if (fileStatus === 'recording') {
        messageApi.warning('This file is currently being recorded. Preview may show incomplete content.');
      }
    } catch (error) {
      console.warn('Failed to check file status:', error);
    }

    const url = `/api/fileMgr/download?path=${encodeURIComponent(
      fullPath
    )}&storage=${encodeURIComponent(currentStorage)}&authorization=${token}`;
    const videoUrl = `${baseIP}${url}`;
    setPreviewImageUrl(videoUrl);
    setPreviewFileName(filename);
    setPreviewModalVisible(true);
  };

  // Close media preview
  const handleCloseMediaPreview = () => {
    setPreviewModalVisible(false);
    setPreviewImageUrl("");
    setPreviewFileName("");
    setPreviewLoading(false);
  };

  // Build file browser breadcrumb
  const buildBrowserBreadcrumbItems = (): NonNullable<
    BreadcrumbProps["items"]
  > => {
    const pathParts = currentPath ? currentPath.split("/") : [];
    const rootTitle = currentStorage === "local" ? "/userdata" : "/mnt/sd";
    const items: NonNullable<BreadcrumbProps["items"]> = [
      {
        title: <span className="text-platinum hover:text-white cursor-pointer">{rootTitle}</span>,
        href: "#",
        onClick: (e: React.MouseEvent) => {
          e.preventDefault();
          // Return to storage root directory and refresh data
          setCurrentPath("");
          setSelectedFile(null);
          loadFileList(currentStorage, "");
        },
      },
      ...pathParts.map((part, index) => {
        const isLast = index === pathParts.length - 1;
        if (isLast) return { title: <span className="text-platinum">{part}</span> };
        const newPath = pathParts.slice(0, index + 1).join("/");
        return {
          title: <span className="text-platinum hover:text-white cursor-pointer">{part}</span>,
          href: "#",
          onClick: (e: React.MouseEvent) => {
            e.preventDefault();
            loadFileList(currentStorage, newPath);
          },
        };
      }),
    ];
    return items;
  };

  // Create new folder
  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) {
      messageApi.error("Please enter a folder name");
      return;
    }

    try {
      const fullPath = currentPath
        ? `${currentPath}/${newFolderName}`
        : newFolderName;
      await makeDirectory(currentStorage, fullPath);
      messageApi.success("Folder created successfully");
      setNewFolderModalVisible(false);
      setNewFolderName("");
      loadFileList(currentStorage, currentPath);
    } catch (error) {
      console.error("Failed to create folder:", error);
      messageApi.error("Failed to create folder");
    }
  };

  // Upload files
  const handleUpload = async () => {
    if (uploadFileList.length === 0) {
      messageApi.error("Please select files to upload");
      return;
    }

    try {
      // Convert uploadFileList to FileList
      const filesArray = uploadFileList
        .map((f) => f.originFileObj)
        .filter((f): f is RcFile => Boolean(f));

      const dataTransfer = new DataTransfer();
      filesArray.forEach((file) => dataTransfer.items.add(file));
      const fileList = dataTransfer.files;

      // Show upload progress
      setUploadProgress({
        visible: true,
        progress: 0,
        currentFile: "",
        currentFileProgress: 0,
        totalFiles: fileList.length,
        completedFiles: 0,
      });

      // Use chunked upload uniformly
      await uploadFiles(
        currentStorage,
        currentPath,
        fileList,
        (progressInfo: UploadProgressInfo) => {
          setUploadProgress((prev) => ({
            ...prev,
            currentFile: progressInfo.currentFileName,
            currentFileProgress: progressInfo.currentFileProgress,
            totalFiles: progressInfo.totalFiles,
            completedFiles: progressInfo.completedFiles,
          }));
        }
      );

      // Hide progress modal
      setUploadProgress({ visible: false, progress: 0 });
      messageApi.success("Files uploaded successfully");
      setUploadModalVisible(false);
      setUploadFileList([]);
      loadFileList(currentStorage, currentPath);
    } catch (error) {
      console.error("Failed to upload files:", error);
      messageApi.error("Failed to upload files");
      // Hide progress modal
      setUploadProgress({ visible: false, progress: 0 });
    }
  };

  // Delete file/folder
  const handleDelete = async (name: string, isDirectory: boolean) => {
    // Protection: protected folders in root cannot be deleted
    if (currentPath === "" && isDirectory && isProtectedRootDir(name)) {
      messageApi.warning("This directory cannot be deleted");
      return;
    }

    Modal.confirm({
      title: <span className="text-platinum">{`Delete ${isDirectory ? "Folder" : "File"}`}</span>,
      content: <span className="text-platinum/80">{`Are you sure you want to delete "${name}"? This action cannot be undone.`}</span>,
      okText: "Delete",
      okType: "danger",
      cancelText: "Cancel",
      styles: modalStyles,
      onOk: async () => {
        try {
          const fullPath = currentPath ? `${currentPath}/${name}` : name;
          await removeEntry(currentStorage, fullPath);
          messageApi.success("Deleted successfully");
          loadFileList(currentStorage, currentPath);
        } catch (error) {
          console.error("Failed to delete:", error);
          messageApi.error("Failed to delete");
        }
      },
    });
  };

  // Rename file/folder
  const handleRename = async () => {
    if (!selectedFile || !newFileName.trim()) {
      messageApi.error("Please enter a new name");
      return;
    }

    // Protection: protected folders in root cannot be renamed
    if (currentPath === "" && isProtectedRootDir(selectedFile)) {
      messageApi.warning("This directory cannot be renamed");
      return;
    }

    try {
      const oldPath = currentPath
        ? `${currentPath}/${selectedFile}`
        : selectedFile;
      const newPath = currentPath
        ? `${currentPath}/${newFileName}`
        : newFileName;
      await renameEntry(currentStorage, oldPath, newPath);
      messageApi.success("Renamed successfully");
      setRenameModalVisible(false);
      setNewFileName("");
      setSelectedFile(null);
      loadFileList(currentStorage, currentPath);
    } catch (error) {
      console.error("Failed to rename:", error);
      messageApi.error("Failed to rename");
    }
  };

  // Download file
  const handleDownload = async (name: string) => {
    try {
      setLoading(true);
      const fullPath = currentPath ? `${currentPath}/${name}` : name;
      const blob = await downloadAsBlob(currentStorage, fullPath);

      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = name;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      messageApi.success("Downloaded successfully");
    } catch (error) {
      console.error("Failed to download:", error);
      messageApi.error("Failed to download");
    } finally {
      setLoading(false);
    }
  };

  // File operation menu (Dropdown.menu)
  const buildFileMenu = (
    item: DirectoryEntry | FileEntry,
    isDirectory: boolean
  ): NonNullable<DropdownProps["menu"]> => {
    // Protection: protected folders in root cannot be deleted or renamed
    const disableModify =
      currentPath === "" && isDirectory && isProtectedRootDir(item.name);

    const items: MenuProps["items"] = [
      ...(isDirectory
        ? [{ key: "open", icon: <FolderFilled />, label: "Open" }]
        : [
            ...(isImageFile(item.name)
              ? [
                  {
                    key: "preview",
                    icon: <PictureOutlined />,
                    label: "Preview",
                  },
                ]
              : isVideoFile(item.name)
              ? [
                  {
                    key: "preview",
                    icon: <VideoCameraOutlined />,
                    label: "Preview",
                  },
                ]
              : []),
            { key: "download", icon: <DownloadOutlined />, label: "Download" },
          ]),
      {
        key: "rename",
        icon: <EditOutlined />,
        label: "Rename",
        disabled: disableModify,
      },
      {
        key: "delete",
        icon: <DeleteOutlined />,
        label: "Delete",
        danger: true,
        disabled: disableModify,
      },
    ];

    return {
      items,
      onClick: ({ key }) => {
        if (key === "open") {
          navigateToDirectory(item.name);
          return;
        }
        if (key === "preview") {
          if (isImageFile(item.name)) {
            handleImagePreview(item.name);
          } else if (isVideoFile(item.name)) {
            handleVideoPreview(item.name);
          }
          return;
        }
        if (key === "download") {
          handleDownload(item.name);
          return;
        }
        if (key === "rename") {
          if (disableModify) return;
          setSelectedFile(item.name);
          setNewFileName(item.name);
          setRenameModalVisible(true);
          return;
        }
        if (key === "delete") {
          if (disableModify) return;
          handleDelete(item.name, isDirectory);
        }
      },
    };
  };

  // Render list item (new list view)
  const renderListItem = (
    item: DirectoryEntry | FileEntry,
    isDirectory: boolean
  ) => {
    const selected = selectedFile === item.name;
    const contextMenu = buildFileMenu(item, isDirectory);

    const handleClick = () => setSelectedFile(item.name);
    const handleDoubleClick = () => {
      if (isDirectory) {
        navigateToDirectory(item.name);
      } else if (isImageFile(item.name)) {
        handleImagePreview(item.name);
      } else if (isVideoFile(item.name)) {
        handleVideoPreview(item.name);
      } else {
        handleDownload(item.name);
      }
    };

    return (
      <Dropdown key={item.name} menu={contextMenu} trigger={["contextMenu"]}>
        <div
          className={`flex items-center px-16 py-12 cursor-pointer transition-all duration-150 border-b border-white/10 ${
            selected
              ? "bg-primary/30"
              : "hover:bg-white/5"
          }`}
          onClick={handleClick}
          onDoubleClick={handleDoubleClick}
        >
          {/* Icon */}
          <div className="w-40 flex justify-center">
            {getFileIcon(item.name, isDirectory)}
          </div>
          
          {/* Name */}
          <div className="flex-1 ml-12 min-w-0">
            <div className={`text-15 truncate ${selected ? 'text-white font-medium' : 'text-platinum'}`}>
              {item.name}
            </div>
            {!isDirectory && (
              <div className="text-12 text-platinum/50 mt-2">
                {formatFileSize((item as FileEntry).size)}
              </div>
            )}
          </div>
          
          {/* Arrow for directories */}
          {isDirectory && (
            <RightOutlined className="text-platinum/40 text-12" />
          )}
        </div>
      </Dropdown>
    );
  };

  // Render file browser interface
  const renderFileBrowser = () => {
    const { directories, files } = getSortedItems();

    return (
      <div className="h-full flex flex-col">
        {/* Top area: breadcrumb and action buttons */}
        <div className="rounded-lg p-12 mb-16" style={translucentCardStyle}>
          <div className="flex items-center justify-between">
            <Breadcrumb items={buildBrowserBreadcrumbItems()} />
            <div className="flex items-center gap-8">
              <Button
                icon={<UploadOutlined />}
                onClick={() => setUploadModalVisible(true)}
                type="primary"
                size="small"
              >
                Upload
              </Button>
              <Button
                icon={<PlusOutlined />}
                onClick={() => setNewFolderModalVisible(true)}
                size="small"
              >
                New Folder
              </Button>
            </div>
          </div>
        </div>

        {/* File list - limited height to fit on screen */}
        <div className="overflow-auto rounded-lg" style={{ ...translucentCardStyle, maxHeight: '350px' }}>
          {loading ? (
            <div className="flex justify-center items-center h-full py-40">
              <Spin size="large" />
            </div>
          ) : directories.length === 0 && files.length === 0 ? (
            <div className="flex justify-center items-center h-full py-40">
              <Empty 
                description={<span className="text-platinum/70">This folder is empty</span>}
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            </div>
          ) : (
            <div>
              {/* Directories */}
              {directories.length > 0 && (
                <div>
                  <div className="px-16 py-8 text-12 text-platinum/50 uppercase tracking-wide border-b border-white/10">
                    Folders ({directories.length})
                  </div>
                  {directories.map((dir: DirectoryEntry) => renderListItem(dir, true))}
                </div>
              )}
              
              {/* Files */}
              {files.length > 0 && (
                <div>
                  <div className="px-16 py-8 text-12 text-platinum/50 uppercase tracking-wide border-b border-white/10">
                    Files ({files.length})
                  </div>
                  {files.map((file: FileEntry) => renderListItem(file, false))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Status bar */}
        <div className="mt-8 px-16 py-8 text-12 text-platinum/60 rounded-lg" style={{ backgroundColor: 'rgba(31, 31, 27, 0.5)' }}>
          <div className="flex justify-between items-center">
            <span>
              {directories.length + files.length} items
              {directories.length > 0 && ` • ${directories.length} folders`}
              {files.length > 0 && ` • ${files.length} files`}
            </span>
            {selectedFile && (
              <span className="text-platinum/80">Selected: {selectedFile}</span>
            )}
          </div>
        </div>
      </div>
    );
  };

  // Render storage selector with storage usage display
  const renderStorageSelector = () => {
    const handleStorageChange = (e: RadioChangeEvent) => {
      const newStorage = e.target.value as StorageType;
      setCurrentStorage(newStorage);
      setCurrentPath("");
      setSelectedFile(null);
      loadFileList(newStorage, "");
      // Load storage info for the new storage type
      loadStorageInfo(newStorage);
    };

    const radioOptions = [
      { label: "Local Files", value: "local" },
      ...(sdCardAvailable ? [{ label: "SD Card", value: "sd" }] : []),
    ];

    // Calculate storage usage percentage
    const usagePercent = storageInfo.total > 0
      ? Math.round((storageInfo.used / storageInfo.total) * 100)
      : 0;

    return (
      <div className="mb-16">
        <div className="flex items-center justify-between mb-12">
          <div className="flex items-center">
            {sdCardAvailable ? (
              <Radio.Group
                value={currentStorage}
                onChange={handleStorageChange}
                optionType="button"
                buttonStyle="solid"
                options={radioOptions}
              />
            ) : (
              <div className="text-20 font-bold text-platinum">Local Files</div>
            )}
          </div>
          {/* Format SD Card button - only show when SD card is selected */}
          {currentStorage === "sd" && sdCardAvailable && (
            <Button
              danger
              onClick={handleFormatSDCard}
              loading={formatSDLoading}
              size="small"
            >
              Format SD Card
            </Button>
          )}
        </div>
        
        {/* Storage Usage Display */}
        <div className="p-16 rounded-lg" style={translucentCardStyle}>
          <div className="flex items-center gap-12 mb-12">
            <DatabaseOutlined style={{ fontSize: 20, color: '#9be564' }} />
            <span className="text-14 font-medium text-platinum">
              Storage Usage ({currentStorage === "local" ? "Local" : "SD Card"})
            </span>
          </div>
          <Progress
            percent={usagePercent}
            strokeColor={{
              '0%': '#2328bb',
              '100%': usagePercent > 90 ? '#e3170a' : '#9be564',
            }}
            trailColor="rgba(255, 255, 255, 0.1)"
            showInfo={false}
          />
          <div className="flex justify-between mt-8 text-12">
            <span className="text-platinum/70">
              Used: <span className="text-platinum font-medium">{formatFileSize(storageInfo.used)}</span>
            </span>
            <span className="text-platinum/70">
              Total: <span className="text-platinum font-medium">{formatFileSize(storageInfo.total)}</span>
            </span>
          </div>
          <div className="text-center mt-4 text-11 text-platinum/50">
            {formatFileSize(storageInfo.available)} available ({100 - usagePercent}% free)
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="h-full p-16">
      {messageContextHolder}

      {/* Storage selector */}
      {renderStorageSelector()}

      {/* File browser */}
      {renderFileBrowser()}

      {/* Media Preview Modal */}
      <Modal
        title={<span className="text-platinum">{previewFileName || "Media Preview"}</span>}
        open={previewModalVisible}
        onCancel={handleCloseMediaPreview}
        footer={null}
        centered
        width={800}
        styles={modalStyles}
      >
        <div className="flex justify-center">
          {isImageFile(previewFileName) ? (
            previewLoading ? (
              <div className="flex flex-col justify-center items-center h-64">
                <Spin size="large" />
                <div className="mt-4 text-platinum/60">Loading image...</div>
              </div>
            ) : (
              <img
                src={previewImageUrl}
                alt={previewFileName}
                style={{
                  maxWidth: "100%",
                  maxHeight: "70vh",
                  objectFit: "contain",
                }}
                onError={(e) => {
                  console.error("Image load error:", e);
                  messageApi.error("Failed to display image");
                }}
              />
            )
          ) : isVideoFile(previewFileName) ? (
            <video
              src={previewImageUrl}
              controls
              autoPlay
              style={{
                maxWidth: "100%",
                maxHeight: "70vh",
                objectFit: "contain",
              }}
              onError={(e) => {
                console.error("Video load error:", e);
                messageApi.error("Failed to load video");
              }}
            >
              Your browser does not support the video tag.
            </video>
          ) : (
            <div className="text-center text-platinum/60">
              Unsupported file type for preview
            </div>
          )}
        </div>
      </Modal>

      {/* New Folder Modal */}
      <Modal
        title={<span className="text-platinum">New Folder</span>}
        open={newFolderModalVisible}
        onOk={handleCreateFolder}
        onCancel={() => {
          setNewFolderModalVisible(false);
          setNewFolderName("");
        }}
        okText="Create"
        cancelText="Cancel"
        styles={modalStyles}
      >
        <Input
          placeholder="Enter folder name"
          value={newFolderName}
          onChange={(e) => setNewFolderName(e.target.value)}
          onPressEnter={handleCreateFolder}
          style={{
            backgroundColor: 'rgba(255, 255, 255, 0.1)',
            borderColor: 'rgba(224, 224, 224, 0.3)',
            color: '#e0e0e0',
          }}
        />
      </Modal>

      {/* Upload Files Modal - Improved Readability */}
      <Modal
        title={<span style={{ color: '#333333', fontWeight: 600, fontSize: 18 }}>Upload Files</span>}
        open={uploadModalVisible}
        onOk={handleUpload}
        onCancel={() => {
          setUploadModalVisible(false);
          setUploadFileList([]);
        }}
        okText="Upload"
        cancelText="Cancel"
        confirmLoading={loading}
        styles={uploadModalStyles}
        okButtonProps={{
          style: {
            backgroundColor: '#2328bb',
            borderColor: '#2328bb',
            fontWeight: 600,
            fontSize: 15,
            height: 40,
            paddingLeft: 24,
            paddingRight: 24,
          }
        }}
        cancelButtonProps={{
          style: {
            fontWeight: 500,
            fontSize: 15,
            height: 40,
          }
        }}
      >
        <Upload
          fileList={uploadFileList}
          beforeUpload={() => false}
          onChange={({ fileList }) => setUploadFileList(fileList)}
          multiple
          className="upload-modal-list"
        >
          <Button
            icon={<UploadOutlined />}
            style={{
              backgroundColor: '#f5f5f5',
              borderColor: '#d9d9d9',
              color: '#333333',
              fontWeight: 500,
            }}
          >
            Select files
          </Button>
        </Upload>
        {uploadFileList.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <div style={{
              color: '#666666',
              fontSize: 13,
              marginBottom: 8,
              fontWeight: 500
            }}>
              Selected files ({uploadFileList.length}):
            </div>
            <div style={{
              maxHeight: 200,
              overflowY: 'auto',
              border: '1px solid #e0e0e0',
              borderRadius: 8,
              padding: 8,
              backgroundColor: '#fafafa',
            }}>
              {uploadFileList.map((file, index) => (
                <div
                  key={file.uid || index}
                  style={{
                    padding: '8px 12px',
                    borderBottom: index < uploadFileList.length - 1 ? '1px solid #e8e8e8' : 'none',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <FileOutlined style={{ color: '#666666', fontSize: 16 }} />
                  <span style={{
                    color: '#333333',
                    fontSize: 14,
                    flex: 1,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {file.name}
                  </span>
                  <span style={{ color: '#999999', fontSize: 12 }}>
                    {file.size ? formatFileSize(file.size) : ''}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </Modal>

      {/* Rename Modal */}
      <Modal
        title={<span className="text-platinum">Rename</span>}
        open={renameModalVisible}
        onOk={handleRename}
        onCancel={() => {
          setRenameModalVisible(false);
          setNewFileName("");
          setSelectedFile(null);
        }}
        okText="Rename"
        cancelText="Cancel"
        styles={modalStyles}
      >
        <Input
          placeholder="Enter a new name"
          value={newFileName}
          onChange={(e) => setNewFileName(e.target.value)}
          onPressEnter={handleRename}
          style={{
            backgroundColor: 'rgba(255, 255, 255, 0.1)',
            borderColor: 'rgba(224, 224, 224, 0.3)',
            color: '#e0e0e0',
          }}
        />
      </Modal>

      {/* Upload Progress Modal */}
      <Modal
        title={<span className="text-platinum">Upload Progress</span>}
        open={uploadProgress.visible}
        footer={null}
        closable={false}
        centered
        width={500}
        styles={modalStyles}
      >
        <div className="space-y-4">
          {/* Current file progress */}
          {uploadProgress.currentFile && (
            <div>
              <Progress
                percent={uploadProgress.currentFileProgress || 0}
                status="active"
                strokeColor="#2328bb"
              />
              <div className="text-xs text-platinum/60 mt-1 truncate">
                {uploadProgress.currentFile}
              </div>
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
};

export default Files;

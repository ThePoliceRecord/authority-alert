package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"supervisor/internal/api"
	"supervisor/internal/config"
)

// FileHandler handles file management API requests.
type FileHandler struct {
	localDir string
	sdDir    string
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler() *FileHandler {
	cfg := config.Get()
	return &FileHandler{
		localDir: cfg.LocalDir,
		sdDir:    cfg.SDDir,
	}
}

// isSDAvailable checks if SD card is available.
func (h *FileHandler) isSDAvailable() bool {
	info, err := os.Stat(h.sdDir)
	return err == nil && info.IsDir()
}

// isValidStorage validates the storage parameter.
func (h *FileHandler) isValidStorage(storage string) bool {
	return storage == "" || storage == "local" || storage == "sd"
}

// decodePath URL-decodes a path.
func decodePath(path string) string {
	decoded, err := url.QueryUnescape(path)
	if err != nil {
		return path
	}
	return decoded
}

// isValidPath validates a path to prevent directory traversal and hidden files.
func isValidPath(path string) bool {
	if path == "" {
		return true
	}
	if path == "." || path == ".." {
		return false
	}

	// Reject absolute paths
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return false
	}

	// Check for path traversal patterns including encoded versions
	// Check multiple times to catch double-encoding
	checkPath := path
	for i := 0; i < 3; i++ {
		// Decode URL encoding
		if decoded, err := url.QueryUnescape(checkPath); err == nil {
			checkPath = decoded
		}
		// Also check for backslash encoding
		checkPath = strings.ReplaceAll(checkPath, "%5c", "\\")
		checkPath = strings.ReplaceAll(checkPath, "%5C", "\\")
	}

	// Check for various traversal patterns
	if strings.Contains(checkPath, "..") {
		return false
	}
	if strings.Contains(checkPath, "\\") {
		return false
	}

	// Check for null bytes (path truncation attack)
	if strings.Contains(path, "\x00") {
		return false
	}

	// Check for hidden files or directories (starting with '.')
	cleanPath := strings.TrimPrefix(path, "/")
	for _, component := range strings.Split(cleanPath, "/") {
		if component != "" && strings.HasPrefix(component, ".") {
			return false
		}
	}

	return true
}

// isValidFilename validates a filename.
func isValidFilename(filename string) bool {
	if filename == "" || filename == "." || filename == ".." {
		return false
	}
	invalidChars := "<>:\"|?*\\"
	for _, c := range invalidChars {
		if strings.ContainsRune(filename, c) {
			return false
		}
	}
	return true
}

// getFullPath constructs the full path based on storage type.
// This function performs path traversal prevention by:
// 1. Resolving both base directory and target path to absolute paths
// 2. Verifying the resolved target path is within the base directory
// 3. Returning the base directory if any attack is detected
// The returned path is guaranteed to be within baseDir and cannot escape it.
func (h *FileHandler) getFullPath(relativePath, storage string) string {
	baseDir := h.localDir
	if storage == "sd" {
		baseDir = h.sdDir
	}

	// Resolve base directory to an absolute path
	baseDirAbs, err := filepath.Abs(baseDir)
	if err != nil {
		// If we cannot resolve, fall back to the original baseDir
		baseDirAbs = baseDir
	}

	// If no relative path is provided, return the (absolute) base directory
	if relativePath == "" {
		return baseDirAbs
	}

	// Clean the relative path to remove redundant separators and dots
	cleanRel := filepath.Clean(relativePath)

	// Join and resolve the target path
	targetPath := filepath.Join(baseDirAbs, cleanRel)
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		// On error resolving the target, return the base directory
		return baseDirAbs
	}

	// Ensure that the resulting path is within the base directory.
	// Use filepath.Rel to compute the relative path from base to target.
	// This is more robust than string prefix checking for detecting escapes.
	rel, err := filepath.Rel(baseDirAbs, targetAbs)
	if err != nil {
		// On error computing the relative path, constrain to base directory
		return baseDirAbs
	}

	// Reject any path that attempts to traverse outside the base directory.
	// If rel starts with "..", it means targetAbs is outside baseDirAbs.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		// Attempted directory traversal or escape; constrain to base directory
		return baseDirAbs
	}

	return targetAbs
}

// getSafePath returns a sanitized path that is guaranteed to be within the storage directory.
// This wrapper makes the validation explicit for static analysis tools.
// It combines input validation with path construction to prevent path traversal attacks.
func (h *FileHandler) getSafePath(path, storage string) (string, error) {
	// Validate the path first
	if !isValidPath(path) {
		return "", filepath.ErrBadPattern
	}

	// Get the full path with built-in traversal prevention
	fullPath := h.getFullPath(path, storage)

	// Verify we're still within expected directory (defense in depth)
	baseDir := h.localDir
	if storage == "sd" {
		baseDir = h.sdDir
	}

	baseDirAbs, _ := filepath.Abs(baseDir)
	if !strings.HasPrefix(fullPath, baseDirAbs) {
		return "", filepath.ErrBadPattern
	}

	return fullPath, nil
}

// jsonBodyCache stores parsed JSON body for a request
type jsonBodyCache struct {
	mu    sync.RWMutex
	cache map[*http.Request]map[string]interface{}
}

var jsonCache = &jsonBodyCache{
	cache: make(map[*http.Request]map[string]interface{}),
}

// getJSONBody parses and caches the JSON body of a request
func getJSONBody(r *http.Request) map[string]interface{} {
	jsonCache.mu.RLock()
	if cached, ok := jsonCache.cache[r]; ok {
		jsonCache.mu.RUnlock()
		return cached
	}
	jsonCache.mu.RUnlock()

	// Check if content type is JSON
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil
	}

	// Read and parse body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	// Cache the result
	jsonCache.mu.Lock()
	jsonCache.cache[r] = data
	jsonCache.mu.Unlock()

	return data
}

// clearJSONCache removes the cached JSON body for a request (call at end of handler)
func clearJSONCache(r *http.Request) {
	jsonCache.mu.Lock()
	delete(jsonCache.cache, r)
	jsonCache.mu.Unlock()
}

// getParam gets a parameter from URL query, form body, or JSON body.
func getParam(r *http.Request, name string) string {
	// Try URL query first
	val := r.URL.Query().Get(name)
	if val != "" {
		return val
	}

	// Try JSON body
	if jsonBody := getJSONBody(r); jsonBody != nil {
		if v, ok := jsonBody[name]; ok {
			switch vt := v.(type) {
			case string:
				return vt
			case float64:
				return strconv.FormatFloat(vt, 'f', -1, 64)
			case int:
				return strconv.Itoa(vt)
			case bool:
				return strconv.FormatBool(vt)
			}
		}
	}

	// For POST requests, try form value
	if r.Method == http.MethodPost {
		return r.FormValue(name)
	}
	return ""
}

// List lists directory contents.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter. Use 'local' or 'sd'.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath, err := h.getSafePath(path, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid path.")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		api.WriteError(w, -1, "Path does not exist.")
		return
	}

	if !info.IsDir() {
		api.WriteError(w, -1, "Path is not a directory.")
		return
	}

	data := map[string]interface{}{
		"path":        path,
		"storage":     effectiveStorage,
		"directories": []interface{}{},
		"files":       []interface{}{},
	}

	// Add storage info for root path
	if path == "" || path == "/" {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(fullPath, &stat); err == nil {
			data["total"] = stat.Blocks * uint64(stat.Bsize)
			data["free"] = stat.Bavail * uint64(stat.Bsize)
			data["used"] = (stat.Blocks - stat.Bavail) * uint64(stat.Bsize)
		}
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		api.WriteError(w, -1, "Failed to read directory: "+err.Error())
		return
	}

	dirs := []map[string]interface{}{}
	files := []map[string]interface{}{}

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := map[string]interface{}{
			"name":     name,
			"modified": info.ModTime().Unix(),
		}

		if entry.IsDir() {
			item["type"] = "directory"
			item["size"] = 0
			dirs = append(dirs, item)
		} else {
			item["type"] = "file"
			item["size"] = info.Size()
			files = append(files, item)
		}
	}

	data["directories"] = dirs
	data["files"] = files

	api.WriteSuccessMessage(w, "Success", data)
}

// Mkdir creates a directory.
func (h *FileHandler) Mkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if path == "" {
		api.WriteError(w, -1, "Path is empty.")
		return
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath, err := h.getSafePath(path, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid path.")
		return
	}

	if _, err := os.Stat(fullPath); err == nil {
		api.WriteError(w, -1, "Path already exists.")
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		api.WriteError(w, -1, "Failed to create directory: "+err.Error())
		return
	}

	api.WriteSuccessMessage(w, "Directory created.", map[string]interface{}{
		"path":    path,
		"storage": effectiveStorage,
	})
}

// Remove deletes a file or directory.
func (h *FileHandler) Remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if path == "" {
		api.WriteError(w, -1, "Path is empty.")
		return
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath := h.getFullPath(path, effectiveStorage)

	if _, err := os.Stat(fullPath); err != nil {
		api.WriteError(w, -1, "Path does not exist.")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		api.WriteError(w, -1, "Path does not exist.")
		return
	}

	fileType := "file"
	if info.IsDir() {
		fileType = "directory"
		// Check if directory is empty
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			api.WriteError(w, -1, "Failed to read directory.")
			return
		}
		if len(entries) > 0 {
			api.WriteError(w, -1, "Directory is not empty.")
			return
		}
	}

	if err := os.Remove(fullPath); err != nil {
		api.WriteError(w, -1, "Failed to remove "+fileType+": "+err.Error())
		return
	}

	api.WriteSuccessMessage(w, fileType+" removed.", map[string]interface{}{
		"path":    path,
		"type":    fileType,
		"storage": effectiveStorage,
	})
}

// Upload handles file uploads.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")
	offsetStr := getParam(r, "offset")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	if path != "" && !isValidPath(path) {
		api.WriteError(w, -1, "Invalid path.")
		return
	}

	uploadDir := h.getFullPath(path, effectiveStorage)

	if path != "" {
		if info, err := os.Stat(uploadDir); err != nil || !info.IsDir() {
			api.WriteError(w, -1, "Upload directory does not exist.")
			return
		}
	}

	// Parse offset
	var offset int64
	if offsetStr != "" {
		var err error
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			api.WriteError(w, -1, "Invalid offset parameter.")
			return
		}
	}

	// Parse multipart form (100MB max)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		api.WriteError(w, -1, "Failed to parse form: "+err.Error())
		return
	}

	results := []map[string]interface{}{}
	successCount := 0

	files := r.MultipartForm.File["file"]
	for _, fh := range files {
		if fh.Filename == "" {
			continue
		}

		if !isValidFilename(fh.Filename) {
			results = append(results, map[string]interface{}{
				"filename": fh.Filename,
				"status":   "failed",
				"message":  "Invalid filename",
			})
			continue
		}

		fullPath := filepath.Join(uploadDir, fh.Filename)

		// Open source file
		src, err := fh.Open()
		if err != nil {
			results = append(results, map[string]interface{}{
				"filename": fh.Filename,
				"status":   "failed",
				"message":  "Cannot open uploaded file",
			})
			continue
		}

		// Create/open destination file
		var dst *os.File
		if offset > 0 {
			dst, err = os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
		} else {
			dst, err = os.Create(fullPath)
		}
		if err != nil {
			src.Close()
			results = append(results, map[string]interface{}{
				"filename": fh.Filename,
				"status":   "failed",
				"message":  "Cannot create file",
			})
			continue
		}

		if offset > 0 {
			if _, err := dst.Seek(offset, 0); err != nil {
				src.Close()
				dst.Close()
				results = append(results, map[string]interface{}{
					"filename": fh.Filename,
					"status":   "failed",
					"message":  "Cannot seek to offset",
				})
				continue
			}
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if err != nil {
			results = append(results, map[string]interface{}{
				"filename": fh.Filename,
				"status":   "failed",
				"message":  "Upload error: " + err.Error(),
			})
			continue
		}

		results = append(results, map[string]interface{}{
			"filename": fh.Filename,
			"status":   "success",
			"size":     written,
			"offset":   offset,
			"storage":  effectiveStorage,
		})
		successCount++
	}

	data := map[string]interface{}{
		"uploads": results,
		"count":   successCount,
		"storage": effectiveStorage,
	}

	if successCount > 0 {
		api.WriteSuccessMessage(w, "Upload completed.", data)
	} else {
		api.WriteErrorWithData(w, -1, "Upload failed.", data)
	}
}

// Download serves a file for download.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if path == "" {
		api.WriteError(w, -1, "Path is empty.")
		return
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath, err := h.getSafePath(path, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid path.")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		api.WriteError(w, -1, "File does not exist.")
		return
	}

	if info.IsDir() {
		api.WriteError(w, -1, "Path is not a file.")
		return
	}

	// Check if file is being actively written (modified within last 5 seconds)
	isLiveFile := time.Since(info.ModTime()) < 5*time.Second

	// Explicitly set Content-Type for video files to ensure proper browser handling
	ext := strings.ToLower(filepath.Ext(fullPath))
	switch ext {
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	case ".ogg":
		w.Header().Set("Content-Type", "video/ogg")
	case ".avi":
		w.Header().Set("Content-Type", "video/x-msvideo")
	case ".mov":
		w.Header().Set("Content-Type", "video/quicktime")
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	}

	// Set Accept-Ranges header to explicitly indicate range request support
	// This is critical for video seeking functionality in browsers
	w.Header().Set("Accept-Ranges", "bytes")

	// For live/actively recording files, disable caching and set streaming headers
	if isLiveFile {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		// Add custom header to indicate this is an actively recording file
		w.Header().Set("X-File-Status", "recording")
		// Add Transfer-Encoding hint that content may grow
		w.Header().Set("X-Content-Duration", "infinite")
	} else {
		w.Header().Set("X-File-Status", "complete")
	}

	// http.ServeFile handles range requests automatically, but we ensure headers are set first
	http.ServeFile(w, r, fullPath)
}

// Rename renames a file or directory.
func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	oldPath := getParam(r, "old_path")
	newPath := getParam(r, "new_path")
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if oldPath == "" || newPath == "" {
		api.WriteError(w, -1, "Paths are empty.")
		return
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullOldPath, err := h.getSafePath(oldPath, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid paths.")
		return
	}

	fullNewPath, err := h.getSafePath(newPath, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid paths.")
		return
	}

	if _, err := os.Stat(fullOldPath); err != nil {
		api.WriteError(w, -1, "Source does not exist.")
		return
	}

	if _, err := os.Stat(fullNewPath); err == nil {
		api.WriteError(w, -1, "Destination already exists.")
		return
	}

	if err := os.Rename(fullOldPath, fullNewPath); err != nil {
		api.WriteError(w, -1, "Failed to rename: "+err.Error())
		return
	}

	api.WriteSuccessMessage(w, "Renamed successfully.", map[string]interface{}{
		"old_path": oldPath,
		"new_path": newPath,
		"storage":  effectiveStorage,
	})
}

// Info returns file or directory info.
func (h *FileHandler) Info(w http.ResponseWriter, r *http.Request) {
	path := decodePath(getParam(r, "path"))
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if path == "" {
		api.WriteError(w, -1, "Path is empty.")
		return
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath, err := h.getSafePath(path, effectiveStorage)
	if err != nil {
		api.WriteError(w, -1, "Invalid path.")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		api.WriteError(w, -1, "Path does not exist.")
		return
	}

	data := map[string]interface{}{
		"path":         path,
		"is_directory": info.IsDir(),
		"is_file":      info.Mode().IsRegular(),
		"storage":      effectiveStorage,
		"modified":     info.ModTime().Unix(),
	}

	if info.Mode().IsRegular() {
		data["size"] = info.Size()
	} else {
		data["size"] = 0
	}

	api.WriteSuccessMessage(w, "Info retrieved.", data)
}

// StorageInfo returns storage space information.
func (h *FileHandler) StorageInfo(w http.ResponseWriter, r *http.Request) {
	storage := getParam(r, "storage")

	effectiveStorage := storage
	if effectiveStorage == "" {
		effectiveStorage = "local"
	}

	if !h.isValidStorage(effectiveStorage) {
		api.WriteError(w, -1, "Invalid storage parameter. Use 'local' or 'sd'.")
		return
	}

	if effectiveStorage == "sd" && !h.isSDAvailable() {
		api.WriteError(w, -1, "SD card not available.")
		return
	}

	fullPath := h.getFullPath("", effectiveStorage)

	var stat syscall.Statfs_t
	if err := syscall.Statfs(fullPath, &stat); err != nil {
		api.WriteError(w, -1, "Failed to get storage info: "+err.Error())
		return
	}

	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - available

	data := map[string]interface{}{
		"storage":   effectiveStorage,
		"total":     total,
		"used":      used,
		"available": available,
		"mounted":   true,
	}

	api.WriteSuccessMessage(w, "Storage info retrieved.", data)
}

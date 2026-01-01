//
//  EnhancedVideoViewer.swift
//  Enhanced macOS Video Viewer with Rich UI
//
//  Full-featured video viewer with controls, stats, and multi-camera support
//

import SwiftUI
import Combine
import Foundation

// MARK: - Models

struct CameraInfo: Identifiable {
    let id = UUID()
    var cameraID: String
    var name: String
    var isActive: Bool = false
}

struct VideoStats {
    var framesReceived: Int = 0
    var fps: Double = 0
    var lastFrameTime: Date = Date()
    var dataReceived: Int64 = 0
    var connectionDuration: TimeInterval = 0
}

// MARK: - WebSocket Video Client

class WebSocketVideoClient: NSObject, ObservableObject {
    @Published var currentFrame: NSImage?
    @Published var isConnected: Bool = false
    @Published var errorMessage: String?
    @Published var viewerCount: Int = 0
    @Published var stats = VideoStats()
    
    private var webSocketTask: URLSessionWebSocketTask?
    private var urlSession: URLSession!
    private let relayURL: URL
    private let cameraID: String
    private let authToken: String?
    private var connectionStartTime: Date?
    private var frameTimestamps: [Date] = []
    
    init(relayURL: URL, cameraID: String, authToken: String? = nil) {
        self.relayURL = relayURL
        self.cameraID = cameraID
        self.authToken = authToken
        super.init()
        
        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = 30
        configuration.timeoutIntervalForResource = 300
        self.urlSession = URLSession(configuration: configuration, delegate: self, delegateQueue: nil)
    }
    
    func connect() {
        var request = URLRequest(url: relayURL)
        request.setValue("viewer", forHTTPHeaderField: "Role")
        
        if let token = authToken {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        
        webSocketTask = urlSession.webSocketTask(with: request)
        webSocketTask?.resume()
        
        connectionStartTime = Date()
        isConnected = true
        errorMessage = nil
        
        subscribe(to: cameraID)
        receiveMessage()
        startStatsTimer()
        
        print("Connected to relay server: \(relayURL)")
    }
    
    func disconnect() {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        isConnected = false
        currentFrame = nil
        connectionStartTime = nil
    }
    
    private func subscribe(to cameraID: String) {
        let subscribeMessage = """
        {
            "type": "subscribe",
            "camera_id": "\(cameraID)"
        }
        """
        
        let message = URLSessionWebSocketTask.Message.string(subscribeMessage)
        webSocketTask?.send(message) { error in
            if let error = error {
                DispatchQueue.main.async {
                    self.errorMessage = "Failed to subscribe: \(error.localizedDescription)"
                }
            }
        }
    }
    
    private func receiveMessage() {
        webSocketTask?.receive { [weak self] result in
            guard let self = self else { return }
            
            switch result {
            case .success(let message):
                switch message {
                case .data(let data):
                    self.processVideoFrame(data)
                case .string(let text):
                    self.processControlMessage(text)
                @unknown default:
                    break
                }
                self.receiveMessage()
                
            case .failure(let error):
                DispatchQueue.main.async {
                    self.isConnected = false
                    self.errorMessage = "Connection error: \(error.localizedDescription)"
                }
            }
        }
    }
    
    private func processVideoFrame(_ data: Data) {
        if let image = NSImage(data: data) {
            DispatchQueue.main.async {
                self.currentFrame = image
                self.errorMessage = nil
                
                // Update stats
                self.stats.framesReceived += 1
                self.stats.dataReceived += Int64(data.count)
                self.stats.lastFrameTime = Date()
                
                // Track FPS
                self.frameTimestamps.append(Date())
                self.frameTimestamps = self.frameTimestamps.filter { 
                    Date().timeIntervalSince($0) < 1.0 
                }
            }
        }
    }
    
    private func processControlMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            return
        }
        
        switch type {
        case "camera_offline":
            DispatchQueue.main.async {
                self.errorMessage = "Camera is offline"
            }
        case "error":
            if let message = json["message"] as? String {
                DispatchQueue.main.async {
                    self.errorMessage = message
                }
            }
        case "viewer_count":
            if let count = json["count"] as? Int {
                DispatchQueue.main.async {
                    self.viewerCount = count
                }
            }
        default:
            break
        }
    }
    
    private func startStatsTimer() {
        Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { [weak self] _ in
            guard let self = self, self.isConnected else { return }
            
            DispatchQueue.main.async {
                self.stats.fps = Double(self.frameTimestamps.count)
                if let startTime = self.connectionStartTime {
                    self.stats.connectionDuration = Date().timeIntervalSince(startTime)
                }
            }
        }
    }
}

extension WebSocketVideoClient: URLSessionWebSocketDelegate {
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, 
                   didOpenWithProtocol protocol: String?) {
        DispatchQueue.main.async {
            self.isConnected = true
            self.errorMessage = nil
        }
    }
    
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, 
                   didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) {
        DispatchQueue.main.async {
            self.isConnected = false
        }
    }
}

// MARK: - Enhanced UI Views

struct EnhancedContentView: View {
    @StateObject private var videoClient: WebSocketVideoClient
    @State private var showSettings = false
    @State private var showStats = true
    @State private var isFullscreen = false
    @State private var relayURL = "ws://localhost:8080/ws"
    @State private var cameraID = "camera_12345"
    @State private var authToken = ""
    
    init() {
        let url = URL(string: "ws://localhost:8080/ws")!
        _videoClient = StateObject(wrappedValue: WebSocketVideoClient(
            relayURL: url,
            cameraID: "camera_12345",
            authToken: nil
        ))
    }
    
    var body: some View {
        ZStack {
            // Main video display
            VideoDisplayView(
                client: videoClient,
                showStats: showStats
            )
            
            // Top toolbar
            VStack {
                TopToolbar(
                    client: videoClient,
                    showStats: $showStats,
                    showSettings: $showSettings,
                    isFullscreen: $isFullscreen
                )
                Spacer()
            }
            
            // Bottom control bar
            VStack {
                Spacer()
                BottomControlBar(
                    client: videoClient,
                    onConnect: {
                        if videoClient.isConnected {
                            videoClient.disconnect()
                        } else {
                            videoClient.connect()
                        }
                    }
                )
            }
        }
        .background(Color.black)
        .sheet(isPresented: $showSettings) {
            SettingsView(
                relayURL: $relayURL,
                cameraID: $cameraID,
                authToken: $authToken,
                onSave: {
                    if let url = URL(string: relayURL) {
                        // Would recreate client here in production
                    }
                    showSettings = false
                }
            )
        }
    }
}

struct VideoDisplayView: View {
    @ObservedObject var client: WebSocketVideoClient
    var showStats: Bool
    
    var body: some View {
        ZStack {
            Color.black
            
            if let frame = client.currentFrame {
                Image(nsImage: frame)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .transition(.opacity)
            } else if client.isConnected {
                VStack(spacing: 20) {
                    ProgressView()
                        .scaleEffect(1.5)
                        .progressViewStyle(CircularProgressViewStyle(tint: .white))
                    Text("Waiting for video...")
                        .foregroundColor(.white)
                        .font(.title3)
                }
            } else {
                VStack(spacing: 20) {
                    Image(systemName: "video.slash")
                        .font(.system(size: 80))
                        .foregroundColor(.gray)
                    Text("Not connected")
                        .foregroundColor(.gray)
                        .font(.title2)
                }
            }
            
            // Stats overlay
            if showStats && client.isConnected {
                VStack {
                    HStack {
                        StatsOverlay(stats: client.stats)
                        Spacer()
                    }
                    Spacer()
                }
                .padding()
            }
            
            // Error overlay
            if let error = client.errorMessage {
                VStack {
                    Spacer()
                    HStack {
                        Image(systemName: "exclamationmark.triangle.fill")
                        Text(error)
                    }
                    .foregroundColor(.white)
                    .padding()
                    .background(Color.red.opacity(0.9))
                    .cornerRadius(8)
                    .padding()
                }
            }
        }
    }
}

struct TopToolbar: View {
    @ObservedObject var client: WebSocketVideoClient
    @Binding var showStats: Bool
    @Binding var showSettings: Bool
    @Binding var isFullscreen: Bool
    
    var body: some View {
        HStack {
            // Camera info
            HStack(spacing: 8) {
                Image(systemName: "video.fill")
                    .foregroundColor(.white)
                Text(client.cameraID)
                    .foregroundColor(.white)
                    .font(.headline)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(Color.black.opacity(0.5))
            .cornerRadius(8)
            
            Spacer()
            
            // Toolbar buttons
            HStack(spacing: 16) {
                // Stats toggle
                Button(action: { showStats.toggle() }) {
                    Image(systemName: showStats ? "chart.bar.fill" : "chart.bar")
                        .foregroundColor(.white)
                }
                .buttonStyle(PlainButtonStyle())
                .help("Toggle statistics")
                
                // Fullscreen
                Button(action: { isFullscreen.toggle() }) {
                    Image(systemName: isFullscreen ? "arrow.down.right.and.arrow.up.left" : "arrow.up.left.and.arrow.down.right")
                        .foregroundColor(.white)
                }
                .buttonStyle(PlainButtonStyle())
                .help("Toggle fullscreen")
                
                // Settings
                Button(action: { showSettings = true }) {
                    Image(systemName: "gearshape.fill")
                        .foregroundColor(.white)
                }
                .buttonStyle(PlainButtonStyle())
                .help("Settings")
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(Color.black.opacity(0.5))
            .cornerRadius(8)
        }
        .padding()
    }
}

struct StatsOverlay: View {
    let stats: VideoStats
    
    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            StatRow(label: "FPS", value: String(format: "%.1f", stats.fps))
            StatRow(label: "Frames", value: "\(stats.framesReceived)")
            StatRow(label: "Data", value: formatBytes(stats.dataReceived))
            StatRow(label: "Uptime", value: formatDuration(stats.connectionDuration))
        }
        .padding(12)
        .background(Color.black.opacity(0.7))
        .cornerRadius(8)
        .font(.system(.caption, design: .monospaced))
    }
    
    private func formatBytes(_ bytes: Int64) -> String {
        let mb = Double(bytes) / 1_048_576
        return String(format: "%.1f MB", mb)
    }
    
    private func formatDuration(_ duration: TimeInterval) -> String {
        let minutes = Int(duration) / 60
        let seconds = Int(duration) % 60
        return String(format: "%02d:%02d", minutes, seconds)
    }
}

struct StatRow: View {
    let label: String
    let value: String
    
    var body: some View {
        HStack {
            Text(label + ":")
                .foregroundColor(.gray)
            Text(value)
                .foregroundColor(.white)
        }
    }
}

struct BottomControlBar: View {
    @ObservedObject var client: WebSocketVideoClient
    var onConnect: () -> Void
    
    var body: some View {
        HStack {
            // Connection status
            HStack(spacing: 8) {
                Circle()
                    .fill(client.isConnected ? Color.green : Color.red)
                    .frame(width: 10, height: 10)
                Text(client.isConnected ? "Connected" : "Disconnected")
                    .foregroundColor(.white)
                    .font(.caption)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(Color.black.opacity(0.7))
            .cornerRadius(8)
            
            // Viewer count
            if client.viewerCount > 0 {
                HStack(spacing: 6) {
                    Image(systemName: "eye.fill")
                        .foregroundColor(.white)
                    Text("\(client.viewerCount)")
                        .foregroundColor(.white)
                        .font(.caption)
                }
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                .background(Color.black.opacity(0.7))
                .cornerRadius(8)
            }
            
            Spacer()
            
            // Connect/Disconnect button
            Button(action: onConnect) {
                HStack {
                    Image(systemName: client.isConnected ? "stop.circle.fill" : "play.circle.fill")
                    Text(client.isConnected ? "Disconnect" : "Connect")
                }
                .foregroundColor(.white)
                .padding(.horizontal, 20)
                .padding(.vertical, 10)
                .background(client.isConnected ? Color.red : Color.green)
                .cornerRadius(8)
            }
            .buttonStyle(PlainButtonStyle())
        }
        .padding()
        .background(Color(NSColor.windowBackgroundColor).opacity(0.95))
    }
}

struct SettingsView: View {
    @Binding var relayURL: String
    @Binding var cameraID: String
    @Binding var authToken: String
    var onSave: () -> Void
    
    @Environment(\.presentationMode) var presentationMode
    
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Connection Settings")
                    .font(.title)
                    .fontWeight(.semibold)
                Spacer()
            }
            .padding()
            .background(Color(NSColor.controlBackgroundColor))
            
            Divider()
            
            // Settings form
            Form {
                Section(header: Text("Relay Server").font(.headline)) {
                    TextField("WebSocket URL", text: $relayURL)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .help("Example: ws://localhost:8080/ws or wss://relay.example.com/ws")
                }
                .padding(.vertical, 8)
                
                Section(header: Text("Camera").font(.headline)) {
                    TextField("Camera ID", text: $cameraID)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .help("Unique identifier for the camera to view")
                }
                .padding(.vertical, 8)
                
                Section(header: Text("Authentication").font(.headline)) {
                    SecureField("Bearer Token (optional)", text: $authToken)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .help("JWT token if authentication is enabled on relay server")
                    
                    Text("Leave empty if authentication is disabled")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
                .padding(.vertical, 8)
            }
            .padding()
            
            Divider()
            
            // Buttons
            HStack {
                Button("Cancel") {
                    presentationMode.wrappedValue.dismiss()
                }
                .keyboardShortcut(.cancelAction)
                
                Spacer()
                
                Button("Save & Connect") {
                    onSave()
                }
                .keyboardShortcut(.defaultAction)
                .buttonStyle(.borderedProminent)
            }
            .padding()
            .background(Color(NSColor.controlBackgroundColor))
        }
        .frame(width: 600, height: 500)
    }
}

// MARK: - App Entry Point

@main
struct EnhancedVideoViewerApp: App {
    var body: some Scene {
        WindowGroup {
            EnhancedContentView()
                .frame(minWidth: 800, minHeight: 600)
        }
        .commands {
            CommandGroup(replacing: .newItem) { }
        }
    }
}

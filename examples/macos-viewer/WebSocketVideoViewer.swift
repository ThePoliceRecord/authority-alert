//
//  WebSocketVideoViewer.swift
//  Authority Alert Video Viewer
//
//  macOS app to receive and display video from websocket relay server
//

import SwiftUI
import Combine
import Foundation

// MARK: - WebSocket Video Client

class WebSocketVideoClient: NSObject, ObservableObject {
    @Published var currentFrame: NSImage?
    @Published var isConnected: Bool = false
    @Published var errorMessage: String?
    @Published var viewerCount: Int = 0
    
    private var webSocketTask: URLSessionWebSocketTask?
    private var urlSession: URLSession!
    private let relayURL: URL
    private let cameraID: String
    private let authToken: String?
    
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
        
        isConnected = true
        errorMessage = nil
        
        // Subscribe to camera
        subscribe(to: cameraID)
        
        // Start receiving messages
        receiveMessage()
        
        print("Connected to relay server: \(relayURL)")
    }
    
    func disconnect() {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        isConnected = false
        currentFrame = nil
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
                print("Subscribe error: \(error)")
            } else {
                print("Subscribed to camera: \(cameraID)")
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
                    // Binary message - should be JPEG frame
                    self.processVideoFrame(data)
                    
                case .string(let text):
                    // Text message - control/status message
                    self.processControlMessage(text)
                    
                @unknown default:
                    break
                }
                
                // Continue receiving messages
                self.receiveMessage()
                
            case .failure(let error):
                DispatchQueue.main.async {
                    self.isConnected = false
                    self.errorMessage = "Connection error: \(error.localizedDescription)"
                }
                print("Receive error: \(error)")
            }
        }
    }
    
    private func processVideoFrame(_ data: Data) {
        // Convert JPEG data to NSImage
        if let image = NSImage(data: data) {
            DispatchQueue.main.async {
                self.currentFrame = image
                self.errorMessage = nil
            }
        }
    }
    
    private func processControlMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            return
        }
        
        print("Control message: \(type)")
        
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
}

// MARK: - URLSessionWebSocketDelegate

extension WebSocketVideoClient: URLSessionWebSocketDelegate {
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didOpenWithProtocol protocol: String?) {
        DispatchQueue.main.async {
            self.isConnected = true
            self.errorMessage = nil
        }
        print("WebSocket connected")
    }
    
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) {
        DispatchQueue.main.async {
            self.isConnected = false
        }
        print("WebSocket disconnected: \(closeCode)")
    }
}

// MARK: - SwiftUI Views

struct ContentView: View {
    @StateObject private var videoClient: WebSocketVideoClient
    @State private var relayURL: String = "ws://localhost:8080/ws"
    @State private var cameraID: String = "camera_12345"
    @State private var authToken: String = ""
    @State private var showSettings: Bool = false
    
    init() {
        // Initialize with default values
        let url = URL(string: "ws://localhost:8080/ws")!
        _videoClient = StateObject(wrappedValue: WebSocketVideoClient(
            relayURL: url,
            cameraID: "camera_12345",
            authToken: nil
        ))
    }
    
    var body: some View {
        VStack(spacing: 0) {
            // Video display area
            ZStack {
                Color.black
                
                if let frame = videoClient.currentFrame {
                    Image(nsImage: frame)
                        .resizable()
                        .aspectRatio(contentMode: .fit)
                } else if videoClient.isConnected {
                    VStack {
                        ProgressView()
                            .scaleEffect(1.5)
                        Text("Waiting for video...")
                            .foregroundColor(.white)
                            .padding()
                    }
                } else {
                    VStack {
                        Image(systemName: "video.slash")
                            .font(.system(size: 60))
                            .foregroundColor(.gray)
                        Text("Not connected")
                            .foregroundColor(.gray)
                            .padding()
                    }
                }
                
                // Error overlay
                if let error = videoClient.errorMessage {
                    VStack {
                        Spacer()
                        Text(error)
                            .foregroundColor(.white)
                            .padding()
                            .background(Color.red.opacity(0.8))
                            .cornerRadius(8)
                            .padding()
                    }
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            
            // Control bar
            HStack {
                // Connection status
                HStack {
                    Circle()
                        .fill(videoClient.isConnected ? Color.green : Color.red)
                        .frame(width: 10, height: 10)
                    Text(videoClient.isConnected ? "Connected" : "Disconnected")
                        .font(.caption)
                }
                
                if videoClient.viewerCount > 0 {
                    Divider()
                    HStack {
                        Image(systemName: "eye")
                        Text("\(videoClient.viewerCount)")
                            .font(.caption)
                    }
                }
                
                Spacer()
                
                // Connect/Disconnect button
                Button(action: {
                    if videoClient.isConnected {
                        videoClient.disconnect()
                    } else {
                        videoClient.connect()
                    }
                }) {
                    Text(videoClient.isConnected ? "Disconnect" : "Connect")
                }
                
                // Settings button
                Button(action: { showSettings.toggle() }) {
                    Image(systemName: "gearshape")
                }
            }
            .padding()
            .background(Color(NSColor.windowBackgroundColor))
        }
        .sheet(isPresented: $showSettings) {
            SettingsView(
                relayURL: $relayURL,
                cameraID: $cameraID,
                authToken: $authToken,
                onSave: {
                    if let url = URL(string: relayURL) {
                        let newClient = WebSocketVideoClient(
                            relayURL: url,
                            cameraID: cameraID,
                            authToken: authToken.isEmpty ? nil : authToken
                        )
                        // Note: This creates a new client. In a real app, you'd want to
                        // disconnect the old client first and update the existing one.
                    }
                    showSettings = false
                }
            )
        }
    }
}

struct SettingsView: View {
    @Binding var relayURL: String
    @Binding var cameraID: String
    @Binding var authToken: String
    var onSave: () -> Void
    
    @Environment(\.presentationMode) var presentationMode
    
    var body: some View {
        VStack(spacing: 20) {
            Text("Connection Settings")
                .font(.title)
            
            Form {
                TextField("Relay URL", text: $relayURL)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .help("WebSocket URL of the relay server (e.g., ws://localhost:8080/ws)")
                
                TextField("Camera ID", text: $cameraID)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .help("Unique identifier for the camera")
                
                SecureField("Auth Token (optional)", text: $authToken)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .help("Bearer token for authentication (if enabled on relay)")
            }
            .padding()
            
            HStack {
                Button("Cancel") {
                    presentationMode.wrappedValue.dismiss()
                }
                
                Button("Save") {
                    onSave()
                }
                .keyboardShortcut(.defaultAction)
            }
        }
        .padding()
        .frame(width: 500, height: 300)
    }
}

// MARK: - App Entry Point

@main
struct VideoViewerApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                .frame(minWidth: 800, minHeight: 600)
        }
        .commands {
            CommandGroup(replacing: .newItem) { }
        }
    }
}

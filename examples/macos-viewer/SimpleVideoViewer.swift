//
//  SimpleVideoViewer.swift
//  Simple AppKit-based WebSocket Video Viewer
//
//  A minimal example without SwiftUI for more control
//

import Cocoa
import Foundation

class WebSocketVideoConnection: NSObject {
    private var webSocketTask: URLSessionWebSocketTask?
    private var urlSession: URLSession!
    private let relayURL: URL
    private let cameraID: String
    var onFrameReceived: ((Data) -> Void)?
    var onStatusChanged: ((Bool, String?) -> Void)?
    
    init(relayURL: URL, cameraID: String) {
        self.relayURL = relayURL
        self.cameraID = cameraID
        super.init()
        
        let config = URLSessionConfiguration.default
        self.urlSession = URLSession(configuration: config, delegate: self, delegateQueue: nil)
    }
    
    func connect() {
        var request = URLRequest(url: relayURL)
        request.setValue("viewer", forHTTPHeaderField: "Role")
        
        webSocketTask = urlSession.webSocketTask(with: request)
        webSocketTask?.resume()
        
        // Subscribe to camera
        let subscribeMessage = """
        {"type":"subscribe","camera_id":"\(cameraID)"}
        """
        webSocketTask?.send(.string(subscribeMessage)) { error in
            if let error = error {
                print("Subscribe error: \(error)")
            }
        }
        
        receiveMessage()
        onStatusChanged?(true, nil)
    }
    
    func disconnect() {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        onStatusChanged?(false, nil)
    }
    
    private func receiveMessage() {
        webSocketTask?.receive { [weak self] result in
            guard let self = self else { return }
            
            switch result {
            case .success(let message):
                switch message {
                case .data(let data):
                    // JPEG frame received
                    self.onFrameReceived?(data)
                case .string(let text):
                    print("Control message: \(text)")
                @unknown default:
                    break
                }
                self.receiveMessage()
                
            case .failure(let error):
                self.onStatusChanged?(false, error.localizedDescription)
                print("Receive error: \(error)")
            }
        }
    }
}

extension WebSocketVideoConnection: URLSessionWebSocketDelegate {
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, 
                   didOpenWithProtocol protocol: String?) {
        print("WebSocket connected")
    }
    
    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, 
                   didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) {
        print("WebSocket closed")
    }
}

// MARK: - Simple Window Controller

class VideoWindowController: NSWindowController {
    private var connection: WebSocketVideoConnection?
    private let imageView = NSImageView()
    private let connectButton = NSButton(title: "Connect", target: nil, action: nil)
    private let statusLabel = NSTextField(labelWithString: "Not connected")
    
    convenience init(relayURL: String, cameraID: String) {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 800, height: 600),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "Video Viewer - \(cameraID)"
        window.center()
        
        self.init(window: window)
        
        guard let url = URL(string: relayURL) else {
            print("Invalid URL: \(relayURL)")
            return
        }
        
        connection = WebSocketVideoConnection(relayURL: url, cameraID: cameraID)
        setupUI()
        setupCallbacks()
    }
    
    private func setupUI() {
        guard let contentView = window?.contentView else { return }
        
        // Image view for video
        imageView.imageScaling = .scaleProportionallyUpOrDown
        imageView.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(imageView)
        
        // Control bar container
        let controlBar = NSView()
        controlBar.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(controlBar)
        
        // Status label
        statusLabel.translatesAutoresizingMaskIntoConstraints = false
        statusLabel.isEditable = false
        statusLabel.isBordered = false
        statusLabel.backgroundColor = .clear
        controlBar.addSubview(statusLabel)
        
        // Connect button
        connectButton.translatesAutoresizingMaskIntoConstraints = false
        connectButton.target = self
        connectButton.action = #selector(connectButtonClicked)
        controlBar.addSubview(connectButton)
        
        // Layout constraints
        NSLayoutConstraint.activate([
            // Image view fills top area
            imageView.topAnchor.constraint(equalTo: contentView.topAnchor),
            imageView.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            imageView.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            imageView.bottomAnchor.constraint(equalTo: controlBar.topAnchor),
            
            // Control bar at bottom
            controlBar.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            controlBar.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            controlBar.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
            controlBar.heightAnchor.constraint(equalToConstant: 50),
            
            // Status label on left
            statusLabel.leadingAnchor.constraint(equalTo: controlBar.leadingAnchor, constant: 20),
            statusLabel.centerYAnchor.constraint(equalTo: controlBar.centerYAnchor),
            
            // Connect button on right
            connectButton.trailingAnchor.constraint(equalTo: controlBar.trailingAnchor, constant: -20),
            connectButton.centerYAnchor.constraint(equalTo: controlBar.centerYAnchor)
        ])
    }
    
    private func setupCallbacks() {
        connection?.onFrameReceived = { [weak self] data in
            guard let image = NSImage(data: data) else { return }
            DispatchQueue.main.async {
                self?.imageView.image = image
            }
        }
        
        connection?.onStatusChanged = { [weak self] isConnected, errorMessage in
            DispatchQueue.main.async {
                if isConnected {
                    self?.statusLabel.stringValue = "Connected"
                    self?.statusLabel.textColor = .systemGreen
                    self?.connectButton.title = "Disconnect"
                } else {
                    self?.statusLabel.stringValue = errorMessage ?? "Disconnected"
                    self?.statusLabel.textColor = .systemRed
                    self?.connectButton.title = "Connect"
                }
            }
        }
    }
    
    @objc private func connectButtonClicked() {
        if connectButton.title == "Connect" {
            connection?.connect()
        } else {
            connection?.disconnect()
        }
    }
}

// MARK: - App Delegate

@main
class AppDelegate: NSObject, NSApplicationDelegate {
    private var windowController: VideoWindowController?
    
    func applicationDidFinishLaunching(_ notification: Notification) {
        // Parse command line arguments
        let args = CommandLine.arguments
        let relayURL = args.count > 1 ? args[1] : "ws://localhost:8080/ws"
        let cameraID = args.count > 2 ? args[2] : "camera_12345"
        
        print("Connecting to: \(relayURL)")
        print("Camera ID: \(cameraID)")
        
        // Create and show window
        windowController = VideoWindowController(relayURL: relayURL, cameraID: cameraID)
        windowController?.showWindow(nil)
    }
    
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return true
    }
}

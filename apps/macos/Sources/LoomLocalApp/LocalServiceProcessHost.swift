import Darwin
import Combine
import Foundation
import LoomLocalAppCore
import os

@MainActor
final class LocalServiceProcessHost: ObservableObject {
    nonisolated private static let socketHealthTimeoutMilliseconds: Int32 = 100
    nonisolated private static let allowedBundledServiceFeatureFlags: Set<String> = [
        "LOOM_DEBUG_DAEMON",
        "LOOM_ENABLE_WEB_TOOLS",
        "LOOM_SANDBOX_REQUIRED",
    ]
    private let logger = Logger(
        subsystem: "com.earendilworks.loom.local",
        category: "service-bootstrap"
    )
    private var process: Process?
    private var consoleHandle: FileHandle?
    private var restartTask: Task<Void, Never>?
    private var keepsBundledServiceAlive = true

    func start() async -> Bool {
        if process?.isRunning == true {
            logger.info("bundled service already available")
            return true
        }
        if await Self.probeSocketOffMain(at: defaultSocketPath) {
            logger.info("bundled service already available")
            return true
        }
        let helper = Bundle.main.bundleURL
            .appendingPathComponent("Contents/Library/Helpers/loomd")
            .standardizedFileURL
        var isDirectory: ObjCBool = false
        let helperValues = try? helper.resourceValues(
            forKeys: [.isSymbolicLinkKey]
        )
        guard FileManager.default.fileExists(
                atPath: helper.path,
                isDirectory: &isDirectory
              ),
              !isDirectory.boolValue,
              helperValues?.isSymbolicLink != true,
              FileManager.default.isExecutableFile(atPath: helper.path) else {
            logger.error("bundled service helper unavailable")
            return false
        }

        let child = Process()
        child.executableURL = helper
        child.arguments = [
            "--local-app-service",
            "--parent-pid",
            String(getpid()),
        ]
        guard let servicePath else {
            logger.error("bundled service environment unavailable")
            return false
        }
        child.environment = bundledServiceEnvironment(path: servicePath)
        guard let console = daemonConsoleHandle() else {
            logger.error("bundled service diagnostics unavailable")
            return false
        }
        child.standardOutput = console
        child.standardError = console
        child.terminationHandler = { [weak self] _ in
            Task { @MainActor [weak self] in
                guard let self, self.keepsBundledServiceAlive else { return }
                self.process = nil
                self.consoleHandle = nil
                self.scheduleBundledServiceRestart()
            }
        }
        do {
            try child.run()
            process = child
            consoleHandle = console
            logger.info("bundled service process started")
            return true
        } catch {
            try? console.close()
            logger.error("bundled service process failed to start")
            return false
        }
    }

    func waitForDefaultSocket(
        maxAttempts: Int = 120,
        delayNanoseconds: UInt64 = 500_000_000
    ) async -> Bool {
        let socketPath = defaultSocketPath
        for attempt in 0..<max(maxAttempts, 1) {
            if await Self.probeSocketOffMain(at: socketPath) { return true }
            guard attempt + 1 < maxAttempts else { break }
            do {
                try await Task.sleep(nanoseconds: delayNanoseconds)
            } catch {
                return false
            }
        }
        logger.notice("managed service socket did not appear")
        return await Self.probeSocketOffMain(at: socketPath)
    }

    deinit {
        keepsBundledServiceAlive = false
        restartTask?.cancel()
        if process?.isRunning == true {
            process?.terminate()
        }
        try? consoleHandle?.close()
    }

    private func scheduleBundledServiceRestart() {
        guard restartTask == nil else { return }
        restartTask = Task { @MainActor [weak self] in
            defer { self?.restartTask = nil }
            do {
                try await Task.sleep(nanoseconds: 750_000_000)
            } catch {
                return
            }
            guard let self, self.keepsBundledServiceAlive else { return }
            let socketIsReachable = await Self.probeSocketOffMain(
                at: self.defaultSocketPath
            )
            guard !socketIsReachable else {
                return
            }
            if await self.start() {
                self.logger.notice("bundled service restarted after unexpected termination")
            } else {
                self.logger.error("bundled service restart failed")
            }
        }
    }

    private func bundledServiceEnvironment(path: String) -> [String: String] {
        var values = [
            "HOME": FileManager.default.homeDirectoryForCurrentUser
                .standardizedFileURL.path,
            "PATH": path,
        ]
        var index = 0
        while let entry = environ[index] {
            let pair = String(cString: entry)
            if let equals = pair.firstIndex(of: "=") {
                let key = String(pair[..<equals])
                let value = String(pair[pair.index(after: equals)...])
                if Self.allowedBundledServiceFeatureFlags.contains(key), value == "1" {
                    values[key] = "1"
                }
            }
            index += 1
        }
        return values
    }

    private var defaultSocketPath: String {
        FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/Loom/run/loomd.sock")
            .path
    }

    nonisolated private static func probeSocketOffMain(
        at path: String
    ) async -> Bool {
        await Task.detached(priority: .utility) {
            Self.socketIsReachable(at: path)
        }.value
    }

    nonisolated static func socketIsReachable(at path: String) -> Bool {
        guard LocalIPCClient.isTrustedSocket(at: path) else { return false }

        let descriptor = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard descriptor >= 0 else { return false }
        defer { Darwin.close(descriptor) }
        let flags = Darwin.fcntl(descriptor, F_GETFL, 0)
        guard flags >= 0,
              Darwin.fcntl(descriptor, F_SETFL, flags | O_NONBLOCK) == 0 else {
            return false
        }

        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let pathBytes = Array(path.utf8)
        guard pathBytes.count < MemoryLayout.size(ofValue: address.sun_path) else {
            return false
        }
        withUnsafeMutableBytes(of: &address.sun_path) { buffer in
            buffer.copyBytes(from: pathBytes)
        }
        let addressLength = socklen_t(MemoryLayout<sockaddr_un>.size)
        let connected = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(descriptor, $0, addressLength)
            }
        }
        if connected == 0 { return true }
        return Self.nonblockingConnectIsHealthy(descriptor, initialError: errno)
    }

    nonisolated private static func nonblockingConnectIsHealthy(
        _ descriptor: Int32,
        initialError: Int32
    ) -> Bool {
        switch initialError {
        case EISCONN:
            return true
        case EAGAIN:
            return false
        case EINPROGRESS, EALREADY:
            var event = pollfd(
                fd: descriptor,
                events: Int16(POLLOUT),
                revents: 0
            )
            guard Darwin.poll(
                &event,
                1,
                Self.socketHealthTimeoutMilliseconds
            ) > 0,
            event.revents & Int16(POLLNVAL) == 0 else {
                return false
            }
            var socketError: Int32 = 0
            var socketErrorLength = socklen_t(MemoryLayout.size(ofValue: socketError))
            guard Darwin.getsockopt(
                descriptor,
                SOL_SOCKET,
                SO_ERROR,
                &socketError,
                &socketErrorLength
            ) == 0 else {
                return false
            }
            return socketError == 0
        default:
            return false
        }
    }

    private var servicePath: String? {
        let plist = Bundle.main.bundleURL.appendingPathComponent(
            "Contents/Library/LaunchAgents/com.earendilworks.loom.local.daemon.plist"
        )
        guard let data = try? Data(contentsOf: plist),
              let root = try? PropertyListSerialization.propertyList(
                from: data,
                options: [],
                format: nil
              ) as? [String: Any],
              let environment = root["EnvironmentVariables"] as? [String: Any],
              let path = environment["PATH"] as? String,
              !path.isEmpty,
              path.utf8.count <= 4_096,
              !path.contains("\n"),
              !path.contains("\0") else {
            return nil
        }
        return path
    }

    private func daemonConsoleHandle() -> FileHandle? {
        let directory = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/Loom/diagnostics")
        do {
            try FileManager.default.createDirectory(
                at: directory,
                withIntermediateDirectories: true,
                attributes: [.posixPermissions: 0o700]
            )
            try FileManager.default.setAttributes(
                [.posixPermissions: 0o700],
                ofItemAtPath: directory.path
            )
        } catch {
            return nil
        }
        let path = directory.appendingPathComponent("loomd-console.log").path
        guard rotateDaemonConsoleIfNeeded(path) else { return nil }
        let descriptor = path.withCString {
            Darwin.open($0, O_WRONLY | O_CREAT | O_APPEND | O_NOFOLLOW, S_IRUSR | S_IWUSR)
        }
        guard descriptor >= 0 else { return nil }
        var status = stat()
        guard fstat(descriptor, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              fchmod(descriptor, S_IRUSR | S_IWUSR) == 0 else {
            Darwin.close(descriptor)
            return nil
        }
        return FileHandle(fileDescriptor: descriptor, closeOnDealloc: true)
    }

    private func rotateDaemonConsoleIfNeeded(_ path: String) -> Bool {
        var status = stat()
        if lstat(path, &status) != 0 {
            return errno == ENOENT
        }
        guard status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG else { return false }
        guard status.st_size >= 512 * 1_024 else { return true }
        let rotated = path + ".1"
        var rotatedStatus = stat()
        if lstat(rotated, &rotatedStatus) == 0 {
            guard rotatedStatus.st_uid == geteuid(),
                  rotatedStatus.st_mode & S_IFMT == S_IFREG,
                  unlink(rotated) == 0 else { return false }
        } else if errno != ENOENT {
            return false
        }
        return rename(path, rotated) == 0 && chmod(rotated, S_IRUSR | S_IWUSR) == 0
    }
}

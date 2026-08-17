import Darwin
import Combine
import Foundation
import os

@MainActor
final class LocalServiceProcessHost: ObservableObject {
    private let logger = Logger(
        subsystem: "com.earendilworks.loom.local",
        category: "service-bootstrap"
    )
    private var process: Process?
    private var consoleHandle: FileHandle?

    func start() -> Bool {
        if process?.isRunning == true || defaultSocketExists {
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
        // Merge the caller's environment (which carries explicit operator
        // opt-ins such as LOOM_ENABLE_WEB_TOOLS) with the canonical HOME/PATH
        // the bundled service needs. Replacing the whole environment here
        // would silently drop those opt-ins.
        var serviceEnvironment = bundledServiceEnvironment()
        serviceEnvironment["HOME"] =
            FileManager.default.homeDirectoryForCurrentUser.path
        serviceEnvironment["PATH"] = servicePath
        child.environment = serviceEnvironment
        guard let console = daemonConsoleHandle() else {
            logger.error("bundled service diagnostics unavailable")
            return false
        }
        child.standardOutput = console
        child.standardError = console
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
        maxAttempts: Int = 8,
        delayNanoseconds: UInt64 = 250_000_000
    ) async -> Bool {
        for attempt in 0..<max(maxAttempts, 1) {
            if defaultSocketExists { return true }
            guard attempt + 1 < maxAttempts else { break }
            do {
                try await Task.sleep(nanoseconds: delayNanoseconds)
            } catch {
                return false
            }
        }
        logger.notice("managed service socket did not appear")
        return defaultSocketExists
    }

    deinit {
        if process?.isRunning == true {
            process?.terminate()
        }
        try? consoleHandle?.close()
    }

    private func bundledServiceEnvironment() -> [String: String] {
        // Read the inherited environment via the Darwin C boundary so the
        // bundled service inherits explicit operator opt-ins.
        var values = [String: String]()
        var index = 0
        while let entry = environ[index] {
            let pair = String(cString: entry)
            if let equals = pair.firstIndex(of: "=") {
                let key = String(pair[..<equals])
                let value = String(pair[pair.index(after: equals)...])
                if !key.isEmpty && !value.isEmpty {
                    values[key] = value
                }
            }
            index += 1
        }
        return values
    }

    private var defaultSocketExists: Bool {
        let socket = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/Loom/run/loomd.sock")
        return FileManager.default.fileExists(atPath: socket.path)
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

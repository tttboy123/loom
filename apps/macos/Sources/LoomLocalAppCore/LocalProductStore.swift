import CryptoKit
import Darwin
import Foundation
import SwiftUI

public struct LocalProductContextDisclosureIdentity: Hashable, Sendable {
  public let threadID: String
  public let segmentID: String
  public let contextCapsuleDigest: String
  public let disclosureReceiptDigest: String

  public init(
    threadID: String,
    segmentID: String,
    contextCapsuleDigest: String,
    disclosureReceiptDigest: String
  ) {
    self.threadID = threadID
    self.segmentID = segmentID
    self.contextCapsuleDigest = contextCapsuleDigest
    self.disclosureReceiptDigest = disclosureReceiptDigest
  }

  init(disclosure: LocalProductContextDisclosure) {
    self.init(
      threadID: disclosure.threadID,
      segmentID: disclosure.segmentID,
      contextCapsuleDigest: disclosure.contextCapsuleDigest,
      disclosureReceiptDigest: disclosure.disclosureReceiptDigest
    )
  }
}

enum LocalPrivateRegistryStorage {
  private static let directoryPermissions = mode_t(S_IRWXU)
  private static let filePermissions = mode_t(S_IRUSR | S_IWUSR)
  private static let legacyFilePermissions = mode_t(S_IRUSR | S_IWUSR | S_IRGRP | S_IROTH)
  private static let maximumFileBytes = 256 * 1_024

  private struct FileIdentity: Equatable {
    let device: dev_t
    let inode: ino_t
  }

  private enum StorageError: Error {
    case unsafePath
    case ioFailure
    case oversized
  }

  static func isSafeRegularFileMetadata(
    mode: mode_t,
    ownerUID: uid_t,
    linkCount: nlink_t,
    effectiveUID: uid_t = geteuid()
  ) -> Bool {
    mode & S_IFMT == S_IFREG
      && mode & mode_t(0o777) == filePermissions
      && ownerUID == effectiveUID
      && linkCount == 1
  }

  static func read(from url: URL) throws -> Data? {
    try withDirectoryDescriptor(for: url) { directoryDescriptor, fileName in
      let descriptor = fileName.withCString {
        openat(directoryDescriptor, $0, O_RDWR | O_NOFOLLOW)
      }
      if descriptor < 0 {
        guard errno == ENOENT else { throw StorageError.unsafePath }
        return nil
      }
      defer { Darwin.close(descriptor) }

      var status = stat()
      guard fstat(descriptor, &status) == 0 else {
        throw StorageError.unsafePath
      }
      if isLegacyRegularFileMetadata(
        mode: status.st_mode,
        ownerUID: status.st_uid,
        linkCount: status.st_nlink
      ) {
        guard fchmod(descriptor, filePermissions) == 0,
          fsync(descriptor) == 0,
          fstat(descriptor, &status) == 0
        else {
          throw StorageError.ioFailure
        }
      }
      guard isSafeRegularFileMetadata(
        mode: status.st_mode,
        ownerUID: status.st_uid,
        linkCount: status.st_nlink
      ),
        status.st_size >= 0,
        status.st_size <= maximumFileBytes
      else {
        throw StorageError.unsafePath
      }

      var data = Data(count: Int(status.st_size))
      var offset = 0
      let dataCount = data.count
      while offset < dataCount {
        let count = data.withUnsafeMutableBytes { bytes in
          Darwin.read(
            descriptor,
            bytes.baseAddress!.advanced(by: offset),
            dataCount - offset
          )
        }
        guard count > 0 else { throw StorageError.ioFailure }
        offset += count
      }
      return data
    }
  }

  private static func isLegacyRegularFileMetadata(
    mode: mode_t,
    ownerUID: uid_t,
    linkCount: nlink_t
  ) -> Bool {
    mode & S_IFMT == S_IFREG
      && mode & mode_t(0o777) == legacyFilePermissions
      && ownerUID == geteuid()
      && linkCount == 1
  }

  static func write(_ data: Data, to url: URL) throws {
    guard data.count <= maximumFileBytes else { throw StorageError.oversized }
    try withDirectoryDescriptor(for: url) { directoryDescriptor, fileName in
      let originalIdentity = try existingFileIdentity(
        directoryDescriptor: directoryDescriptor,
        fileName: fileName
      )
      let temporaryName = ".\(fileName).\(UUID().uuidString).tmp"
      let descriptor = temporaryName.withCString {
        openat(
          directoryDescriptor,
          $0,
          O_WRONLY | O_CREAT | O_EXCL | O_NOFOLLOW,
          filePermissions
        )
      }
      guard descriptor >= 0 else { throw StorageError.ioFailure }
      var shouldRemoveTemporary = true
      defer {
        Darwin.close(descriptor)
        if shouldRemoveTemporary {
          temporaryName.withCString { _ = unlinkat(directoryDescriptor, $0, 0) }
        }
      }

      var temporaryStatus = stat()
      guard fchmod(descriptor, filePermissions) == 0,
        fstat(descriptor, &temporaryStatus) == 0,
        isSafeRegularFileMetadata(
          mode: temporaryStatus.st_mode,
          ownerUID: temporaryStatus.st_uid,
          linkCount: temporaryStatus.st_nlink
        )
      else {
        throw StorageError.unsafePath
      }

      var offset = 0
      let dataCount = data.count
      while offset < dataCount {
        let count = data.withUnsafeBytes { bytes in
          Darwin.write(
            descriptor,
            bytes.baseAddress!.advanced(by: offset),
            dataCount - offset
          )
        }
        guard count > 0 else { throw StorageError.ioFailure }
        offset += count
      }
      guard fsync(descriptor) == 0 else { throw StorageError.ioFailure }

      let currentIdentity = try existingFileIdentity(
        directoryDescriptor: directoryDescriptor,
        fileName: fileName
      )
      guard currentIdentity == originalIdentity else { throw StorageError.unsafePath }
      let renamed = temporaryName.withCString { temporaryPointer in
        fileName.withCString { filePointer in
          renameat(directoryDescriptor, temporaryPointer, directoryDescriptor, filePointer)
        }
      }
      guard renamed == 0 else { throw StorageError.ioFailure }
      shouldRemoveTemporary = false
      _ = fsync(directoryDescriptor)
    }
  }

  private static func existingFileIdentity(
    directoryDescriptor: Int32,
    fileName: String
  ) throws -> FileIdentity? {
    var status = stat()
    let result = fileName.withCString {
      fstatat(directoryDescriptor, $0, &status, AT_SYMLINK_NOFOLLOW)
    }
    if result != 0 {
      guard errno == ENOENT else { throw StorageError.unsafePath }
      return nil
    }
    guard isSafeRegularFileMetadata(
      mode: status.st_mode,
      ownerUID: status.st_uid,
      linkCount: status.st_nlink
    ) else {
      throw StorageError.unsafePath
    }
    return FileIdentity(device: status.st_dev, inode: status.st_ino)
  }

  private static func withDirectoryDescriptor<T>(
    for url: URL,
    _ operation: (Int32, String) throws -> T
  ) throws -> T {
    guard url.isFileURL else { throw StorageError.unsafePath }
    let standardizedURL = url.standardizedFileURL
    guard standardizedURL.path == url.path else { throw StorageError.unsafePath }
    let directoryURL = standardizedURL.deletingLastPathComponent()
    let fileName = standardizedURL.lastPathComponent
    guard !fileName.isEmpty, fileName != ".", fileName != ".." else {
      throw StorageError.unsafePath
    }

    var directoryStatus = stat()
    if lstat(directoryURL.path, &directoryStatus) != 0 {
      guard errno == ENOENT,
        mkdir(directoryURL.path, directoryPermissions) == 0
      else {
        throw StorageError.unsafePath
      }
    }
    let directoryDescriptor = Darwin.open(
      directoryURL.path,
      O_RDONLY | O_DIRECTORY | O_NOFOLLOW
    )
    guard directoryDescriptor >= 0 else { throw StorageError.unsafePath }
    defer { Darwin.close(directoryDescriptor) }
    guard fstat(directoryDescriptor, &directoryStatus) == 0,
      directoryStatus.st_mode & S_IFMT == S_IFDIR,
      directoryStatus.st_mode & mode_t(0o777) == directoryPermissions,
      directoryStatus.st_uid == geteuid()
    else {
      throw StorageError.unsafePath
    }
    return try operation(directoryDescriptor, fileName)
  }
}

public enum LocalProductWorkspaceTaskKind: String, Equatable, Sendable {
  case draft
  case history
  case team
  case attention
}

public enum LocalProductInspectorTab: String, CaseIterable, Identifiable, Sendable {
  case team = "Team"
  case context = "Context"
  case changes = "Changes"
  case evidence = "Evidence"

  public var id: String { rawValue }
}

public struct LocalProductTaskContinuity: Equatable, Sendable {
  public var composerDraft: String
  public var conversationProfileID: String
  public var inspector: LocalProductInspectorTab
  public var threadAnchor: String
  public var developerDetailsExpanded: Bool

  public init(
    composerDraft: String = "",
    conversationProfileID: String = "",
    inspector: LocalProductInspectorTab = .team,
    threadAnchor: String = "",
    developerDetailsExpanded: Bool = false
  ) {
    self.composerDraft = composerDraft
    self.conversationProfileID = conversationProfileID
    self.inspector = inspector
    self.threadAnchor = threadAnchor
    self.developerDetailsExpanded = developerDetailsExpanded
  }
}

public struct LocalProductWorkspaceTask: Identifiable, Equatable, Sendable {
  public let id: String
  public let title: String
  public let subtitle: String
  public let kind: LocalProductWorkspaceTaskKind

  public init(
    id: String,
    title: String,
    subtitle: String,
    kind: LocalProductWorkspaceTaskKind
  ) {
    self.id = id
    self.title = title
    self.subtitle = subtitle
    self.kind = kind
  }
}

public struct LocalProductMissionPresentation: Codable, Equatable, Sendable {
  public let missionID: String
  public let title: String
  public let conversationThreadID: String
  public let workspacePath: String?
  public let updatedAt: Date

  public init(
    missionID: String,
    title: String,
    conversationThreadID: String = "",
    workspacePath: String? = nil,
    updatedAt: Date = Date()
  ) {
    self.missionID = missionID
    self.title = title
    self.conversationThreadID = conversationThreadID
    self.workspacePath = workspacePath
    self.updatedAt = updatedAt
  }
}

public struct LocalProductWorkspaceState: Equatable, Sendable {
  public static let newTaskID = "local:new-task"

  public private(set) var selectedTaskID: String
  public private(set) var tasks: [LocalProductWorkspaceTask]
  public private(set) var selectedFolderDisplayName: String?
  private var continuityByTask: [String: LocalProductTaskContinuity]

  public init() {
    let newTask = LocalProductWorkspaceTask(
      id: Self.newTaskID,
      title: "New task",
      subtitle: "Describe what you want to accomplish",
      kind: .draft
    )
    selectedTaskID = Self.newTaskID
    tasks = [newTask]
    selectedFolderDisplayName = nil
    continuityByTask = [Self.newTaskID: LocalProductTaskContinuity()]
  }

  public var selectedTask: LocalProductWorkspaceTask {
    tasks.first(where: { $0.id == selectedTaskID }) ?? tasks[0]
  }

  public var selectedContinuity: LocalProductTaskContinuity {
    continuityByTask[selectedTaskID] ?? LocalProductTaskContinuity()
  }

  public mutating func selectTask(_ id: String) {
    guard tasks.contains(where: { $0.id == id }) else { return }
    selectedTaskID = id
    if continuityByTask[id] == nil {
      continuityByTask[id] = LocalProductTaskContinuity()
    }
  }

  public mutating func updateComposerDraft(_ value: String) {
    mutateSelected { $0.composerDraft = String(value.prefix(4_096)) }
  }

  public mutating func selectConversationProfile(_ profileID: String) {
    mutateSelected {
      $0.conversationProfileID = String(profileID.prefix(256))
    }
  }

  public mutating func selectInspector(_ inspector: LocalProductInspectorTab) {
    mutateSelected { $0.inspector = inspector }
  }

  public mutating func updateThreadAnchor(_ value: String) {
    mutateSelected { $0.threadAnchor = String(value.prefix(256)) }
  }

  public mutating func currentThreadID() -> String {
    let anchor = selectedContinuity.threadAnchor
      .trimmingCharacters(in: .whitespacesAndNewlines)
    if !anchor.isEmpty {
      return anchor
    }

    let digest = SHA256.hash(data: Data(selectedTaskID.utf8))
      .map { String(format: "%02x", $0) }
      .joined()
    let threadID = "thread-\(digest.prefix(32))"
    updateThreadAnchor(threadID)
    return threadID
  }

  public mutating func selectFolderDisplayName(_ value: String) {
    let bounded = String(value.prefix(256))
      .replacingOccurrences(of: "/", with: " ")
      .replacingOccurrences(of: "\\", with: " ")
    let safe =
      bounded
      .components(separatedBy: .whitespacesAndNewlines)
      .filter { !$0.isEmpty }
      .joined(separator: " ")
    selectedFolderDisplayName = safe.isEmpty ? nil : safe
  }

  public mutating func setDeveloperDetailsExpanded(_ expanded: Bool) {
    mutateSelected { $0.developerDetailsExpanded = expanded }
  }

  public mutating func mergeAuthoritativeTasks(
    _ authoritative: [LocalProductWorkspaceTask]
  ) {
    let newTask =
      tasks.first(where: { $0.id == Self.newTaskID })
      ?? LocalProductWorkspaceTask(
        id: Self.newTaskID,
        title: "New task",
        subtitle: "Describe what you want to accomplish",
        kind: .draft
      )
    var seen = Set([Self.newTaskID])
    let bounded = authoritative.filter { task in
      !task.id.isEmpty && seen.insert(task.id).inserted
    }
    tasks = [newTask] + bounded
    for task in tasks where continuityByTask[task.id] == nil {
      continuityByTask[task.id] = LocalProductTaskContinuity()
    }
    continuityByTask = continuityByTask.filter { id, _ in
      tasks.contains(where: { $0.id == id })
    }
    if !tasks.contains(where: { $0.id == selectedTaskID }) {
      selectedTaskID = Self.newTaskID
    }
  }

  public func filteredTasks(matching query: String) -> [LocalProductWorkspaceTask] {
    let normalized = query.trimmingCharacters(
      in: .whitespacesAndNewlines
    )
    guard !normalized.isEmpty else { return tasks }
    let selected = selectedTaskID
    return tasks.filter { task in
      task.id == selected || task.title.localizedCaseInsensitiveContains(normalized)
        || task.subtitle.localizedCaseInsensitiveContains(normalized)
    }
  }

  private mutating func mutateSelected(
    _ mutation: (inout LocalProductTaskContinuity) -> Void
  ) {
    var continuity = selectedContinuity
    mutation(&continuity)
    continuityByTask[selectedTaskID] = continuity
  }
}

public enum LocalProductConnectionState: Equatable, Sendable {
  case loading
  case online
  case partial(reason: String)
  case stale(reason: String)
  case offline(reason: String)
  case fatal(reason: String)
}

public enum LocalProductTimelineState: Equatable, Sendable {
  case idle
  case loading
  case loaded
  case unavailable
  case fatal
}

public enum LocalProductSetupState: Equatable, Sendable {
  case idle
  case loading
  case ready
  case unavailable(reason: String)
  case fatal(reason: String)
}

public enum LocalProductExecutionState: Equatable, Sendable {
  case idle
  case preflighting
  case ready
  case starting
  case cancelling
  case running
  case awaitingRecovery
  case succeeded
  case cancelled
  case failed(reason: String)
}

public enum LocalProductConversationTrustBoundaryDimension:
  String, CaseIterable, Equatable, Sendable
{
  case trustDomain
  case retentionMode
  case dataRegion
}

public struct LocalProductConversationTrustBoundaryChange:
  Identifiable, Equatable, Sendable
{
  public var id: LocalProductConversationTrustBoundaryDimension { dimension }
  public let dimension: LocalProductConversationTrustBoundaryDimension
  public let sourceValue: String
  public let targetValue: String
}

public struct LocalProductConversationRouteTransition:
  Identifiable, Equatable, Sendable
{
  public let id: String
  public let source: LocalProductConversationProfile
  public let target: LocalProductConversationProfile
  public let threadID: String
  public let sourceSegmentID: String
  public let sourceBindingDigest: String
  public let sourceExecutionBinding: LocalProductConversationExecutionBinding?
  public let targetExecutionBinding: LocalProductConversationExecutionBinding
  public let sourceReasoningEffort: String
  public let targetReasoningEffort: String
  public let trustBoundaryChanges: [LocalProductConversationTrustBoundaryChange]
  public let trustBoundaryAuthorityUnavailable: Bool
  public let rebindsCurrentRoute: Bool
  fileprivate let generation: UInt64

  init(
    id: String = UUID().uuidString.lowercased(),
    source: LocalProductConversationProfile,
    target: LocalProductConversationProfile,
    threadID: String,
    sourceSegmentID: String = "",
    sourceBindingDigest: String = "",
    sourceExecutionBinding: LocalProductConversationExecutionBinding? = nil,
    targetExecutionBinding: LocalProductConversationExecutionBinding? = nil,
    sourceReasoningEffort: String = "",
    targetReasoningEffort: String = "",
    rebindsCurrentRoute: Bool = false,
    generation: UInt64
  ) {
    self.id = id
    self.source = source
    self.target = target
    self.threadID = threadID
    self.sourceSegmentID = sourceSegmentID
    self.sourceBindingDigest = sourceBindingDigest
    self.sourceExecutionBinding = sourceExecutionBinding
    self.targetExecutionBinding = targetExecutionBinding ??
      LocalProductConversationExecutionBinding(
        schemaVersion: 4,
        harnessAdapter: target.harnessAdapter,
        providerID: target.providerID,
        providerAccountID: target.providerAccountID,
        credentialRevision: target.credentialRevision,
        modelID: target.modelID,
        providerAccountPolicyVersion: target.policyVersion,
        providerAccountPolicyRevision: target.policyRevision,
        providerAccountPolicyDigest: target.policyDigest,
        trustDomain: target.trustDomain,
        retentionMode: target.retentionMode,
        dataRegion: target.dataRegion
      )
    self.sourceReasoningEffort = sourceReasoningEffort
    self.targetReasoningEffort = targetReasoningEffort
    let sourceAuthorityComplete = sourceExecutionBinding.map(
      conversationExecutionBindingHasCompletePolicyAuthority
    ) ?? false
    let targetAuthorityComplete =
      conversationExecutionBindingHasCompletePolicyAuthority(
        self.targetExecutionBinding
      )
    let reviewsAllTrustDimensions =
      !sourceAuthorityComplete || !targetAuthorityComplete
    trustBoundaryAuthorityUnavailable = reviewsAllTrustDimensions
    let sourceTrustDomain = normalizedConversationTrustAuthorityValue(
      sourceExecutionBinding?.trustDomain ?? ""
    )
    let sourceRetentionMode = normalizedConversationTrustAuthorityValue(
      sourceExecutionBinding?.retentionMode ?? ""
    )
    let sourceDataRegion = normalizedConversationTrustAuthorityValue(
      sourceExecutionBinding?.dataRegion ?? ""
    )
    trustBoundaryChanges = [
      LocalProductConversationTrustBoundaryChange(
        dimension: .trustDomain,
        sourceValue: sourceTrustDomain,
        targetValue: normalizedConversationTrustAuthorityValue(
          self.targetExecutionBinding.trustDomain
        )
      ),
      LocalProductConversationTrustBoundaryChange(
        dimension: .retentionMode,
        sourceValue: sourceRetentionMode,
        targetValue: normalizedConversationTrustAuthorityValue(
          self.targetExecutionBinding.retentionMode
        )
      ),
      LocalProductConversationTrustBoundaryChange(
        dimension: .dataRegion,
        sourceValue: sourceDataRegion,
        targetValue: normalizedConversationTrustAuthorityValue(
          self.targetExecutionBinding.dataRegion
        )
      ),
    ].filter {
      reviewsAllTrustDimensions || $0.sourceValue != $0.targetValue
    }
    self.rebindsCurrentRoute = rebindsCurrentRoute
    self.generation = generation
  }
}

public struct LocalProductWorkPackageOption: Identifiable, Equatable, Sendable {
  public let id: String
  public let title: String
  public let digest: String

  public static let coding = Self(
    id: "work-package.coding",
    title: "Coding",
    digest: "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f"
  )
  public static let knowledge = Self(
    id: "work-package.knowledge",
    title: "Knowledge",
    digest: "5a834baad4a0e4c557d96883f242860f4d136de208dd6721b85d8c2b8c5a0393"
  )
  public static let accepted = [coding, knowledge]
}

public enum LocalProductSection: String, CaseIterable, Identifiable, Sendable {
  case home = "Home"
  case work = "Work"
  case teams = "Teams"
  case inbox = "Inbox"
  case system = "System"
  case execution = "Execution"
  case production = "Production"

  public var id: String { rawValue }
}

public struct LocalProductChatOperationFailure: Equatable, Sendable {
  public let code: LocalIPCRemoteError.Code
  public let stage: LocalIPCRemoteError.Stage
  public let recoverable: Bool
  public let incidentID: String
	public let httpStatus: Int
	public let providerCode: String
	public let retryAfterSeconds: Int64
  public let title: String
  public let detail: String

  /// True when the failure is a provider-route problem (account balance,
  /// authentication, model availability, provider availability) where retrying
  /// in place keeps failing; the actionable recovery is to switch Provider.
  public var isRouteRecoveryAvailable: Bool {
    switch code {
    case .providerInsufficientBalance, .providerAuth,
         .providerModelUnavailable, .providerUnavailable,
         .conversationUnavailable:
      return true
    default:
      return false
    }
  }

  public init(
    code: LocalIPCRemoteError.Code,
    stage: LocalIPCRemoteError.Stage,
    recoverable: Bool,
    incidentID: String,
	httpStatus: Int = 0,
	providerCode: String = "",
	retryAfterSeconds: Int64 = 0,
    title: String,
    detail: String
  ) {
    self.code = code
    self.stage = stage
    self.recoverable = recoverable
    self.incidentID = incidentID
	self.httpStatus = httpStatus
	self.providerCode = providerCode
	self.retryAfterSeconds = retryAfterSeconds
    self.title = title
    self.detail = detail
  }
}

public protocol LocalProductClientProtocol {
  /// Whether this client can heartbeat-probe the resident service socket while
  /// the store currently believes it is online. Production IPC clients answer
  /// true; test doubles default to false so they never probe unexpectedly.
  var supportsHeartbeatProbe: Bool { get }

  /// Heartbeat probe against the resident service socket. Production clients
  /// round-trip a `ping` IPC call; test doubles without an implementation fall
  /// back to the default below. Declared on the protocol (not only in the
  /// extension) so existential dispatch reaches the conforming type's real
  /// implementation instead of statically selecting the default.
  func ping() async throws -> Bool

  func snapshot(limit: Int) async throws -> LocalProductSnapshot
  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage
  func chatThread(threadID: String) async throws -> LocalProductChatThread
  func chatContextDisclosure(
    threadID: String,
    segmentID: String
  ) async throws -> LocalProductContextDisclosure
  func cancelChatResponse(
    threadID: String,
    incidentID: String
  ) async throws
  func deleteChatThread(threadID: String) async throws
  func sendChatMessage(threadID: String, content: String) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String
  ) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    incidentID: String
  ) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    contextMode: LocalProductConversationContextMode?,
    incidentID: String
  ) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    contextMode: LocalProductConversationContextMode?,
    expectedExecutionBinding: LocalProductConversationExecutionBinding?,
    incidentID: String
  ) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    modelID: String,
    reasoningEffort: String,
    contextMode: LocalProductConversationContextMode?,
    expectedExecutionBinding: LocalProductConversationExecutionBinding?,
    trustBoundaryAcknowledgement: LocalProductTrustBoundaryAcknowledgement?,
    incidentID: String
  ) async throws -> LocalProductChatThread
  func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    modelID: String,
    reasoningEffort: String,
    contextMode: LocalProductConversationContextMode?,
    expectedExecutionBinding: LocalProductConversationExecutionBinding?,
    incidentID: String
  ) async throws -> LocalProductChatThread
}

extension LocalProductClientProtocol {
  public var supportsHeartbeatProbe: Bool { false }

  public func ping() async throws -> Bool {
    throw LocalProductClientError.unavailable
  }

  public func chatThread(threadID: String) async throws -> LocalProductChatThread {
    throw LocalProductClientError.unavailable
  }

  public func chatContextDisclosure(
    threadID: String,
    segmentID: String
  ) async throws -> LocalProductContextDisclosure {
    throw LocalProductClientError.unavailable
  }

  public func cancelChatResponse(
    threadID: String,
    incidentID: String
  ) async throws {
    throw LocalProductClientError.unavailable
  }

  public func deleteChatThread(threadID: String) async throws {
    throw LocalProductClientError.unavailable
  }

  public func sendChatMessage(threadID: String, content: String) async throws
    -> LocalProductChatThread
  {
    throw LocalProductClientError.unavailable
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String
  ) async throws -> LocalProductChatThread {
    try await sendChatMessage(threadID: threadID, content: content)
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    modelID: String,
    reasoningEffort: String,
    contextMode: LocalProductConversationContextMode?,
    expectedExecutionBinding: LocalProductConversationExecutionBinding?,
    trustBoundaryAcknowledgement: LocalProductTrustBoundaryAcknowledgement?,
    incidentID: String
  ) async throws -> LocalProductChatThread {
    try await sendChatMessage(
      threadID: threadID,
      content: content,
      profileID: profileID,
      modelID: modelID,
      reasoningEffort: reasoningEffort,
      contextMode: contextMode,
      expectedExecutionBinding: expectedExecutionBinding,
      incidentID: incidentID
    )
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    modelID: String = "",
    reasoningEffort: String = "",
    contextMode: LocalProductConversationContextMode? = nil,
    expectedExecutionBinding: LocalProductConversationExecutionBinding? = nil,
    incidentID: String = ""
  ) async throws -> LocalProductChatThread {
    // Default conformance for mocks: delegate to the context-carrying terminal
    // overload so profile, mode and binding are preserved.
    try await sendChatMessage(
      threadID: threadID,
      content: content,
      profileID: profileID,
      contextMode: contextMode,
      expectedExecutionBinding: expectedExecutionBinding,
      incidentID: incidentID
    )
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    contextMode: LocalProductConversationContextMode?,
    expectedExecutionBinding: LocalProductConversationExecutionBinding?,
    incidentID: String
  ) async throws -> LocalProductChatThread {
    try await sendChatMessage(
      threadID: threadID,
      content: content,
      profileID: profileID,
      contextMode: contextMode,
      incidentID: incidentID
    )
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    incidentID: String
  ) async throws -> LocalProductChatThread {
    try await sendChatMessage(
      threadID: threadID,
      content: content,
      profileID: profileID
    )
  }

  public func sendChatMessage(
    threadID: String,
    content: String,
    profileID: String,
    contextMode: LocalProductConversationContextMode?,
    incidentID: String
  ) async throws -> LocalProductChatThread {
    try await sendChatMessage(
      threadID: threadID,
      content: content,
      profileID: profileID,
      incidentID: incidentID
    )
  }
}

public protocol LocalProductDecisionClientProtocol {
  func readMissionDecision(
    _ command: LocalProductDecisionCommand
  ) async throws -> LocalProductDecisionSheet
  func decideMission(
    _ command: LocalProductDecisionCommand
  ) async throws -> LocalProductDecisionResult
}

public protocol LocalProductHandoffClientProtocol {
  func proposeSideTask(
    _ request: LocalProductSideTaskProposalRequest
  ) async throws -> LocalProductSideTaskProposalResult
  func createSideTask(
    _ request: LocalProductSideTaskCreateRequest
  ) async throws -> LocalProductSideTaskCreateResult
  func readSideTask(
    _ request: LocalProductSideTaskReadRequest
  ) async throws -> LocalProductSideTaskReadResult
  func decideSideTask(
    _ request: LocalProductSideTaskDecisionRequest
  ) async throws -> LocalProductSideTaskDecisionResult
}

public protocol LocalRoundtableClientProtocol {
  func roundtableCreateSession(
    _ request: LocalRoundtableSessionCreateRequest
  ) async throws -> LocalRoundtableView
  func roundtableAddSeat(
    _ request: LocalRoundtableAddSeatRequest
  ) async throws -> LocalRoundtableView
  func roundtableRetireSeat(
    _ request: LocalRoundtableRetireSeatRequest
  ) async throws -> LocalRoundtableView
  func roundtableOpenRound(
    _ request: LocalRoundtableOpenRoundRequest
  ) async throws -> LocalRoundtableView
  func roundtableProposeMessage(
    _ request: LocalRoundtableProposeMessageRequest
  ) async throws -> LocalRoundtableView
  func roundtableRelayMessage(
    _ request: LocalRoundtableRelayMessageRequest
  ) async throws -> LocalRoundtableView
  func roundtableAckMessage(
    _ request: LocalRoundtableAckMessageRequest
  ) async throws -> LocalRoundtableView
  func roundtableInsertMessage(
    _ request: LocalRoundtableInsertMessageRequest
  ) async throws -> LocalRoundtableView
  func roundtableDropMessage(
    _ request: LocalRoundtableDropMessageRequest
  ) async throws -> LocalRoundtableView
  func roundtableConclude(
    _ request: LocalRoundtableConcludeRequest
  ) async throws -> LocalRoundtableView
  func roundtableSnapshot(
    _ request: LocalRoundtableSnapshotRequest
  ) async throws -> LocalRoundtableView
}

public protocol LocalProductSetupClientProtocol {
  func setupSnapshot() async throws -> LocalProductSetupSnapshot
  func configureProviderAccountPolicy(
    providerID: String,
    providerAccountID: String,
    expectedRevision: Int64,
    maximumConcurrentAttempts: Int,
    dispatchWindowSeconds: Int64,
    maximumDispatchStarts: Int,
    maximumAssignedBudgetUnits: Int64,
    trustDomain: String,
    retentionMode: String,
    dataRegion: String
  ) async throws -> LocalProductProviderAccountPolicyResult
  func configureProviderModelRateCard(
    providerID: String,
    providerAccountID: String,
    modelID: String,
    expectedRevision: Int64,
    currency: String,
    inputTokenBasis: String,
    inputMicrounitsPerMillion: Int64,
    outputMicrounitsPerMillion: Int64,
    cacheReadMicrounitsPerMillion: Int64,
    cacheWriteMicrounitsPerMillion: Int64
  ) async throws -> LocalProductProviderModelRateCardResult
  func configureRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult
  func revokeRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentRevokeCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult
  func connectCodex() async throws -> LocalProductProviderConnectResult
  func startBuilder(
    source: String,
    sourceID: String,
    sourceVersion: Int,
    sourceDigest: String
  ) async throws -> LocalProductBuilderSession
  func answerBuilder(
    session: LocalProductBuilderSession,
    answer: String
  ) async throws -> LocalProductBuilderSession
  func editBuilder(
    session: LocalProductBuilderSession,
    field: String,
    value: String
  ) async throws -> LocalProductBuilderSession
  func editBuilder(
    session: LocalProductBuilderSession,
    field: String,
    value: String,
    roleAgentDefinitionID: String
  ) async throws -> LocalProductBuilderSession
  func validateBuilder(
    session: LocalProductBuilderSession
  ) async throws -> LocalProductBuilderSession
  func confirmBuilder(
    session: LocalProductBuilderSession,
    definitionID: String
  ) async throws -> LocalProductBuilderConfirmation
  func archiveTeam(
    definitionID: String,
    expectedHead: Int64
  ) async throws -> LocalProductSetupSavedTeam
  func restoreTeam(
    definitionID: String,
    expectedHead: Int64
  ) async throws -> LocalProductSetupSavedTeam
  func configureMiniMax(
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func verifyMiniMax(
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func replaceMiniMax(
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func revokeMiniMax(
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func configureCredential(
    providerID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func configureCredential(
    providerID: String,
    providerAccountID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func importCredentialCandidate(
    candidateID: String,
    providerID: String,
    providerAccountID: String
  ) async throws -> LocalProductCredentialSetupResult
  func approveEndpointCandidate(
    candidate: LocalProductCredentialImportCandidate,
    providerAccountID: String
  ) async throws -> LocalProductEndpointReviewResult
  func importReviewedEndpointCandidate(
    candidate: LocalProductCredentialImportCandidate,
    review: LocalProductEndpointReviewResult,
    providerAccountID: String
  ) async throws -> LocalProductCredentialSetupResult
  func runProviderFailureLab(
    scenario: String,
    providerAccountID: String,
    healthyPeerAccountID: String
  ) async throws -> LocalProductFailureLabResult
  func verifyCredential(
    providerID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func verifyCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func replaceCredential(
    providerID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func replaceCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult
  func revokeCredential(
    providerID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func revokeCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult
  func rotateCredentialVault() async throws
  func lockCredentialVault() async throws
  func unlockCredentialVault() async throws
  func resetCredentialVault() async throws
  func exportCredentialVault(
    passphrase: String,
    destination: String
  ) async throws -> LocalProductCredentialVaultExportResult
}

extension LocalProductSetupClientProtocol {
  public func editBuilder(
    session: LocalProductBuilderSession,
    field: String,
    value: String,
    roleAgentDefinitionID: String
  ) async throws -> LocalProductBuilderSession {
    guard roleAgentDefinitionID.isEmpty else {
      throw LocalProductClientError.invalidRequest
    }
    return try await editBuilder(session: session, field: field, value: value)
  }

  public func configureProviderAccountPolicy(
    providerID: String,
    providerAccountID: String,
    expectedRevision: Int64,
    maximumConcurrentAttempts: Int,
    dispatchWindowSeconds: Int64,
    maximumDispatchStarts: Int,
    maximumAssignedBudgetUnits: Int64,
    trustDomain: String,
    retentionMode: String,
    dataRegion: String
  ) async throws -> LocalProductProviderAccountPolicyResult {
    throw LocalProductClientError.unavailable
  }

  public func configureProviderModelRateCard(
    providerID: String,
    providerAccountID: String,
    modelID: String,
    expectedRevision: Int64,
    currency: String,
    inputTokenBasis: String,
    inputMicrounitsPerMillion: Int64,
    outputMicrounitsPerMillion: Int64,
    cacheReadMicrounitsPerMillion: Int64,
    cacheWriteMicrounitsPerMillion: Int64
  ) async throws -> LocalProductProviderModelRateCardResult {
    throw LocalProductClientError.unavailable
  }

  public func configureRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
    throw LocalProductClientError.unavailable
  }

  public func revokeRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentRevokeCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
    throw LocalProductClientError.unavailable
  }

  public func rotateCredentialVault() async throws {
    throw LocalProductClientError.unavailable
  }

  public func lockCredentialVault() async throws {
    throw LocalProductClientError.unavailable
  }

  public func unlockCredentialVault() async throws {
    throw LocalProductClientError.unavailable
  }

  public func resetCredentialVault() async throws {
    throw LocalProductClientError.unavailable
  }

  public func exportCredentialVault(
    passphrase: String,
    destination: String
  ) async throws -> LocalProductCredentialVaultExportResult {
    throw LocalProductClientError.unavailable
  }

  public func configureCredential(
    providerID: String,
    providerAccountID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerAccountID == providerID + ".primary" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await configureCredential(providerID: providerID, secret: secret)
  }

  public func importCredentialCandidate(
    candidateID: String,
    providerID: String,
    providerAccountID: String
  ) async throws -> LocalProductCredentialSetupResult {
    throw LocalProductClientError.unavailable
  }

  public func approveEndpointCandidate(
    candidate: LocalProductCredentialImportCandidate,
    providerAccountID: String
  ) async throws -> LocalProductEndpointReviewResult {
    throw LocalProductClientError.unavailable
  }

  public func importReviewedEndpointCandidate(
    candidate: LocalProductCredentialImportCandidate,
    review: LocalProductEndpointReviewResult,
    providerAccountID: String
  ) async throws -> LocalProductCredentialSetupResult {
    throw LocalProductClientError.unavailable
  }

  public func runProviderFailureLab(
    scenario: String,
    providerAccountID: String,
    healthyPeerAccountID: String
  ) async throws -> LocalProductFailureLabResult {
    throw LocalProductClientError.unavailable
  }

  public func verifyCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerAccountID == providerID + ".primary" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await verifyCredential(
      providerID: providerID,
      reference: reference,
      revision: revision
    )
  }

  public func replaceCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerAccountID == providerID + ".primary" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await replaceCredential(
      providerID: providerID,
      reference: reference,
      revision: revision,
      secret: secret
    )
  }

  public func revokeCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerAccountID == providerID + ".primary" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await revokeCredential(
      providerID: providerID,
      reference: reference,
      revision: revision
    )
  }

  public func configureCredential(
    providerID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerID == "minimax" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await configureMiniMax(secret: secret)
  }

  public func verifyCredential(
    providerID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerID == "minimax" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await verifyMiniMax(reference: reference, revision: revision)
  }

  public func replaceCredential(
    providerID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerID == "minimax" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await replaceMiniMax(
      reference: reference,
      revision: revision,
      secret: secret
    )
  }

  public func revokeCredential(
    providerID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    guard providerID == "minimax" else {
      throw LocalProductClientError.invalidRequest
    }
    return try await revokeMiniMax(reference: reference, revision: revision)
  }
}

@MainActor
public final class LocalProductStore: ObservableObject {
  @Published public private(set) var snapshot: LocalProductSnapshot?
  @Published public private(set) var timeline: LocalProductTimelinePage?
  @Published public private(set) var timelineState: LocalProductTimelineState = .idle
  @Published public private(set) var connectionState: LocalProductConnectionState = .loading
  @Published public private(set) var setupSnapshot: LocalProductSetupSnapshot?
  @Published public var selectedConversationModelID: String = ""
  @Published public var selectedConversationReasoningEffort: String = ""
  @Published public private(set) var conversationModelSelectionNotice: String?
  @Published public private(set) var builderSession: LocalProductBuilderSession?
  @Published public private(set) var setupState: LocalProductSetupState = .idle
  @Published public private(set) var lastConfirmation: LocalProductBuilderConfirmation?
  @Published public private(set) var builderRecoveryMessage: String?
  @Published public private(set) var credentialStatus: LocalProductCredentialSetupResult?
  @Published public private(set) var miniMaxVerificationStatus = "Idle"
  @Published public private(set) var isVerifyingMiniMax = false
  @Published public private(set) var providerOperationStatus: [String: String] = [:]
  @Published public private(set) var providerOperationDetail: [String: String] = [:]
  @Published public private(set) var providerOperationStage: [String: String] = [:]
  @Published public private(set) var providerOperationIncidentID: [String: String] = [:]
  @Published public private(set) var providerOperationRetryable: [String: Bool] = [:]
  @Published public private(set) var providersInFlight: Set<String> = []
  @Published public private(set) var approvedEndpointReviews:
    [String: LocalProductEndpointReviewResult] = [:]
  @Published public private(set) var failureLabResults:
    [LocalProductFailureLabResult] = []
  @Published public private(set) var failureLabInFlight = false
  @Published public private(set) var failureLabError: String?
  @Published public private(set) var providerPolicyAccountsInFlight: Set<String> = []
  @Published public private(set) var providerPolicyOperationDetail: [String: String] = [:]
  @Published public private(set) var providerRateCardsInFlight: Set<String> = []
  @Published public private(set) var providerRateCardOperationDetail: [String: String] = [:]
  @Published public private(set) var remoteToolEnrollmentsInFlight: Set<String> = []
  @Published public private(set) var remoteToolEnrollmentOperationDetail: [String: String] = [:]
  @Published public private(set) var remoteToolEnrollmentStage: [String: String] = [:]
  @Published public private(set) var remoteToolEnrollmentIncidentID: [String: String] = [:]
  @Published public private(set) var remoteToolEnrollmentRetryable: [String: Bool] = [:]
  @Published public private(set) var isRotatingCredentialVault = false
  @Published public private(set) var isUpdatingCredentialVaultLock = false
  @Published public private(set) var isResettingCredentialVault = false
  @Published public private(set) var isExportingCredentialVault = false
  @Published public private(set) var credentialVaultOperationDetail: String?
  @Published public private(set) var credentialVaultOperationFailed = false
  @Published public private(set) var providerConnectionStatus: LocalProductProviderConnectResult?
  @Published public private(set) var workspace = LocalProductWorkspaceState()
  @Published public private(set) var workbench = MissionWorkspaceState()
  @Published public private(set) var chatThread: LocalProductChatThread?
  @Published public private(set) var chatSessions: [LocalProductChatSession] = []
  @Published public private(set) var missionPresentations:
    [String: LocalProductMissionPresentation] = [:]
  @Published public private(set) var selectedChatSessionID: String = ""
  @Published public private(set) var activeChatResponsesByThreadID: [String: String] = [:]
  @Published public private(set) var cancellingChatResponseThreadIDs: Set<String> = []
  public var isSendingChatMessage: Bool {
    activeChatResponsesByThreadID[currentChatThreadID()] != nil
  }
  public var isCancellingChatResponse: Bool {
    cancellingChatResponseThreadIDs.contains(currentChatThreadID())
  }
  public var activeChatResponseThreadID: String? {
    let threadID = currentChatThreadID()
    return activeChatResponsesByThreadID[threadID] == nil ? nil : threadID
  }
  public var activeChatResponseIncidentID: String? {
    activeChatResponsesByThreadID[currentChatThreadID()]
  }
  @Published public private(set) var chatOperationFailure: LocalProductChatOperationFailure?
  @Published public private(set) var conversationContextDisclosures:
    [LocalProductContextDisclosureIdentity: LocalProductContextDisclosure] = [:]
  @Published public private(set) var conversationContextDisclosuresInFlight:
    Set<LocalProductContextDisclosureIdentity> = []
  @Published public private(set) var conversationContextDisclosureFailures:
    [LocalProductContextDisclosureIdentity: String] = [:]
  @Published public var conversationContextMode: LocalProductConversationContextMode = .summaryOnly
  @Published public private(set) var activeDecisionSheet: LocalProductDecisionSheet?
  @Published public private(set) var executionState: LocalProductExecutionState = .idle
  @Published public private(set) var executionPreflight: LocalProductExecutionPreflight?
  @Published public private(set) var executionResult: LocalProductExecutionResult?
  @Published public private(set) var agentInputsInFlight: Set<String> = []
  @Published public private(set) var agentInputReceipts: [String: LocalProductAgentInputReceipt] = [:]
  @Published public private(set) var agentInputFailures: [String: LocalProductChatOperationFailure] = [:]
  @Published public private(set) var agentRecoveryCandidates: [LocalProductAgentRecoveryCandidate] = []
  @Published public private(set) var agentRecoveryDecisions: [String: LocalProductAgentRecoveryDecision] = [:]
  @Published public private(set) var agentRecoveriesInFlight: Set<String> = []
  @Published public private(set) var isPreviewingAgentRecoveries = false
  @Published public private(set) var agentRecoveryFailure: LocalProductChatOperationFailure?
	@Published public private(set) var toolRecoveryCandidates: [LocalProductToolRecoveryCandidate] = []
	@Published public private(set) var toolRecoveryDecisions: [String: LocalProductToolRecoveryDecision] = [:]
	@Published public private(set) var toolRecoveriesInFlight: Set<String> = []
	@Published public private(set) var isPreviewingToolRecoveries = false
	@Published public private(set) var toolRecoveryFailure: LocalProductChatOperationFailure?
  @Published public private(set) var sideTaskProposal: LocalProductSideTaskProposalResult?
  @Published public private(set) var sideTaskOperationStatus = "Idle"
  @Published public private(set) var evolutionAssets: EvolutionAssetSnapshot?
  @Published public private(set) var evolutionAssetDiff: EvolutionAssetDiff?
  @Published public private(set) var evolutionAssetStatus = "Idle"
  @Published public private(set) var evolutionAssetJourneyID = LocalProductStore.initialJourneyID()
  @Published public private(set) var permissionSnapshot: PermissionSnapshot?
  @Published public private(set) var permissionAttention: PermissionAttention?
  @Published public private(set) var executionSnapshot: ExecutionSnapshot?
  @Published public private(set) var productionSnapshot: ProductionSnapshot?
  @Published public private(set) var roundtableError: String?
  @Published public private(set) var roundtableLastSessionID: String?
  @Published public var selectedSection: LocalProductSection = .home
  @Published public var selectedTeamID: String?

  private let client: LocalProductClientProtocol
  private let setupClient: LocalProductSetupClientProtocol?
  private let decisionClient: LocalProductDecisionClientProtocol?
  private let executionClient: LocalProductExecutionClientProtocol?
  private let agentInputClient: LocalProductAgentInputClientProtocol?
  private let agentRecoveryClient: LocalProductAgentRecoveryClientProtocol?
	private let toolRecoveryClient: LocalProductToolRecoveryClientProtocol?
  private let handoffClient: LocalProductHandoffClientProtocol?
  private let roundtableClient: LocalRoundtableClientProtocol?
  private let assetClient: LocalProductAssetClientProtocol?
  private let permissionClient: LocalProductPermissionClientProtocol?
  private let executionSnapshotClient: LocalProductExecutionSnapshotClientProtocol?
  private let productionSnapshotClient: LocalProductProductionSnapshotClientProtocol?
  private var selectedConversationWorkspacePath = ""
  private var executionObjective = ""
  private var executionWorkspacePath = ""
  private var executionNewAttempt = false
  private var executionConfirmedConstraints: [String] = []
  private var executionAcceptedDecisions: [String] = []
  private var executionPreflightGeneration: UInt64 = 0
  private var pendingSideTaskProposalRequest: LocalProductSideTaskProposalRequest?
  private var timelineLoadGeneration: UInt64 = 0
  private var chatGeneration: UInt64 = 0
  private var conversationRouteTransitionGeneration: UInt64 = 0
  private var hasPendingConversationRoute = false
  private var conversationRouteSourceProfileID = ""
  private var forceNewConversationSegment = false
  private var confirmedConversationExecutionBinding:
    LocalProductConversationExecutionBinding?
  private var confirmedConversationTrustBoundaryAcknowledgement:
    LocalProductTrustBoundaryAcknowledgement?

  private static let timelinePageLimit = 64
  private static let maximumTimelinePages = 8
  private static let maximumTimelineRecords = 512

  private enum TimelineLoadSelection: Equatable {
    case team(String)
    case mission(id: String, teamID: String)
  }

  private let chatSessionsFileURL: URL
  private let missionPresentationsFileURL: URL

  public init(
    client: LocalProductClientProtocol,
    initialSetupSnapshot: LocalProductSetupSnapshot? = nil,
    chatSessionsFileURL: URL? = nil,
    missionPresentationsFileURL: URL? = nil
  ) {
    if let chatSessionsFileURL {
      self.chatSessionsFileURL = chatSessionsFileURL
    } else {
      let home = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Application Support/Loom")
      self.chatSessionsFileURL = home.appendingPathComponent("chat-sessions.json")
    }
    if let missionPresentationsFileURL {
      self.missionPresentationsFileURL = missionPresentationsFileURL
    } else {
      self.missionPresentationsFileURL = self.chatSessionsFileURL
        .deletingLastPathComponent()
        .appendingPathComponent("mission-presentations.json")
    }
    self.client = client
    setupSnapshot = initialSetupSnapshot
    setupClient = client as? LocalProductSetupClientProtocol
    decisionClient = client as? LocalProductDecisionClientProtocol
    executionClient = client as? LocalProductExecutionClientProtocol
    agentInputClient = client as? LocalProductAgentInputClientProtocol
    agentRecoveryClient = client as? LocalProductAgentRecoveryClientProtocol
	toolRecoveryClient = client as? LocalProductToolRecoveryClientProtocol
    handoffClient = client as? LocalProductHandoffClientProtocol
    roundtableClient = client as? LocalRoundtableClientProtocol
    assetClient = client as? LocalProductAssetClientProtocol
    permissionClient = client as? LocalProductPermissionClientProtocol
    executionSnapshotClient = client as? LocalProductExecutionSnapshotClientProtocol
    productionSnapshotClient = client as? LocalProductProductionSnapshotClientProtocol
    let restoredEmptyChatRegistry = loadPersistedChatSessions()
    loadPersistedMissionPresentations()
    if chatSessions.isEmpty {
      let initialThreadID = restoredEmptyChatRegistry
        ? "thread-" + UUID().uuidString.lowercased()
        : workspace.currentThreadID()
      let session = LocalProductChatSession(
        threadID: initialThreadID,
        title: "Conversation"
      )
      chatSessions = [session]
      selectedChatSessionID = initialThreadID
      persistChatSessions()
    }
    if selectedChatSessionID.isEmpty
      || !chatSessions.contains(where: { $0.threadID == selectedChatSessionID })
    {
      selectedChatSessionID = chatSessions[0].threadID
      persistChatSessions()
    }
    workspace.updateThreadAnchor(selectedChatSessionID)
  }

  public var providerManagementReachable: Bool {
    setupClient != nil
  }

  public var missionExecutionReachable: Bool { executionClient != nil }

  public var agentInputReachable: Bool { agentInputClient != nil }

  public var agentRecoveryReachable: Bool { agentRecoveryClient != nil }

	public var toolRecoveryReachable: Bool { toolRecoveryClient != nil }

  public func canSendAgentInput(to node: LocalProductNode) -> Bool {
    currentAgentInputBinding(for: node) != nil && !agentInputsInFlight.contains(node.logicalNodeID)
  }

  public func sendAgentInput(
    to node: LocalProductNode,
    mode: LocalProductAgentInputMode,
    content: String
  ) async {
    guard let agentInputClient, let binding = currentAgentInputBinding(for: node),
      let data = content.data(using: .utf8), (1...32_768).contains(data.count),
      !content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    else {
      agentInputFailures[node.logicalNodeID] = Self.agentInputFailure(
        code: .invalidRequest, stage: .inputAdmission, recoverable: false,
        incidentID: "", title: "Input not sent",
        detail: "Choose a running Agent and enter a bounded instruction."
      )
      return
    }
    let incidentID = "loom-agent-input-\(UUID().uuidString.lowercased())"
    let request = LocalProductAgentInputRequest(
      segmentID: node.routeSegmentID, agentInstanceID: node.agentInstanceID,
      workItemID: node.workItemID, runID: node.runID,
      claimGeneration: binding.claimGeneration, mode: mode, content: data
    )
    agentInputsInFlight.insert(node.logicalNodeID)
    agentInputFailures[node.logicalNodeID] = nil
    defer { agentInputsInFlight.remove(node.logicalNodeID) }
    do {
      let receipt = try await agentInputClient.sendAgentInput(request, incidentID: incidentID)
      guard receipt.incidentID == incidentID, receipt.mode == mode else {
        throw LocalProductClientError.invalidResponse
      }
      agentInputReceipts[node.logicalNodeID] = receipt
    } catch let remote as LocalIPCRemoteError {
      agentInputFailures[node.logicalNodeID] = Self.agentInputFailure(
        code: remote.code, stage: remote.stage ?? .agentInputAdmission,
        recoverable: remote.recoverable, incidentID: remote.incidentID ?? incidentID,
        title: remote.code == .staleGeneration ? "Agent changed" : "Input not sent",
        detail: Self.agentInputRecoveryDetail(remote)
      )
    } catch {
      agentInputFailures[node.logicalNodeID] = Self.agentInputFailure(
        code: .stateUnavailable, stage: .udsTransport, recoverable: true,
        incidentID: incidentID, title: "Input not sent",
        detail: "The local Agent route is unavailable. Retry after the Mission refreshes."
      )
    }
  }

  public func previewAgentAttemptRecoveries() async {
    guard let agentRecoveryClient, !isPreviewingAgentRecoveries,
      agentRecoveriesInFlight.isEmpty
    else { return }
    let incidentID = Self.newAgentRecoveryIncidentID()
    isPreviewingAgentRecoveries = true
    agentRecoveryFailure = nil
    defer { isPreviewingAgentRecoveries = false }
    do {
      let response = try await agentRecoveryClient.recoverAgentAttempt(
        .preview, incidentID: incidentID
      )
      let digests = Set(response.candidates.map(\.candidateDigest))
      guard response.incidentID == incidentID, response.operation == .preview,
        response.decision == nil, response.resume == nil,
        digests.count == response.candidates.count
      else { throw LocalProductClientError.invalidResponse }
      agentRecoveryCandidates = response.candidates
      agentRecoveryDecisions.removeAll()
    } catch {
      agentRecoveryFailure = Self.agentRecoveryFailure(error, incidentID: incidentID)
    }
  }

  public func canConfirmAgentAttemptRecovery(
    _ candidate: LocalProductAgentRecoveryCandidate
  ) -> Bool {
    agentRecoveryClient != nil && agentRecoveryCandidates.contains(candidate)
      && candidate.status == .available
      && agentRecoveryDecisions[candidate.candidateDigest] == nil
      && !isPreviewingAgentRecoveries
      && !agentRecoveriesInFlight.contains(candidate.candidateDigest)
  }

  public func canResumeAgentAttemptRecovery(
    _ candidate: LocalProductAgentRecoveryCandidate
  ) -> Bool {
    agentRecoveryCandidates.contains(candidate)
      && agentRecoveryAuthorization(for: candidate) != nil
      && !isPreviewingAgentRecoveries
      && !agentRecoveriesInFlight.contains(candidate.candidateDigest)
  }

  public func confirmAgentAttemptRecovery(
    _ candidate: LocalProductAgentRecoveryCandidate,
    principalID: String = "local-user"
  ) async {
    guard let agentRecoveryClient else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    guard !isPreviewingAgentRecoveries,
      !agentRecoveriesInFlight.contains(candidate.candidateDigest)
    else { return }
    guard canConfirmAgentAttemptRecovery(candidate) else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    let incidentID = Self.newAgentRecoveryIncidentID()
    let decisionID = UUID().uuidString.lowercased()
    let request = LocalProductAgentRecoveryRequest.confirm(
      decisionID: decisionID, principalID: principalID,
      candidateDigest: candidate.candidateDigest,
      capabilityDigest: candidate.capabilityDigest
    )
    guard request.isValid else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    agentRecoveriesInFlight.insert(candidate.candidateDigest)
    agentRecoveryFailure = nil
    defer { agentRecoveriesInFlight.remove(candidate.candidateDigest) }
    do {
      let response = try await agentRecoveryClient.recoverAgentAttempt(
        request, incidentID: incidentID
      )
      guard response.incidentID == incidentID, response.operation == .confirm,
        let decision = response.decision, response.resume == nil,
        response.candidates.isEmpty, decision.decisionID == decisionID,
        decision.candidateDigest == candidate.candidateDigest,
        decision.capabilityDigest == candidate.capabilityDigest,
        decision.action == candidate.action
      else { throw LocalProductClientError.invalidResponse }
      agentRecoveryDecisions[candidate.candidateDigest] = decision
    } catch {
      agentRecoveryFailure = Self.agentRecoveryFailure(error, incidentID: incidentID)
    }
  }

  public func resumeAgentAttemptRecovery(
    _ candidate: LocalProductAgentRecoveryCandidate
  ) async {
    guard let agentRecoveryClient else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    guard !isPreviewingAgentRecoveries,
      !agentRecoveriesInFlight.contains(candidate.candidateDigest)
    else { return }
    guard canResumeAgentAttemptRecovery(candidate),
      let authorization = agentRecoveryAuthorization(for: candidate)
    else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    let incidentID = Self.newAgentRecoveryIncidentID()
    let request = LocalProductAgentRecoveryRequest.resume(
      decisionID: authorization.decisionID,
      candidateDigest: candidate.candidateDigest,
      capabilityDigest: candidate.capabilityDigest
    )
    guard request.isValid else {
      agentRecoveryFailure = Self.agentRecoveryApprovalRequiredFailure()
      return
    }
    agentRecoveriesInFlight.insert(candidate.candidateDigest)
    agentRecoveryFailure = nil
    defer { agentRecoveriesInFlight.remove(candidate.candidateDigest) }
    do {
      let response = try await agentRecoveryClient.recoverAgentAttempt(
        request, incidentID: incidentID
      )
      guard response.incidentID == incidentID, response.operation == .resume,
        let receipt = response.resume, response.decision == nil,
        response.candidates.isEmpty, receipt.status == "completed",
        receipt.decisionID == authorization.decisionID,
        receipt.candidateDigest == candidate.candidateDigest,
        receipt.capabilityDigest == candidate.capabilityDigest,
        receipt.attemptID == candidate.attemptID,
        receipt.runtimeInstanceID == candidate.runtimeInstanceID
      else { throw LocalProductClientError.invalidResponse }
      agentRecoveryCandidates.removeAll {
        $0.candidateDigest == candidate.candidateDigest
      }
      agentRecoveryDecisions[candidate.candidateDigest] = nil
      await refresh()
    } catch {
      agentRecoveryCandidates.removeAll {
        $0.candidateDigest == candidate.candidateDigest
      }
      agentRecoveryDecisions[candidate.candidateDigest] = nil
      agentRecoveryFailure = Self.agentRecoveryFailure(error, incidentID: incidentID)
    }
  }

  private struct AgentRecoveryAuthorization {
    let decisionID: String
  }

	public func previewToolRecoveries() async {
		guard let toolRecoveryClient, !isPreviewingToolRecoveries,
		  toolRecoveriesInFlight.isEmpty else { return }
		let incidentID = Self.newToolRecoveryIncidentID()
		isPreviewingToolRecoveries = true
		toolRecoveryFailure = nil
		defer { isPreviewingToolRecoveries = false }
		do {
			let response = try await toolRecoveryClient.recoverToolCall(
				.preview, incidentID: incidentID
			)
			let digests = Set(response.candidates.map(\.candidateDigest))
			guard response.incidentID == incidentID, response.operation == .preview,
			  response.decision == nil, digests.count == response.candidates.count
			else { throw LocalProductClientError.invalidResponse }
			toolRecoveryCandidates = response.candidates
			toolRecoveryDecisions.removeAll()
		} catch {
			toolRecoveryFailure = Self.toolRecoveryOperationFailure(error, incidentID: incidentID)
		}
	}

	public func canResolveToolRecovery(
		_ candidate: LocalProductToolRecoveryCandidate,
		action: LocalProductToolRecoveryAction
	) -> Bool {
		toolRecoveryClient != nil && toolRecoveryCandidates.contains(candidate)
		  && candidate.status == .available
		  && candidate.availableActions.contains(action)
		  && !isPreviewingToolRecoveries
		  && !toolRecoveriesInFlight.contains(candidate.candidateDigest)
	}

	public func resolveToolRecovery(
		_ candidate: LocalProductToolRecoveryCandidate,
		action: LocalProductToolRecoveryAction,
		principalID: String = "local-user"
	) async {
		guard let toolRecoveryClient else {
			toolRecoveryFailure = Self.toolRecoveryDecisionRequiredFailure()
			return
		}
		guard !toolRecoveriesInFlight.contains(candidate.candidateDigest) else {
			return
		}
		guard canResolveToolRecovery(candidate, action: action) else {
			toolRecoveryFailure = Self.toolRecoveryDecisionRequiredFailure()
			return
		}
		let incidentID = Self.newToolRecoveryIncidentID()
		let decisionID = UUID().uuidString.lowercased()
		let request = LocalProductToolRecoveryRequest.resolve(
			decisionID: decisionID, principalID: principalID, action: action,
			candidateDigest: candidate.candidateDigest
		)
		guard request.isValid else {
			toolRecoveryFailure = Self.toolRecoveryDecisionRequiredFailure()
			return
		}
		toolRecoveriesInFlight.insert(candidate.candidateDigest)
		toolRecoveryFailure = nil
		defer { toolRecoveriesInFlight.remove(candidate.candidateDigest) }
		do {
			let response = try await toolRecoveryClient.recoverToolCall(
				request, incidentID: incidentID
			)
			guard response.incidentID == incidentID, response.operation == .resolve,
			  response.candidates.isEmpty, let decision = response.decision,
			  decision.decisionID == decisionID,
			  decision.candidateDigest == candidate.candidateDigest,
			  decision.executionID == candidate.executionID,
			  decision.action == action
			else { throw LocalProductClientError.invalidResponse }
			toolRecoveryDecisions[candidate.candidateDigest] = decision
			toolRecoveryCandidates.removeAll {
				$0.candidateDigest == candidate.candidateDigest
			}
			await refreshExecutions()
		} catch {
			toolRecoveryCandidates.removeAll {
				$0.candidateDigest == candidate.candidateDigest
			}
			toolRecoveryDecisions[candidate.candidateDigest] = nil
			toolRecoveryFailure = Self.toolRecoveryOperationFailure(error, incidentID: incidentID)
		}
	}

  private func agentRecoveryAuthorization(
    for candidate: LocalProductAgentRecoveryCandidate
  ) -> AgentRecoveryAuthorization? {
    guard agentRecoveryClient != nil else { return nil }
    if let decision = agentRecoveryDecisions[candidate.candidateDigest],
      decision.candidateDigest == candidate.candidateDigest,
      decision.capabilityDigest == candidate.capabilityDigest,
      decision.action == candidate.action
    {
      return AgentRecoveryAuthorization(decisionID: decision.decisionID)
    }
    guard candidate.status == .authorized, !candidate.decisionID.isEmpty else {
      return nil
    }
    return AgentRecoveryAuthorization(decisionID: candidate.decisionID)
  }

  public var sideTaskHandoffReachable: Bool { handoffClient != nil }

  public var roundtableReachable: Bool { roundtableClient != nil }

  private func roundtableCorrelationID() -> String {
    UUID().uuidString.lowercased()
  }

  private func runRoundtable(
    _ operation: () async throws -> LocalRoundtableView
  ) async -> LocalRoundtableView? {
    do {
      let view = try await operation()
      roundtableError = nil
      return view
    } catch {
      roundtableError = Self.roundtableErrorMessage(error)
      return nil
    }
  }

  static func roundtableErrorMessage(_ error: Error) -> String {
    if let remote = error as? LocalIPCRemoteError {
      var suffix = ""
      if let stage = remote.stage, stage != .conversationDispatch {
        suffix = " (stage: \(stage.rawValue))"
      }
      return "Roundtable \(remote.code): \(remote.safeMessage)\(suffix)"
    }
    return "Roundtable unavailable: \(error.localizedDescription)"
  }

  public func roundtableCreateSession(
    sessionID: String,
    title: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    let boundedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let roundtableClient,
          !sessionID.isEmpty, !boundedTitle.isEmpty, !moderatorSeat.isEmpty else {
      roundtableError = "Roundtable invalid_request: session, title and moderator seat are required"
      return nil
    }
    let created = await runRoundtable {
      try await roundtableClient.roundtableCreateSession(
        LocalRoundtableSessionCreateRequest(
          schemaVersion: 1, sessionID: sessionID, moderatorSeat: moderatorSeat,
          title: boundedTitle, correlationID: roundtableCorrelationID()
        )
      )
    }
    if created != nil {
      roundtableLastSessionID = created?.session.id ?? sessionID
    }
    return created
  }

  /// Reopens an existing governed handoff session by ID so closing and
  /// reopening the Roundtable workbench never loses the user's session.
  public func roundtableLoadSession(
    sessionID: String
  ) async -> LocalRoundtableView? {
    let bounded = sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let roundtableClient, !bounded.isEmpty else {
      roundtableError = "Roundtable invalid_request: session is required"
      return nil
    }
    let view = await runRoundtable {
      try await roundtableClient.roundtableSnapshot(
        LocalRoundtableSnapshotRequest(
          schemaVersion: 1, sessionID: bounded
        )
      )
    }
    if view != nil {
      roundtableLastSessionID = bounded
    }
    return view
  }

  public func roundtableAddSeat(
    sessionID: String,
    seatID: String,
    displayName: String
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !seatID.isEmpty,
          !displayName.isEmpty else {
      roundtableError = "Roundtable invalid_request: seat identity and display name are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableAddSeat(
        LocalRoundtableAddSeatRequest(
          schemaVersion: 1, sessionID: sessionID, seatID: seatID,
          displayName: displayName, correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableRetireSeat(
    sessionID: String,
    seatID: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !seatID.isEmpty,
          !moderatorSeat.isEmpty, seatID != moderatorSeat else {
      roundtableError = "Roundtable invalid_request: a non-moderator seat is required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableRetireSeat(
        LocalRoundtableRetireSeatRequest(
          schemaVersion: 1, sessionID: sessionID, seatID: seatID,
          moderatorSeat: moderatorSeat,
          correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableOpenRound(
    sessionID: String,
    roundID: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !roundID.isEmpty,
          !moderatorSeat.isEmpty else {
      roundtableError = "Roundtable invalid_request: session, round and moderator seat are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableOpenRound(
        LocalRoundtableOpenRoundRequest(
          schemaVersion: 1, sessionID: sessionID, roundID: roundID,
          moderatorSeat: moderatorSeat, correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableProposeMessage(
    sessionID: String,
    roundID: String,
    messageID: String,
    body: String,
    writerSeat: String,
    targetSeat: String
  ) async -> LocalRoundtableView? {
    let boundedBody = body.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let roundtableClient, !sessionID.isEmpty, !roundID.isEmpty,
          !messageID.isEmpty, !boundedBody.isEmpty, !writerSeat.isEmpty,
          !targetSeat.isEmpty, boundedBody.utf8.count <= 8_192 else {
      roundtableError = "Roundtable invalid_body: message body must be 1...8192 bytes"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableProposeMessage(
        LocalRoundtableProposeMessageRequest(
          schemaVersion: 1, sessionID: sessionID, roundID: roundID,
          messageID: messageID, writerSeat: writerSeat, targetSeat: targetSeat,
          body: boundedBody, artifactRefs: [],
          correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableRelayMessage(
    sessionID: String,
    messageID: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !messageID.isEmpty,
          !moderatorSeat.isEmpty else {
      roundtableError = "Roundtable invalid_request: session, message and moderator seat are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableRelayMessage(
        LocalRoundtableRelayMessageRequest(
          schemaVersion: 1, sessionID: sessionID, messageID: messageID,
          moderatorSeat: moderatorSeat, correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableAckMessage(
    sessionID: String,
    messageID: String,
    seatID: String
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !messageID.isEmpty,
          !seatID.isEmpty else {
      roundtableError = "Roundtable invalid_request: session, message and seat are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableAckMessage(
        LocalRoundtableAckMessageRequest(
          schemaVersion: 1, sessionID: sessionID, messageID: messageID,
          seatID: seatID, correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableInsertMessage(
    sessionID: String,
    messageID: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !messageID.isEmpty,
          !moderatorSeat.isEmpty else {
      roundtableError = "Roundtable invalid_request: session, message and moderator seat are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableInsertMessage(
        LocalRoundtableInsertMessageRequest(
          schemaVersion: 1, sessionID: sessionID, messageID: messageID,
          moderatorSeat: moderatorSeat, correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableConclude(
    sessionID: String,
    moderatorSeat: String = "seat-moderator"
  ) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty, !moderatorSeat.isEmpty else {
      roundtableError = "Roundtable invalid_request: session and moderator seat are required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableConclude(
        LocalRoundtableConcludeRequest(
          schemaVersion: 1, sessionID: sessionID, moderatorSeat: moderatorSeat,
          correlationID: roundtableCorrelationID()
        )
      )
    }
  }

  public func roundtableSnapshot(sessionID: String) async -> LocalRoundtableView? {
    guard let roundtableClient, !sessionID.isEmpty else {
      roundtableError = "Roundtable invalid_request: session id is required"
      return nil
    }
    return await runRoundtable {
      try await roundtableClient.roundtableSnapshot(
        LocalRoundtableSnapshotRequest(schemaVersion: 1, sessionID: sessionID)
      )
    }
  }

  public func canCreateSideTask(for missionID: String) -> Bool {
    sideTaskParentBinding(missionID: missionID) != nil
  }

  public func canDecideSideTask(
    _ sideTask: LocalProductSideTaskSummary
  ) -> Bool {
    guard !sideTask.availableDecisions.isEmpty,
      let binding = projectedSideTaskParentBinding(
        missionID: sideTask.parentMissionID,
        teamInstanceID: sideTask.parentTeamInstanceID,
        workItemID: sideTask.parentTaskID,
        runID: sideTask.parentRunID,
        executionDigest: sideTask.parentExecutionDigest
      )
    else { return false }
    return binding.workItemID == sideTask.parentTaskID
      && binding.runID == sideTask.parentRunID
      && binding.claimGeneration == sideTask.parentClaimGeneration
      && binding.executionDigest == sideTask.parentExecutionDigest
  }

  public func proposeSideTask(
    missionID: String,
    purpose: String,
    mode: String,
    title: String,
    authorizedRequest: String
  ) async {
    let boundedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
    let boundedRequest = authorizedRequest.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let handoffClient, let snapshot,
      let binding = sideTaskParentBinding(missionID: missionID),
      (1...128).contains(boundedTitle.utf8.count),
      (1...4_096).contains(boundedRequest.utf8.count),
      ["research", "comparison", "diagnosis", "verification", "read_only_review"].contains(purpose),
      ["report_only", "decision_required", "merge_candidate"].contains(mode)
    else {
      sideTaskOperationStatus = "proposal_unavailable"
      return
    }
    let request = LocalProductSideTaskProposalRequest(
      parentMissionID: missionID,
      parentTeamInstanceID: binding.teamInstanceID,
      parentTaskID: binding.workItemID,
      parentRunID: binding.runID,
      parentClaimGeneration: binding.claimGeneration,
      parentExecutionDigest: binding.executionDigest,
      purpose: purpose,
      mode: mode,
      title: boundedTitle,
      authorizedRequest: boundedRequest,
      permissionScopes: [],
      decisionTimeoutSeconds: mode == "report_only" ? 0 : 900,
      expectedViewVersion: snapshot.viewVersion,
      correlationID: evolutionAssetJourneyID
    )
    sideTaskOperationStatus = "Proposing"
    do {
      let proposal = try await handoffClient.proposeSideTask(request)
      guard proposal.viewVersion == snapshot.viewVersion,
        proposal.purpose == purpose, proposal.mode == mode,
        proposal.title == boundedTitle
      else {
        throw LocalProductClientError.invalidResponse
      }
      pendingSideTaskProposalRequest = request
      sideTaskProposal = proposal
      sideTaskOperationStatus = "Confirmation required"
    } catch {
      pendingSideTaskProposalRequest = nil
      sideTaskProposal = nil
      sideTaskOperationStatus = closedClientReason(error)
    }
  }

  public func confirmSideTaskProposal() async {
    guard let handoffClient, let proposal = sideTaskProposal,
      let request = pendingSideTaskProposalRequest
    else {
      sideTaskOperationStatus = "proposal_required"
      return
    }
    sideTaskOperationStatus = "Creating"
    do {
      let result = try await handoffClient.createSideTask(
        LocalProductSideTaskCreateRequest(
          proposal: request,
          proposalDigest: proposal.proposalDigest,
          confirmed: true
        )
      )
      guard result.proposalDigest == proposal.proposalDigest else {
        throw LocalProductClientError.invalidResponse
      }
      pendingSideTaskProposalRequest = nil
      sideTaskProposal = nil
      sideTaskOperationStatus = missionHumanSideTaskStatus(result.status)
      await refresh()
    } catch {
      sideTaskOperationStatus = closedClientReason(error)
    }
  }

  public func discardSideTaskProposal() {
    pendingSideTaskProposalRequest = nil
    sideTaskProposal = nil
    sideTaskOperationStatus = "Idle"
  }

  public func decideSideTask(
    _ sideTask: LocalProductSideTaskSummary,
    decision: String
  ) async {
    guard let handoffClient, let snapshot, canDecideSideTask(sideTask),
      sideTask.availableDecisions.contains(decision),
      let binding = projectedSideTaskParentBinding(
        missionID: sideTask.parentMissionID,
        teamInstanceID: sideTask.parentTeamInstanceID,
        workItemID: sideTask.parentTaskID,
        runID: sideTask.parentRunID,
        executionDigest: sideTask.parentExecutionDigest
      ),
      binding.workItemID == sideTask.parentTaskID,
      binding.runID == sideTask.parentRunID,
      binding.claimGeneration == sideTask.parentClaimGeneration,
      binding.executionDigest == sideTask.parentExecutionDigest
    else {
      sideTaskOperationStatus = "decision_unavailable"
      return
    }
    let effectDigest = sideTaskDigest(
      [
        "parent-effect-v1", sideTask.sideTaskID, sideTask.parentTeamInstanceID,
        sideTask.parentTaskID, sideTask.parentRunID,
        String(sideTask.parentClaimGeneration), sideTask.parentExecutionDigest,
        sideTask.handoffDigest, decision,
      ]
    )
    let request = LocalProductSideTaskDecisionRequest(
      sideTask: sideTask,
      parentLogicalNodeID: binding.logicalNodeID,
      parentAttemptNumber: binding.attemptNumber,
      parentExecutionDigest: sideTask.parentExecutionDigest,
      decision: decision,
      effectDigest: effectDigest,
      expectedViewVersion: snapshot.viewVersion,
      correlationID: evolutionAssetJourneyID
    )
    sideTaskOperationStatus = "Applying decision"
    do {
      let result = try await handoffClient.decideSideTask(request)
      guard result.sideTaskID == sideTask.sideTaskID,
        result.decision == decision
      else {
        throw LocalProductClientError.invalidResponse
      }
      sideTaskOperationStatus = missionHumanSideTaskStatus(result.status)
      await refresh()
    } catch {
      sideTaskOperationStatus = closedClientReason(error)
    }
  }

  private struct SideTaskParentBinding {
    let teamInstanceID: String
    let logicalNodeID: String
    let attemptNumber: Int
    let workItemID: String
    let runID: String
    let claimGeneration: Int64
    let executionDigest: String
  }

  private func sideTaskParentBinding(
    missionID: String,
    workItemID: String = "",
    runID: String = ""
  ) -> SideTaskParentBinding? {
    guard let executionResult,
      executionResult.missionID == missionID,
      let mission = snapshot?.missions.first(where: { $0.missionID == missionID }),
      executionResult.teamInstanceID == mission.teamInstanceID
    else { return nil }
    return projectedSideTaskParentBinding(
      missionID: missionID,
      teamInstanceID: mission.teamInstanceID,
      workItemID: workItemID,
      runID: runID,
      executionDigest: executionResult.executionDigest
    )
  }

  private func projectedSideTaskParentBinding(
    missionID: String,
    teamInstanceID: String,
    workItemID: String,
    runID: String,
    executionDigest: String
  ) -> SideTaskParentBinding? {
    guard let snapshot, let timeline,
      timeline.gap == nil,
      executionDigest.count == 64,
      let mission = snapshot.missions.first(where: { $0.missionID == missionID }),
      mission.teamInstanceID == teamInstanceID,
      timeline.teamInstanceID == teamInstanceID,
      timeline.board.teamInstanceID == teamInstanceID,
      timeline.viewVersion == snapshot.viewVersion,
      let node = timeline.board.nodes.first(where: {
        !$0.workItemID.isEmpty && !$0.runID.isEmpty && $0.currentAttempt > 0
          && (workItemID.isEmpty || $0.workItemID == workItemID)
          && (runID.isEmpty || $0.runID == runID)
      }),
      let run = snapshot.runs.first(where: {
        $0.runID == node.runID && $0.workItemID == node.workItemID
      })
    else { return nil }
    return SideTaskParentBinding(
      teamInstanceID: teamInstanceID,
      logicalNodeID: node.logicalNodeID,
      attemptNumber: node.currentAttempt,
      workItemID: node.workItemID,
      runID: node.runID,
      claimGeneration: run.claimGeneration,
      executionDigest: executionDigest
    )
  }

  private func sideTaskDigest(_ fields: [String]) -> String {
    SHA256.hash(data: Data(fields.joined(separator: "\0").utf8))
      .map { String(format: "%02x", $0) }.joined()
  }

  private func missionHumanSideTaskStatus(_ value: String) -> String {
    value.replacingOccurrences(of: "_", with: " ").capitalized
  }

  public var missionCancelAvailable: Bool {
    currentMissionCancelBinding() != nil
  }

  public func preflightMission(
    objective: String,
    team: LocalProductTeamSummary,
    workPackage: LocalProductWorkPackageOption,
    confirmedConstraints: [String] = [],
    acceptedDecisions: [String] = [],
    newAttempt: Bool = false
  ) async {
    let workspacePath = selectedConversationWorkspacePath
    executionPreflightGeneration &+= 1
    let generation = executionPreflightGeneration
    executionState = .preflighting
    executionPreflight = nil
    executionResult = nil
    executionObjective = ""
    executionWorkspacePath = ""
    executionNewAttempt = newAttempt
    executionConfirmedConstraints = []
    executionAcceptedDecisions = []
    let bounded = objective.trimmingCharacters(in: .whitespacesAndNewlines)
    let constraints = canonicalMissionContextValues(confirmedConstraints)
    let decisions = canonicalMissionContextValues(acceptedDecisions)
    await refresh()
    guard generation == executionPreflightGeneration else { return }
    guard let executionClient,
      let snapshot,
      connectionState == .online,
      team.confirmed, team.executable, !team.readOnly,
      snapshot.teams.contains(where: {
        $0.teamInstanceID == team.teamInstanceID && $0.confirmed && $0.executable
          && !$0.readOnly
      }),
      (1...4_096).contains(bounded.utf8.count),
      let constraints, let decisions,
      Set(constraints + decisions).count == constraints.count + decisions.count,
      workPackage == .coding || workPackage == .knowledge
    else {
      executionState = .failed(reason: "preflight_unavailable")
      return
    }
    let missionID = "mission/\(team.teamInstanceID)"
    let command = LocalProductExecutionCommand.preflight(
      missionID: missionID,
      teamInstanceID: team.teamInstanceID,
      workPackageID: workPackage.id,
      workPackageDigest: workPackage.digest,
      objective: bounded,
      workspacePath: workspacePath,
      confirmedConstraints: constraints,
      acceptedDecisions: decisions,
      newAttempt: newAttempt,
      expectedViewVersion: snapshot.viewVersion,
      correlationID: UUID().uuidString.lowercased()
    )
    do {
      let envelope = try await executionClient.executeMission(command)
      guard generation == executionPreflightGeneration else { return }
      guard let preflight = envelope.preflight else {
        throw LocalProductClientError.invalidResponse
      }
      executionObjective = bounded
      executionWorkspacePath = command.workspacePath
      executionNewAttempt = preflight.newAttempt
      executionConfirmedConstraints = constraints
      executionAcceptedDecisions = decisions
      executionPreflight = preflight
      executionState =
        preflight.nodes.contains { $0.status == "ready" }
        ? .ready
        : .failed(reason: "agent_preflight_blocked")
      await refresh()
    } catch let remote as LocalIPCRemoteError {
      guard generation == executionPreflightGeneration else { return }
      executionState = .failed(reason: missionExecutionFailureReason(remote))
    } catch {
      guard generation == executionPreflightGeneration else { return }
      executionState = .failed(reason: closedClientReason(error))
    }
  }

  public func invalidateMissionPreflight() {
    switch executionState {
    case .idle, .preflighting, .ready, .failed:
      executionPreflightGeneration &+= 1
      executionPreflight = nil
      executionObjective = ""
      executionWorkspacePath = ""
      executionNewAttempt = false
      executionConfirmedConstraints = []
      executionAcceptedDecisions = []
      if executionResult == nil {
        executionState = .idle
      }
    case .starting, .cancelling, .running, .awaitingRecovery, .succeeded, .cancelled:
      break
    }
  }

  public func startPreflightedMission() async {
    guard let executionClient,
      let preflight = executionPreflight,
      !executionObjective.isEmpty,
      preflight.nodes.contains(where: { $0.status == "ready" })
    else {
      if executionResult != nil { return }
      executionState = .failed(reason: "preflight_required")
      return
    }

    executionState = .starting
    do {
      let command = LocalProductExecutionCommand.start(
        preflight: preflight,
        objective: executionObjective,
        workspacePath: executionWorkspacePath,
        confirmedConstraints: executionConfirmedConstraints,
        acceptedDecisions: executionAcceptedDecisions,
        newAttempt: executionNewAttempt,
        correlationID: UUID().uuidString.lowercased()
      )
      let envelope = try await executionClient.executeMission(command)
      guard let result = envelope.result else {
        throw LocalProductClientError.invalidResponse
      }
      executionPreflight = nil
      executionResult = result
      switch result.status {
      case "running": executionState = .running
      case "awaiting_recovery": executionState = .awaitingRecovery
      case "succeeded": executionState = .succeeded
      default: executionState = .failed(reason: result.status)
      }
      await refresh()
      guard
        snapshot?.missions.contains(where: {
          $0.missionID == result.missionID && $0.teamInstanceID == result.teamInstanceID
        }) == true
      else {
        executionState = .failed(reason: "projection_visibility_failed")
        return
      }
      await openMissionAndActivate(result.missionID)
    } catch let remote as LocalIPCRemoteError {
      // A governed start can race a projection update after the user reviews
      // preflight. Refresh the authoritative view so the UI invalidates the
      // stale digest and offers Review preflight again, without auto-starting
      // a changed plan or silently bypassing confirmation.
      if remote.code == .conflict || remote.code == .staleView {
        await refresh()
      }
      executionState = .failed(reason: missionExecutionFailureReason(remote))
    } catch {
      executionState = .failed(reason: closedClientReason(error))
    }
  }

  private func canonicalMissionContextValues(_ values: [String]) -> [String]? {
    guard values.count <= 64 else { return nil }
    var seen = Set<String>()
    var result: [String] = []
    for value in values {
      let bounded = value.trimmingCharacters(in: .whitespacesAndNewlines)
      guard bounded == value, (1...4_096).contains(value.utf8.count),
        seen.insert(value).inserted
      else { return nil }
      result.append(value)
    }
    return result
  }

  private func missionExecutionFailureReason(_ remote: LocalIPCRemoteError) -> String {
    guard remote.code == .conflict, let stage = remote.stage else {
      return remote.code.rawValue
    }
    return "conflict.\(stage.rawValue)"
  }

  public func cancelCurrentMission() async {
    guard let executionClient,
      let result = executionResult,
      let binding = currentMissionCancelBinding()
    else {
      executionState = .failed(reason: "cancel_unavailable")
      return
    }
    executionState = .cancelling
    let command = LocalProductExecutionCommand.control(
      missionID: result.missionID,
      teamInstanceID: result.teamInstanceID,
      expectedViewVersion: binding.viewVersion,
      action: "cancel",
      executionDigest: result.executionDigest,
      logicalNodeID: binding.logicalNodeID,
      attemptNumber: binding.attemptNumber,
      claimGeneration: binding.claimGeneration,
      correlationID: UUID().uuidString.lowercased()
    )
    do {
      let envelope = try await executionClient.executeMission(command)
      guard let cancelled = envelope.result,
        cancelled.status == "cancelled",
        cancelled.missionID == result.missionID,
        cancelled.teamInstanceID == result.teamInstanceID,
        cancelled.executionDigest == result.executionDigest
      else {
        throw LocalProductClientError.invalidResponse
      }
      executionResult = cancelled
      await refreshMission(cancelled.missionID)
      guard
        snapshot?.missions.contains(where: {
          $0.missionID == cancelled.missionID && $0.status == "cancelled"
        }) == true
      else {
        executionState = .failed(reason: "cancel_visibility_failed")
        return
      }
      executionState = .cancelled
    } catch {
      executionState = .failed(reason: closedClientReason(error))
    }
  }

  private struct MissionCancelBinding {
    let viewVersion: String
    let logicalNodeID: String
    let attemptNumber: Int
    let claimGeneration: Int64
  }

  private struct AgentInputBinding {
    let claimGeneration: Int64
  }

  private func currentAgentInputBinding(for node: LocalProductNode) -> AgentInputBinding? {
    guard agentInputClient != nil, let snapshot, let timeline,
      timeline.gap == nil,
      timeline.viewVersion == snapshot.viewVersion,
      timeline.board.viewVersion == snapshot.viewVersion,
      timeline.board.nodes.contains(node), node.currentAttempt > 0,
      node.executionBindingAvailable, node.contextCapsuleAvailable,
      node.routeSegmentAvailable, !node.routeSegmentID.isEmpty,
      !node.agentInstanceID.isEmpty, !node.workItemID.isEmpty, !node.runID.isEmpty,
      !["succeeded", "failed", "cancelled", "blocked"].contains(node.status),
      let run = snapshot.runs.first(where: {
        $0.runID == node.runID && $0.workItemID == node.workItemID
          && $0.agentInstanceID == node.agentInstanceID
          && $0.claimGeneration > 0 && $0.terminalStatus.isEmpty
      })
    else { return nil }
    return AgentInputBinding(claimGeneration: run.claimGeneration)
  }

  private static func agentInputFailure(
    code: LocalIPCRemoteError.Code,
    stage: LocalIPCRemoteError.Stage,
    recoverable: Bool,
    incidentID: String,
    title: String,
    detail: String
  ) -> LocalProductChatOperationFailure {
    LocalProductChatOperationFailure(
      code: code, stage: stage, recoverable: recoverable,
      incidentID: incidentID, title: title, detail: detail
    )
  }

  private static func agentInputRecoveryDetail(_ remote: LocalIPCRemoteError) -> String {
    switch remote.code {
    case .staleGeneration, .notFound:
      return "This Attempt is no longer active. Refresh the Mission and choose its current Agent."
    case .conflict:
      return "The Agent advanced while this input was being admitted. Refresh and retry."
    case .invalidRequest:
      return "The input or Agent target is invalid. Review the mode and try again."
    default:
      return remote.recoverable
        ? "The local Agent route could not admit this input. Retry after the Mission refreshes."
        : "This Agent cannot accept the requested input in its current state."
    }
  }

  private static func newAgentRecoveryIncidentID() -> String {
    "loom-agent-recovery-\(UUID().uuidString.lowercased())"
  }

	private static func newToolRecoveryIncidentID() -> String {
		"loom-tool-recovery-\(UUID().uuidString.lowercased())"
	}

	private static func toolRecoveryDecisionRequiredFailure()
		-> LocalProductChatOperationFailure
	{
		LocalProductChatOperationFailure(
			code: .humanRequired, stage: .toolRecovery,
			recoverable: false, incidentID: "", title: "Tool recovery review required",
			detail: "Refresh the exact ToolCall candidate and choose one of its available governed actions. Loom will not rerun the original call."
		)
	}

	private static func toolRecoveryOperationFailure(
		_ error: Error,
		incidentID: String
	) -> LocalProductChatOperationFailure {
		let remote: LocalIPCRemoteError
		if let value = error as? LocalIPCRemoteError {
			remote = value
		} else if error as? LocalProductClientError == .invalidRequest {
			remote = LocalIPCRemoteError(
				code: .invalidRequest, recoverable: false,
				stage: .toolRecovery, incidentID: incidentID
			)
		} else {
			remote = LocalIPCRemoteError(
				code: .stateUnavailable, recoverable: true,
				stage: .udsTransport, incidentID: incidentID
			)
		}
		let presentation: (String, String)
		switch remote.code {
		case .conflict, .staleGeneration, .digestMismatch, .notFound:
			presentation = (
				"Tool recovery state changed",
				"Refresh the candidate before deciding again. The original ToolCall was not rerun."
			)
		case .denied, .humanRequired, .unauthorizedPeer:
			presentation = (
				"Tool recovery action unavailable",
				"This action has no trusted evidence or replacement Attempt. Choose an available action after refresh."
			)
		case .timeout:
			presentation = (
				"Tool recovery outcome unknown",
				"Refresh before another decision. Do not assume the previous decision was uncommitted."
			)
		default:
			presentation = (
				"Tool recovery unavailable",
				"Loom could not record this decision. Refresh the candidate and review diagnostics."
			)
		}
		return LocalProductChatOperationFailure(
			code: remote.code, stage: remote.stage ?? .toolRecovery,
			recoverable: remote.recoverable,
			incidentID: remote.incidentID ?? incidentID,
			title: presentation.0, detail: presentation.1
		)
	}

  private static func agentRecoveryApprovalRequiredFailure()
    -> LocalProductChatOperationFailure
  {
    LocalProductChatOperationFailure(
      code: .humanRequired, stage: .agentAttemptReconcile,
      recoverable: false, incidentID: "", title: "Recovery approval required",
      detail: "Review and confirm this exact Agent, Provider Account, model, and credential revision before resuming."
    )
  }

  private static func agentRecoveryFailure(
    _ error: Error,
    incidentID: String
  ) -> LocalProductChatOperationFailure {
    let remote: LocalIPCRemoteError
    if let value = error as? LocalIPCRemoteError {
      remote = value
    } else if let local = error as? LocalProductClientError {
      switch local {
      case .invalidRequest:
        remote = LocalIPCRemoteError(
          code: .invalidRequest, recoverable: false,
          stage: .agentAttemptReconcile, incidentID: incidentID
        )
      case .timeout:
        remote = LocalIPCRemoteError(
          code: .timeout, recoverable: true,
          stage: .udsTransport, incidentID: incidentID
        )
      case .invalidSocket, .invalidResponse, .unavailable, .notFound:
        remote = LocalIPCRemoteError(
          code: .stateUnavailable, recoverable: true,
          stage: .udsTransport, incidentID: incidentID
        )
      }
    } else {
      remote = LocalIPCRemoteError(
        code: .internal, recoverable: true,
        stage: .udsTransport, incidentID: incidentID
      )
    }
    let stage = remote.stage ?? .agentAttemptReconcile
    let presentation: (String, String)
    switch remote.code {
    case .conflict, .staleGeneration, .digestMismatch, .notFound:
      presentation = (
        "Recovery state changed",
        "Refresh recovery candidates before making another decision. No Provider work was started by this failed request."
      )
    case .humanRequired:
      presentation = (
        "Recovery approval required",
        "Review and confirm the exact recovery candidate before resuming."
      )
    case .denied, .unauthorizedPeer:
      presentation = (
        "Recovery denied",
        "This recovery decision is not authorized. Refresh and review its current authority."
      )
    case .timeout:
      presentation = (
        "Recovery timed out",
        "Loom could not confirm the recovery outcome. Refresh candidates before retrying."
      )
    case .invalidRequest:
      presentation = (
        "Recovery request rejected",
        "The recovery candidate or decision is invalid. Refresh before retrying."
      )
    default:
      presentation = (
        "Recovery unavailable",
        "Loom could not complete this Agent recovery action. Refresh and review diagnostics."
      )
    }
    return LocalProductChatOperationFailure(
      code: remote.code, stage: stage, recoverable: remote.recoverable,
      incidentID: remote.incidentID ?? incidentID,
      title: presentation.0, detail: presentation.1
    )
  }

  private func currentMissionCancelBinding() -> MissionCancelBinding? {
    guard let result = executionResult,
      let snapshot,
      let timeline,
      timeline.gap == nil,
      case .mission(let selectedMissionID) = workbench.route,
      selectedMissionID == result.missionID,
      timeline.teamInstanceID == result.teamInstanceID,
      timeline.board.teamInstanceID == result.teamInstanceID,
      timeline.viewVersion == snapshot.viewVersion,
      timeline.board.viewVersion == snapshot.viewVersion,
      result.executionDigest.count == 64
    else { return nil }
    for node in timeline.board.nodes
    where
      node.currentAttempt > 0 && !node.runID.isEmpty
      && !(["succeeded", "failed", "cancelled"].contains(node.status))
    {
      guard
        let run = snapshot.runs.first(where: {
          $0.runID == node.runID && $0.claimGeneration > 0 && $0.terminalStatus.isEmpty
        })
      else { continue }
      return MissionCancelBinding(
        viewVersion: snapshot.viewVersion,
        logicalNodeID: node.logicalNodeID,
        attemptNumber: node.currentAttempt,
        claimGeneration: run.claimGeneration
      )
    }
    return nil
  }

  public var teamTimelineMessage: String {
    switch timelineState {
    case .idle:
      return "Select a Team from Teams to open its authoritative timeline."
    case .loading:
      return "Loading Team timeline."
    case .loaded:
      return ""
    case .unavailable:
      return "Team timeline is unavailable."
    case .fatal:
      return "Team timeline needs attention."
    }
  }

  public static let teamsEmptyMessage = "No Teams exist in this Journal yet."

  public func refresh(
    preserveColdStartLoadingOnFailure: Bool = false
  ) async {
    if snapshot == nil {
      connectionState = .loading
    }
    do {
      let next = try await client.snapshot(limit: 64)
      snapshot = next
      if let preflight = executionPreflight,
        preflight.viewVersion != next.viewVersion
      {
        executionPreflight = nil
        if executionResult == nil {
          executionState = .failed(reason: "preflight_expired")
        }
      }
      reconcileWorkspace()
      reconcileMissions()
      if next.stale {
        connectionState = .stale(
          reason: closedReason(next.reason, fallback: "stale_view")
        )
      } else if next.partial && isUserVisibleDegradation(next) {
        connectionState = .partial(
          reason: closedReason(next.reason, fallback: "partial_view")
        )
      } else {
        connectionState = .online
      }
      if let selectedTeamID,
        !next.teams.contains(where: { $0.teamInstanceID == selectedTeamID })
      {
        invalidateTimelineLoad()
        self.selectedTeamID = nil
        timeline = nil
        timelineState = .idle
      }
    } catch let remote as LocalIPCRemoteError {
      if remote.recoverable && preserveColdStartLoadingOnFailure && snapshot == nil {
        connectionState = .loading
      } else {
        connectionState =
          remote.recoverable
          ? .offline(reason: remote.code.rawValue)
          : .fatal(reason: remote.code.rawValue)
      }
    } catch {
      connectionState = preserveColdStartLoadingOnFailure && snapshot == nil
        ? .loading
        : .offline(reason: closedClientReason(error))
    }
  }

  public func connectWithRetry(
    maxAttempts: Int = 12,
    delayNanoseconds: UInt64 = 250_000_000
  ) async {
    let boundedAttempts = min(max(maxAttempts, 1), 20)
    for attempt in 0..<boundedAttempts {
      await refresh(
        preserveColdStartLoadingOnFailure: snapshot == nil
          && attempt + 1 < boundedAttempts
      )
      let shouldRetry: Bool
      switch connectionState {
      case .loading, .offline:
        shouldRetry = true
      default:
        shouldRetry = false
      }
      guard shouldRetry, attempt + 1 < boundedAttempts else { return }
      do {
        try await Task.sleep(nanoseconds: delayNanoseconds)
      } catch {
        return
      }
    }
  }

  /// Keeps watching the resident service after the initial bounded retry window
  /// and re-establishes the connection on its own whenever it becomes
  /// unavailable, stale, or partial. Rehydrates the refresh-dependent surfaces
  /// on every transition back to online. Runs until the enclosing task is
  /// cancelled (the hosting view owns the task lifecycle).
  public func reconnectWhileUnavailable(
    pollNanoseconds: UInt64 = 5_000_000_000,
    retryDelayNanoseconds: UInt64 = 250_000_000
  ) async {
    var wasOnlineAtLastPoll = connectionState == .online
    while !Task.isCancelled {
      if connectionState != .online {
        await connectWithRetry(
          maxAttempts: 12,
          delayNanoseconds: retryDelayNanoseconds
        )
      } else if client.supportsHeartbeatProbe {
        // While nominally online, probe the resident socket so a silent daemon
        // exit is detected within the poll window instead of on the next user
        // action. A failed probe flips the store offline and the next poll
        // reconnects on its own as soon as the daemon is back.
        do {
          _ = try await client.ping()
        } catch {
          connectionState = .offline(reason: closedClientReason(error))
        }
      }
      let isOnline = connectionState == .online
      // Product snapshot and setup/profile snapshot are independent IPC
      // surfaces. A healthy daemon can answer the former while a transient
      // startup failure leaves the latter unavailable, which must recover
      // without asking the user to restart Loom or open Runtime & Providers.
      let setupNeedsRecovery: Bool
      switch setupState {
      case .idle, .unavailable:
        setupNeedsRecovery = true
      case .loading, .ready, .fatal:
        setupNeedsRecovery = setupSnapshot == nil ||
          !(setupSnapshot.map(hasUsableSetupInventory) ?? false) ||
          (setupSnapshot.map(hasPendingConversationProjection) ?? false)
      }
      if isOnline && (!wasOnlineAtLastPoll || setupNeedsRecovery) {
        async let setup: Void = refreshSetup()
        async let permissions: Void = refreshPermissions()
        async let executions: Void = refreshExecutions()
        async let production: Void = refreshProduction()
        _ = await (setup, permissions, executions, production)
      }
      wasOnlineAtLastPoll = isOnline
      do {
        try await Task.sleep(nanoseconds: pollNanoseconds)
      } catch {
        return
      }
    }
  }

  /// Confirmed, executable, non-read-only Teams a Mission can run on.
  public var executableTeams: [LocalProductTeamSummary] {
    currentTeamConfigurations(snapshot?.teams ?? []).filter {
      $0.confirmed && $0.executable && !$0.readOnly
    }
  }

  public func selectTeam(_ team: LocalProductTeamSummary) {
    invalidateTimelineLoad()
    selectedTeamID = team.teamInstanceID
    selectedSection = .teams
    timeline = nil
    timelineState = .loading
  }

  public func activateSelectedTeam() async {
    guard let selectedTeamID,
      snapshot?.teams.contains(where: {
        $0.teamInstanceID == selectedTeamID
      }) == true
    else {
      timelineState = .idle
      return
    }
    await loadTimeline(
      teamInstanceID: selectedTeamID,
      selection: .team(selectedTeamID)
    )
  }

  private func loadTimeline(
    teamInstanceID: String,
    selection: TimelineLoadSelection,
    preserveObservedTentativeRecords: Bool = false
  ) async {
    let retainedTentativeRecords: [LocalProductTimelineRecord]
    if preserveObservedTentativeRecords,
      timeline?.teamInstanceID == teamInstanceID
    {
      retainedTentativeRecords = timeline?.records.filter {
        $0.authority == "tentative" && !$0.payload.textDelta.isEmpty
      } ?? []
    } else {
      retainedTentativeRecords = []
    }
    let generation = beginTimelineLoad()
    if !preserveObservedTentativeRecords || timeline == nil {
      timeline = nil
      timelineState = .loading
    }
    do {
      var cursor = ""
      var seenCursors = Set<String>()
      var seenDeliveryIDs = Set<String>()
      var records: [LocalProductTimelineRecord] = []
      var firstPage: LocalProductTimelinePage?

      for pageIndex in 0..<Self.maximumTimelinePages {
        try Task.checkCancellation()
        let page = try await client.timeline(
          teamInstanceID: teamInstanceID,
          cursor: cursor,
          limit: Self.timelinePageLimit
        )
        try Task.checkCancellation()
        guard
          timelineLoadIsCurrent(
            generation: generation,
            selection: selection
          )
        else { return }
        try Self.validateTimelinePage(
          page,
          teamInstanceID: teamInstanceID,
          firstPage: firstPage,
          seenDeliveryIDs: &seenDeliveryIDs
        )
        if page.gap != nil {
          guard pageIndex == 0, firstPage == nil, !page.hasMore,
            page.nextCursor.isEmpty
          else {
            throw LocalProductClientError.invalidResponse
          }
          timeline = LocalProductTimelinePage(
            schemaVersion: page.schemaVersion,
            teamInstanceID: page.teamInstanceID,
            viewVersion: page.viewVersion,
            nextCursor: page.nextCursor,
            hasMore: page.hasMore,
            gap: page.gap,
            records: Self.timelineRecords(
              page.records,
              retaining: retainedTentativeRecords
            ),
            board: page.board,
            attention: page.attention
          )
          timelineState = .unavailable
          return
        }
        if firstPage == nil { firstPage = page }
        records.append(contentsOf: page.records)
        guard records.count <= Self.maximumTimelineRecords else {
          throw LocalProductClientError.invalidResponse
        }
        if !page.hasMore {
          guard let firstPage else {
            throw LocalProductClientError.invalidResponse
          }
          let visibleRecords = Self.timelineRecords(
            records,
            retaining: retainedTentativeRecords
          )
          timeline = LocalProductTimelinePage(
            schemaVersion: firstPage.schemaVersion,
            teamInstanceID: teamInstanceID,
            viewVersion: firstPage.viewVersion,
            nextCursor: page.nextCursor,
            hasMore: false,
            gap: nil,
            records: visibleRecords,
            board: firstPage.board,
            attention: firstPage.attention
          )
          timelineState = .loaded
          return
        }
        guard pageIndex + 1 < Self.maximumTimelinePages,
          records.count < Self.maximumTimelineRecords,
          !page.nextCursor.isEmpty,
          page.nextCursor != cursor,
          seenCursors.insert(page.nextCursor).inserted
        else {
          throw LocalProductClientError.invalidResponse
        }
        cursor = page.nextCursor
      }
      throw LocalProductClientError.invalidResponse
    } catch is CancellationError {
      guard
        timelineLoadIsCurrent(
          generation: generation,
          selection: selection
        )
      else { return }
      if !preserveObservedTentativeRecords {
        timeline = nil
        timelineState = .idle
      }
    } catch let remote as LocalIPCRemoteError {
      if Task.isCancelled {
        guard
          timelineLoadIsCurrent(
            generation: generation,
            selection: selection
          )
        else { return }
        if !preserveObservedTentativeRecords {
          timeline = nil
          timelineState = .idle
        }
        return
      }
      guard
        timelineLoadIsCurrent(
          generation: generation,
          selection: selection
        )
      else { return }
      if remote.code == .cursorConflict,
        preserveObservedTentativeRecords,
        timeline?.teamInstanceID == teamInstanceID
      {
        // Active Missions can append an event between two timeline pages.
        // Keep the last complete activity snapshot visible and let the next
        // bounded poll restart from the authoritative head cursor.
        connectionState = .online
        timelineState = .loaded
        return
      }
      if remote.code == .cursorConflict || remote.code == .streamGap {
        connectionState = .stale(reason: remote.code.rawValue)
        timelineState = .unavailable
      } else if !remote.recoverable {
        connectionState = .fatal(reason: remote.code.rawValue)
        timelineState = .fatal
      } else {
        connectionState = .offline(reason: remote.code.rawValue)
        timelineState = .unavailable
      }
    } catch {
      if Task.isCancelled {
        guard
          timelineLoadIsCurrent(
            generation: generation,
            selection: selection
          )
        else { return }
        if !preserveObservedTentativeRecords {
          timeline = nil
          timelineState = .idle
        }
        return
      }
      guard
        timelineLoadIsCurrent(
          generation: generation,
          selection: selection
        )
      else { return }
      connectionState = .offline(reason: closedClientReason(error))
      timelineState = .unavailable
    }
  }

  private func beginTimelineLoad() -> UInt64 {
    timelineLoadGeneration &+= 1
    return timelineLoadGeneration
  }

  private static func timelineRecords(
    _ current: [LocalProductTimelineRecord],
    retaining observedTentative: [LocalProductTimelineRecord]
  ) -> [LocalProductTimelineRecord] {
    guard !observedTentative.isEmpty else { return current }
    var deliveryIDs = Set(current.map(\.deliveryID))
    var result = current
    for record in observedTentative where result.count < maximumTimelineRecords {
      guard deliveryIDs.insert(record.deliveryID).inserted else { continue }
      result.append(record)
    }
    return result
  }

  private func invalidateTimelineLoad() {
    timelineLoadGeneration &+= 1
  }

  private func closeTimelineLoadPresentation() {
    invalidateTimelineLoad()
    timeline = nil
    timelineState = .idle
  }

  private func timelineLoadIsCurrent(
    generation: UInt64,
    selection: TimelineLoadSelection
  ) -> Bool {
    guard generation == timelineLoadGeneration else { return false }
    switch selection {
    case .team(let teamID):
      return selectedTeamID == teamID
    case .mission(let missionID, _):
      return workbench.route == .mission(missionID)
    }
  }

  private static func validateTimelinePage(
    _ page: LocalProductTimelinePage,
    teamInstanceID: String,
    firstPage: LocalProductTimelinePage?,
    seenDeliveryIDs: inout Set<String>
  ) throws {
    guard page.schemaVersion == 1,
      page.teamInstanceID == teamInstanceID,
      !page.viewVersion.isEmpty,
      page.board.schemaVersion == 1,
      page.board.teamInstanceID == teamInstanceID,
      page.board.viewVersion == page.viewVersion
    else {
      throw LocalProductClientError.invalidResponse
    }
    if let gap = page.gap {
      guard gap.schemaVersion == 1,
        gap.kind == "stream_gap",
        gap.teamInstanceID == teamInstanceID,
        gap.currentViewVersion == page.viewVersion,
        !gap.deliveryID.isEmpty,
        !gap.reason.isEmpty
      else {
        throw LocalProductClientError.invalidResponse
      }
    }
    var attentionIDs = Set<String>()
    for attention in page.attention {
      guard attention.schemaVersion == 1,
        attention.teamInstanceID == teamInstanceID,
        !attention.attentionID.isEmpty,
        attentionIDs.insert(attention.attentionID).inserted
      else {
        throw LocalProductClientError.invalidResponse
      }
    }
    if let firstPage {
      guard page.viewVersion == firstPage.viewVersion,
        page.board == firstPage.board,
        page.attention == firstPage.attention
      else {
        throw LocalProductClientError.invalidResponse
      }
    }
    for record in page.records {
      guard record.schemaVersion == 1,
        record.teamInstanceID == teamInstanceID,
        !record.deliveryID.isEmpty,
        seenDeliveryIDs.insert(record.deliveryID).inserted
      else {
        throw LocalProductClientError.invalidResponse
      }
    }
  }

  public func refreshSetup() async {
    guard let setupClient else {
      setupState = .unavailable(reason: "setup_unavailable")
      return
    }
    setupState = .loading
    do {
      let refreshed = try await setupClient.setupSnapshot()
      // A transient daemon projection race can produce a structurally valid
      // snapshot with every catalog empty. Treat it as recoverable rather
      // than erasing a known-good provider/runtime directory.
      guard hasUsableSetupInventory(refreshed) else {
        setupState = .unavailable(reason: "empty_setup")
        return
      }
      setupSnapshot = refreshed
      reconcileWorkspace()
      setupState = hasPendingConversationProjection(refreshed)
        ? .unavailable(reason: "conversation_profiles_pending")
        : .ready
    } catch let remote as LocalIPCRemoteError {
      setupState =
        remote.recoverable
        ? .unavailable(reason: remote.code.rawValue)
        : .fatal(reason: remote.code.rawValue)
    } catch {
      setupState = .unavailable(reason: closedClientReason(error))
    }
  }

  private func hasUsableSetupInventory(
    _ snapshot: LocalProductSetupSnapshot
  ) -> Bool {
    !snapshot.providers.isEmpty ||
      !snapshot.providerAccounts.isEmpty ||
      !snapshot.conversationProfiles.isEmpty ||
      !snapshot.runtimes.isEmpty ||
      !snapshot.savedTeams.isEmpty ||
      !snapshot.templates.isEmpty ||
      !snapshot.roleOptions.isEmpty ||
      !snapshot.skills.isEmpty ||
      !snapshot.resources.isEmpty
  }

  private func hasPendingConversationProjection(
    _ snapshot: LocalProductSetupSnapshot
  ) -> Bool {
    guard snapshot.conversationProfiles.isEmpty else { return false }
    return snapshot.providers.contains { provider in
      ["available", "configured", "verified"].contains(provider.status)
    } || snapshot.runtimes.contains { runtime in
      ["online", "ready", "available"].contains(runtime.status)
    }
  }

  public func refreshPermissions() async {
    guard let permissionClient else {
      return
    }
    do {
      async let snapshot = permissionClient.permissionsSnapshot(
        journeyID: evolutionAssetJourneyID
      )
      async let attention = permissionClient.permissionsAttention(
        journeyID: evolutionAssetJourneyID
      )
      let (snap, attn) = try await (snapshot, attention)
      permissionSnapshot = snap
      permissionAttention = attn
    } catch {
      permissionSnapshot = nil
      permissionAttention = nil
    }
  }

  public func refreshExecutions() async {
    guard let executionSnapshotClient else {
      return
    }
    do {
      executionSnapshot = try await executionSnapshotClient.executionSnapshot(
        journeyID: evolutionAssetJourneyID
      )
    } catch {
      executionSnapshot = nil
    }
  }

  public func refreshProduction() async {
    guard let productionSnapshotClient else {
      return
    }
    do {
      productionSnapshot = try await productionSnapshotClient.productionSnapshot(
        journeyID: evolutionAssetJourneyID
      )
    } catch {
      productionSnapshot = nil
    }
  }

  public func openMission(_ id: String) {
    invalidateTimelineLoad()
    workbench.openMission(id)
  }

  @discardableResult
  public func openMissionAndActivate(_ id: String) async -> Bool {
    guard
      let mission = snapshot?.missions.first(where: {
        $0.missionID == id
      })
    else { return false }
    workbench.openMission(id)
    await loadTimeline(
      teamInstanceID: mission.teamInstanceID,
      selection: .mission(id: id, teamID: mission.teamInstanceID)
    )
    return workbench.route == .mission(id)
  }

  public func refreshMission(_ id: String) async {
    await refresh()
    await openMissionAndActivate(id)
  }

  /// Refreshes only the Mission the user is still viewing. Tentative model
  /// output is retained in this App session when the terminal projection no
  /// longer republishes it; it is never promoted to Journal authority.
  @discardableResult
  public func refreshVisibleMissionActivity(_ id: String) async -> Bool {
    guard workbench.route == .mission(id) else { return false }
    await refresh()
    guard workbench.route == .mission(id),
      let mission = snapshot?.missions.first(where: { $0.missionID == id })
    else { return false }
    await loadTimeline(
      teamInstanceID: mission.teamInstanceID,
      selection: .mission(id: id, teamID: mission.teamInstanceID),
      preserveObservedTentativeRecords: true
    )
    // A newly-started Mission can be visible in the authoritative snapshot
    // before its timeline projection is readable. Keep following the Mission
    // through that transient gap; timeline availability is not route authority.
    return workbench.route == .mission(id)
      && snapshot?.missions.contains(where: { $0.missionID == id }) == true
  }

  /// Polling is intentionally App-owned and bounded to the visible Mission.
  /// Leaving the room cancels the view task and the selection guard prevents a
  /// late response from reopening or mutating a different surface.
  public func followVisibleMissionActivity(
    _ id: String,
    pollNanoseconds: UInt64 = 1_000_000_000
  ) async {
    let boundedPoll = min(max(pollNanoseconds, 250_000_000), 10_000_000_000)
    while !Task.isCancelled {
      guard await refreshVisibleMissionActivity(id) else { return }
      guard let status = snapshot?.missions.first(where: {
        $0.missionID == id
      })?.status,
        !Self.terminalMissionActivityStatus(status)
      else { return }
      do {
        try await Task.sleep(nanoseconds: boundedPoll)
      } catch {
        return
      }
    }
  }

  private static func terminalMissionActivityStatus(_ status: String) -> Bool {
    ["succeeded", "failed", "cancelled", "blocked", "human_required"]
      .contains(status)
  }

  public func showMissionBoard() {
    closeTimelineLoadPresentation()
    workbench.showBoard()
  }

  public func showMissionTeams() {
    closeTimelineLoadPresentation()
    workbench.showTeams()
  }

  public func showMissionAttention() {
    closeTimelineLoadPresentation()
    workbench.showAttention()
  }

  public func showMissionLibrary() {
    closeTimelineLoadPresentation()
    workbench.showLibrary()
    Task { await loadEvolutionAssets() }
  }

  public var selectedChatSession: LocalProductChatSession? {
    chatSessions.first { $0.threadID == selectedChatSessionID }
  }

  public func currentChatThreadID() -> String {
    if !selectedChatSessionID.isEmpty {
      return selectedChatSessionID
    }
    let anchor = workspace.selectedContinuity.threadAnchor
    if !anchor.isEmpty {
      return anchor
    }
    return workspace.currentThreadID()
  }

  /// Creates a brand-new conversation (a fresh backend thread) and switches to
  /// it. The thread is materialized lazily on the first message.
  public func newConversation() {
    chatGeneration &+= 1
    conversationRouteTransitionGeneration &+= 1
    hasPendingConversationRoute = false
    conversationRouteSourceProfileID = ""
    forceNewConversationSegment = false
    confirmedConversationExecutionBinding = nil
    confirmedConversationTrustBoundaryAcknowledgement = nil
    let threadID = "thread-" + UUID().uuidString.lowercased()
    let session = LocalProductChatSession(
      threadID: threadID,
      title: "Conversation \(chatSessions.count + 1)"
    )
    chatSessions.append(session)
    selectedChatSessionID = threadID
    workspace.updateThreadAnchor(threadID)
    workspace.updateComposerDraft("")
    chatThread = nil
    chatOperationFailure = nil
    resetConversationContextDisclosures()
    persistChatSessions()
    Task { await loadChatThread() }
  }

  /// Switches the active conversation to an existing session thread.
  public func selectChatSession(_ threadID: String) {
    guard !threadID.isEmpty,
      chatSessions.contains(where: { $0.threadID == threadID })
    else {
      return
    }
    selectedChatSessionID = threadID
    workspace.updateThreadAnchor(threadID)
    chatThread = nil
    chatOperationFailure = nil
    resetConversationContextDisclosures()
    persistChatSessions()
    Task { await loadChatThread() }
  }

  public func saveMissionPresentation(
    missionID: String,
    title: String,
    conversationThreadID: String?,
    workspacePath: String? = nil
  ) {
    let boundedMissionID = String(
      missionID.trimmingCharacters(in: .whitespacesAndNewlines).prefix(128)
    )
    let boundedTitle = String(
      title.trimmingCharacters(in: .whitespacesAndNewlines).prefix(96)
    )
    let boundedThreadID = String(
      (conversationThreadID ?? "")
        .trimmingCharacters(in: .whitespacesAndNewlines).prefix(256)
    )
    let boundedWorkspacePath = Self.missionPresentationWorkspacePath(
      workspacePath ?? executionWorkspacePath
    )
    guard !boundedMissionID.isEmpty, !boundedTitle.isEmpty,
      boundedThreadID.isEmpty || chatSessions.contains(where: {
        $0.threadID == boundedThreadID
      })
    else { return }
    missionPresentations[boundedMissionID] = LocalProductMissionPresentation(
      missionID: boundedMissionID,
      title: boundedTitle,
      conversationThreadID: boundedThreadID,
      workspacePath: boundedWorkspacePath
    )
    persistMissionPresentations()
  }

  public func conversationLinkedToMission(
    _ missionID: String
  ) -> LocalProductChatSession? {
    guard let threadID = missionPresentations[missionID]?.conversationThreadID,
      !threadID.isEmpty
    else { return nil }
    return chatSessions.first(where: { $0.threadID == threadID })
  }

  public func missionIDsLinkedToConversation(
    _ threadID: String
  ) -> [String] {
    guard !threadID.isEmpty else { return [] }
    return missionPresentations.values
      .filter { $0.conversationThreadID == threadID }
      .sorted {
        if $0.updatedAt != $1.updatedAt { return $0.updatedAt > $1.updatedAt }
        return $0.missionID < $1.missionID
      }
      .map(\.missionID)
  }

  /// Deletes a conversation thread from the daemon and removes it from the
  /// session registry, freeing a bounded conversation slot. The active session
  /// falls back to the next available one (or stays blank).
  public func deleteChatSession(_ threadID: String) async {
    guard chatSessions.contains(where: { $0.threadID == threadID }) else {
      return
    }
    do {
      try await client.deleteChatThread(threadID: threadID)
      chatSessions.removeAll { $0.threadID == threadID }
      removeConversationContextDisclosures(for: threadID)
      unlinkMissionPresentations(from: threadID)
      if selectedChatSessionID == threadID {
        chatGeneration &+= 1
        selectedChatSessionID = chatSessions.first?.threadID ?? ""
        workspace.updateThreadAnchor(selectedChatSessionID)
        chatThread = nil
        chatOperationFailure = nil
      }
      persistChatSessions()
      if !selectedChatSessionID.isEmpty {
        Task { await loadChatThread() }
      }
    } catch {
      // A failed delete keeps the session; surface the same failure mapping.
      chatOperationFailure = Self.chatFailure(error, incidentID: Self.newChatIncidentID())
    }
  }

  public func renameChatSession(_ threadID: String, title: String) {
    guard let index = chatSessions.firstIndex(where: { $0.threadID == threadID })
    else {
      return
    }
    let clean = title.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !clean.isEmpty else { return }
    let bounded = String(clean.prefix(80))
    chatSessions[index].title = bounded
    chatSessions[index].updatedAt = Date()
    persistChatSessions()
  }

  public func touchChatSession(_ threadID: String) {
    guard let index = chatSessions.firstIndex(where: { $0.threadID == threadID })
    else {
      return
    }
    chatSessions[index].updatedAt = Date()
    persistChatSessions()
  }

  private static let chatSessionsSchemaVersion = 1

  private struct PersistedChatSessions: Codable {
    var schemaVersion: Int
    var selectedSessionID: String
    var sessions: [LocalProductChatSession]
  }

  private func loadPersistedChatSessions() -> Bool {
    guard let data = try? LocalPrivateRegistryStorage.read(from: chatSessionsFileURL) else {
      return false
    }
    guard let persisted = try? JSONDecoder().decode(
      PersistedChatSessions.self, from: data
    ), persisted.schemaVersion == Self.chatSessionsSchemaVersion,
      persisted.sessions.count <= 256,
      persisted.selectedSessionID.utf8.count <= 256,
      persisted.sessions.allSatisfy({ session in
        !session.threadID.isEmpty && session.threadID.utf8.count <= 256
          && !session.title.isEmpty && session.title.utf8.count <= 80
      })
    else {
      return false
    }
    chatSessions = persisted.sessions
    selectedChatSessionID = persisted.selectedSessionID
    return persisted.sessions.isEmpty
  }

  private func persistChatSessions() {
    let boundedSessions = Array(chatSessions.prefix(256))
    let persisted = PersistedChatSessions(
      schemaVersion: Self.chatSessionsSchemaVersion,
      selectedSessionID: String(selectedChatSessionID.prefix(256)),
      sessions: boundedSessions
    )
    guard let data = try? JSONEncoder().encode(persisted) else { return }
    do {
      try LocalPrivateRegistryStorage.write(data, to: chatSessionsFileURL)
    } catch {
      // Persistence is best-effort; the in-memory session registry still works
      // for the current launch.
    }
  }

  private static let missionPresentationsSchemaVersion = 1

  private struct PersistedMissionPresentations: Codable {
    var schemaVersion: Int
    var presentations: [LocalProductMissionPresentation]
  }

  private func loadPersistedMissionPresentations() {
    guard let data = try? LocalPrivateRegistryStorage.read(
      from: missionPresentationsFileURL
    ),
      let persisted = try? JSONDecoder().decode(
        PersistedMissionPresentations.self, from: data
      ),
      persisted.schemaVersion == Self.missionPresentationsSchemaVersion
    else { return }
    var loaded: [String: LocalProductMissionPresentation] = [:]
    for presentation in persisted.presentations.prefix(512) {
      let missionID = presentation.missionID
        .trimmingCharacters(in: .whitespacesAndNewlines)
      let title = presentation.title
        .trimmingCharacters(in: .whitespacesAndNewlines)
      guard !missionID.isEmpty, missionID.utf8.count <= 128,
        !title.isEmpty, title.utf8.count <= 96,
        presentation.conversationThreadID.utf8.count <= 256
      else { continue }
      loaded[missionID] = LocalProductMissionPresentation(
        missionID: missionID,
        title: title,
        conversationThreadID: presentation.conversationThreadID,
        workspacePath: Self.missionPresentationWorkspacePath(
          presentation.workspacePath
        ),
        updatedAt: presentation.updatedAt
      )
    }
    missionPresentations = loaded
  }

  private func persistMissionPresentations() {
    let presentations = missionPresentations.values.sorted {
      $0.updatedAt > $1.updatedAt
    }.prefix(512)
    let persisted = PersistedMissionPresentations(
      schemaVersion: Self.missionPresentationsSchemaVersion,
      presentations: Array(presentations)
    )
    guard let data = try? JSONEncoder().encode(persisted) else { return }
    do {
      try LocalPrivateRegistryStorage.write(data, to: missionPresentationsFileURL)
    } catch {
      // Presentation metadata is non-authoritative; Mission execution remains
      // available even when this local navigation index cannot be persisted.
    }
  }

  private func unlinkMissionPresentations(from threadID: String) {
    let linkedMissionIDs = missionPresentations.values.compactMap { presentation in
      presentation.conversationThreadID == threadID ? presentation.missionID : nil
    }
    guard !linkedMissionIDs.isEmpty else { return }
    let updatedAt = Date()
    for missionID in linkedMissionIDs {
      guard let presentation = missionPresentations[missionID] else { continue }
      missionPresentations[missionID] = LocalProductMissionPresentation(
        missionID: presentation.missionID,
        title: presentation.title,
        conversationThreadID: "",
        workspacePath: presentation.workspacePath,
        updatedAt: updatedAt
      )
    }
    persistMissionPresentations()
  }

  private static func missionPresentationWorkspacePath(
    _ value: String?
  ) -> String? {
    guard let value else { return nil }
    let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty, trimmed == value, trimmed.hasPrefix("/"),
      trimmed.utf8.count <= 4_096
    else { return nil }
    let standardized = URL(fileURLWithPath: trimmed).standardizedFileURL.path
    return standardized == trimmed ? standardized : nil
  }

  public func loadChatThread() async {
    let generation = chatGeneration
    let threadID = currentChatThreadID()
    do {
      let thread = try await client.chatThread(threadID: threadID)
      guard generation == chatGeneration,
        workspace.selectedContinuity.threadAnchor == threadID,
        thread.threadID == threadID
      else {
        return
      }
      chatThread = thread
      if let failure = thread.availabilityFailure {
        chatOperationFailure = Self.chatFailure(
          LocalIPCRemoteError(
            code: failure.code,
            recoverable: failure.retryable,
            stage: failure.stage,
            incidentID: failure.incidentID
          ),
          incidentID: failure.incidentID
        )
      } else if let stage = chatOperationFailure?.stage,
        [.migrationRead, .migrationCommit, .migrationCleanup].contains(stage)
      {
        chatOperationFailure = nil
      }
      if !thread.profileID.isEmpty {
        workspace.selectConversationProfile(thread.profileID)
        hasPendingConversationRoute = false
        conversationRouteSourceProfileID = ""
      }
    } catch {
      // Preserve existing chat thread; offline state is already surfaced by refresh.
    }
  }

  public func loadConversationContextDisclosure(
    _ segment: LocalProductConversationSegment
  ) async {
    guard let identity = conversationContextDisclosureIdentity(for: segment),
      !conversationContextDisclosuresInFlight.contains(identity)
    else { return }
    conversationContextDisclosuresInFlight.insert(identity)
    conversationContextDisclosureFailures.removeValue(forKey: identity)
    defer { conversationContextDisclosuresInFlight.remove(identity) }
    do {
      let disclosure = try await client.chatContextDisclosure(
        threadID: identity.threadID,
        segmentID: segment.segmentID
      )
      guard chatThread?.threadID == identity.threadID,
        LocalProductContextDisclosureIdentity(disclosure: disclosure) == identity,
        disclosure.disclosed.count == segment.disclosedContextCount,
        disclosure.omitted.count == segment.omittedContextCount,
        disclosure.contextCapacityStatus == segment.contextCapacityStatus,
        disclosure.contextWindowTokens == segment.contextWindowTokens,
        disclosure.reservedOutputTokens == segment.reservedOutputTokens,
        disclosure.adapterToolOverheadTokens == segment.adapterToolOverheadTokens,
        disclosure.admittedInputBudgetTokens == segment.admittedInputBudgetTokens,
        disclosure.contextTokenCounterID == segment.contextTokenCounterID,
        disclosure.contextTokenCounterVersion == segment.contextTokenCounterVersion,
        disclosure.admittedContributionTokens == segment.admittedContributionTokens,
        disclosure.budgetOmittedContributionTokens ==
          segment.budgetOmittedContributionTokens,
        disclosure.contextCapacityContributions == segment.contextCapacityContributions
      else {
        throw LocalProductClientError.invalidResponse
      }
      conversationContextDisclosures[identity] = disclosure
    } catch {
      guard chatThread?.threadID == identity.threadID else { return }
      conversationContextDisclosureFailures[identity] = closedClientReason(error)
    }
  }

  public func conversationContextDisclosureIdentity(
    for segment: LocalProductConversationSegment
  ) -> LocalProductContextDisclosureIdentity? {
    guard let thread = chatThread,
      !segment.disclosureReceiptDigest.isEmpty,
      thread.segments.contains(where: {
        $0.segmentID == segment.segmentID
          && $0.contextCapsuleDigest == segment.contextCapsuleDigest
          && $0.disclosureReceiptDigest == segment.disclosureReceiptDigest
      })
    else { return nil }
    return LocalProductContextDisclosureIdentity(
      threadID: thread.threadID,
      segmentID: segment.segmentID,
      contextCapsuleDigest: segment.contextCapsuleDigest,
      disclosureReceiptDigest: segment.disclosureReceiptDigest
    )
  }

  private func resetConversationContextDisclosures() {
    conversationContextDisclosures = [:]
    conversationContextDisclosuresInFlight = []
    conversationContextDisclosureFailures = [:]
  }

  private func removeConversationContextDisclosures(for threadID: String) {
    conversationContextDisclosures = conversationContextDisclosures.filter {
      $0.key.threadID != threadID
    }
    conversationContextDisclosuresInFlight = conversationContextDisclosuresInFlight.filter {
      $0.threadID != threadID
    }
    conversationContextDisclosureFailures = conversationContextDisclosureFailures.filter {
      $0.key.threadID != threadID
    }
  }

  public var availableConversationProfiles: [LocalProductConversationProfile] {
    setupSnapshot?.conversationProfiles ?? []
  }

  public var selectedConversationProfileID: String {
    let selected = workspace.selectedContinuity.conversationProfileID
    if availableConversationProfiles.contains(where: { $0.profileID == selected }) {
      return selected
    }
    return availableConversationProfiles.first?.profileID ?? ""
  }

  public var selectedConversationProfile: LocalProductConversationProfile? {
    let profileID = selectedConversationProfileID
    return availableConversationProfiles.first { $0.profileID == profileID }
  }

  nonisolated public static func canSubmitChatMessage(
    content: String,
    isSending: Bool,
    profileID: String
  ) -> Bool {
    !isSending
      && !profileID.isEmpty
      && !content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
  }

  /// Providers with a usable conversation credential: native profiles
  /// (codex/openai, opencode) plus every verified brokered account profile.
  public var verifiedConversationProviderIDs: Set<String> {
    Set(availableConversationProfiles.map(\.providerID))
  }

  /// Conversation models advertised by the current installed Runtime. OpenCode
  /// owns a dynamic multi-provider catalog, so stale compile-time entries must
  /// never override the CLI inventory returned by the daemon.
  public func conversationModels(
    providerID: String
  ) -> [LocalProductConversationModelOption] {
    let baseline = localProductConversationModels(providerID: providerID)
    guard providerID == "opencode" else { return baseline }
    guard let snapshot = setupSnapshot else { return baseline }
    let runtimes = snapshot.runtimes.filter { $0.adapterType == "opencode" }
    guard !runtimes.isEmpty else { return baseline }
    let modelIDs = runtimes
      .filter { $0.status == "online" }
      .flatMap(\.modelIDs)
    guard !modelIDs.isEmpty else { return [] }
    let metadata = Dictionary(uniqueKeysWithValues: baseline.map { ($0.modelID, $0) })
    return Array(Set(modelIDs)).sorted().map { modelID in
      metadata[modelID] ?? LocalProductConversationModelOption(
        modelID: modelID,
        displayName: modelID,
        reasoningEfforts: []
      )
    }
  }

  /// Models are routed by the selected execution profile, not by Provider ID
  /// alone. This keeps native Codex, native OpenCode, and account-scoped
  /// OpenCode profiles separate even when they share the same Provider.
  public func conversationModels(
    profile: LocalProductConversationProfile?
  ) -> [LocalProductConversationModelOption] {
    guard let profile else { return [] }
    guard profile.harnessAdapter == "opencode" else {
      return localProductConversationModels(providerID: profile.providerID)
    }
    let catalog = conversationModels(providerID: "opencode")
    return catalog.filter {
      let owner = conversationModelOwnerProvider(
        providerID: "opencode",
        modelID: $0.modelID
      )
      return profile.providerID == "opencode"
        ? owner == "opencode"
        : owner == profile.providerID
    }
  }

  /// Owning Provider of a model for the selected Provider: a qualified
  /// "provider/model" identity owns its prefix (OpenCode bucket), otherwise
  /// the model belongs to the selected Provider.
  public func conversationModelOwnerProvider(
    providerID: String,
    modelID: String
  ) -> String {
    if let slash = modelID.firstIndex(of: "/") {
      switch String(modelID[..<slash]) {
      case "minimax-cn": return "minimax"
      case "zai": return "zhipu"
      default: return String(modelID[..<slash])
      }
    }
    return providerID
  }

  /// A model is selectable only when its owning Provider credential is
  /// usable: native harness profiles (opencode/openai) need no extra account,
  /// and every brokered model requires a verified Loom account for that
  /// Provider. This is the selection-time gate that prevents picking e.g. a
  /// MiniMax model before MiniMax is verified.
  public func isConversationModelAvailable(
    providerID: String,
    modelID: String
  ) -> Bool {
    let owner = conversationModelOwnerProvider(
      providerID: providerID,
      modelID: modelID
    )
    switch owner {
    case "opencode", "openai":
      return true
    default:
      return verifiedConversationProviderIDs.contains(owner)
    }
  }

  public func isConversationModelAvailable(
    profile: LocalProductConversationProfile?,
    modelID: String
  ) -> Bool {
    guard let profile,
      conversationModels(profile: profile).contains(where: {
        $0.modelID == modelID
      })
    else { return false }
    let owner = conversationModelOwnerProvider(
      providerID: profile.providerID,
      modelID: modelID
    )
    switch profile.harnessAdapter {
    case "opencode":
      return profile.providerID == "opencode"
        ? owner == "opencode" && profile.authMode == "native_auth"
        : owner == profile.providerID &&
          !profile.providerAccountID.isEmpty &&
          profile.credentialRevision > 0 && profile.authMode == "brokered"
    case "codex", "pi":
      return profile.authMode == "native_auth"
    default:
      return owner == profile.providerID &&
        !profile.providerAccountID.isEmpty &&
        profile.credentialRevision > 0 && profile.authMode == "brokered"
    }
  }

  public func conversationModelUnavailableReason(
    providerID: String,
    modelID: String
  ) -> String? {
    guard !isConversationModelAvailable(
      providerID: providerID,
      modelID: modelID
    ) else { return nil }
    let owner = conversationModelOwnerProvider(
      providerID: providerID,
      modelID: modelID
    )
    return "Requires a verified \(owner) Provider credential. Open Provider Account to configure and verify \(owner), or select a different model."
  }

  /// Effective model for the selected Provider: the user selection when it is
  /// still usable, otherwise the profile default, otherwise the first usable
  /// model in the Provider catalog. Never returns a model whose owning
  /// Provider credential is not verified (for example after an account is
  /// revoked).
  public var effectiveConversationModelID: String {
    let profile = selectedConversationProfile
    let catalog = conversationModels(profile: profile)
    let selected = selectedConversationModelID
    // A user selection is honored only when it is a real model of the
    // selected Provider AND its owning Provider credential is usable. This
    // prevents a stale cross-Provider selection (for example "deepseek-chat"
    // left over from the DeepSeek profile) from displaying on a MiniMax or
    // OpenCode conversation.
    if !selected.isEmpty,
      catalog.contains(where: { $0.modelID == selected }),
      isConversationModelAvailable(
        profile: profile,
        modelID: selected
      ) {
      return selected
    }
    if let defaultModel = profile?.modelID,
      !defaultModel.isEmpty,
      catalog.contains(where: { $0.modelID == defaultModel }),
      isConversationModelAvailable(
        profile: profile,
        modelID: defaultModel
      ) {
      return defaultModel
    }
    return catalog.first {
      isConversationModelAvailable(
        profile: profile,
        modelID: $0.modelID
      )
    }?.modelID ?? ""
  }

  /// Effective reasoning effort: the user selection, or a provider-default
  /// ("medium") when the selected model supports reasoning and nothing was
  /// chosen yet.
  public var effectiveConversationReasoningEffort: String {
    let efforts = availableConversationReasoningEfforts
    if efforts.contains(selectedConversationReasoningEffort) {
      return selectedConversationReasoningEffort
    }
    return efforts.contains("medium") ? "medium" : (efforts.first ?? "")
  }

  private var availableConversationReasoningEfforts: [String] {
    return conversationModels(profile: selectedConversationProfile)
      .first { $0.modelID == effectiveConversationModelID }?
      .reasoningEfforts ?? []
  }

  public func selectConversationProfile(_ profileID: String) {
    conversationRouteTransitionGeneration &+= 1
    applyConversationProfileSelection(profileID)
  }

  public func selectConversationModel(_ modelID: String) {
    let previousModelID = effectiveConversationModelID
    let previousReasoningEffort = effectiveConversationReasoningEffort
    let profile = selectedConversationProfile
    let providerID = profile?.providerID ?? ""
    let catalog = conversationModels(profile: profile)
    guard catalog.contains(where: { $0.modelID == modelID }),
      isConversationModelAvailable(
        profile: profile,
        modelID: modelID
      )
    else {
      conversationModelSelectionNotice =
        conversationModelUnavailableReason(
          providerID: providerID,
          modelID: modelID
        ) ?? "Model is not available for this Provider."
      return
    }
    conversationModelSelectionNotice = nil
    selectedConversationModelID = modelID
    selectedConversationReasoningEffort = ""
    let efforts = conversationModels(profile: profile)
      .first { $0.modelID == modelID }?
      .reasoningEfforts ?? []
    selectedConversationReasoningEffort =
      efforts.contains("medium") ? "medium" : (efforts.first ?? "")
    if effectiveConversationModelID != previousModelID ||
      effectiveConversationReasoningEffort != previousReasoningEffort
    {
      invalidateConfirmedConversationRouteTransition()
    }
  }

  public func selectConversationReasoningEffort(_ reasoningEffort: String) {
    guard reasoningEffort.isEmpty ||
      availableConversationReasoningEfforts.contains(reasoningEffort)
    else { return }
    let previousReasoningEffort = effectiveConversationReasoningEffort
    selectedConversationReasoningEffort = reasoningEffort
    if effectiveConversationReasoningEffort != previousReasoningEffort {
      invalidateConfirmedConversationRouteTransition()
    }
  }

  public func requestConversationProfileSelection(
    _ profileID: String
  ) -> LocalProductConversationRouteTransition? {
    guard let target = availableConversationProfiles.first(where: {
      $0.profileID == profileID
    }) else { return nil }
    let sourceID = selectedConversationProfileID
    guard sourceID != profileID else { return nil }
    // A conversation has continuity only when it actually contains messages;
    // the thread anchor is now the active session handle and is non-empty even
    // for a brand-new conversation, so it must not gate route review.
    let hasContinuity = !(chatThread?.messages.isEmpty ?? true)
    guard hasContinuity,
      let source = availableConversationProfiles.first(where: {
        $0.profileID == sourceID
      })
    else {
      selectConversationProfile(profileID)
      return nil
    }

    conversationRouteTransitionGeneration &+= 1
    return LocalProductConversationRouteTransition(
      id: UUID().uuidString.lowercased(),
      source: source,
      target: target,
      threadID: workspace.selectedContinuity.threadAnchor,
      sourceSegmentID: chatThread?.segments.last?.segmentID ?? "",
      sourceBindingDigest: chatThread?.segments.last?.bindingDigest ?? "",
      sourceExecutionBinding: chatThread?.segments.last?.executionBinding,
      targetExecutionBinding: Self.conversationExecutionBinding(target),
      sourceReasoningEffort: chatThread?.segments.last?.reasoningEffort ?? "",
      generation: conversationRouteTransitionGeneration
    )
  }

  public func requestConversationDispatchTransition(
  ) -> LocalProductConversationRouteTransition? {
    guard let profile = selectedConversationProfile,
      let thread = chatThread,
      !thread.messages.isEmpty,
      !forceNewConversationSegment
    else { return nil }
    if !thread.profileID.isEmpty,
      thread.profileID != profile.profileID,
      let source = availableConversationProfiles.first(where: {
        $0.profileID == thread.profileID
      })
    {
      hasPendingConversationRoute = true
      conversationRouteSourceProfileID = source.profileID
      conversationRouteTransitionGeneration &+= 1
      return LocalProductConversationRouteTransition(
        source: source,
        target: profile,
        threadID: workspace.selectedContinuity.threadAnchor,
        sourceSegmentID: thread.segments.last?.segmentID ?? "",
        sourceBindingDigest: thread.segments.last?.bindingDigest ?? "",
        sourceExecutionBinding: thread.segments.last?.executionBinding,
        targetExecutionBinding: Self.conversationExecutionBinding(
          profile,
          modelID: effectiveConversationModelID
        ),
        sourceReasoningEffort: thread.segments.last?.reasoningEffort ?? "",
        targetReasoningEffort: effectiveConversationReasoningEffort,
        generation: conversationRouteTransitionGeneration
      )
    }
    guard let segment = thread.segments.last,
      segment.profileID == profile.profileID
    else { return nil }
    let current = Self.conversationExecutionBinding(
      profile,
      modelID: effectiveConversationModelID
    )
    let targetReasoningEffort = effectiveConversationReasoningEffort
    guard segment.executionBinding != current ||
      segment.reasoningEffort != targetReasoningEffort
    else { return nil }
    conversationRouteTransitionGeneration &+= 1
    return LocalProductConversationRouteTransition(
      source: profile,
      target: profile,
      threadID: workspace.selectedContinuity.threadAnchor,
      sourceSegmentID: segment.segmentID,
      sourceBindingDigest: segment.bindingDigest,
      sourceExecutionBinding: segment.executionBinding,
      targetExecutionBinding: current,
      sourceReasoningEffort: segment.reasoningEffort,
      targetReasoningEffort: targetReasoningEffort,
      rebindsCurrentRoute: true,
      generation: conversationRouteTransitionGeneration
    )
  }

  public func confirmConversationRouteTransition(
    _ transition: LocalProductConversationRouteTransition,
    contextMode: LocalProductConversationContextMode,
    trustBoundaryAcknowledged: Bool = false
  ) -> Bool {
    guard transition.sourceExecutionBinding != nil,
      transition.trustBoundaryChanges.isEmpty || trustBoundaryAcknowledged
    else { return false }
    if transition.rebindsCurrentRoute {
      guard transition.generation == conversationRouteTransitionGeneration,
        selectedConversationProfileID == transition.target.profileID,
        workspace.selectedContinuity.threadAnchor == transition.threadID,
        chatThread?.threadID == transition.threadID,
        chatThread?.segments.last?.segmentID == transition.sourceSegmentID,
        chatThread?.segments.last?.bindingDigest == transition.sourceBindingDigest,
        availableConversationProfiles.contains(transition.target),
        chatThread?.segments.last?.executionBinding ==
          transition.sourceExecutionBinding,
        Self.conversationExecutionBinding(
          transition.target,
          modelID: effectiveConversationModelID
        ) == transition.targetExecutionBinding,
        effectiveConversationReasoningEffort ==
          transition.targetReasoningEffort,
        transition.targetExecutionBinding !=
          transition.sourceExecutionBinding ||
          transition.targetReasoningEffort !=
            transition.sourceReasoningEffort
      else { return false }
      let acknowledgement = trustBoundaryAcknowledgement(
        for: transition,
        contextMode: contextMode
      )
      guard acknowledgement != nil else { return false }
      conversationRouteTransitionGeneration &+= 1
      conversationContextMode = contextMode
      forceNewConversationSegment = true
      hasPendingConversationRoute = false
      conversationRouteSourceProfileID = ""
      confirmedConversationExecutionBinding = transition.targetExecutionBinding
      confirmedConversationTrustBoundaryAcknowledgement = acknowledgement
      return true
    }
    let selectedProfileID = selectedConversationProfileID
    let matchesSourceSelection =
      selectedProfileID == transition.source.profileID
    let matchesRecoveredTargetSelection =
      selectedProfileID == transition.target.profileID &&
      hasPendingConversationRoute &&
      conversationRouteSourceProfileID == transition.source.profileID
    guard transition.generation == conversationRouteTransitionGeneration,
      matchesSourceSelection || matchesRecoveredTargetSelection,
      workspace.selectedContinuity.threadAnchor == transition.threadID,
      chatThread?.threadID == transition.threadID,
      chatThread?.segments.last?.segmentID == transition.sourceSegmentID,
      chatThread?.segments.last?.bindingDigest == transition.sourceBindingDigest,
      chatThread?.segments.last?.executionBinding ==
        transition.sourceExecutionBinding,
      availableConversationProfiles.contains(transition.source),
      availableConversationProfiles.contains(transition.target),
      Self.conversationExecutionBinding(transition.target) ==
        transition.targetExecutionBinding
    else { return false }

    let acknowledgement = trustBoundaryAcknowledgement(
      for: transition,
      contextMode: contextMode
    )
    guard acknowledgement != nil else { return false }

    conversationRouteTransitionGeneration &+= 1
    conversationContextMode = contextMode
    if matchesSourceSelection {
      applyConversationProfileSelection(transition.target.profileID)
    }
    // A confirmed switch must force a new segment on the next send; otherwise
    // the thread still carries the old profile and every send re-triggers the
    // route-transition sheet (repeated Change-conversation-route popups).
    forceNewConversationSegment = true
    hasPendingConversationRoute = false
    conversationRouteSourceProfileID = ""
    confirmedConversationExecutionBinding = transition.targetExecutionBinding
    confirmedConversationTrustBoundaryAcknowledgement = acknowledgement
    return selectedConversationProfileID == transition.target.profileID
  }

  private func trustBoundaryAcknowledgement(
    for transition: LocalProductConversationRouteTransition,
    contextMode: LocalProductConversationContextMode
  ) -> LocalProductTrustBoundaryAcknowledgement? {
    guard let sourceSegment = chatThread?.segments.last,
      sourceSegment.segmentID == transition.sourceSegmentID,
      sourceSegment.bindingDigest == transition.sourceBindingDigest
    else { return nil }
    return LocalProductTrustBoundaryAcknowledgement.reviewed(
      threadID: transition.threadID,
      sourceSegment: sourceSegment,
      targetProfileID: transition.target.profileID,
      targetExecutionBinding: transition.targetExecutionBinding,
      targetReasoningEffort: transition.targetReasoningEffort,
      contextMode: contextMode
    )
  }

  private func invalidateConfirmedConversationRouteTransition() {
    conversationRouteTransitionGeneration &+= 1
    forceNewConversationSegment = false
    confirmedConversationExecutionBinding = nil
    confirmedConversationTrustBoundaryAcknowledgement = nil
  }

  private func applyConversationProfileSelection(_ profileID: String) {
    guard
      availableConversationProfiles.contains(where: {
        $0.profileID == profileID
      })
    else { return }
    let storedProfileID =
      workspace.selectedContinuity.conversationProfileID
    let previousProfileID =
      storedProfileID.isEmpty
      ? selectedConversationProfileID
      : storedProfileID
    if previousProfileID != profileID {
      confirmedConversationExecutionBinding = nil
      confirmedConversationTrustBoundaryAcknowledgement = nil
      chatGeneration &+= 1
      if !hasPendingConversationRoute {
        conversationRouteSourceProfileID =
          chatThread?.profileID.isEmpty == false
          ? (chatThread?.profileID ?? previousProfileID)
          : previousProfileID
        hasPendingConversationRoute =
          !previousProfileID.isEmpty || !(chatThread?.messages.isEmpty ?? true)
      }
      if profileID == conversationRouteSourceProfileID {
        hasPendingConversationRoute = false
        conversationRouteSourceProfileID = ""
      }
    }
    workspace.selectConversationProfile(profileID)
    selectedConversationModelID =
      selectedConversationProfile?.modelID ?? ""
    selectedConversationReasoningEffort = ""
    conversationModelSelectionNotice = nil
    chatOperationFailure = nil
  }

  public func sendChatMessage(_ content: String) async {
    let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
    let profileID = selectedConversationProfileID
    guard Self.canSubmitChatMessage(
      content: trimmed,
      isSending: isSendingChatMessage,
      profileID: profileID
    ) else { return }
    guard !conversationDispatchRequiresRouteReview else {
      workspace.updateComposerDraft(trimmed)
      return
    }
    // The Credential Vault unlocks passphrase-free (LocalKeyFile). Auto-unlock
    // before sending so a locked vault after App restart never blocks a
    // conversation; the failure banner still offers an explicit Unlock action.
    if setupSnapshot?.credentialVault?.status == "locked" {
      await setCredentialVaultLocked(false)
      if setupSnapshot?.credentialVault?.status == "locked" {
        return
      }
    }
    let threadID = currentChatThreadID()
    let contextMode =
      forceNewConversationSegment ||
        hasPendingConversationRoute && conversationRouteSourceProfileID != profileID
      ? conversationContextMode
      : nil
    let generation = chatGeneration
    let incidentID = Self.newChatIncidentID()
    let previousThread = chatThread
    let currentMessages =
      previousThread?.threadID == threadID
      ? (previousThread?.messages ?? [])
      : []
    chatThread = LocalProductChatThread(
      threadID: threadID,
      profileID: profileID,
      messages: currentMessages + [
        LocalProductChatMessage(
          messageID: "pending-\(UUID().uuidString.lowercased())",
          role: "user",
          content: trimmed,
          tentative: false
        )
      ],
      canReply: false,
      requiresConfirmation: false
    )
    workspace.updateComposerDraft("")
    chatOperationFailure = nil
    activeChatResponsesByThreadID[threadID] = incidentID
    defer {
      if activeChatResponsesByThreadID[threadID] == incidentID {
        activeChatResponsesByThreadID.removeValue(forKey: threadID)
      }
      cancellingChatResponseThreadIDs.remove(threadID)
    }
    do {
      let thread = try await client.sendChatMessage(
        threadID: threadID,
        content: trimmed,
        profileID: profileID,
        modelID: effectiveConversationModelID,
        reasoningEffort: effectiveConversationReasoningEffort,
        contextMode: contextMode,
        expectedExecutionBinding: confirmedConversationExecutionBinding,
        trustBoundaryAcknowledgement:
          confirmedConversationTrustBoundaryAcknowledgement,
        incidentID: incidentID
      )
      guard generation == chatGeneration,
        workspace.selectedContinuity.threadAnchor == threadID
      else {
        return
      }
      chatThread = thread
      hasPendingConversationRoute = false
      conversationRouteSourceProfileID = ""
      forceNewConversationSegment = false
      confirmedConversationExecutionBinding = nil
      confirmedConversationTrustBoundaryAcknowledgement = nil
      if chatSessions.contains(where: {
        $0.threadID == threadID
      }) {
        // The plaintext navigation registry never derives metadata from chat
        // content. Transcripts remain in the encrypted conversation store.
        touchChatSession(threadID)
      }
      if let attempt = thread.attempts.last(where: {
        $0.incidentID == incidentID && $0.status == "failed"
      }) {
        workspace.updateComposerDraft(trimmed)
        chatOperationFailure = Self.chatFailure(
          LocalIPCRemoteError(
            code: LocalIPCRemoteError.Code(rawValue: attempt.failureCode)
              ?? .stateUnavailable,
            recoverable: attempt.retryable,
            stage: LocalIPCRemoteError.Stage(rawValue: attempt.failureStage)
              ?? .conversationDispatch,
			incidentID: attempt.incidentID,
			httpStatus: attempt.httpStatus,
			providerCode: attempt.providerCode,
			safeMessage: attempt.failureMessage,
			retryAfterSeconds: attempt.retryAfterSeconds
          ),
          incidentID: incidentID
        )
      }
    } catch {
      guard generation == chatGeneration,
        workspace.selectedContinuity.threadAnchor == threadID
      else {
        return
      }
      let failure = Self.chatFailure(error, incidentID: incidentID)
      chatThread = previousThread
      if failure.code == .conflict && failure.stage == .conversationDispatch {
        forceNewConversationSegment = false
        confirmedConversationExecutionBinding = nil
        confirmedConversationTrustBoundaryAcknowledgement = nil
        await recoverChatThreadAfterDispatchConflict(
          threadID: threadID,
          selectedProfileID: profileID,
          generation: generation
        )
      } else if failure.code == .invalidRequest &&
        failure.stage == .conversationDispatch
      {
        // The selected profile can go stale after a credential re-import or
        // revision bump. Refresh the authoritative setup and retry once with
        // the now-current profile before surfacing the failure.
        await refreshSetup()
        if selectedConversationProfileID != profileID,
          availableConversationProfiles.contains(where: {
            $0.profileID == selectedConversationProfileID
          })
        {
          let retried = await sendChatMessageOnce(
            trimmed, threadID: threadID,
            profileID: selectedConversationProfileID,
            contextMode: contextMode, generation: generation,
            incidentID: Self.newChatIncidentID()
          )
          if retried {
            return
          }
        }
      }
      workspace.updateComposerDraft(trimmed)
      chatOperationFailure = failure
    }
  }

  public func cancelActiveChatResponse() async {
    let threadID = currentChatThreadID()
    guard !cancellingChatResponseThreadIDs.contains(threadID),
      let incidentID = activeChatResponsesByThreadID[threadID]
    else { return }

    let generation = chatGeneration
    cancellingChatResponseThreadIDs.insert(threadID)
    defer { cancellingChatResponseThreadIDs.remove(threadID) }
    do {
      try await client.cancelChatResponse(
        threadID: threadID,
        incidentID: incidentID
      )
      guard activeChatResponsesByThreadID[threadID] == incidentID,
        generation == chatGeneration,
        workspace.selectedContinuity.threadAnchor == threadID
      else { return }
      chatOperationFailure = nil
    } catch let remote as LocalIPCRemoteError where remote.code == .notFound {
      // The Response may settle between the user pressing Stop and daemon
      // admission. Refresh authoritative state and let the original send finish.
      do {
        let thread = try await client.chatThread(threadID: threadID)
        if generation == chatGeneration,
          workspace.selectedContinuity.threadAnchor == threadID,
          thread.threadID == threadID
        {
          chatThread = thread
        }
      } catch {
        // The still-live send remains responsible for final settlement.
      }
      chatOperationFailure = nil
    } catch {
      guard activeChatResponsesByThreadID[threadID] == incidentID
      else { return }
      chatOperationFailure = Self.chatFailure(error, incidentID: incidentID)
    }
  }

  private var conversationDispatchRequiresRouteReview: Bool {
    guard !forceNewConversationSegment,
      let thread = chatThread,
      !thread.messages.isEmpty
    else { return false }
    guard let profile = selectedConversationProfile,
      thread.profileID == profile.profileID,
      let segment = thread.segments.last,
      segment.profileID == profile.profileID,
      let sourceBinding = segment.executionBinding
    else { return true }
    return sourceBinding != Self.conversationExecutionBinding(
      profile,
      modelID: effectiveConversationModelID
    ) || segment.reasoningEffort != effectiveConversationReasoningEffort
  }

  /// Sends one chat message with explicit parameters and returns whether it
  /// succeeded; used by the stale-profile retry path.
  private func sendChatMessageOnce(
    _ content: String,
    threadID: String,
    profileID: String,
    contextMode: LocalProductConversationContextMode?,
    generation: UInt64,
    incidentID: String
  ) async -> Bool {
    let previousThread = chatThread
    chatOperationFailure = nil
    do {
      let thread = try await client.sendChatMessage(
        threadID: threadID,
        content: content,
        profileID: profileID,
        modelID: effectiveConversationModelID,
        reasoningEffort: effectiveConversationReasoningEffort,
        contextMode: contextMode,
        expectedExecutionBinding: confirmedConversationExecutionBinding,
        incidentID: incidentID
      )
      guard generation == chatGeneration else { return false }
      chatThread = thread
      hasPendingConversationRoute = false
      conversationRouteSourceProfileID = ""
      forceNewConversationSegment = false
      confirmedConversationExecutionBinding = nil
      confirmedConversationTrustBoundaryAcknowledgement = nil
      workspace.updateComposerDraft("")
      return true
    } catch {
      chatThread = previousThread
      return false
    }
  }

  private func recoverChatThreadAfterDispatchConflict(
    threadID: String,
    selectedProfileID: String,
    generation: UInt64
  ) async {
    do {
      let thread = try await client.chatThread(threadID: threadID)
      guard generation == chatGeneration,
        workspace.selectedContinuity.threadAnchor == threadID,
        selectedConversationProfileID == selectedProfileID
      else { return }
      chatThread = thread
      if !thread.profileID.isEmpty && thread.profileID != selectedProfileID {
        hasPendingConversationRoute = true
        conversationRouteSourceProfileID = thread.profileID
      }
    } catch {
      // The original failure remains actionable and the user's draft is preserved.
    }
  }

  private func startNewConversation() {
    chatGeneration &+= 1
    hasPendingConversationRoute = false
    conversationRouteSourceProfileID = ""
    forceNewConversationSegment = false
    confirmedConversationExecutionBinding = nil
    confirmedConversationTrustBoundaryAcknowledgement = nil
    workspace.updateThreadAnchor(
      "thread-\(UUID().uuidString.lowercased())"
    )
    chatThread = nil
    resetConversationContextDisclosures()
  }

  private static func conversationExecutionBinding(
    _ profile: LocalProductConversationProfile,
    modelID: String? = nil
  ) -> LocalProductConversationExecutionBinding {
    let resolvedModelID = modelID?.trimmingCharacters(
      in: .whitespacesAndNewlines
    ) ?? profile.modelID
    return LocalProductConversationExecutionBinding(
      schemaVersion: 4,
      harnessAdapter: profile.harnessAdapter,
      providerID: profile.providerID,
      providerAccountID: profile.providerAccountID,
      credentialRevision: profile.credentialRevision,
      modelID: resolvedModelID,
      providerAccountPolicyVersion: profile.policyVersion,
      providerAccountPolicyRevision: profile.policyRevision,
      providerAccountPolicyDigest: profile.policyDigest,
      trustDomain: profile.trustDomain,
      retentionMode: profile.retentionMode,
      dataRegion: profile.dataRegion
    )
  }

  private static func newChatIncidentID() -> String {
    "loom-chat-\(UUID().uuidString.lowercased())"
  }

  private static func chatFailure(
    _ error: Error,
    incidentID: String
  ) -> LocalProductChatOperationFailure {
    let remote: LocalIPCRemoteError
    if let value = error as? LocalIPCRemoteError {
      remote = value
    } else if let local = error as? LocalProductClientError {
      switch local {
      case .invalidRequest:
        remote = LocalIPCRemoteError(
          code: .invalidRequest,
          recoverable: false,
          stage: .inputAdmission,
          incidentID: incidentID
        )
      case .timeout:
        remote = LocalIPCRemoteError(
          code: .timeout,
          recoverable: true,
          stage: .udsTransport,
          incidentID: incidentID
        )
      case .invalidSocket, .invalidResponse, .unavailable, .notFound:
        remote = LocalIPCRemoteError(
          code: .stateUnavailable,
          recoverable: true,
          stage: .udsTransport,
          incidentID: incidentID
        )
      }
    } else {
      remote = LocalIPCRemoteError(
        code: .internal,
        recoverable: true,
        stage: .udsTransport,
        incidentID: incidentID
      )
    }
    let stage = remote.stage ?? .conversationDispatch
    let presentation: (String, String)
    switch (remote.code, stage) {
    case (_, .migrationRead):
      presentation = (
        "Conversation migration blocked",
        "Loom could not safely read the previous conversation store. Correct its ownership or permissions, then reopen Loom."
      )
    case (_, .migrationCommit):
      presentation = (
        "Conversation migration incomplete",
        "Loom preserved the previous conversation store because the encrypted commit did not finish. Correct the storage issue, then reopen Loom."
      )
    case (_, .migrationCleanup):
      presentation = (
        "Conversation migration cleanup required",
        "Loom committed the encrypted conversation but could not safely remove the previous plaintext store. Correct its permissions, then reopen Loom."
      )
    case (.conflict, .conversationDispatch):
      presentation = (
        "Conversation route changed",
        "Loom preserved this conversation and your draft. Retry to review the new Segment and choose what context to share."
      )
    case (.invalidRequest, .inputAdmission):
      presentation = (
        "Message could not be sent",
        "The message or conversation identity was rejected. Review the draft and try again."
      )
    case (.invalidRequest, .conversationDispatch):
      presentation = (
        "Conversation Profile invalid",
        "The selected Conversation Profile is no longer valid: the Provider Account or its credential revision changed. Re-select the Profile and retry. Your draft is preserved."
      )
    case (.conversationUnavailable, .conversationDispatch):
      presentation = (
        "Conversation could not start",
        "Loom could not dispatch this message with the selected Profile. Open Runtime & Providers to check the runtime and the model's Provider Account, then retry."
      )
    case (.conversationLimit, _):
      presentation = (
        "Conversation limit reached",
        "Loom keeps a bounded set of conversations (1,024). Delete or archive older conversations, then start a new one. Your existing conversations are preserved."
      )
    case (.stateUnavailable, .vaultKeyLoad):
      presentation = (
        "Credential Vault locked",
        "Loom could not load the Credential Vault key for this conversation. Unlock the vault, then retry. Your draft is preserved."
      )
    case (.stateUnavailable, .vaultOpen):
      presentation = (
        "Credential Vault unavailable",
        "Loom could not open the encrypted Credential Vault. Check its local storage permissions, then reopen Loom and retry."
      )
    case (.stateUnavailable, .vaultEncrypt):
      presentation = (
        "Conversation context could not be secured",
        "Loom could not encrypt this conversation's context capsule with the Credential Vault. Unlock the vault or re-verify the selected Provider Account credential, then retry. Your draft is preserved."
      )
    case (.stateUnavailable, .vaultCommit):
      presentation = (
        "Conversation context could not be committed",
        "Loom could not safely commit the encrypted conversation context. Try again; if the issue persists, review the diagnostics."
      )
    case (.providerUnavailable, .providerConnect):
      presentation = (
        "Conversation Provider could not start",
        "Loom could not start the selected conversation runtime (for example OpenCode). Open Runtime & Providers, verify the runtime and the selected model's Provider credential, then retry."
      )
    case (.timeout, _):
      presentation = (
        "Conversation timed out",
        "The selected Provider did not respond in time. Your draft is preserved."
      )
    case (.providerAuth, .providerConnect):
      presentation = (
        "Model Provider credential required",
        "The selected model belongs to a Provider that has no verified Loom account. Open Provider Account to configure and verify it, or select a different model."
      )
    case (.providerAuth, _):
      presentation = (
        "Provider authentication failed",
        "Reconnect or replace the selected Provider Account credential, then retry."
      )
    case (.providerRateLimit, _):
      presentation = (
        "Provider rate limit reached",
        "The selected Provider Account is rate limited. Retry after the limit resets."
      )
    case (.providerRejected, _):
      presentation = (
        "Provider rejected request",
        "Check the selected account's access, balance, endpoint, and model, then retry."
      )
	case (.providerInsufficientBalance, .providerConnect):
	  presentation = (
		"Codex account usage limit reached",
		"Your Codex account has used up its credits. Add credits in Codex settings, or switch to a model this account can use (for example DeepSeek V4 with your Codex/cc-switch configuration)."
	  )
	case (.providerInsufficientBalance, _):
	  presentation = (
		"Provider balance required",
		"Add funds to the selected Provider Account, then retry."
	  )
	case (.providerModelUnavailable, _):
	  presentation = (
		"Model unavailable",
		"Select a model available to this Provider Account, then retry."
	  )
	case (.providerInvalidRequest, _):
	  presentation = (
		"Provider rejected parameters",
		"The selected Provider rejected the generated request parameters."
	  )
	case (.providerUnavailable, _):
	  presentation = (
		"Provider unavailable",
		"The selected Provider service is temporarily unavailable."
	  )
    case (.credentialUnavailable, _):
      presentation = (
        "Credential unavailable",
        "Unlock the Credential Vault or reconnect the selected Provider Account."
      )
    case (.invalidResponse, _):
      presentation = (
        "Invalid Provider response",
        "The selected Provider returned a response Loom could not safely accept."
      )
    case (_, .udsTransport):
      presentation = (
        "Local service unavailable",
        "Loom could not reach its bundled service. Reopen Loom or retry."
      )
    default:
      presentation = (
        "Conversation unavailable",
        "Loom could not dispatch this message with the selected Profile. Your draft is preserved."
      )
    }
    return LocalProductChatOperationFailure(
      code: remote.code,
      stage: stage,
      recoverable: remote.recoverable,
      incidentID: remote.incidentID ?? incidentID,
	  httpStatus: remote.httpStatus,
	  providerCode: remote.providerCode,
	  retryAfterSeconds: remote.retryAfterSeconds,
      title: presentation.0,
	  detail: chatFailureDetail(remote, fallback: presentation.1)
    )
  }

	private static func chatFailureDetail(
	  _ remote: LocalIPCRemoteError,
	  fallback: String
	) -> String {
	  let message = remote.safeMessage.isEmpty ? fallback : remote.safeMessage
	  var metadata: [String] = []
	  if remote.httpStatus > 0 {
		metadata.append("HTTP \(remote.httpStatus)")
	  }
	  if !remote.providerCode.isEmpty {
		metadata.append(remote.providerCode)
	  }
	  if remote.retryAfterSeconds > 0 {
		metadata.append("retry after \(remote.retryAfterSeconds)s")
	  }
	  guard !metadata.isEmpty else { return message }
	  return message + " " + metadata.joined(separator: " · ")
	}

  public var evolutionAssetReachable: Bool { assetClient != nil }

  public func loadEvolutionAssets(cursor: String = "") async {
    guard let assetClient else {
      evolutionAssetStatus = "Assets unavailable"
      return
    }
    evolutionAssetStatus = "Loading"
    do {
      let snapshot = try await assetClient.evolutionAssetSnapshot(
        journeyID: evolutionAssetJourneyID,
        cursor: cursor,
        limit: 64
      )
      evolutionAssets = snapshot
      evolutionAssetStatus = snapshot.hasMore ? "More assets available" : "Current"
    } catch {
      evolutionAssetStatus = closedClientReason(error)
    }
  }

  public func createEvolutionSkill(
    definitionID: String,
    revisionID: String,
    name: String,
    description: String,
    sourcePath: String
  ) async {
    guard let assetClient, let viewVersion = evolutionAssets?.viewVersion else {
      evolutionAssetStatus = "Refresh assets first"
      return
    }
    let digests: (artifact: String, content: String)
    do {
      digests = try Self.canonicalEvolutionAssetDigests(
        kind: "skill", definitionID: definitionID,
        revisionID: revisionID, sourcePath: sourcePath
      )
    } catch {
      evolutionAssetStatus = "Invalid local source"
      return
    }
    let command = EvolutionAssetCommand(
      action: "create_skill",
      operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID,
      expectedViewVersion: viewVersion,
      assetKind: .skill,
      definitionID: definitionID,
      revisionID: revisionID,
      candidateID: "",
      name: name,
      description: description,
      scope: "project",
      sourcePath: sourcePath,
      artifactDigest: digests.artifact,
      contentDigest: digests.content,
      sourceScope: "local",
      sourceReferenceDigest: digests.artifact,
      provenanceDigest: digests.artifact,
      risk: "low"
    )
    evolutionAssetStatus = "Creating Candidate"
    do {
      let receipt = try await assetClient.evolutionAssetCommand(command)
      guard receipt.operationID == command.operationID,
        receipt.action == command.action, !receipt.eventIDs.isEmpty
      else {
        throw LocalProductClientError.invalidResponse
      }
      evolutionAssetStatus = "Candidate created"
      await loadEvolutionAssets()
      await refresh()
    } catch {
      evolutionAssetStatus = closedClientReason(error)
    }
  }

  public func importEvolutionSkill(
    definitionID: String,
    revisionID: String,
    name: String,
    description: String,
    sourcePath: String
  ) async {
    guard let snapshot = evolutionAssets else {
      evolutionAssetStatus = "Refresh assets first"
      return
    }
    do {
      let digests = try Self.canonicalEvolutionAssetDigests(
        kind: "skill", definitionID: definitionID,
        revisionID: revisionID, sourcePath: sourcePath
      )
      let command = EvolutionAssetCommand(
        action: "import_skill", operationID: UUID().uuidString.lowercased(),
        journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
        definitionID: definitionID, revisionID: revisionID,
        candidateID: UUID().uuidString.lowercased(), name: name,
        description: description, scope: "project", sourcePath: sourcePath,
        artifactDigest: digests.artifact, contentDigest: digests.content,
        sourceReferenceDigest: digests.artifact,
        provenanceDigest: digests.artifact, risk: "medium"
      )
      await commitEvolutionAsset(command, progress: "Importing reviewed Candidate")
    } catch {
      evolutionAssetStatus = "Invalid local source"
    }
  }

  public func createEvolutionTemplate(
    kind: EvolutionAssetKind,
    output: String,
    definitionID: String,
    revisionID: String,
    name: String,
    description: String,
    sourcePath: String
  ) async {
    guard let snapshot = evolutionAssets else {
      evolutionAssetStatus = "Refresh assets first"
      return
    }
    let parameterSchema = Self.sha256Text("{\"schema_version\":1,\"parameters\":[]}")
    let permissionCeiling = Self.sha256Text("loom.template.permission.none.v1")
    let scopeCeiling = Self.sha256Text("loom.template.scope.project.v1")
    do {
      let digests = try Self.canonicalEvolutionTemplateDigests(
        kind: kind.rawValue, definitionID: definitionID, revisionID: revisionID,
        sourcePath: sourcePath, templateOutput: output,
        parameterSchemaDigest: parameterSchema,
        permissionCeilingDigest: permissionCeiling,
        scopeCeilingDigest: scopeCeiling
      )
      let command = EvolutionAssetCommand(
        action: "create_template", operationID: UUID().uuidString.lowercased(),
        journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
        assetKind: kind, definitionID: definitionID, revisionID: revisionID,
        name: name, description: description, scope: "project",
        sourcePath: sourcePath, artifactDigest: digests.artifact,
        contentDigest: digests.content, risk: "medium", templateOutput: output,
        parameterSchemaDigest: parameterSchema,
        permissionCeilingDigest: permissionCeiling,
        scopeCeilingDigest: scopeCeiling
      )
      await commitEvolutionAsset(command, progress: "Creating template Candidate")
    } catch {
      evolutionAssetStatus = "Invalid template source"
    }
  }

  public func instantiateEvolutionTemplate(_ revision: EvolutionAssetRevision) async {
    guard let snapshot = evolutionAssets, revision.assetKind != .skill,
      revision.lifecycle != .archived
    else {
      evolutionAssetStatus = "Template revision unavailable"
      return
    }
    let parameterDigest = Self.sha256Text("[]")
    let command = EvolutionAssetCommand(
      action: "instantiate_template", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      assetKind: revision.assetKind, definitionID: revision.definitionID,
      revisionID: revision.revisionID, artifactDigest: revision.artifactDigest,
      parameterValues: [], parameterDigest: parameterDigest
    )
    await commitEvolutionAsset(command, progress: "Instantiating Candidate-only template")
  }

  public func activateEvolutionCandidate(_ candidate: EvolutionAssetCandidate) async {
    guard assetClient != nil, let snapshot = evolutionAssets,
      candidate.decision.isEmpty,
      let revision = snapshot.records
        .first(where: { $0.definition.definitionID == candidate.definitionID })?
        .revisions.first(where: { $0.revisionID == candidate.revisionID })
    else {
      evolutionAssetStatus = "Exact Candidate unavailable"
      return
    }
    let command = EvolutionAssetCommand(
      action: "activate", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      decisionSource: "user_explicit",
      assetKind: candidate.assetKind, definitionID: candidate.definitionID,
      revisionID: candidate.revisionID, candidateID: candidate.candidateID,
      artifactDigest: revision.artifactDigest,
      expectedPreviousRevisionID: snapshot.definitions.first(where: {
        $0.definitionID == candidate.definitionID
      })?.activeRevisionID ?? "",
      evaluationIDs: candidate.requiredEvaluationIDs
    )
    await commitEvolutionAsset(command, progress: "Activating exact revision")
  }

  public func decideEvolutionCandidate(_ candidate: EvolutionAssetCandidate, retain: Bool) async {
    guard let snapshot = evolutionAssets,
      candidate.decision.isEmpty,
      let revision = snapshot.revisions.first(where: {
        $0.definitionID == candidate.definitionID && $0.revisionID == candidate.revisionID
      })
    else {
      evolutionAssetStatus = "Exact Candidate unavailable"
      return
    }
    let command = EvolutionAssetCommand(
      action: retain ? "retain" : "reject", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      decisionSource: "user_explicit",
      assetKind: candidate.assetKind, definitionID: candidate.definitionID,
      revisionID: candidate.revisionID, candidateID: candidate.candidateID,
      artifactDigest: revision.artifactDigest,
      reasonCode: retain ? "keep_for_later" : "user_rejected"
    )
    await commitEvolutionAsset(
      command, progress: retain ? "Keeping Candidate" : "Rejecting Candidate")
  }

  public func setEvolutionRevisionArchived(
    _ revision: EvolutionAssetRevision,
    archived: Bool
  ) async {
    guard let snapshot = evolutionAssets else {
      evolutionAssetStatus = "Refresh assets first"
      return
    }
    let command = EvolutionAssetCommand(
      action: archived ? "archive" : "restore",
      operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      assetKind: revision.assetKind,
      definitionID: revision.definitionID, revisionID: revision.revisionID,
      artifactDigest: revision.artifactDigest,
      reasonCode: archived ? "archive_requested" : "restore_requested"
    )
    await commitEvolutionAsset(command, progress: archived ? "Archiving" : "Restoring")
  }

  public func rollbackEvolutionAsset(
    definition: EvolutionAssetDefinition,
    target: EvolutionAssetRevision
  ) async {
    guard let snapshot = evolutionAssets else {
      evolutionAssetStatus = "Refresh assets first"
      return
    }
    let command = EvolutionAssetCommand(
      action: "rollback", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      assetKind: target.assetKind,
      definitionID: definition.definitionID,
      reasonCode: "rollback_requested", targetRevisionID: target.revisionID,
      targetRevisionDigest: target.artifactDigest,
      fromRevisionID: definition.activeRevisionID,
      fromDigest: snapshot.revisions.first(where: {
        $0.definitionID == definition.definitionID && $0.revisionID == definition.activeRevisionID
      })?.artifactDigest
    )
    await commitEvolutionAsset(command, progress: "Rolling back exact revision")
  }

  public func evaluateEvolutionCandidate(_ candidate: EvolutionAssetCandidate) async {
    guard let snapshot = evolutionAssets,
      candidate.decision.isEmpty,
      let revision = snapshot.revisions.first(where: {
        $0.definitionID == candidate.definitionID && $0.revisionID == candidate.revisionID
      })
    else {
      evolutionAssetStatus = "Exact Candidate unavailable"
      return
    }
    let baseline =
      snapshot.revisions.first(where: {
        $0.definitionID == candidate.definitionID
          && $0.revisionID
            == snapshot.definitions.first(where: {
              $0.definitionID == candidate.definitionID
            })?.activeRevisionID
      }) ?? revision
    let caseIDs = ["artifact_digest", "runtime_compatibility", "security_boundary"]
    // Canonical content-addressed evaluation fixture, byte-identical to
    // app.CanonicalEvolutionEvaluationFixture (Implementation Repair §4).
    let fixture =
      "{\"schema_version\":1,\"fixture_kind\":\"synthetic\",\"case_ids\":[\"artifact_digest\",\"runtime_compatibility\",\"security_boundary\"],\"expected\":{\"quality_result\":\"pass\",\"failure_count\":0,\"usage_observed\":true,\"usage_microunits\":250,\"cost_observed\":true,\"cost_microunits\":1250,\"cost_currency\":\"USD\",\"compatibility_result\":\"compatible\",\"applicable_scope\":\"bounded_fixture\",\"regression_result\":\"equivalent\",\"security_result\":\"pass\"}}"
    let fixtureDigest = SHA256.hash(data: Data(fixture.utf8)).map {
      String(format: "%02x", $0)
    }.joined()
    let command = EvolutionAssetCommand(
      action: "record_evaluation", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      candidateID: candidate.candidateID,
      evaluationID: UUID().uuidString.lowercased(), fixtureKind: "synthetic",
      fixtureDigest: fixtureDigest, baselineRevisionID: baseline.revisionID,
      baselineDigest: baseline.artifactDigest, candidateRevisionID: revision.revisionID,
      candidateDigest: revision.artifactDigest, requestedCaseIDs: caseIDs
    )
    await commitEvolutionAsset(command, progress: "Evaluating exact Candidate")
  }

  public func bindEvolutionRevision(
    _ revision: EvolutionAssetRevision,
    to subject: EvolutionAssetBindingSubject
  ) async {
    guard let snapshot = evolutionAssets,
      revision.lifecycle == .active,
      let digest = try? Self.canonicalEvolutionBindingDigest(revision)
    else {
      evolutionAssetStatus = "Activate exact revision before binding"
      return
    }
    let binding = EvolutionAssetExactBinding(
      assetKind: revision.assetKind, definitionID: revision.definitionID,
      revisionID: revision.revisionID, sha256Digest: revision.artifactDigest,
      sourceScope: revision.sourceScope
    )
    let command = EvolutionAssetCommand(
      action: "set_binding", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      subject: subject, bindings: [binding], assetRevisionSetDigest: digest
    )
    await commitEvolutionAsset(command, progress: "Binding exact revision")
  }

  public func promoteEvolutionRun(_ source: EvolutionAssetPromotionSource) async {
    guard let snapshot = evolutionAssets,
      source.runGeneration > 0,
      !source.evidenceIDs.isEmpty,
      source.evidenceIDs.count == source.evidenceDigests.count
    else {
      evolutionAssetStatus = "Accepted Run lineage unavailable"
      return
    }
    let suffix = String(source.runDigest.prefix(16))
    let summary = "Accepted terminal Run \(source.runID) promoted by explicit user action."
    let summaryDigest = SHA256.hash(data: Data(summary.utf8)).map {
      String(format: "%02x", $0)
    }.joined()
    let command = EvolutionAssetCommand(
      action: "promote_run", operationID: UUID().uuidString.lowercased(),
      journeyID: evolutionAssetJourneyID, expectedViewVersion: snapshot.viewVersion,
      assetKind: .skill, definitionID: "skill.promoted.\(suffix)",
      revisionID: "revision.1", candidateID: UUID().uuidString.lowercased(),
      risk: "medium", sourceRunID: source.runID,
      sourceRunGeneration: source.runGeneration, sourceRunDigest: source.runDigest,
      sourceEvidenceIDs: source.evidenceIDs,
      sourceEvidenceDigests: source.evidenceDigests,
      redactedSummary: summary, redactedSummaryDigest: summaryDigest,
      scopeDifference: "new project-scoped Candidate",
      expectedBenefit: "reuse accepted terminal behavior"
    )
    await commitEvolutionAsset(command, progress: "Promoting accepted Run to Candidate")
  }

  public func compareEvolutionRevisions(
    definitionID: String,
    left: EvolutionAssetRevision,
    right: EvolutionAssetRevision
  ) async {
    guard let assetClient else {
      evolutionAssetStatus = "Assets unavailable"
      return
    }
    evolutionAssetStatus = "Comparing exact revisions"
    do {
      evolutionAssetDiff = try await assetClient.evolutionAssetDiff(
        journeyID: evolutionAssetJourneyID, definitionID: definitionID,
        left: left, right: right
      )
      evolutionAssetStatus = "Current"
    } catch {
      evolutionAssetStatus = closedClientReason(error)
    }
  }

  private func commitEvolutionAsset(_ command: EvolutionAssetCommand, progress: String) async {
    guard let assetClient else {
      evolutionAssetStatus = "Assets unavailable"
      return
    }
    evolutionAssetStatus = progress
    do {
      let receipt = try await assetClient.evolutionAssetCommand(command)
      guard receipt.operationID == command.operationID,
        receipt.action == command.action, !receipt.eventIDs.isEmpty
      else {
        throw LocalProductClientError.invalidResponse
      }
      await loadEvolutionAssets()
      await refresh()
    } catch {
      evolutionAssetStatus = closedClientReason(error)
    }
  }

  nonisolated public static func canonicalEvolutionAssetDigests(
    kind: String, definitionID: String, revisionID: String, sourcePath: String
  ) throws -> (artifact: String, content: String) {
    let url = URL(fileURLWithPath: sourcePath)
    let data = try Data(contentsOf: url, options: [.mappedIfSafe])
    guard !data.isEmpty, data.count <= 1_048_576,
      url.path == sourcePath, !url.lastPathComponent.isEmpty,
      !url.lastPathComponent.contains("/"), !url.lastPathComponent.contains("\\")
    else { throw LocalProductClientError.invalidRequest }
    func digest(_ bytes: Data) -> String {
      SHA256.hash(data: bytes).map { String(format: "%02x", $0) }.joined()
    }
    func quoted(_ value: String) throws -> String {
      let encoded = try JSONEncoder().encode(value)
      guard let text = String(data: encoded, encoding: .utf8) else {
        throw LocalProductClientError.invalidRequest
      }
      return text
    }
    let fileDigest = digest(data)
    let entry =
      "{\"relative_path\":\(try quoted(url.lastPathComponent)),\"file_mode\":384,\"file_size\":\(data.count),\"file_sha256\":\(try quoted(fileDigest)),\"content_base64\":\(try quoted(data.base64EncodedString()))}"
    let entries = "[\(entry)]"
    guard let entriesData = entries.data(using: .utf8) else {
      throw LocalProductClientError.invalidRequest
    }
    let contentDigest = digest(entriesData)
    let artifact =
      "{\"schema_version\":1,\"asset_kind\":\(try quoted(kind)),\"definition_id\":\(try quoted(definitionID)),\"revision_id\":\(try quoted(revisionID)),\"entries\":\(entries),\"content_digest\":\(try quoted(contentDigest))}"
    guard let artifactData = artifact.data(using: .utf8), artifactData.count <= 1_572_864 else {
      throw LocalProductClientError.invalidRequest
    }
    return (digest(artifactData), contentDigest)
  }

  static func canonicalEvolutionTemplateDigests(
    kind: String, definitionID: String, revisionID: String, sourcePath: String,
    templateOutput: String, parameterSchemaDigest: String,
    permissionCeilingDigest: String, scopeCeilingDigest: String
  ) throws -> (artifact: String, content: String) {
    let url = URL(fileURLWithPath: sourcePath)
    let data = try Data(contentsOf: url, options: [.mappedIfSafe])
    guard !data.isEmpty, data.count <= 1_048_576, url.path == sourcePath,
      url.lastPathComponent != "template-contract.json"
    else {
      throw LocalProductClientError.invalidRequest
    }
    func quoted(_ value: String) throws -> String {
      let encoded = try JSONEncoder().encode(value)
      guard let text = String(data: encoded, encoding: .utf8) else {
        throw LocalProductClientError.invalidRequest
      }
      return text
    }
    func digest(_ data: Data) -> String {
      SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }
    let sourceDigest = digest(data)
    let contract =
      "{\"schema_version\":1,\"asset_kind\":\(try quoted(kind)),\"template_output\":\(try quoted(templateOutput)),\"parameter_schema_digest\":\(try quoted(parameterSchemaDigest)),\"permission_ceiling_digest\":\(try quoted(permissionCeilingDigest)),\"scope_ceiling_digest\":\(try quoted(scopeCeilingDigest)),\"source_file_sha256\":\(try quoted(sourceDigest))}"
    let contractData = Data(contract.utf8)
    let contractEntry =
      "{\"relative_path\":\"template-contract.json\",\"file_mode\":384,\"file_size\":\(contractData.count),\"file_sha256\":\(try quoted(digest(contractData))),\"content_base64\":\(try quoted(contractData.base64EncodedString()))}"
    let sourceEntry =
      "{\"relative_path\":\(try quoted(url.lastPathComponent)),\"file_mode\":384,\"file_size\":\(data.count),\"file_sha256\":\(try quoted(sourceDigest)),\"content_base64\":\(try quoted(data.base64EncodedString()))}"
    let entries = "[\(contractEntry),\(sourceEntry)]"
    let contentDigest = digest(Data(entries.utf8))
    let artifact =
      "{\"schema_version\":1,\"asset_kind\":\(try quoted(kind)),\"definition_id\":\(try quoted(definitionID)),\"revision_id\":\(try quoted(revisionID)),\"entries\":\(entries),\"content_digest\":\(try quoted(contentDigest))}"
    let artifactData = Data(artifact.utf8)
    guard artifactData.count <= 1_572_864 else { throw LocalProductClientError.invalidRequest }
    return (digest(artifactData), contentDigest)
  }

  nonisolated public static func sha256Text(_ value: String) -> String {
    SHA256.hash(data: Data(value.utf8)).map { String(format: "%02x", $0) }.joined()
  }

  private static func initialJourneyID() -> String {
    let arguments = CommandLine.arguments
    if let index = arguments.firstIndex(of: "--journey-id"),
      index + 1 < arguments.count,
      validJourneyID(arguments[index + 1])
    {
      return arguments[index + 1]
    }
    return UUID().uuidString.lowercased()
  }

  private static func validJourneyID(_ value: String) -> Bool {
    let bytes = Array(value.utf8)
    guard bytes.count == 36, bytes[8] == 45, bytes[13] == 45,
      bytes[18] == 45, bytes[23] == 45, bytes[14] == 52,
      ["8", "9", "a", "b"].contains(String(UnicodeScalar(bytes[19])))
    else {
      return false
    }
    for (index, byte) in bytes.enumerated() where ![8, 13, 18, 23].contains(index) {
      guard (48...57).contains(byte) || (97...102).contains(byte) else { return false }
    }
    return true
  }

  static func canonicalEvolutionBindingDigest(
    _ revision: EvolutionAssetRevision
  ) throws -> String {
    func quoted(_ value: String) throws -> String {
      let encoded = try JSONEncoder().encode(value)
      guard let text = String(data: encoded, encoding: .utf8) else {
        throw LocalProductClientError.invalidRequest
      }
      return text
    }
    let value =
      "[{\"asset_kind\":\(try quoted(revision.assetKind.rawValue)),\"definition_id\":\(try quoted(revision.definitionID)),\"revision_id\":\(try quoted(revision.revisionID)),\"sha256_digest\":\(try quoted(revision.artifactDigest)),\"source_scope\":\(try quoted(revision.sourceScope))}]"
    return SHA256.hash(data: Data(value.utf8)).map {
      String(format: "%02x", $0)
    }.joined()
  }

  public func updateMissionBoardFilter(_ value: String) {
    workbench.updateBoardFilter(value)
  }

  public func updateMissionBoardHideCompleted(_ value: Bool) {
    workbench.updateBoardHideCompleted(value)
  }

  public func updateMissionComposerDraft(_ value: String) {
    workbench.updateComposerDraft(value)
  }

  public func selectMissionPermissionMode(_ mode: MissionPermissionMode) {
    workbench.selectPermissionMode(mode)
  }

  public func selectMissionInspector(_ tab: MissionInspectorTab) {
    workbench.selectInspector(tab)
  }

  public func setMissionInspectorVisible(_ visible: Bool) {
    workbench.setInspectorVisible(visible)
  }

  public func decideMission(
    _ command: LocalProductDecisionCommand
  ) async throws -> LocalProductDecisionResult {
    guard let decisionClient else {
      throw LocalProductClientError.unavailable
    }
    let result = try await decisionClient.decideMission(command)
    guard result.missionID == command.missionID,
      result.decisionID == command.decisionID,
      result.viewVersion.count == 64,
      command.operation != "submit" || result.authoritative
    else {
      throw LocalProductClientError.invalidResponse
    }
    await refresh()
    return result
  }

  public func openPreparedDecision(
    _ command: LocalProductDecisionCommand
  ) async {
    guard let decisionClient, command.operation == "read" else {
      activeDecisionSheet = nil
      return
    }
    do {
      activeDecisionSheet = try await decisionClient.readMissionDecision(
        command
      )
    } catch {
      activeDecisionSheet = nil
    }
  }

  public func openReadOnlyReviewDecision(
    for mission: LocalProductMissionSummary
  ) {
    guard mission.lane == "Review",
      preparedDecisionCommand(
        for: mission.missionID,
        kind: .review
      ) == nil
    else {
      activeDecisionSheet = nil
      return
    }
    activeDecisionSheet = LocalProductDecisionSheet(
      kind: .review,
      missionID: mission.missionID,
      teamInstanceID: mission.teamInstanceID,
      viewVersion: snapshot?.viewVersion ?? "",
      decisionID: "unprepared-review-\(mission.missionID)",
      decisionDigest: "",
      title: "Review Gate unavailable",
      summary:
        "This Mission is waiting for review, but no authoritative acceptance command is prepared.",
      requester: "Loom authority",
      target: mission.title,
      commandType: "No terminal verification is available",
      networkAccess: "Not accepted",
      credentialAccess: "none",
      permissionScope: "read-only",
      attemptScope: mission.currentNodeID.isEmpty
        ? "No current Attempt"
        : "Current node \(mission.currentNodeID)",
      expectedEvidence: "Accepted terminal Evidence is required before completion",
      technicalDetails: [
        "No Journal mutation is available from this sheet.",
        "Accept Result remains disabled until the authority prepares an exact command.",
      ],
      actions: ["not_now", "request_changes", "accept_result"],
      preparedActions: [],
      prepared: false,
      logicalNodeID: mission.currentNodeID,
      attemptNumber: 0,
      claimGeneration: 0
    )
  }

  public func preparedDecisionCommand(
    for missionID: String,
    kind: LocalProductDecisionKind? = nil
  ) -> LocalProductDecisionCommand? {
    snapshot?.preparedDecisions.first {
      $0.missionID == missionID && (kind == nil || $0.kind == kind)
    }
  }

  public func dismissDecisionSheet() {
    activeDecisionSheet = nil
  }

  public func submitDecisionAction(_ action: String) async {
    guard let sheet = activeDecisionSheet else { return }
    if !sheet.prepared {
      if action == "not_now" || action == "edit_scope" {
        activeDecisionSheet = nil
      }
      return
    }
    let operation =
      action == "not_now" || action == "edit_scope"
      ? "defer"
      : "submit"
    guard sheet.actions.contains(action),
      operation == "defer" || sheet.preparedActions.contains(action)
    else { return }
    let command = LocalProductDecisionCommand(
      operation: operation,
      kind: sheet.kind,
      action: action,
      missionID: sheet.missionID,
      teamInstanceID: sheet.teamInstanceID,
      viewVersion: sheet.viewVersion,
      decisionID: sheet.decisionID,
      decisionDigest: sheet.decisionDigest,
      logicalNodeID: sheet.logicalNodeID,
      attemptNumber: sheet.attemptNumber,
      claimGeneration: sheet.claimGeneration,
      correlationID: UUID().uuidString.lowercased()
    )
    do {
      _ = try await decideMission(command)
      activeDecisionSheet = nil
    } catch {
      // Keep the exact prepared sheet visible after conflict/failure.
    }
  }

  public func connectCodex() async {
    guard let setupClient else {
      setupState = .unavailable(reason: "setup_unavailable")
      return
    }
    setupState = .loading
    do {
      providerConnectionStatus = try await setupClient.connectCodex()
      for attempt in 0..<120 {
        let next = try await setupClient.setupSnapshot()
        setupSnapshot = next
        reconcileWorkspace()
        if next.codex.status == "available" {
          setupState = .ready
          return
        }
        if attempt < 119 {
          try await Task<Never, Never>.sleep(
            nanoseconds: 1_000_000_000
          )
        }
      }
      setupState = .unavailable(reason: "timeout")
    } catch is CancellationError {
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  public func startBlankBuilder() async {
    await startBuilder(
      source: "blank",
      sourceID: "",
      sourceVersion: 0,
      sourceDigest: ""
    )
  }

  public func startBuilder(
    source: String,
    sourceID: String,
    sourceVersion: Int,
    sourceDigest: String
  ) async {
    guard let setupClient else {
      setupState = .unavailable(reason: "setup_unavailable")
      return
    }
    setupState = .loading
    do {
      builderSession = try await setupClient.startBuilder(
        source: source,
        sourceID: sourceID,
        sourceVersion: sourceVersion,
        sourceDigest: sourceDigest
      )
      lastConfirmation = nil
      builderRecoveryMessage = nil
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  public func startBuilder(from team: LocalProductSetupSavedTeam) async {
    guard team.status == "active" else {
      setupState = .unavailable(reason: "incompatible")
      return
    }
    await startBuilder(
      source: "saved_team",
      sourceID: team.id,
      sourceVersion: team.version,
      sourceDigest: team.definitionDigest
    )
  }

  public func startBuilder(from template: LocalProductSetupTemplate) async {
    await startBuilder(
      source: "template",
      sourceID: template.id,
      sourceVersion: template.version,
      sourceDigest: template.digest
    )
  }

  public func answerBuilder(_ answer: String) async {
    guard let setupClient, let builderSession else {
      setupState = .unavailable(reason: "builder_unavailable")
      return
    }
    setupState = .loading
    do {
      self.builderSession = try await setupClient.answerBuilder(
        session: builderSession,
        answer: answer
      )
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  /// Commits any uncommitted Team name / Bounded purpose edits, then confirms
  /// the draft. The builder fields apply on Return; a user who types and
  /// clicks Confirm directly must not be blocked by that hidden step.
  public func confirmBuilderCommitting(
    name: String,
    purpose: String
  ) async {
    guard let session = builderSession else { return }
    let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
    let trimmedPurpose = purpose.trimmingCharacters(
      in: .whitespacesAndNewlines
    )
    if !trimmedName.isEmpty, trimmedName != session.preview.name {
      await editBuilder(field: "team_name", value: trimmedName)
    }
    if !trimmedPurpose.isEmpty, trimmedPurpose != session.preview.purpose {
      await editBuilder(field: "purpose", value: trimmedPurpose)
    }
    await confirmBuilder()
  }

  public func confirmBuilder() async {
    guard setupState != .loading else { return }
    guard let setupClient, let builderSession, builderSession.canConfirm else {
      if builderRecoveryMessage != nil { return }
      setupState = .unavailable(reason: "confirmation_unavailable")
      return
    }
    setupState = .loading
    do {
      let definitionID = "team-\(UUID().uuidString.lowercased())"
      lastConfirmation = try await setupClient.confirmBuilder(
        session: builderSession,
        definitionID: definitionID
      )
      self.builderSession = nil
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      if lastConfirmation?.teamInstanceCreated == true {
        await refresh()
      }
      builderRecoveryMessage = nil
      setupState = .ready
    } catch let error as LocalIPCRemoteError
      where error.code == .conflict || error.code == .notFound
    {
      self.builderSession = nil
      lastConfirmation = nil
      let draftExpired = error.code == .notFound
      builderRecoveryMessage =
        draftExpired
        ? "This Agent Team draft is no longer available. Review the latest Teams, then start a new draft."
        : "This Agent Team draft changed in another window. Review the latest Teams, then start a new draft."
      do {
        setupSnapshot = try await setupClient.setupSnapshot()
        reconcileWorkspace()
      } catch {
        // The stale draft remains discarded even when the refresh is unavailable.
      }
      setupState = .unavailable(
        reason: draftExpired ? "team_draft_expired" : "team_draft_stale"
      )
    } catch {
      handleSetupError(error)
    }
  }

  public func editBuilder(
    field: String,
    value: String,
    roleAgentDefinitionID: String = ""
  ) async {
    guard let setupClient, let builderSession,
      Self.allowsBuilderEditField(field),
      !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
      Self.validBuilderEditTarget(
        field: field,
        roleAgentDefinitionID: roleAgentDefinitionID
      )
    else {
      setupState = .unavailable(reason: "invalid_request")
      return
    }
    setupState = .loading
    do {
	  if roleAgentDefinitionID.isEmpty {
		self.builderSession = try await setupClient.editBuilder(
		  session: builderSession,
		  field: field,
		  value: value
		)
	  } else {
		self.builderSession = try await setupClient.editBuilder(
		  session: builderSession,
		  field: field,
		  value: value,
		  roleAgentDefinitionID: roleAgentDefinitionID
		)
	  }
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  nonisolated public static func allowsBuilderEditField(_ field: String) -> Bool {
    [
      "team_name", "purpose", "main_role", "subagent_role",
	  "subagent_add", "subagent_remove",
	  "main_fallback_role", "subagent_fallback_role",
	  "main_parallel_route_add", "subagent_parallel_route_add",
	  "main_parallel_route_remove", "subagent_parallel_route_remove",
	  "main_harness_route", "subagent_harness_route",
	  "main_provider_account_route", "subagent_provider_account_route",
      "main_model", "subagent_model",
      "main_reasoning_effort", "subagent_reasoning_effort",
      "main_timeout_seconds", "subagent_timeout_seconds",
      "main_budget_units", "subagent_budget_units",
      "main_remote_tool_enrollment", "subagent_remote_tool_enrollment",
    ].contains(field)
  }

  nonisolated public static func validBuilderEditTarget(
    field: String,
    roleAgentDefinitionID: String
  ) -> Bool {
    let target = roleAgentDefinitionID.trimmingCharacters(
      in: .whitespacesAndNewlines
    )
    if field == "subagent_add" {
      return target.isEmpty
    }
    if field.hasPrefix("subagent_") {
      return !target.isEmpty && target.count <= 128
    }
    return target.isEmpty
  }

  public func editBuilderRemoteToolEnrollment(
    enrollment: LocalProductRemoteToolBackendEnrollment,
    roleAgentDefinitionID: String = ""
  ) async {
    let kind = roleAgentDefinitionID.isEmpty ? "main" : "subagent"
    await editBuilder(
      field: "\(kind)_remote_tool_enrollment",
      value: "\(enrollment.enrollmentID):\(enrollment.enrollmentDigest)",
      roleAgentDefinitionID: roleAgentDefinitionID
    )
  }

  public func clearBuilderRemoteToolEnrollment(
    roleAgentDefinitionID: String = ""
  ) async {
    let kind = roleAgentDefinitionID.isEmpty ? "main" : "subagent"
    await editBuilder(
      field: "\(kind)_remote_tool_enrollment",
      value: "none",
      roleAgentDefinitionID: roleAgentDefinitionID
    )
  }

  public func cancelBuilder() {
    builderSession = nil
    lastConfirmation = nil
    builderRecoveryMessage = nil
    setupState = .ready
  }

  public func selectWorkspaceTask(_ id: String) {
    invalidateTimelineLoad()
    workspace.selectTask(id)
  }

  public func activateWorkspaceTask() async {
    let prefix = "team:"
    guard workspace.selectedTaskID.hasPrefix(prefix),
      let team = snapshot?.teams.first(where: {
        "team:\($0.teamInstanceID)" == workspace.selectedTaskID
      })
    else {
      return
    }
    selectTeam(team)
    await activateSelectedTeam()
  }

  public func updateComposerDraft(_ value: String) {
    workspace.updateComposerDraft(value)
  }

  public func selectWorkspaceFolderDisplayName(_ value: String) {
    selectedConversationWorkspacePath = ""
    workspace.selectFolderDisplayName(value)
  }

  public func selectWorkspaceFolder(_ url: URL) {
    guard url.isFileURL else { return }
    let standardizedURL = url.standardizedFileURL
    var isDirectory: ObjCBool = false
    guard standardizedURL.path.hasPrefix("/"),
      FileManager.default.fileExists(
        atPath: standardizedURL.path,
        isDirectory: &isDirectory
      ),
      isDirectory.boolValue
    else { return }
    selectedConversationWorkspacePath = standardizedURL.path
    workspace.selectFolderDisplayName(standardizedURL.lastPathComponent)
  }

  public func selectInspector(_ inspector: LocalProductInspectorTab) {
    workspace.selectInspector(inspector)
  }

  public func setDeveloperDetailsExpanded(_ expanded: Bool) {
    workspace.setDeveloperDetailsExpanded(expanded)
  }

  public func archiveTeam(_ team: LocalProductSetupSavedTeam) async {
    await setTeamStatus(team, restoring: false)
  }

  public func restoreTeam(_ team: LocalProductSetupSavedTeam) async {
    await setTeamStatus(team, restoring: true)
  }

  public func rotateCredentialVault() async {
    guard let setupClient, !isRotatingCredentialVault else { return }
    isRotatingCredentialVault = true
    credentialVaultOperationDetail = nil
    credentialVaultOperationFailed = false
    defer { isRotatingCredentialVault = false }
    do {
      try await setupClient.rotateCredentialVault()
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      credentialVaultOperationDetail = "Vault key rotated"
    } catch let failure as LocalIPCRemoteError {
      credentialVaultOperationFailed = true
      let stage =
        failure.stage?.rawValue.replacingOccurrences(
          of: "_", with: " "
        ) ?? "vault rotation"
      let incident = failure.incidentID.map { " · Incident \($0)" } ?? ""
      credentialVaultOperationDetail = "\(stage.capitalized) failed\(incident)"
    } catch {
      credentialVaultOperationFailed = true
      credentialVaultOperationDetail = "Vault rotation failed"
    }
  }

  public func setCredentialVaultLocked(_ locked: Bool) async {
    guard let setupClient, !isUpdatingCredentialVaultLock else { return }
    isUpdatingCredentialVaultLock = true
    credentialVaultOperationDetail = nil
    credentialVaultOperationFailed = false
    defer { isUpdatingCredentialVaultLock = false }
    do {
      if locked {
        try await setupClient.lockCredentialVault()
      } else {
        try await setupClient.unlockCredentialVault()
      }
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      credentialVaultOperationDetail = locked ? "Vault locked" : "Vault unlocked"
    } catch let failure as LocalIPCRemoteError {
      credentialVaultOperationFailed = true
      let fallback = locked ? "vault lock" : "vault unlock"
      let stage =
        failure.stage?.rawValue.replacingOccurrences(
          of: "_", with: " "
        ) ?? fallback
      let incident = failure.incidentID.map { " · Incident \($0)" } ?? ""
      credentialVaultOperationDetail = "\(stage.capitalized) failed\(incident)"
    } catch {
      credentialVaultOperationFailed = true
      credentialVaultOperationDetail =
        locked
        ? "Vault lock failed" : "Vault unlock failed"
    }
  }

  public func resetCredentialVault() async {
    guard let setupClient, !isResettingCredentialVault else { return }
    isResettingCredentialVault = true
    credentialVaultOperationDetail = nil
    credentialVaultOperationFailed = false
    defer { isResettingCredentialVault = false }
    do {
      try await setupClient.resetCredentialVault()
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      credentialVaultOperationDetail =
        "Vault reset complete · Re-enter each Provider Account key"
    } catch let failure as LocalIPCRemoteError {
      credentialVaultOperationFailed = true
      let stage =
        failure.stage?.rawValue.replacingOccurrences(
          of: "_", with: " "
        ) ?? "vault recovery"
      let incident = failure.incidentID.map { " · Incident \($0)" } ?? ""
      credentialVaultOperationDetail = "\(stage.capitalized) failed\(incident)"
    } catch {
      credentialVaultOperationFailed = true
      credentialVaultOperationDetail = "Vault recovery failed"
    }
  }

  public func exportCredentialVault(
    passphrase: String,
    destination: String
  ) async -> Bool {
    guard let setupClient, !isExportingCredentialVault else { return false }
    isExportingCredentialVault = true
    credentialVaultOperationDetail = nil
    credentialVaultOperationFailed = false
    defer { isExportingCredentialVault = false }
    do {
      let result = try await setupClient.exportCredentialVault(
        passphrase: passphrase,
        destination: destination
      )
      credentialVaultOperationDetail =
        "Encrypted backup saved · \(result.credentialCount) credentials"
      return true
    } catch let failure as LocalIPCRemoteError {
      credentialVaultOperationFailed = true
      let stage =
        failure.stage?.rawValue.replacingOccurrences(
          of: "_", with: " "
        ) ?? "vault export"
      let incident = failure.incidentID.map { " · Incident \($0)" } ?? ""
      credentialVaultOperationDetail = "\(stage.capitalized) failed\(incident)"
    } catch {
      credentialVaultOperationFailed = true
      credentialVaultOperationDetail = "Encrypted backup failed"
    }
    return false
  }

  private func setTeamStatus(
    _ team: LocalProductSetupSavedTeam,
    restoring: Bool
  ) async {
    guard let setupClient, team.streamHead > 0 else {
      setupState = .unavailable(reason: "invalid_request")
      return
    }
    setupState = .loading
    do {
      if restoring {
        _ = try await setupClient.restoreTeam(
          definitionID: team.id,
          expectedHead: team.streamHead
        )
      } else {
        _ = try await setupClient.archiveTeam(
          definitionID: team.id,
          expectedHead: team.streamHead
        )
      }
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  public func configureMiniMax(secret: String) async {
    guard let setupClient else {
      setupState = .unavailable(reason: "credential_unavailable")
      return
    }
    setupState = .loading
    do {
      credentialStatus = try await setupClient.configureMiniMax(
        secret: secret
      )
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  public func connectProvider(providerID: String, secret: String) async {
    await connectProvider(
      providerID: providerID,
      providerAccountID: providerID + ".primary",
      operationKey: providerID,
      secret: secret
    )
  }

  public func connectProvider(
    providerID: String,
    providerAccountID: String,
    secret: String
  ) async {
    await connectProvider(
      providerID: providerID,
      providerAccountID: providerAccountID,
      operationKey: providerAccountID,
      secret: secret
    )
  }

  public func configureProviderAccountPolicy(
    providerID: String,
    providerAccountID: String,
    expectedRevision: Int64,
    maximumConcurrentAttempts: Int,
    dispatchWindowSeconds: Int64,
    maximumDispatchStarts: Int,
    maximumAssignedBudgetUnits: Int64,
    trustDomain: String,
    retentionMode: String,
    dataRegion: String
  ) async -> Bool {
    guard let setupClient,
      LocalIPCClient.validIdentifier(providerID),
      LocalIPCClient.validProviderAccountID(
        providerAccountID,
        providerID: providerID
      ),
      expectedRevision >= 0,
      expectedRevision < Int64(UInt32.max),
      (1...64).contains(maximumConcurrentAttempts),
      (1...86_400).contains(dispatchWindowSeconds),
      (1...1_000_000).contains(maximumDispatchStarts),
      maximumAssignedBudgetUnits >= 0,
      ["external_provider", "enterprise_tenant", "local_runtime"].contains(trustDomain),
      ["provider_default", "zero_data_retention", "limited_retention"].contains(retentionMode),
      ["global", "us", "eu", "apac", "local"].contains(dataRegion),
      !providerPolicyAccountsInFlight.contains(providerAccountID)
    else { return false }

    providerPolicyAccountsInFlight.insert(providerAccountID)
    providerPolicyOperationDetail[providerAccountID] = nil
    defer { providerPolicyAccountsInFlight.remove(providerAccountID) }
    do {
      let result = try await setupClient.configureProviderAccountPolicy(
        providerID: providerID,
        providerAccountID: providerAccountID,
        expectedRevision: expectedRevision,
        maximumConcurrentAttempts: maximumConcurrentAttempts,
        dispatchWindowSeconds: dispatchWindowSeconds,
        maximumDispatchStarts: maximumDispatchStarts,
        maximumAssignedBudgetUnits: maximumAssignedBudgetUnits,
        trustDomain: trustDomain,
        retentionMode: retentionMode,
        dataRegion: dataRegion
      )
      let refreshed = try await setupClient.setupSnapshot()
      guard let account = refreshed.providerAccounts.first(where: {
        $0.providerID == providerID && $0.providerAccountID == providerAccountID
      }), account.policyAvailable,
        account.policyRevision == result.revision,
        account.maximumConcurrentAttempts == result.maximumConcurrentAttempts,
        account.dispatchWindowSeconds == result.dispatchWindowSeconds,
        account.maximumDispatchStarts == result.maximumDispatchStarts,
        account.maximumAssignedBudgetUnits == result.maximumAssignedBudgetUnits,
        account.policyVersion == result.policyVersion,
        account.policyDigest == result.policyDigest,
        account.trustDomain == result.trustDomain,
        account.retentionMode == result.retentionMode,
        account.dataRegion == result.dataRegion
      else {
        throw LocalProductClientError.invalidResponse
      }
      setupSnapshot = refreshed
      providerPolicyOperationDetail[providerAccountID] = "Limits saved"
      setupState = .ready
      return true
    } catch let remote as LocalIPCRemoteError {
      let incident = remote.incidentID.map {
        $0.isEmpty ? "" : " · Incident \($0)"
      } ?? ""
      switch remote.code {
      case .conflict, .staleGeneration, .staleView:
        providerPolicyOperationDetail[providerAccountID] =
          "Limits changed elsewhere. Reload this account and try again\(incident)"
      case .invalidRequest:
        providerPolicyOperationDetail[providerAccountID] =
          "Review the account limits and try again\(incident)"
      case .stateUnavailable, .busy, .timeout:
        providerPolicyOperationDetail[providerAccountID] =
          "Account governance is temporarily unavailable. Try again\(incident)"
      default:
        providerPolicyOperationDetail[providerAccountID] =
          "Loom could not save these account limits\(incident)"
      }
      return false
    } catch {
      providerPolicyOperationDetail[providerAccountID] =
        error as? LocalProductClientError == .invalidResponse
          ? "The local service returned inconsistent account limits. Reload and try again"
          : "Loom could not save these account limits"
      return false
    }
  }

  public func configureProviderModelRateCard(
    providerID: String,
    providerAccountID: String,
    modelID: String,
    expectedRevision: Int64,
    currency: String,
    inputTokenBasis: String,
    inputMicrounitsPerMillion: Int64,
    outputMicrounitsPerMillion: Int64,
    cacheReadMicrounitsPerMillion: Int64,
    cacheWriteMicrounitsPerMillion: Int64
  ) async -> Bool {
    let operationKey = providerAccountID + "\u{1f}" + modelID
    let rates = [
      inputMicrounitsPerMillion, outputMicrounitsPerMillion,
      cacheReadMicrounitsPerMillion, cacheWriteMicrounitsPerMillion,
    ]
    guard let setupClient,
      LocalIPCClient.validIdentifier(providerID),
      LocalIPCClient.validProviderAccountID(providerAccountID, providerID: providerID),
      LocalIPCClient.validModelID(modelID),
      expectedRevision >= 0, expectedRevision < Int64(UInt32.max),
      currency.utf8.count == 3,
      currency.utf8.allSatisfy({ $0 >= 0x41 && $0 <= 0x5a }),
      ["input_includes_cache", "input_excludes_cache"].contains(inputTokenBasis),
      rates.allSatisfy({ (0...1_000_000_000_000).contains($0) }),
      !providerRateCardsInFlight.contains(operationKey)
    else { return false }

    providerRateCardsInFlight.insert(operationKey)
    providerRateCardOperationDetail[operationKey] = nil
    defer { providerRateCardsInFlight.remove(operationKey) }
    do {
      let result = try await setupClient.configureProviderModelRateCard(
        providerID: providerID, providerAccountID: providerAccountID,
        modelID: modelID, expectedRevision: expectedRevision,
        currency: currency, inputTokenBasis: inputTokenBasis,
        inputMicrounitsPerMillion: inputMicrounitsPerMillion,
        outputMicrounitsPerMillion: outputMicrounitsPerMillion,
        cacheReadMicrounitsPerMillion: cacheReadMicrounitsPerMillion,
        cacheWriteMicrounitsPerMillion: cacheWriteMicrounitsPerMillion
      )
      let refreshed = try await setupClient.setupSnapshot()
      guard let account = refreshed.providerAccounts.first(where: {
        $0.providerID == providerID && $0.providerAccountID == providerAccountID
      }), let rateCard = account.rateCards.first(where: { $0.modelID == modelID }),
        rateCard.revision == result.revision,
        rateCard.rateCardDigest == result.rateCardDigest,
        rateCard.currency == result.currency,
        rateCard.inputTokenBasis == result.inputTokenBasis
      else { throw LocalProductClientError.invalidResponse }
      setupSnapshot = refreshed
      providerRateCardOperationDetail[operationKey] = "Rate card saved"
      setupState = .ready
      return true
    } catch let remote as LocalIPCRemoteError {
      let incident = remote.incidentID.map { $0.isEmpty ? "" : " · Incident \($0)" } ?? ""
      switch remote.code {
      case .conflict, .staleGeneration, .staleView:
        providerRateCardOperationDetail[operationKey] =
          "This rate card changed elsewhere. Reload and try again\(incident)"
      case .invalidRequest:
        providerRateCardOperationDetail[operationKey] =
          "Review the model rates and try again\(incident)"
      case .stateUnavailable, .busy, .timeout:
        providerRateCardOperationDetail[operationKey] =
          "Cost governance is temporarily unavailable. Try again\(incident)"
      default:
        providerRateCardOperationDetail[operationKey] =
          "Loom could not save this rate card\(incident)"
      }
      return false
    } catch {
      providerRateCardOperationDetail[operationKey] =
        error as? LocalProductClientError == .invalidResponse
          ? "The local service returned an inconsistent rate card. Reload and try again"
          : "Loom could not save this rate card"
      return false
    }
  }

  public func configureRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentCommand
  ) async -> Bool {
    guard let setupClient, command.valid,
      !remoteToolEnrollmentsInFlight.contains(command.enrollmentID),
      let account = setupSnapshot?.providerAccounts.first(where: {
        $0.providerID == command.providerID
          && $0.providerAccountID == command.providerAccountID
      }), account.policyAvailable,
      account.policyVersion == command.providerAccountPolicyVersion,
      account.policyRevision == command.providerAccountPolicyRevision,
      account.policyDigest == command.providerAccountPolicyDigest,
      let existing = account.remoteToolBackends.first(where: {
        $0.enrollmentID == command.enrollmentID
      }), existing.revision == command.expectedRevision,
      existing.backendKind == command.backendKind,
      existing.adapterID == command.adapterID,
      existing.endpointFingerprint == command.endpointFingerprint,
      existing.mcpServerID == command.mcpServerID
    else { return false }

    remoteToolEnrollmentsInFlight.insert(command.enrollmentID)
    remoteToolEnrollmentOperationDetail[command.enrollmentID] = nil
    clearRemoteToolEnrollmentDiagnostics(command.enrollmentID)
    defer { remoteToolEnrollmentsInFlight.remove(command.enrollmentID) }
    do {
      let result = try await setupClient.configureRemoteToolBackend(command)
      let refreshed = try await setupClient.setupSnapshot()
      guard let projected = refreshed.providerAccounts.first(where: {
        $0.providerID == command.providerID
          && $0.providerAccountID == command.providerAccountID
      })?.remoteToolBackends.first(where: {
        $0.enrollmentID == command.enrollmentID
      }), remoteToolEnrollment(projected, matches: result),
        result.status == "active", result.policyCurrent
      else { throw LocalProductClientError.invalidResponse }
      setupSnapshot = refreshed
      remoteToolEnrollmentOperationDetail[command.enrollmentID] = "Remote tool saved"
      clearRemoteToolEnrollmentDiagnostics(command.enrollmentID)
      setupState = .ready
      return true
    } catch {
      recordRemoteToolEnrollmentDiagnostics(error, enrollmentID: command.enrollmentID)
      remoteToolEnrollmentOperationDetail[command.enrollmentID] =
        remoteToolEnrollmentFailureDetail(error, operation: "save")
      return false
    }
  }

  public func revokeRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentRevokeCommand
  ) async -> Bool {
    guard let setupClient, command.valid,
      !remoteToolEnrollmentsInFlight.contains(command.enrollmentID),
      let existing = setupSnapshot?.providerAccounts.first(where: {
        $0.providerID == command.providerID
          && $0.providerAccountID == command.providerAccountID
      })?.remoteToolBackends.first(where: {
        $0.enrollmentID == command.enrollmentID
      }), existing.revision == command.expectedRevision,
      existing.status == "active"
    else { return false }

    remoteToolEnrollmentsInFlight.insert(command.enrollmentID)
    remoteToolEnrollmentOperationDetail[command.enrollmentID] = nil
    clearRemoteToolEnrollmentDiagnostics(command.enrollmentID)
    defer { remoteToolEnrollmentsInFlight.remove(command.enrollmentID) }
    do {
      let result = try await setupClient.revokeRemoteToolBackend(command)
      let refreshed = try await setupClient.setupSnapshot()
      guard let projected = refreshed.providerAccounts.first(where: {
        $0.providerID == command.providerID
          && $0.providerAccountID == command.providerAccountID
      })?.remoteToolBackends.first(where: {
        $0.enrollmentID == command.enrollmentID
      }), remoteToolEnrollment(projected, matches: result),
        result.status == "revoked"
      else { throw LocalProductClientError.invalidResponse }
      setupSnapshot = refreshed
      remoteToolEnrollmentOperationDetail[command.enrollmentID] = "Remote tool revoked"
      clearRemoteToolEnrollmentDiagnostics(command.enrollmentID)
      setupState = .ready
      return true
    } catch {
      recordRemoteToolEnrollmentDiagnostics(error, enrollmentID: command.enrollmentID)
      remoteToolEnrollmentOperationDetail[command.enrollmentID] =
        remoteToolEnrollmentFailureDetail(error, operation: "revoke")
      return false
    }
  }

  private func remoteToolEnrollment(
    _ enrollment: LocalProductRemoteToolBackendEnrollment,
    matches result: LocalProductRemoteToolBackendEnrollmentResult
  ) -> Bool {
    enrollment.enrollmentID == result.enrollmentID
      && enrollment.backendKind == result.backendKind
      && enrollment.adapterID == result.adapterID
      && enrollment.providerAccountPolicyVersion
        == result.providerAccountPolicyVersion
      && enrollment.providerAccountPolicyRevision
        == result.providerAccountPolicyRevision
      && enrollment.providerAccountPolicyDigest
        == result.providerAccountPolicyDigest
      && enrollment.policyCurrent == result.policyCurrent
      && enrollment.endpointFingerprint == result.endpointFingerprint
      && enrollment.mcpServerID == result.mcpServerID
      && enrollment.allowedTools == result.allowedTools
      && enrollment.revision == result.revision
      && enrollment.status == result.status
      && enrollment.maximumConcurrentCalls == result.maximumConcurrentCalls
      && enrollment.maximumCallsPerAttempt == result.maximumCallsPerAttempt
      && enrollment.timeoutSeconds == result.timeoutSeconds
      && enrollment.maximumResultBytes == result.maximumResultBytes
      && enrollment.maximumBudgetUnits == result.maximumBudgetUnits
      && enrollment.configuredAt == result.configuredAt
      && enrollment.enrollmentDigest == result.enrollmentDigest
  }

  private func remoteToolEnrollmentFailureDetail(
    _ error: Error,
    operation: String
  ) -> String {
    let action = operation == "revoke" ? "revoke" : "save"
    guard let remote = error as? LocalIPCRemoteError else {
      return error as? LocalProductClientError == .invalidResponse
        ? "The local service returned inconsistent remote tool state. Reload and try again"
        : "Loom could not \(action) this remote tool"
    }
    let incident = remote.incidentID.map {
      $0.isEmpty ? "" : " · Incident \($0)"
    } ?? ""
    switch remote.code {
    case .conflict, .staleGeneration, .staleView:
      return "Remote tool policy changed. Reload this account and try again\(incident)"
    case .invalidRequest:
      return "Review the remote tool limits and try again\(incident)"
    case .notFound:
      return "This remote tool no longer exists. Reload the account\(incident)"
    case .stateUnavailable, .busy, .timeout:
      return "Remote tool governance is temporarily unavailable. Try again\(incident)"
    default:
      return "Loom could not \(action) this remote tool\(incident)"
    }
  }

  private func recordRemoteToolEnrollmentDiagnostics(
    _ error: Error,
    enrollmentID: String
  ) {
    guard let remote = error as? LocalIPCRemoteError else {
      clearRemoteToolEnrollmentDiagnostics(enrollmentID)
      return
    }
    remoteToolEnrollmentStage[enrollmentID] = providerStageDisplayName(remote.stage)
    remoteToolEnrollmentIncidentID[enrollmentID] = remote.incidentID
    remoteToolEnrollmentRetryable[enrollmentID] = remote.recoverable
  }

  private func clearRemoteToolEnrollmentDiagnostics(_ enrollmentID: String) {
    remoteToolEnrollmentStage[enrollmentID] = nil
    remoteToolEnrollmentIncidentID[enrollmentID] = nil
    remoteToolEnrollmentRetryable[enrollmentID] = nil
  }

  private func connectProvider(
    providerID: String,
    providerAccountID: String,
    operationKey: String,
    secret: String
  ) async {
    guard let setupClient,
      LocalIPCClient.validProviderAccountID(
        providerAccountID,
        providerID: providerID
      ),
      !providersInFlight.contains(operationKey)
    else { return }
    providersInFlight.insert(operationKey)
    providerOperationStatus[operationKey] = "Connecting"
    providerOperationDetail[operationKey] = nil
    clearProviderOperationDiagnostics(operationKey)
    defer { providersInFlight.remove(operationKey) }
    do {
      if providerAccountID == providerID + ".primary" {
        credentialStatus = try await setupClient.configureCredential(
          providerID: providerID,
          secret: secret
        )
      } else {
        credentialStatus = try await setupClient.configureCredential(
          providerID: providerID,
          providerAccountID: providerAccountID,
          secret: secret
        )
      }
      setupSnapshot = try await setupClient.setupSnapshot()
      guard
        providerCredentialBinding(
          providerID: providerID,
          providerAccountID: providerAccountID
        ) != nil
      else {
        throw LocalProductClientError.invalidResponse
      }
      await verifyProvider(
        providerID: providerID,
        providerAccountID: providerAccountID,
        operationKey: operationKey,
        ownsFlight: true
      )
    } catch {
      recordProviderOperationError(
        providerID: providerID,
        operationKey: operationKey,
        error: error
      )
      handleSetupError(error)
    }
  }

  public func importCredentialCandidate(
    _ candidate: LocalProductCredentialImportCandidate,
    providerAccountID: String? = nil
  ) async {
    guard let setupClient,
      ["exact_provider", "custom_endpoint_review"].contains(candidate.importMode),
      candidate.credentialAvailable,
      candidate.protocolName == "openai_responses"
    else { return }
    let accountID = providerAccountID ?? candidate.targetProviderID + ".primary"
    guard LocalIPCClient.validProviderAccountID(
      accountID,
      providerID: candidate.targetProviderID
    ) else { return }
    let operationKey = accountID
    guard !providersInFlight.contains(operationKey) else { return }
    if providerCredentialBinding(
      providerID: candidate.targetProviderID,
      providerAccountID: accountID
    ) != nil {
      providerOperationStatus[operationKey] = "Already connected"
      providerOperationDetail[operationKey] =
        "This Provider Account already has a Credential Vault binding."
      return
    }
    providersInFlight.insert(operationKey)
    providerOperationStatus[operationKey] = "Importing"
    providerOperationDetail[operationKey] = nil
    clearProviderOperationDiagnostics(operationKey)
    defer { providersInFlight.remove(operationKey) }
    do {
      let result: LocalProductCredentialSetupResult
      if candidate.importMode == "custom_endpoint_review" {
        guard let review = approvedEndpointReview(
          candidateDigest: candidate.candidateDigest,
          providerAccountID: accountID
        ) else {
          throw LocalProductClientError.invalidRequest
        }
        result = try await setupClient.importReviewedEndpointCandidate(
          candidate: candidate,
          review: review,
          providerAccountID: accountID
        )
      } else {
        result = try await setupClient.importCredentialCandidate(
          candidateID: candidate.id,
          providerID: candidate.targetProviderID,
          providerAccountID: accountID
        )
      }
      let refreshed = try await setupClient.setupSnapshot()
      guard
        let binding = providerCredentialBinding(
          in: refreshed,
          providerID: candidate.targetProviderID,
          providerAccountID: accountID
        ),
        binding.revision == result.revision,
        binding.status == result.status,
        binding.reason == result.reason,
        result.status == "verified"
      else {
        throw LocalProductClientError.invalidResponse
      }
      credentialStatus = result
      setupSnapshot = refreshed
      recordProviderTerminalStatus(operationKey: operationKey, result: result)
      if accountID == candidate.targetProviderID + ".primary",
        let profile = refreshed.conversationProfiles.first(where: {
          $0.providerID == candidate.targetProviderID
        })
      {
        selectConversationProfile(profile.profileID)
      }
      reconcileWorkspace()
      setupState = .ready
    } catch {
      recordProviderOperationError(
        providerID: candidate.targetProviderID,
        operationKey: operationKey,
        error: error
      )
      handleSetupError(error)
    }
  }

  public func approveEndpointCandidate(
    _ candidate: LocalProductCredentialImportCandidate,
    providerAccountID: String? = nil
  ) async {
    guard let setupClient,
      candidate.importMode == "custom_endpoint_review",
      candidate.credentialAvailable
    else { return }
    let accountID = providerAccountID ?? candidate.targetProviderID + ".primary"
    guard LocalIPCClient.validProviderAccountID(
      accountID, providerID: candidate.targetProviderID
    ), !providersInFlight.contains(accountID) else { return }
    providersInFlight.insert(accountID)
    providerOperationStatus[accountID] = "Reviewing"
    providerOperationDetail[accountID] = nil
    clearProviderOperationDiagnostics(accountID)
    defer { providersInFlight.remove(accountID) }
    do {
      let result = try await setupClient.approveEndpointCandidate(
        candidate: candidate,
        providerAccountID: accountID
      )
      guard result.candidateDigest == candidate.candidateDigest,
        result.endpointFingerprint == candidate.endpointFingerprint,
        result.providerID == candidate.targetProviderID,
        result.providerAccountID == accountID,
        result.reviewPolicyVersion == candidate.reviewPolicyVersion,
        result.reviewPolicyDigest == candidate.reviewPolicyDigest,
        result.status == "approved", result.revision == 2
      else {
        throw LocalProductClientError.invalidResponse
      }
      approvedEndpointReviews[Self.endpointReviewCacheKey(
        candidateDigest: candidate.candidateDigest,
        providerAccountID: accountID
      )] = result
      providerOperationStatus[accountID] = "Approved"
      providerOperationDetail[accountID] =
        "Endpoint approved for this Provider Account. Import is ready for the next 15 minutes."
    } catch {
      recordProviderOperationError(
        providerID: candidate.targetProviderID,
        operationKey: accountID,
        error: error
      )
      handleSetupError(error)
    }
  }

  public func runProviderFailureLabSuite() async {
    guard let setupClient, !failureLabInFlight else { return }
    failureLabInFlight = true
    failureLabError = nil
    failureLabResults = []
    defer { failureLabInFlight = false }
    let scenarios = [
      "auth", "rate_limit", "timeout", "insufficient_balance",
      "corrupt_vault_record", "revision_conflict",
    ]
    do {
      var results: [LocalProductFailureLabResult] = []
      for scenario in scenarios {
        let result = try await setupClient.runProviderFailureLab(
          scenario: scenario,
          providerAccountID: "failure-lab.target",
          healthyPeerAccountID: "failure-lab.healthy-peer"
        )
        results.append(result)
      }
      failureLabResults = results
    } catch {
      failureLabError =
        "The local failure isolation suite did not complete. Retry and inspect the incident diagnostics."
      handleSetupError(error)
    }
  }

  public func runProviderFailureLabScenario(_ scenario: String) async {
    let scenarios = [
      "auth", "rate_limit", "timeout", "insufficient_balance",
      "corrupt_vault_record", "revision_conflict",
    ]
    guard let setupClient, scenarios.contains(scenario), !failureLabInFlight else { return }
    failureLabInFlight = true
    failureLabError = nil
    defer { failureLabInFlight = false }
    do {
      let result = try await setupClient.runProviderFailureLab(
        scenario: scenario,
        providerAccountID: "failure-lab.target",
        healthyPeerAccountID: "failure-lab.healthy-peer"
      )
      var updated = failureLabResults.filter { $0.scenario != scenario }
      updated.append(result)
      failureLabResults = updated.sorted {
        (scenarios.firstIndex(of: $0.scenario) ?? scenarios.count)
          < (scenarios.firstIndex(of: $1.scenario) ?? scenarios.count)
      }
    } catch {
      failureLabError =
        "This failure check did not complete. Retry or inspect the incident diagnostics."
      handleSetupError(error)
    }
  }

  public func approvedEndpointReview(
    candidateDigest: String,
    providerAccountID: String
  ) -> LocalProductEndpointReviewResult? {
    approvedEndpointReviews[Self.endpointReviewCacheKey(
      candidateDigest: candidateDigest,
      providerAccountID: providerAccountID
    )]
  }

  public func verifyProvider(
    providerID: String,
    ownsFlight: Bool = false
  ) async {
    await verifyProvider(
      providerID: providerID,
      providerAccountID: providerID + ".primary",
      operationKey: providerID,
      ownsFlight: ownsFlight
    )
  }

  public func verifyProvider(
    providerID: String,
    providerAccountID: String
  ) async {
    await verifyProvider(
      providerID: providerID,
      providerAccountID: providerAccountID,
      operationKey: providerAccountID,
      ownsFlight: false
    )
  }

  private func verifyProvider(
    providerID: String,
    providerAccountID: String,
    operationKey: String,
    ownsFlight: Bool
  ) async {
    guard let setupClient,
      let binding = providerCredentialBinding(
        providerID: providerID,
        providerAccountID: providerAccountID
      ),
      ownsFlight || !providersInFlight.contains(operationKey)
    else { return }
    if !ownsFlight { providersInFlight.insert(operationKey) }
    providerOperationStatus[operationKey] = "Testing"
    providerOperationDetail[operationKey] = nil
    clearProviderOperationDiagnostics(operationKey)
    defer { if !ownsFlight { providersInFlight.remove(operationKey) } }
    do {
      let result: LocalProductCredentialSetupResult
      if providerAccountID == providerID + ".primary" {
        result = try await setupClient.verifyCredential(
          providerID: providerID,
          reference: binding.reference,
          revision: binding.revision
        )
      } else {
        result = try await setupClient.verifyCredential(
          providerID: providerID,
          providerAccountID: providerAccountID,
          reference: binding.reference,
          revision: binding.revision
        )
      }
      let refreshed = try await setupClient.setupSnapshot()
      guard
        let current = providerCredentialBinding(
          in: refreshed,
          providerID: providerID,
          providerAccountID: providerAccountID
        ), current.revision == result.revision,
        current.status == result.status,
        current.reason == result.reason
      else {
        throw LocalProductClientError.invalidResponse
      }
      credentialStatus = result
      setupSnapshot = refreshed
      recordProviderTerminalStatus(
        operationKey: operationKey,
        result: result
      )
      if providerAccountID == providerID + ".primary",
        result.status == "verified",
        let profile = refreshed.conversationProfiles.first(where: {
          $0.providerID == providerID
        })
      {
        selectConversationProfile(profile.profileID)
      }
      reconcileWorkspace()
      setupState = .ready
    } catch {
      recordProviderOperationError(
        providerID: providerID,
        operationKey: operationKey,
        error: error
      )
      handleSetupError(error)
    }
  }

  public func replaceProvider(providerID: String, secret: String) async {
    await replaceProvider(
      providerID: providerID,
      providerAccountID: providerID + ".primary",
      operationKey: providerID,
      secret: secret
    )
  }

  public func replaceProvider(
    providerID: String,
    providerAccountID: String,
    secret: String
  ) async {
    await replaceProvider(
      providerID: providerID,
      providerAccountID: providerAccountID,
      operationKey: providerAccountID,
      secret: secret
    )
  }

  private func replaceProvider(
    providerID: String,
    providerAccountID: String,
    operationKey: String,
    secret: String
  ) async {
    guard let setupClient,
      let binding = providerCredentialBinding(
        providerID: providerID,
        providerAccountID: providerAccountID
      ), !providersInFlight.contains(operationKey)
    else { return }
    let migratingToVault = binding.status == "migration_required"
    providersInFlight.insert(operationKey)
    providerOperationStatus[operationKey] = "Replacing"
    providerOperationDetail[operationKey] = nil
    clearProviderOperationDiagnostics(operationKey)
    defer { providersInFlight.remove(operationKey) }
    do {
      if providerAccountID == providerID + ".primary" {
        credentialStatus = try await setupClient.replaceCredential(
          providerID: providerID,
          reference: binding.reference,
          revision: binding.revision,
          secret: secret
        )
      } else {
        credentialStatus = try await setupClient.replaceCredential(
          providerID: providerID,
          providerAccountID: providerAccountID,
          reference: binding.reference,
          revision: binding.revision,
          secret: secret
        )
      }
      setupSnapshot = try await setupClient.setupSnapshot()
      if migratingToVault {
        await verifyProvider(
          providerID: providerID,
          providerAccountID: providerAccountID,
          operationKey: operationKey,
          ownsFlight: true
        )
        return
      }
      providerOperationStatus[operationKey] = "Configured"
      reconcileWorkspace()
      setupState = .ready
    } catch {
      recordProviderOperationError(
        providerID: providerID,
        operationKey: operationKey,
        error: error
      )
      handleSetupError(error)
    }
  }

  public func revokeProvider(providerID: String) async {
    await revokeProvider(
      providerID: providerID,
      providerAccountID: providerID + ".primary",
      operationKey: providerID
    )
  }

  public func revokeProvider(
    providerID: String,
    providerAccountID: String
  ) async {
    await revokeProvider(
      providerID: providerID,
      providerAccountID: providerAccountID,
      operationKey: providerAccountID
    )
  }

  private func revokeProvider(
    providerID: String,
    providerAccountID: String,
    operationKey: String
  ) async {
    guard let setupClient,
      let binding = providerCredentialBinding(
        providerID: providerID,
        providerAccountID: providerAccountID
      ), !providersInFlight.contains(operationKey)
    else { return }
    providersInFlight.insert(operationKey)
    providerOperationStatus[operationKey] = "Removing"
    providerOperationDetail[operationKey] = nil
    clearProviderOperationDiagnostics(operationKey)
    defer { providersInFlight.remove(operationKey) }
    do {
      if providerAccountID == providerID + ".primary" {
        credentialStatus = try await setupClient.revokeCredential(
          providerID: providerID,
          reference: binding.reference,
          revision: binding.revision
        )
      } else {
        credentialStatus = try await setupClient.revokeCredential(
          providerID: providerID,
          providerAccountID: providerAccountID,
          reference: binding.reference,
          revision: binding.revision
        )
      }
      setupSnapshot = try await setupClient.setupSnapshot()
      providerOperationStatus[operationKey] = "Not connected"
      reconcileWorkspace()
      setupState = .ready
    } catch {
      recordProviderOperationError(
        providerID: providerID,
        operationKey: operationKey,
        error: error
      )
      handleSetupError(error)
    }
  }

  private typealias ProviderCredentialBinding = (
    reference: String,
    revision: Int64,
    status: String,
    reason: String
  )

  private func providerCredentialBinding(
    providerID: String,
    providerAccountID: String
  ) -> ProviderCredentialBinding? {
    guard let setupSnapshot else { return nil }
    return providerCredentialBinding(
      in: setupSnapshot,
      providerID: providerID,
      providerAccountID: providerAccountID
    )
  }

  private func providerCredentialBinding(
    in snapshot: LocalProductSetupSnapshot,
    providerID: String,
    providerAccountID: String
  ) -> ProviderCredentialBinding? {
    if let account = snapshot.providerAccounts.first(where: {
      $0.providerID == providerID && $0.providerAccountID == providerAccountID
    }), !account.credentialReference.isEmpty, account.revision > 0 {
      return (
        account.credentialReference,
        account.revision,
        account.status,
        account.reason
      )
    }
    guard providerAccountID == providerID + ".primary",
      let provider = snapshot.providers.first(where: {
        $0.providerID == providerID
      }), !provider.credentialReference.isEmpty, provider.revision > 0
    else {
      return nil
    }
    return (
      provider.credentialReference,
      provider.revision,
      provider.status,
      provider.reason
    )
  }

  private func providerDirectoryEntry(
    _ providerID: String
  ) -> LocalProductProviderDirectoryEntry? {
    setupSnapshot?.providers.first { $0.providerID == providerID }
  }

  private func providerTerminalStatus(
    _ result: LocalProductCredentialSetupResult
  ) -> String {
    switch (result.status, result.reason) {
    case ("verified", ""): return "Verified"
    case ("rejected", "provider_rejected"): return "Rejected"
    default: return "Unavailable"
    }
  }

  private func recordProviderTerminalStatus(
    operationKey: String,
    result: LocalProductCredentialSetupResult
  ) {
    providerOperationStatus[operationKey] = providerTerminalStatus(result)
    switch (result.status, result.reason) {
    case ("verified", ""):
      providerOperationDetail[operationKey] = nil
    case ("rejected", "provider_rejected"):
      providerOperationDetail[operationKey] =
        "The provider rejected this key. Check that it is active and belongs to the selected provider."
    case ("rejected", "timeout"):
      providerOperationStatus[operationKey] = "Timed out"
      providerOperationDetail[operationKey] =
        "The provider did not respond in time. Try again."
    default:
      providerOperationDetail[operationKey] =
        "The credential could not be verified. Try again or replace the key."
    }
  }

  private func recordProviderOperationError(
    providerID: String,
    operationKey: String,
    error: Error
  ) {
    let presentation: (String, String)
    if let remote = error as? LocalIPCRemoteError {
      let stagedDetail: String? =
        switch remote.stage {
        case .inputAdmission:
          "Loom rejected the credential input before sending it. Remove unexpected characters and try again."
        case .udsTransport:
          "The Loom App could not complete the secure request to its bundled service. Reopen Loom and try again."
        case .daemonAdmission:
          "The bundled service rejected the credential request. Review the input and try again."
        case .helperValidation:
          "Loom rejected the local credential request before Keychain access. Reopen Loom and try again."
        case .helperStart:
          "The bundled credential helper could not start. Reinstall Loom, then try again."
        case .helperAuthorization:
          "macOS could not verify Loom's bundled credential helper. Reinstall or reopen Loom, then try again."
        case .helperRequest, .helperResponse, .helperExit:
          "The secure transfer to macOS Keychain did not complete. Reopen Loom and try again."
        case .helperTimeout:
          "The credential helper did not respond in time. Reopen Loom and try again."
        case .keychainAccess:
          "macOS Keychain denied or could not complete this operation. Check Keychain access and try again."
        case .vaultKeyLoad:
          "Loom could not unlock its local Credential Vault. Restart Loom and review diagnostics."
        case .vaultOpen:
          "Loom could not open the encrypted Credential Vault. Review its local storage permissions."
        case .vaultEncrypt, .vaultCommit:
          "Loom could not safely commit this key to the Credential Vault. Try again."
        case .vaultDecrypt, .vaultAADValidation:
          "The encrypted credential did not match this Provider Account and revision. Re-enter the key."
        case .vaultRotation:
          "Credential Vault rotation did not complete. Review diagnostics before retrying."
        case .vaultRecovery:
          "Credential Vault recovery did not complete. Review diagnostics before retrying."
        case .vaultExport:
          "Credential Vault export did not complete. Choose a new file and retry."
        case .credentialLeaseIssue, .credentialLeaseExpire, .credentialLeaseRevoke:
          "Loom could not issue the exact short-lived credential lease. Re-enter or verify this account's key."
        case .migrationRead, .migrationCommit, .migrationCleanup:
          "Credential migration did not complete. The existing credential was not removed; retry or re-enter the key."
        case .metadataCommit:
          "The key was rolled back because Loom could not commit its non-secret configuration. Try again."
        case .projectionRefresh:
          "The key was stored, but Loom could not refresh the visible configuration. Reopen Loom and try again."
        case .providerDNS, .providerTLS, .providerConnect, .providerHTTP:
          "Loom could not reach the provider securely. Check the connection and try again."
        case .providerAuth:
          "The provider did not accept this credential. Check the key and selected provider."
        case .providerRateLimit:
          "The selected Provider Account is rate limited. Wait, then try again."
        case .profilePublish:
          "The credential was verified, but its conversation Profile could not be published. Try again."
		case .toolRecovery:
		  "The ToolCall recovery decision could not be recorded. Refresh the candidate before retrying."
        case .conversationDispatch:
          "The conversation could not be dispatched with the selected Profile. Try again."
        case .agentAttemptDispatch:
          "The Agent Attempt could not be dispatched with its frozen binding. Review the Agent configuration."
        case .agentAttemptReconcile:
          "The Agent Attempt needs an explicit recovery decision after Loom restarted. Review the Agent diagnostics."
        case .agentInputAdmission:
          "The running Agent could not admit this input. Refresh the Mission and retry its current Attempt."
        case .preflightLease, .viewDrift, .preflightDigest:
          "The reviewed Mission preflight changed or expired. Review it again, then retry."
        case .dispatchCapacity:
          "The selected Runtime is at capacity. Retry when its current work finishes."
        case .dispatchTeamAuthority:
          "The previous Team Attempt has not fully settled. Refresh Missions, then start a new audited Attempt."
        case .dispatchRecoveryRequired:
          "This Team has an interrupted Attempt that needs recovery before a new run."
		case .workspacePublication, .workspacePublicationInputValidation,
		  .workspacePublicationSourceSnapshot, .workspacePublicationSourceDrift,
		  .workspacePublicationStageCreate, .workspacePublicationChangeValidation,
		  .workspacePublicationChangeConflict, .workspacePublicationDestructiveChange,
		  .workspacePublicationStageWrite, .workspacePublicationApply,
		  .workspacePublicationFinalDigest, .workspacePublicationSync,
		  .workspacePublicationMainNodeMissing, .workspacePublicationMainCandidateMissing,
		  .workspacePublicationMainChangesMissing, .workspacePublicationMainDigestMissing,
		  .workspacePublicationCancelled:
		  "The accepted Mission result could not be published to the selected project. Retry the Mission and review its incident details."
        case .parentContinuation, .flightConflict, .dispatchAdmission,
          .dispatchAttemptValidation, .dispatchViewConflict,
          .dispatchIdentityUnavailable, .dispatchValidation,
          .dispatchContextValidation, .dispatchIncomplete:
          "The Mission could not be admitted at its reported governance stage. Open Missions to inspect the current Attempt."
        case nil:
          nil
        }
      switch remote.code {
      case .invalidRequest:
        presentation = (
          "Invalid key input",
          "Remove unexpected characters and try again. Loom trims only spaces, tabs, and line breaks at the beginning or end."
        )
      case .credentialUnavailable, .credentialRollbackFailed:
        presentation = (
          "Credential Vault unavailable",
          stagedDetail
            ?? "Loom could not safely complete this Credential Vault operation. Review diagnostics and try again."
        )
      case .denied, .unauthorizedPeer:
        presentation = (
          "Access denied",
          stagedDetail
            ?? "macOS denied access to the Loom credential helper. Reopen Loom and try again."
        )
      case .credentialRejected:
        presentation = (
          "Provider rejected key",
          "The provider rejected this key. Check that it is active and belongs to the selected provider."
        )
      case .timeout:
        presentation = (
          "Timed out",
          stagedDetail ?? "The Credential Vault or provider operation timed out. Try again."
        )
      default:
        presentation = (
          "Connection failed",
          "Loom could not complete this credential operation. Try again."
        )
      }
      providerOperationStage[operationKey] = providerStageDisplayName(remote.stage)
      providerOperationIncidentID[operationKey] = remote.incidentID
      providerOperationRetryable[operationKey] = remote.recoverable
    } else if let local = error as? LocalProductClientError {
      switch local {
      case .invalidRequest:
        presentation = (
          "Invalid key input",
          "Remove unexpected characters and try again. Loom trims only spaces, tabs, and line breaks at the beginning or end."
        )
      case .timeout:
        presentation = ("Timed out", "The local service did not respond in time. Try again.")
      case .invalidSocket, .unavailable:
        presentation = ("Local service unavailable", "Reopen Loom and try again.")
      case .invalidResponse:
        presentation = (
          "Invalid service response", "Loom received an unexpected response. Try again."
        )
      case .notFound:
        presentation = ("Credential not found", "Reconnect this provider with its API key.")
      }
    } else {
      presentation = (
        "Connection failed", "Loom could not complete this credential operation. Try again."
      )
    }
    providerOperationStatus[operationKey] = presentation.0
    providerOperationDetail[operationKey] = presentation.1
  }

  private func clearProviderOperationDiagnostics(_ providerID: String) {
    providerOperationStage[providerID] = nil
    providerOperationIncidentID[providerID] = nil
    providerOperationRetryable[providerID] = nil
  }

  private func providerStageDisplayName(
    _ stage: LocalIPCRemoteError.Stage?
  ) -> String? {
    switch stage {
    case .inputAdmission: return "Input admission"
    case .udsTransport: return "Local service transport"
    case .daemonAdmission: return "Local service admission"
    case .helperValidation: return "Credential helper validation"
    case .helperStart: return "Credential helper start"
    case .helperAuthorization: return "Credential helper authorization"
    case .helperRequest: return "Credential helper request"
    case .helperTimeout: return "Credential helper timeout"
    case .helperResponse: return "Credential helper response"
    case .helperExit: return "Credential helper exit"
    case .keychainAccess: return "Keychain access"
    case .vaultKeyLoad: return "Vault key load"
    case .vaultOpen: return "Vault open"
    case .vaultEncrypt: return "Vault encryption"
    case .vaultCommit: return "Vault commit"
    case .vaultDecrypt: return "Vault decryption"
    case .vaultAADValidation: return "Vault binding validation"
    case .vaultRotation: return "Vault rotation"
    case .vaultRecovery: return "Vault recovery"
    case .vaultExport: return "Vault export"
    case .credentialLeaseIssue: return "Credential lease issue"
    case .credentialLeaseExpire: return "Credential lease expiry"
    case .credentialLeaseRevoke: return "Credential lease revoke"
    case .migrationRead: return "Credential migration read"
    case .migrationCommit: return "Credential migration commit"
    case .migrationCleanup: return "Credential migration cleanup"
    case .metadataCommit: return "Configuration commit"
    case .projectionRefresh: return "Configuration refresh"
    case .providerDNS: return "Provider DNS"
    case .providerTLS: return "Provider TLS"
    case .providerConnect: return "Provider connection"
    case .providerHTTP: return "Provider response"
    case .providerAuth: return "Provider authentication"
    case .providerRateLimit: return "Provider rate limit"
    case .profilePublish: return "Conversation profile publish"
    case .conversationDispatch: return "Conversation dispatch"
    case .agentAttemptDispatch: return "Agent attempt dispatch"
    case .agentAttemptReconcile: return "Agent attempt recovery"
    case .agentInputAdmission: return "Agent input admission"
	case .toolRecovery: return "Tool recovery"
    case .preflightLease: return "Mission preflight lease"
    case .viewDrift: return "Mission view drift"
    case .preflightDigest: return "Mission preflight digest"
    case .parentContinuation: return "Mission parent continuation"
    case .flightConflict: return "Mission active execution"
    case .dispatchAdmission: return "Mission dispatch admission"
    case .dispatchTeamAuthority: return "Team execution authority"
    case .dispatchCapacity: return "Mission dispatch capacity"
    case .dispatchAttemptValidation: return "Mission attempt validation"
    case .dispatchViewConflict: return "Mission dispatch view"
    case .dispatchIdentityUnavailable: return "Mission dispatch identity"
    case .dispatchRecoveryRequired: return "Mission recovery"
    case .dispatchValidation: return "Mission dispatch validation"
    case .dispatchContextValidation: return "Mission context validation"
    case .dispatchIncomplete: return "Mission dispatch visibility"
	case .workspacePublication: return "Workspace publication"
	case .workspacePublicationInputValidation: return "Workspace publication input"
	case .workspacePublicationSourceSnapshot: return "Workspace source snapshot"
	case .workspacePublicationSourceDrift: return "Workspace source drift"
	case .workspacePublicationStageCreate: return "Workspace publication staging"
	case .workspacePublicationChangeValidation: return "Workspace change validation"
	case .workspacePublicationChangeConflict: return "Workspace change conflict"
	case .workspacePublicationDestructiveChange: return "Workspace destructive change review"
	case .workspacePublicationStageWrite: return "Workspace publication write"
	case .workspacePublicationApply: return "Workspace publication apply"
	case .workspacePublicationFinalDigest: return "Workspace publication verification"
	case .workspacePublicationSync: return "Workspace publication sync"
	case .workspacePublicationMainNodeMissing: return "Main Agent publication route"
	case .workspacePublicationMainCandidateMissing: return "Main Agent result collection"
	case .workspacePublicationMainChangesMissing: return "Main Agent workspace changes"
	case .workspacePublicationMainDigestMissing: return "Main Agent workspace verification"
	case .workspacePublicationCancelled: return "Workspace publication cancelled"
    case nil: return nil
    }
  }

  public func verifyMiniMax() async {
    guard let setupClient,
      let provider = setupSnapshot?.miniMax,
      !provider.credentialReference.isEmpty,
      provider.revision > 0,
      !isVerifyingMiniMax
    else {
      setupState = .unavailable(reason: "credential_unavailable")
      return
    }
    let expectedRevision = provider.revision + 1
    isVerifyingMiniMax = true
    miniMaxVerificationStatus = "Testing"
    defer { isVerifyingMiniMax = false }
    setupState = .loading
    do {
      let result = try await setupClient.verifyMiniMax(
        reference: provider.credentialReference,
        revision: provider.revision
      )
      guard result.providerID == "minimax",
        result.revision == expectedRevision,
        let terminal = miniMaxTerminalStatus(result)
      else {
        throw LocalProductClientError.invalidResponse
      }
      let refreshed = try await setupClient.setupSnapshot()
      guard refreshed.miniMax.providerID == "minimax",
        refreshed.miniMax.credentialReference == provider.credentialReference,
        refreshed.miniMax.revision == expectedRevision,
        refreshed.miniMax.status == result.status,
        refreshed.miniMax.reason == result.reason
      else {
        throw LocalProductClientError.invalidResponse
      }
      credentialStatus = result
      setupSnapshot = refreshed
      miniMaxVerificationStatus = terminal
      reconcileWorkspace()
      setupState = .ready
    } catch {
      if let remote = error as? LocalIPCRemoteError,
        remote.code == .conflict
      {
        miniMaxVerificationStatus = "Conflict"
      } else {
        miniMaxVerificationStatus = "Unavailable"
      }
      handleSetupError(error)
    }
  }

  private func miniMaxTerminalStatus(
    _ result: LocalProductCredentialSetupResult
  ) -> String? {
    switch (result.status, result.reason) {
    case ("verified", ""):
      return "Verified"
    case ("rejected", "provider_rejected"):
      return "Rejected"
    case ("rejected", "unavailable"), ("rejected", "timeout"):
      return "Unavailable"
    default:
      return nil
    }
  }

  public func replaceMiniMax(secret: String) async {
    guard let setupClient,
      let provider = setupSnapshot?.miniMax,
      !provider.credentialReference.isEmpty,
      provider.revision > 0
    else {
      setupState = .unavailable(reason: "credential_unavailable")
      return
    }
    setupState = .loading
    do {
      credentialStatus = try await setupClient.replaceMiniMax(
        reference: provider.credentialReference,
        revision: provider.revision,
        secret: secret
      )
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  public func revokeMiniMax() async {
    guard let setupClient,
      let provider = setupSnapshot?.miniMax,
      !provider.credentialReference.isEmpty,
      provider.revision > 0
    else {
      setupState = .unavailable(reason: "credential_unavailable")
      return
    }
    setupState = .loading
    do {
      credentialStatus = try await setupClient.revokeMiniMax(
        reference: provider.credentialReference,
        revision: provider.revision
      )
      setupSnapshot = try await setupClient.setupSnapshot()
      reconcileWorkspace()
      setupState = .ready
    } catch {
      handleSetupError(error)
    }
  }

  private func handleSetupError(_ error: Error) {
    if let remote = error as? LocalIPCRemoteError {
      setupState =
        remote.recoverable
        ? .unavailable(reason: remote.code.rawValue)
        : .fatal(reason: remote.code.rawValue)
    } else {
      setupState = .unavailable(reason: closedClientReason(error))
    }
  }

  private func reconcileWorkspace() {
    if let reviews = setupSnapshot?.endpointReviews {
      var restored: [String: LocalProductEndpointReviewResult] = [:]
      var valid = true
      for review in reviews {
        let key = Self.endpointReviewCacheKey(
          candidateDigest: review.candidateDigest,
          providerAccountID: review.providerAccountID
        )
        if restored.updateValue(review, forKey: key) != nil {
          valid = false
          break
        }
      }
      approvedEndpointReviews = valid ? restored : [:]
    }
    let teams = (snapshot?.teams ?? []).map { team in
      LocalProductWorkspaceTask(
        id: "team:\(team.teamInstanceID)",
        title: workspaceVisibleName(
          team.displayName,
          internalID: team.teamInstanceID,
          fallback: "Saved team"
        ),
        subtitle: humanWorkspaceStatus(team.state),
        kind: .team
      )
    }
    let runs = (snapshot?.runs ?? []).map { run in
      LocalProductWorkspaceTask(
        id: "run:\(run.runID)",
        title: "Recent work",
        subtitle: humanWorkspaceStatus(
          run.terminalStatus.isEmpty ? run.phase : run.terminalStatus
        ),
        kind: .history
      )
    }
    let attention = (snapshot?.attention ?? []).map { item in
      LocalProductWorkspaceTask(
        id: "attention:\(item.attentionID)",
        title: workspaceAttentionTitle(item),
        subtitle: humanWorkspaceStatus(item.status),
        kind: .attention
      )
    }
    let saved = (setupSnapshot?.savedTeams ?? []).map { team in
      LocalProductWorkspaceTask(
        id: "saved:\(team.id)",
        title: team.name.isEmpty ? "Saved team" : team.name,
        subtitle: humanWorkspaceStatus(team.status),
        kind: .team
      )
    }
    workspace.mergeAuthoritativeTasks(attention + runs + teams + saved)
    let selectedProfileID = workspace.selectedContinuity.conversationProfileID
    if !availableConversationProfiles.contains(where: {
      $0.profileID == selectedProfileID
    }) {
      if let fallback = availableConversationProfiles.first {
        selectConversationProfile(fallback.profileID)
      } else {
        if let thread = chatThread, !thread.profileID.isEmpty {
          workspace.updateThreadAnchor(
            "thread-\(UUID().uuidString.lowercased())"
          )
          chatThread = nil
        }
        workspace.selectConversationProfile("")
      }
    }
  }

  private static func endpointReviewCacheKey(
    candidateDigest: String,
    providerAccountID: String
  ) -> String {
    candidateDigest + "\u{0}" + providerAccountID
  }

  private func reconcileMissions() {
    workbench.mergeMissions(
      snapshot?.missions.compactMap(\.listItem) ?? []
    )
  }

  private func humanWorkspaceStatus(_ value: String) -> String {
    value.replacingOccurrences(of: "_", with: " ").capitalized
  }

  private func workspaceAttentionTitle(
    _ item: LocalProductAttention
  ) -> String {
    let action = workspaceAttentionSafeText(item.actionRequired, limit: 48)
      .trimmingCharacters(in: .whitespacesAndNewlines)
    if !action.isEmpty {
      return humanWorkspaceStatus(action)
    }
    let kind = workspaceAttentionSafeText(item.kind, limit: 48)
      .trimmingCharacters(in: .whitespacesAndNewlines)
    if !kind.isEmpty {
      return humanWorkspaceStatus(kind)
    }
    return "Needs your attention"
  }

  private func workspaceAttentionSafeText(
    _ value: String,
    limit: Int
  ) -> String {
    let boundedLimit = max(1, min(limit, 4_096))
    var scalars: [UnicodeScalar] = []
    var index = value.unicodeScalars.startIndex
    while index < value.unicodeScalars.endIndex {
      let scalar = value.unicodeScalars[index]
      if scalar.value == 0x1B {
        index = skipWorkspaceAttentionEscape(
          in: value.unicodeScalars,
          from: index
        )
        continue
      }
      if scalar == "\n" || scalar == "\t" {
        if scalars.last != " " { scalars.append(" ") }
      } else if !CharacterSet.controlCharacters.contains(scalar),
        !Self.workspaceAttentionBidiControls.contains(scalar.value)
      {
        scalars.append(scalar)
      }
      index = value.unicodeScalars.index(after: index)
    }
    let cleaned = String(String.UnicodeScalarView(scalars))
    guard cleaned.count > boundedLimit else { return cleaned }
    return String(cleaned.prefix(boundedLimit)) + "…"
  }

  private func skipWorkspaceAttentionEscape(
    in scalars: String.UnicodeScalarView,
    from start: String.UnicodeScalarView.Index
  ) -> String.UnicodeScalarView.Index {
    var cursor = scalars.index(after: start)
    guard cursor < scalars.endIndex else { return cursor }
    if scalars[cursor] == "[" {
      cursor = scalars.index(after: cursor)
      while cursor < scalars.endIndex {
        let value = scalars[cursor].value
        cursor = scalars.index(after: cursor)
        if value >= 0x40 && value <= 0x7E { break }
      }
      return cursor
    }
    if scalars[cursor] == "]" {
      cursor = scalars.index(after: cursor)
      while cursor < scalars.endIndex {
        if scalars[cursor].value == 0x07 {
          return scalars.index(after: cursor)
        }
        if scalars[cursor].value == 0x1B {
          let next = scalars.index(after: cursor)
          if next < scalars.endIndex, scalars[next] == "\\" {
            return scalars.index(after: next)
          }
        }
        cursor = scalars.index(after: cursor)
      }
      return cursor
    }
    return scalars.index(after: cursor)
  }

  private static let workspaceAttentionBidiControls: Set<UInt32> = [
    0x061C, 0x200E, 0x200F,
    0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
    0x2066, 0x2067, 0x2068, 0x2069,
  ]

  private func workspaceVisibleName(
    _ candidate: String,
    internalID: String,
    fallback: String
  ) -> String {
    let bounded = String(candidate.prefix(96))
    let safe = bounded.unicodeScalars.filter { scalar in
      !CharacterSet.controlCharacters.contains(scalar)
        && !Self.workspaceBidiOverrides.contains(scalar.value)
    }
    let visible = String(String.UnicodeScalarView(safe))
      .trimmingCharacters(in: .whitespacesAndNewlines)
    return visible.isEmpty || visible == internalID ? fallback : visible
  }

  private static let workspaceBidiOverrides: Set<UInt32> = [
    0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
    0x2066, 0x2067, 0x2068, 0x2069,
  ]

  private func closedReason(_ value: String, fallback: String) -> String {
    let allowed = Set([
      "projection_refresh_failed", "partial_view", "stale_view",
      "observer_models_timeout", "observer_version_timeout",
      "side_tasks_limit_reached",
    ])
    return allowed.contains(value) ? value : fallback
  }

  /// Decides whether a partial snapshot is a user-visible degradation.
  ///
  /// The daemon marks `partial` for two very different reasons:
  /// - real degradation: runtime observation timouts, truncated side-task
  ///   decisions, or unknown server conditions (fail conservatively);
  /// - navigation pagination: runtimes/teams/missions lists that overflow a
  ///   page are a normal paginated view. The relevant page owns its "load
  ///   more" affordance; the global connection surface must stay online so a
  ///   healthy conversation is not presented as unavailable.
  ///
  /// Historical run/evidence lists also overflow at 64 entries, but that is a
  /// normal rich-data condition: the newest records stay fully useful and the
  /// UI shows an explicit "Showing the 64 most recent …" footnote instead of
  /// pretending the list is complete. Those must not downgrade the whole
  /// connection surface to "Some information is unavailable".
  private func isUserVisibleDegradation(_ snapshot: LocalProductSnapshot) -> Bool {
    switch snapshot.reason {
    case "observer_models_timeout", "observer_version_timeout",
      "side_tasks_limit_reached":
      return true
    default:
      break
    }
    if snapshot.runtimePage.hasMore
      || snapshot.teamPage.hasMore
      || snapshot.missionPage.hasMore
      || snapshot.runPage.hasMore
      || snapshot.evidencePage.hasMore
    {
      return false
    }
    // Unknown partial states stay conservative.
    return true
  }

  private func closedClientReason(_ error: Error) -> String {
    if let clientError = error as? LocalProductClientError {
      return clientError.rawValue
    }
    if let remoteError = error as? LocalIPCRemoteError {
      return remoteError.code.rawValue
    }
    if let wireError = error as? LocalProductWireError {
      switch wireError {
      case .unsupportedSchema: return "unsupported_schema"
      case .invalidJSON, .invalidValue, .unknownField: return "invalid_response"
      }
    }
    return "unavailable"
  }
}

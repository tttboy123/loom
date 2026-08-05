package production

import "os"

// installPlist writes the user-level launchd plist with 0600 permissions.
func installPlist(paths Paths) error {
	if paths.LaunchAgents == "" || paths.DaemonPath == "" {
		return ErrInvalidProductionInput
	}
	content := desiredPlist(paths.DaemonPath, paths.AppSupport)
	return atomicWrite(paths.PlistPath(), content, privateFileMode)
}

func removePlist(paths Paths) error {
	if paths.LaunchAgents == "" {
		return ErrInvalidProductionInput
	}
	return removeFileIfExists(paths.PlistPath())
}

func plistExists(paths Paths) bool {
	_, err := os.Stat(paths.PlistPath())
	return err == nil
}

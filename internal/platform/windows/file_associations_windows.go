//go:build windows

package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"my-go/internal/application"
)

const (
	auroraProgID       = "AuroraPlayer.Video.1"
	auroraCapabilities = `Software\Aurora Player\Capabilities`
)

// RegisterFileAssociations registers Aurora for its supported video types for
// the current Windows user. It never changes a file type's default application;
// the user makes that choice in Windows Settings or Open with.
func RegisterFileAssociations(executablePath string) error {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath == "" {
		return fmt.Errorf("executable path is empty")
	}
	absolutePath, err := filepath.Abs(executablePath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("inspect executable %q: %w", absolutePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("executable path %q is not a regular file", absolutePath)
	}

	executableName := filepath.Base(absolutePath)
	command := `"` + absolutePath + `" "%1"`
	icon := `"` + absolutePath + `",0`

	// Register a ProgID and its launch command. Every supported extension maps
	// to this application-specific identifier.
	entries := []struct {
		key   string
		name  string
		value string
	}{
		{ `Software\Classes\` + auroraProgID, "", "Aurora Player Video" },
		{ `Software\Classes\` + auroraProgID, "FriendlyTypeName", "Aurora Player Video" },
		{ `Software\Classes\` + auroraProgID + `\DefaultIcon`, "", icon },
		{ `Software\Classes\` + auroraProgID + `\shell\open\command`, "", command },

		{ `Software\Classes\Applications\` + executableName, "FriendlyAppName", "Aurora Player" },
		{ `Software\Classes\Applications\` + executableName + `\DefaultIcon`, "", icon },
		{ `Software\Classes\Applications\` + executableName + `\shell\open\command`, "", command },

		{ auroraCapabilities, "ApplicationName", "Aurora Player" },
		{ auroraCapabilities, "ApplicationDescription", "Play your video files with Aurora Player." },
		{ `Software\RegisteredApplications`, "Aurora Player", auroraCapabilities },
	}
	for _, entry := range entries {
		if err := setCurrentUserString(entry.key, entry.name, entry.value); err != nil {
			return fmt.Errorf("register Aurora file association at %s: %w", entry.key, err)
		}
	}

	for _, extension := range application.SupportedVideoExtensions() {
		// Applications\<exe>\SupportedTypes makes Aurora discoverable through
		// the Open with application registration.
		if err := setCurrentUserString(
			`Software\Classes\Applications\`+executableName+`\SupportedTypes`,
			extension,
			"",
		); err != nil {
			return fmt.Errorf("register Aurora supported type %s: %w", extension, err)
		}

		// Capabilities advertises Aurora in Windows Default Apps. OpenWithProgids
		// adds it as an alternative without taking ownership of the extension.
		if err := setCurrentUserString(
			auroraCapabilities+`\FileAssociations`,
			extension,
			auroraProgID,
		); err != nil {
			return fmt.Errorf("register Aurora capability for %s: %w", extension, err)
		}
		if err := setCurrentUserString(
			`Software\Classes\`+extension+`\OpenWithProgids`,
			auroraProgID,
			"",
		); err != nil {
			return fmt.Errorf("register Aurora Open with entry for %s: %w", extension, err)
		}
	}
	return nil
}

func setCurrentUserString(keyPath, name, value string) error {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		keyPath,
		registry.SET_VALUE|registry.CREATE_SUB_KEY,
	)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(name, value)
}

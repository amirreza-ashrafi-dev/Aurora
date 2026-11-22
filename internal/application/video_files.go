package application

import (
	"path/filepath"
	"strings"
)

var supportedVideoExtensions = []string{
	".mp4",
	".mkv",
	".avi",
	".mov",
	".wmv",
	".webm",
	".m4v",
	".mpg",
	".mpeg",
	".ts",
	".m2ts",
	".flv",
	".ogv",
	".3gp",
}

// SupportedVideoExtensions returns the file extensions Aurora advertises to
// Windows and accepts from Explorer. The returned slice is safe to modify.
func SupportedVideoExtensions() []string {
	return append([]string(nil), supportedVideoExtensions...)
}

// VideoFileDialogExtensions returns the same extensions without leading dots,
// as required by MyGo's native file dialog filter.
func VideoFileDialogExtensions() []string {
	extensions := make([]string, 0, len(supportedVideoExtensions))
	for _, extension := range supportedVideoExtensions {
		extensions = append(extensions, strings.TrimPrefix(extension, "."))
	}
	return extensions
}

// IsSupportedVideoFile reports whether path has an extension Aurora advertises
// as a video type. It does not check whether the file exists or is readable.
func IsSupportedVideoFile(path string) bool {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	for _, supported := range supportedVideoExtensions {
		if extension == supported {
			return true
		}
	}
	return false
}

// FilterVideoLaunchPaths keeps only supported video paths from the command line.
// Existence and regular-file validation remain the responsibility of FilterNewMedia.
func FilterVideoLaunchPaths(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" || !IsSupportedVideoFile(path) {
			continue
		}
		filtered = append(filtered, path)
	}
	return filtered
}

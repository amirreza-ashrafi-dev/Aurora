//go:build !windows

package windows

// RegisterFileAssociations is a no-op outside Windows. Aurora's native player
// window is Windows-only, but this keeps the bootstrap package buildable.
func RegisterFileAssociations(string) error { return nil }

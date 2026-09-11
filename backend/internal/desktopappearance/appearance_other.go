//go:build !darwin || !cgo

package desktopappearance

// Windows/Linux expose no native backdrop. The shared WebView policy handles
// CSS capabilities; no AppKit code is linked into these builds.
func Apply(dark bool) State { return stateFromFlags(0) }

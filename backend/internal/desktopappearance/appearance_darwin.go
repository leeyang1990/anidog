//go:build darwin && cgo

package desktopappearance

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework Cocoa -framework WebKit
int ADApplyAppearance(int dark);
*/
import "C"

func Apply(dark bool) State {
	value := C.int(0)
	if dark {
		value = 1
	}
	return stateFromFlags(int(C.ADApplyAppearance(value)))
}

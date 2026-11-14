//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void set2FAActivationPolicy(int regular) {
	[NSApplication sharedApplication];
	NSApplicationActivationPolicy policy = regular
		? NSApplicationActivationPolicyRegular
		: NSApplicationActivationPolicyAccessory;
	[NSApp setActivationPolicy:policy];
	if (regular) {
		[NSApp activateIgnoringOtherApps:YES];
	}
}
*/
import "C"

func setMacOSActivationPolicy(regular bool) {
	value := C.int(0)
	if regular {
		value = 1
	}
	C.set2FAActivationPolicy(value)
}

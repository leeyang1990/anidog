// Run on macOS without showing a window:
// clang -fobjc-arc -framework Cocoa -framework WebKit material_smoke.m -o /tmp/anidog-material-smoke
// /tmp/anidog-material-smoke
#import "../appearance_darwin.m"

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        NSWindow *window = [[NSWindow alloc]
            initWithContentRect:NSMakeRect(0, 0, 1440, 900)
            styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskResizable | NSWindowStyleMaskFullSizeContentView
            backing:NSBackingStoreBuffered defer:NO];
        NSView *original = window.contentView;
        WKWebView *webview = [[WKWebView alloc] initWithFrame:original.bounds];
        webview.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        [original addSubview:webview];
        ADWindowMaterial *material = [ADWindowMaterial new];
        material.window = window;
        material.content = original;
        int flags = [material update];
        NSCAssert((flags & 3) != 0, @"Native material must initialize");
        NSCAssert(ADFindWebView(window.contentView) == webview, @"Keep original WebView and bindings");
        NSView *first = window.contentView;
        [material update];
        NSCAssert(window.contentView == first, @"Repeated calls must not nest material layers");
        material.dark = YES;
        [material update];
        NSCAssert([window.appearance.name isEqualToString:NSAppearanceNameDarkAqua], @"Dark mode must reach AppKit");
        NSCAssert(window.contentView == first, @"Theme changes must preserve the layer and WebView");
        [window setContentSize:NSMakeSize(1024, 680)];
        [window.contentView layoutSubtreeIfNeeded];
        NSCAssert(NSEqualSizes(webview.bounds.size, window.contentLayoutRect.size), @"WebView must resize below the traffic lights");
        material.dark = NO;
        [material update];
        NSCAssert([window.appearance.name isEqualToString:NSAppearanceNameAqua], @"Light mode must reach AppKit");
        NSLog(@"Native appearance smoke test passed: %@, preserved WebView, light/dark, resize", NSStringFromClass(first.class));
    }
    return 0;
}

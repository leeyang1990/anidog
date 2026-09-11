//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// Public macOS 26 API, dynamically resolved so older SDKs and systems still work.
@protocol ADGlassView
@property(strong) NSView *contentView;
@property CGFloat cornerRadius;
@end

// Glass covers the whole window, but web content stays below the native traffic lights.
@interface ADContentHost : NSView
@property(strong) NSView *content;
@end

@implementation ADContentHost
- (void)layout {
    [super layout];
    NSRect available = self.window ? [self convertRect:self.window.contentLayoutRect fromView:nil] : self.bounds;
    self.content.frame = NSIntersectionRect(self.bounds, available);
}
- (void)resizeSubviewsWithOldSize:(NSSize)oldSize {
    [super resizeSubviewsWithOldSize:oldSize];
    [self layout];
}
@end

static WKWebView *ADFindWebView(NSView *root) {
    if ([root isKindOfClass:WKWebView.class]) return (WKWebView *)root;
    for (NSView *child in root.subviews) {
        WKWebView *found = ADFindWebView(child);
        if (found) return found;
    }
    return nil;
}

@interface ADWindowMaterial : NSObject
@property(weak) NSWindow *window;
@property(strong) NSView *content;
@property(strong) NSView *material;
@property(strong) id observer;
@property BOOL dark;
@property int mode;
- (int)update;
@end

@implementation ADWindowMaterial
- (int)update {
    NSWindow *window = self.window;
    BOOL reduce = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceTransparency;
    BOOL motion = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
    Class glassClass = NSClassFromString(@"NSGlassEffectView");
    int mode = reduce ? 3 : (glassClass ? 1 : 2);
    if (@available(macOS 10.14, *)) {
        window.appearance = [NSAppearance appearanceNamed:self.dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
    } else {
        window.appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
    }
    window.opaque = reduce;
    window.backgroundColor = reduce ? NSColor.windowBackgroundColor : NSColor.clearColor;
    if (self.mode != mode) {
        // Keep the original Wails content and webview intact (bindings, input, focus).
        NSView *content = self.content;
        NSRect frame = window.contentView.bounds;
        [content removeFromSuperview];
        ADContentHost *host = [[ADContentHost alloc] initWithFrame:frame];
        host.content = content;
        host.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        [host addSubview:content];
        NSView *material;
        if (mode == 1) {
            NSView<ADGlassView> *glass = [[glassClass alloc] initWithFrame:frame];
            glass.cornerRadius = 0;
            glass.contentView = host;
            material = glass;
        } else {
            NSVisualEffectView *effect = [[NSVisualEffectView alloc] initWithFrame:frame];
            effect.material = NSVisualEffectMaterialSidebar;
            effect.blendingMode = NSVisualEffectBlendingModeBehindWindow;
            effect.state = NSVisualEffectStateFollowsWindowActiveState;
            [effect addSubview:host];
            material = effect;
        }
        content.frame = frame;
        content.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        material.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        window.contentView = material;
        [host layout];
        self.material = material;
        self.mode = mode;
        NSLog(@"AniDog native material: %@ (mode=%d)", NSStringFromClass(material.class), mode);
    }
    return mode | (reduce ? 4 : 0) | (motion ? 8 : 0);
}
- (void)dealloc {
    if (self.observer) [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:self.observer];
}
@end

static char ADMaterialKey;

int ADApplyAppearance(int dark) {
    __block int result = 0;
    void (^apply)(void) = ^{
        for (NSWindow *window in NSApp.windows) {
            WKWebView *webview = ADFindWebView(window.contentView);
            if (!webview || ![window isKindOfClass:NSClassFromString(@"WailsWindow")]) continue;
            ADWindowMaterial *material = objc_getAssociatedObject(window, &ADMaterialKey);
            if (!material) {
                material = [ADWindowMaterial new];
                material.window = window;
                material.content = window.contentView;
                objc_setAssociatedObject(window, &ADMaterialKey, material, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
                __weak ADWindowMaterial *weakMaterial = material;
                material.observer = [NSWorkspace.sharedWorkspace.notificationCenter
                    addObserverForName:NSWorkspaceAccessibilityDisplayOptionsDidChangeNotification
                    object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
                        ADWindowMaterial *strongMaterial = weakMaterial;
                        if (!strongMaterial) return;
                        [strongMaterial update];
                        [ADFindWebView(strongMaterial.content)
                            evaluateJavaScript:@"window.dispatchEvent(new Event('anidog:native-appearance-change'))"
                            completionHandler:nil];
                    }];
            }
            material.dark = dark != 0;
            result = [material update];
            break;
        }
    };
    if (NSThread.isMainThread) apply();
    else dispatch_sync(dispatch_get_main_queue(), apply);
    return result;
}

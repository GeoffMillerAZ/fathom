//go:build darwin && !server && capabilityprobe && !workflowprobe && appkit_contained

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>
#include <string.h>

// This private preflight deliberately tests a nil/foreign origin. WebKit's
// public caller promises nonnull parameters; do not violate that compiler
// contract when directly invoking our stronger, nil-denying implementation.
@protocol AppKitMediaPreflight
- (void)webView:(WKWebView *)webView
    requestMediaCapturePermissionForOrigin:(WKSecurityOrigin * _Nullable)origin
    initiatedByFrame:(WKFrameInfo * _Nullable)frame type:(WKMediaCaptureType)type
    decisionHandler:(void (^)(WKPermissionDecision))decisionHandler API_AVAILABLE(macos(12.0));
@end

static NSNumber *capabilityDecisionAcknowledged(NSUInteger calls, NSInteger outcome) API_AVAILABLE(macos(12.0));
static NSNumber *capabilityDecisionAcknowledged(NSUInteger calls, NSInteger outcome) {
    // C logical expressions have type int. Boxing with @(a && b) emits JSON
    // 0/1, which the typed Go observer must reject rather than coerce to bool.
    return [NSNumber numberWithBool:(calls == 1 && outcome == (NSInteger)WKPermissionDecisionDeny)];
}

int capabilityObservationWireContract(void) {
    @autoreleasepool {
        if (@available(macOS 12.0, *)) {
            NSArray *values = @[capabilityDecisionAcknowledged(0, WKPermissionDecisionDeny),
                capabilityDecisionAcknowledged(1, WKPermissionDecisionDeny),
                capabilityDecisionAcknowledged(2, WKPermissionDecisionDeny),
                capabilityDecisionAcknowledged(1, WKPermissionDecisionPrompt),
                capabilityDecisionAcknowledged(1, WKPermissionDecisionGrant)];
            NSData *data = [NSJSONSerialization dataWithJSONObject:values options:0 error:nil];
            NSString *json = [[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] autorelease];
            return [json isEqualToString:@"[false,true,false,false,false]"] ? 1 : 0;
        }
        return 0;
    }
}

static void capabilityFindWebViews(NSView *view, WKWebView **found, NSUInteger *count) {
    if (view == nil || *count > 1) return;
    if ([view isKindOfClass:[WKWebView class]]) {
        *found = (WKWebView *)view;
        (*count)++;
    }
    for (NSView *child in view.subviews) capabilityFindWebViews(child, found, count);
}

char *capabilityObserve(void *nativeWindow, const char *origin, int preflight) {
    @autoreleasepool {
        if (![NSThread isMainThread] || nativeWindow == NULL || origin == NULL) return NULL;
        id object = (id)nativeWindow;
        if (![object isKindOfClass:[NSWindow class]]) return NULL;
        WKWebView *webView = nil;
        NSUInteger count = 0;
        capabilityFindWebViews([(NSWindow *)object contentView], &webView, &count);
        if (count != 1 || webView == nil) return NULL;
        id<WKUIDelegate> delegate = webView.UIDelegate;
        NSMutableDictionary *result = [@{@"available": @NO, @"origin_configured": @NO,
            @"denials": @[@0, @0, @0], @"origin_matched": @NO,
            @"preflight_denied": @[@NO, @NO, @NO], @"preflight_calls": @[@0, @0, @0],
            @"preflight_results": @[@-1, @-1, @-1],
            @"window_visible": @([(NSWindow *)object isVisible]),
            @"window_key": @([(NSWindow *)object isKeyWindow]),
            @"application_active": @([NSApp isActive])} mutableCopy];
        @try {
            if (@available(macOS 12.0, *)) {
                NSArray<NSString *> *keys = @[@"httpOnlyCameraDenials", @"httpOnlyMicrophoneDenials",
                    @"httpOnlyCombinedDenials", @"httpOnlyMediaOriginMatched", @"httpOnlyOrigin"];
                BOOL ready = delegate != nil && delegate == (id)[(NSWindow *)object delegate] &&
                    [delegate respondsToSelector:@selector(webView:requestMediaCapturePermissionForOrigin:initiatedByFrame:type:decisionHandler:)];
                for (NSString *key in keys) ready = ready && [delegate respondsToSelector:NSSelectorFromString(key)];
                if (ready) {
                    id observed = (id)delegate;
                    NSString *expected = [NSString stringWithUTF8String:origin];
                    BOOL configured = [[observed valueForKey:@"httpOnlyOrigin"] isEqual:expected];
                    result[@"available"] = @YES;
                    result[@"origin_configured"] = @(configured);
                    // Never exercise even the direct callback on an uncontained
                    // window. Require a fresh delegate before our three calls.
                    BOOL fresh = YES;
                    for (NSUInteger i = 0; i < 3; i++) fresh = fresh && [[observed valueForKey:keys[i]] unsignedIntegerValue] == 0;
                    if (preflight && configured && fresh) {
                        const WKMediaCaptureType types[] = {WKMediaCaptureTypeCamera,
                            WKMediaCaptureTypeMicrophone, WKMediaCaptureTypeCameraAndMicrophone};
                        NSMutableArray *decisions = [NSMutableArray array];
                        NSMutableArray *invocations = [NSMutableArray array];
                        NSMutableArray *outcomes = [NSMutableArray array];
                        for (NSUInteger i = 0; i < 3; i++) {
                            __block NSUInteger calls = 0;
                            __block NSInteger outcome = -1;
                            // No capture API, origin object fabrication or device
                            // access. Nil origin must still be denied by this profile.
                            [(id<AppKitMediaPreflight>)delegate webView:webView requestMediaCapturePermissionForOrigin:nil
                                initiatedByFrame:nil type:types[i] decisionHandler:^(WKPermissionDecision decision) {
                                    calls++;
                                    outcome = (NSInteger)decision;
                                }];
                            [decisions addObject:capabilityDecisionAcknowledged(calls, outcome)];
                            [invocations addObject:@(calls)];
                            [outcomes addObject:@(outcome)];
                        }
                        result[@"preflight_denied"] = decisions;
                        result[@"preflight_calls"] = invocations;
                        result[@"preflight_results"] = outcomes;
                    }
                    result[@"denials"] = @[[observed valueForKey:keys[0]], [observed valueForKey:keys[1]], [observed valueForKey:keys[2]]];
                    result[@"origin_matched"] = [observed valueForKey:@"httpOnlyMediaOriginMatched"];
                }
            }
            NSData *data = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
            NSString *json = [[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] autorelease];
            [result release];
            return json == nil ? NULL : strdup(json.UTF8String);
        } @catch (__unused NSException *exception) {
            [result release];
            return NULL;
        }
    }
}

//go:build darwin && !server && (workflowprobe || capabilityprobe) && appkit_contained

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdint.h>

extern void workflowEvalComplete(uint64_t callbackID, char *value, size_t valueLen,
                                 char *errorDomain, long errorCode, int status);

static const NSUInteger workflowEvalMaxBytes = 64 * 1024;

static void workflowFindWebViews(NSView *view, WKWebView **found, NSUInteger *count) {
    if (view == nil || *count > 1) {
        return;
    }
    if ([view isKindOfClass:[WKWebView class]]) {
        *found = (WKWebView *)view;
        (*count)++;
    }
    for (NSView *child in view.subviews) {
        workflowFindWebViews(child, found, count);
        if (*count > 1) {
            return;
        }
    }
}

int workflowEvaluateJavaScript(void *nativeWindow, const char *script, uint64_t callbackID) {
    if (nativeWindow == NULL) {
        return 1;
    }
    id object = (id)nativeWindow;
    if (![object isKindOfClass:[NSWindow class]]) {
        return 1;
    }
    WKWebView *webView = nil;
    NSUInteger count = 0;
    workflowFindWebViews([(NSWindow *)object contentView], &webView, &count);
    if (count == 0 || webView == nil) {
        return 2;
    }
    if (count != 1) {
        return 3;
    }
    NSString *source = script == NULL ? nil : [NSString stringWithUTF8String:script];
    if (source == nil) {
        return 4;
    }

    @try {
        [webView evaluateJavaScript:source completionHandler:^(id value, NSError *error) {
            @autoreleasepool {
                if (error != nil) {
                    const char *domain = "";
                    if ([error.domain lengthOfBytesUsingEncoding:NSUTF8StringEncoding] <= 128) {
                        domain = error.domain.UTF8String ?: "";
                    }
                    workflowEvalComplete(callbackID, NULL, 0, (char *)(domain ?: ""), error.code, 1);
                    return;
                }
                if (![value isKindOfClass:[NSString class]]) {
                    workflowEvalComplete(callbackID, NULL, 0, NULL, 0, 2);
                    return;
                }
                NSString *text = (NSString *)value;
                NSUInteger length = [text lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
                if (length > workflowEvalMaxBytes) {
                    workflowEvalComplete(callbackID, NULL, 0, NULL, 0, 3);
                    return;
                }
                const char *utf8 = text.UTF8String;
                if (utf8 == NULL) {
                    workflowEvalComplete(callbackID, NULL, 0, NULL, 0, 4);
                    return;
                }
                workflowEvalComplete(callbackID, (char *)utf8, length, NULL, 0, 0);
            }
        }];
    } @catch (__unused NSException *exception) {
        workflowEvalComplete(callbackID, NULL, 0, NULL, 0, 4);
    }
    return 0;
}

// imports.m -- #import / #include / @import coverage: a quoted import of a sibling
// fixture header, an angle-bracket framework import, a module import and a C include.
#import "Shape.h"
#import <Foundation/Foundation.h>
#include "imports_util.h"
@import UIKit;
@import Foundation.NSString;

@interface Importer : NSObject
- (void)run;
@end

@implementation Importer

static int local_count(int n) {
    return n * 2;
}

- (void)run {
    Shape *s = [Shape shapeWithName:@"a"];
    [s draw];
    local_count(1);
    util_ping();
    id any = s;
    [any draw];
}

@end

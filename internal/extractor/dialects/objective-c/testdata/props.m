#import "props.h"
@implementation Foo {
    int _x;
}
@synthesize delegate;
- (void)run:(int)a { [self.delegate run:a]; void (^b)(void) = ^{ [self run:1]; }; b(); }
- (id)objectAtIndexedSubscript:(NSUInteger)idx { return nil; }
- foo { return nil; }
- (void)variadic:(NSString *)fmt, ... { NSLog(@"%@", fmt); }
+ (void)load { [NSObject new]; }
@end

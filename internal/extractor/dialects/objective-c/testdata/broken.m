// broken.m -- a syntax error the parser must survive: the damaged method
// does not take the methods around it down.
#import "Shape.h"

@implementation Broken

- (void)before {
    [self after];
}

- (void)damaged:(int {
    [self before
}

- (void)after {
}

@end

#import "Shape.h"

static int counter = 0;

int helper(int a) {
    return a + 1;
}

@implementation Shape

- (instancetype)initWithFrame:(CGRect)frame style:(int)style {
    self = [super init];
    if (self) {
        counter = helper(style);
        [self reset];
    }
    return self;
}

+ (Shape *)shapeWithName:(NSString *)name {
    Shape *s = [[Shape alloc] initWithFrame:CGRectZero style:1];
    s.name = name;
    return s;
}

- (void)draw {
    [self drawInRect:CGRectZero];
    [[NSNotificationCenter defaultCenter] postNotificationName:@"x" object:nil];
}

- (void)drawInRect:(CGRect)rect {
}

- (void)reset {
}

@end

@implementation Shape (Debug)
- (NSString *)debugDescription {
    return [self description];
}
@end

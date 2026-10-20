#import <Foundation/Foundation.h>
#import "Util.h"

@protocol Drawable <NSObject>
- (void)drawInRect:(CGRect)rect;
@optional
+ (instancetype)shared;
@end

typedef struct {
    int width;
    int height;
} Size2D;

typedef NS_ENUM(NSInteger, ShapeKind) {
    ShapeKindCircle,
    ShapeKindSquare,
};

#define SHAPE_MAX 10

@interface Shape : NSObject <Drawable> {
    NSString *_label;
    int _sides;
}

@property (nonatomic, copy) NSString *name;
@property (nonatomic, assign) int sides;

- (instancetype)initWithFrame:(CGRect)frame style:(int)style;
+ (Shape *)shapeWithName:(NSString *)name;
- (void)draw;

@end

@interface Shape (Debug)
- (NSString *)debugDescription;
@end

@interface Shape ()
@property (nonatomic, strong) NSMutableArray *items;
- (void)reset;
@end

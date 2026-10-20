#import <Foundation/Foundation.h>
typedef void (^Completion)(NSError *error);
typedef NS_ENUM(NSInteger, Mode) { ModeA, ModeB };
typedef NS_OPTIONS(NSUInteger, Flags) { FlagA = 1 << 0, FlagB = 1 << 1 };
typedef enum { ColorRed, ColorBlue } Color;
enum Plain { PlainA, PlainB };
extern NSString *const kName;
FOUNDATION_EXPORT NSString * const kOther;
@class Other;
@interface Foo : NSObject
@property (nonatomic, copy) Completion completion;
@property (nonatomic, copy) void (^handler)(int);
@property (nonatomic, readonly) NSArray<NSString *> *names;
@property (class, nonatomic) Foo *shared;
@property id delegate;
- (void)run:(int)a;
- (id)objectAtIndexedSubscript:(NSUInteger)idx;
- (void)variadic:(NSString *)fmt, ...;
- (void)handle:(Completion)block NS_SWIFT_NAME(handle(block:));
+ (void)load;
@end

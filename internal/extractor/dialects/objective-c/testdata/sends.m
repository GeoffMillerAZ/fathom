// sends.m -- message sends: zero-argument, keyword, nested, super, class and
// instance receivers, an id-typed receiver, a multi-line method signature, a
// class extension and a category.
#import <Foundation/Foundation.h>

@protocol Greeter <NSObject>
@required
- (NSString *)greetingForName:(NSString *)name;
@optional
- (void)didGreet;
@end

@interface Sender : NSObject <Greeter>
@property (nonatomic, strong) NSMutableArray *queue;
@end

@interface Sender ()
- (void)flush;
@end

@interface Sender (Logging)
- (void)logMessage:(NSString *)message level:(NSInteger)level;
@end

@implementation Sender

- (instancetype)init {
    self = [super init];
    if (self) {
        _queue = [NSMutableArray array];
    }
    return self;
}

- (NSString *)greetingForName:(NSString *)name {
    return [NSString stringWithFormat:@"hello %@", name];
}

- (void)sendMessage:(NSString *)message
                 to:(NSString *)recipient
           priority:(NSInteger)priority
         completion:(void (^)(BOOL ok))completion {
    NSString *line = [self greetingForName:[recipient uppercaseString]];
    [self.queue addObject:@[line, message]];
    [self logMessage:line level:priority];
    [self flush];
    id target = self.queue.firstObject;
    [target performSelector:@selector(flush)];
    completion(YES);
}

- (void)flush {
    [self.queue removeAllObjects];
}

+ (instancetype)sharedSender {
    static Sender *shared;
    shared = [[self alloc] init];
    return shared;
}

@end

@implementation Sender (Logging)

- (void)logMessage:(NSString *)message level:(NSInteger)level {
    NSLog(@"%ld %@", (long)level, message);
}

@end

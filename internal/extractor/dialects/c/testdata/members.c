/* dialects/c/testdata/members.c — the definitions scip-clang names that the
 * dialect did not (O367): struct and union fields named after their owner
 * (an anonymous member struct's fields after the named one around it, an
 * anonymous typedef'd struct's after the typedef), enumerators, and global
 * variables, extern ones included; locals and parameters stay out. */

#define BUF_LEN 64
#define MIN(a, b) ((a) < (b) ? (a) : (b))

struct node {
  struct node *next;
  char name[BUF_LEN];
  void (*visit)(struct node *n);
  unsigned flags : 4, dirty : 1;
  union {
    int as_int;
    double as_real;
  } value;
};

typedef struct {
  long tv_sec;
  long tv_nsec;
} span_t;

typedef struct list_s {
  struct node *head;
} list_t;

enum level { LEVEL_LOW, LEVEL_HIGH };

typedef enum {
  MODE_READ,
  MODE_WRITE
} mode_kind;

int counter;
static const char *const level_names[] = {"low", "high"};
extern struct node root_node;
static void (*on_event)(int code);
char **argv_copy = 0;
int first_a, first_b;

#ifdef USE_EXTRA
static int extra_hits = 0;
#endif

int prototype_only(int x);

static int walk(struct node *n) {
  int depth = 0;
  static int calls;
  struct pair { int left; int right; } p;
#ifdef USE_EXTRA
  int extra = 1;
#endif
  for (; n; n = n->next) {
    depth++;
  }
  calls++;
  p.left = depth;
  return MIN(depth, BUF_LEN) + p.left;
}

/* dialects/cpp/testdata/broken.cpp — a syntax error the parser must survive
 * (definitions before and after the broken one still extract). */

int good() {
  return helper();
}

int broken(int a, @#$) {
  return a;
}

class Survivor {
public:
  int ok;
};

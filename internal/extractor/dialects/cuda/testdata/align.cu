// align.cu — __align__(N) between a type and its name, which the C++ grammar cannot read.
#include <cuda_runtime.h>

typedef struct __align__(16) {
  float x;
  float y;
} Pair;

struct __align__(8) Cell {
  int key;
  int val;
};

__global__ void gather(Pair *out, const Cell *cells, int n) {
  __shared__ int __align__(8) stage[128];
  int i = threadIdx.x;
  if (i < n) stage[i] = cells[i].val;
  __syncthreads();
  if (i < n) out[i].x = (float)stage[i];
}

template <typename T> struct Box { T v; };

template <typename Real>
void pick(int *flag) {
  if (typeid(Real) == typeid(double)) *flag = 1;
}

template struct Box<float>;

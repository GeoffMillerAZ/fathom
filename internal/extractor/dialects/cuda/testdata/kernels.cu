// kernels.cu — CUDA function-space qualifiers and the kinds they decorate.
#include <cuda_runtime.h>

#define BLOCK 256
#define CHECK(call) do { (call); } while (0)

namespace kern {

typedef float real;
using index_t = unsigned int;

enum class mode { fast, safe };

struct Params {
  real alpha;
  index_t n;
};

__device__ real square(real v) { return v * v; }

__host__ __device__ inline real clamp01(real v) {
  return v < 0 ? 0 : (v > 1 ? 1 : v);
}

__global__ void scale(real *out, const real *in, Params p) {
  index_t i = blockIdx.x * blockDim.x + threadIdx.x;
  if (i < p.n) {
    out[i] = square(in[i]) * p.alpha;
  }
}

template <typename T>
__global__ void __launch_bounds__(BLOCK, 2) fill(
    T *dst,
    T value,
    int count) {
  int i = threadIdx.x + blockIdx.x * blockDim.x;
  if (i < count) dst[i] = value;
}

class Reducer {
 public:
  __device__ Reducer() : acc_(0) {}
  __device__ void add(real v) { acc_ += clamp01(v); }
  __host__ real total() const { return acc_; }

 private:
  real acc_;
};

}  // namespace kern

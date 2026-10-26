// decorators.cu — storage-space and inline qualifiers on data and functions.
#include <cuda_runtime.h>

__constant__ float coeffs[16];
__device__ int counter;
__managed__ float shared_buf[64];

__device__ __noinline__ int bump(int v) { return v + 1; }

__global__ void tile(float *out, const float *__restrict__ in) {
  extern __shared__ float cache[];
  __shared__ int flag;
  cache[threadIdx.x] = in[threadIdx.x] * coeffs[0];
  __syncthreads();
  if (threadIdx.x == 0) flag = bump(counter);
  out[threadIdx.x] = cache[threadIdx.x] + flag;
}

void host_entry(float *out, const float *in) {
  tile<<<1, 32, 32 * sizeof(float)>>>(out, in);
}

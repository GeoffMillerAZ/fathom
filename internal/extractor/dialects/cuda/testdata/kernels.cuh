// kernels.cuh — a header a .cu file includes.
#pragma once
#include <cuda_runtime.h>

#define BLOCK 256

namespace kern {

struct Params;
template <typename T> __global__ void fill(T *dst, T value, int count);
__global__ void scale(float *out, const float *in, Params p);

template <typename T>
struct Box {
  T v;
  __host__ __device__ T get() const { return v; }
};

__device__ __forceinline__ int lane() { return threadIdx.x & 31; }

}  // namespace kern

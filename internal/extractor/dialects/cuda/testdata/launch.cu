// launch.cu — host code: triple-chevron launches, runtime calls, includes.
#include "kernels.cuh"
#include <cstdio>

static void run(float *d_out, const float *d_in, int n) {
  dim3 block(BLOCK);
  dim3 grid((n + BLOCK - 1) / BLOCK);
  kern::Params p{2.0f, (unsigned)n};
  kern::scale<<<grid, block>>>(d_out, d_in, p);
  kern::fill<float><<<grid, block, 0, nullptr>>>(d_out, 1.0f, n);
  cudaDeviceSynchronize();
}

int main(int argc, char **argv) {
  float *a = nullptr;
  float *b = nullptr;
  cudaMalloc(&a, 1024 * sizeof(float));
  cudaMalloc(&b, 1024 * sizeof(float));
  run(b, a, 1024);
  kern::Reducer r;
  r.add(0.5f);
  printf("%f\n", r.total());
  cudaFree(a);
  cudaFree(b);
  return 0;
}

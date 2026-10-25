// A function-like macro invocation is a call in CUDA, in any spelling (owner
// ruling C-R3): checkCudaErrors and CUDA_CHECK are calls, as C counts them.
#define checkCudaErrors(val) check((val), #val, __FILE__, __LINE__)
#define CUDA_CHECK(x) checkCudaErrors(x)

void check(int result, const char *what, const char *file, int line);

template <typename T>
__global__ void fill(T *out, int n) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  if (i < n) out[i] = T(0);
}

int run(float *d, int n) {
  checkCudaErrors(cudaMalloc(&d, n * sizeof(float)));
  fill<float><<<1, n>>>(d, n);
  CUDA_CHECK(cudaDeviceSynchronize());
  return static_cast<int>(n);
}

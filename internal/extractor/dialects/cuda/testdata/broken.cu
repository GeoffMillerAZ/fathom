// broken.cu — a syntax error the parser must survive.
__global__ void ok(int *p) { p[0] = 1; }

__global__ void bad(int *p {
  p[0] = ;
}

__device__ int after(int v) { return v; }

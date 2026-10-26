// Authored CL3.0 oracle capability probe. No external subject content.
inline float scale_value(float value) {
  return value * 2.0f;
}

__kernel void apply_scale(__global float4 *output, __global const float *input) {
  size_t index = get_global_id(0);
  float value = scale_value(input[index]);
  float4 vector = (float4)(value, value, value, value);
  output[index] = vector;
}

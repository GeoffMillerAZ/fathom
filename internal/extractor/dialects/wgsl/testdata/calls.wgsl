// Calls: plain, builtin, constructor, templated and nested.
fn square(x: f32) -> f32 {
  return x * x;
}

fn length_sq(v: vec3<f32>) -> f32 {
  return square(v.x) + square(v.y) + square(v.z);
}

fn shade(uv: vec2<f32>) -> vec4<f32> {
  let c = textureSample(albedo, samp, uv);
  let k = max(length_sq(c.xyz), 0.5);
  return vec4<f32>(c.xyz * k, 1.0);
}

fn fill() -> f32 {
  var a = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
  let m = mat2x2<f32>(vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0));
  return a[0] + m[0][0] + f32(square(2.0));
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
  return shade(uv);
}

const ORIGIN = vec3<f32>(0.0, 0.0, 0.0);
const COUNT = max(2, 3);

// Global declarations: structs, aliases, variables, constants, overrides.
struct Light {
  position: vec3<f32>,
  color: vec3<f32>,
}

@group(0) @binding(0) var<uniform> camera: mat4x4<f32>;
@group(0) @binding(1) var<storage, read> lights: array<Light>;
@group(1) @binding(0) var albedo: texture_2d<f32>;
@group(1) @binding(1) var samp: sampler;

var<private> counter: u32 = 0u;
const PI: f32 = 3.14159265;
const TAU = PI * 2.0;
override workgroup_size: u32 = 64;
override scale: f32;

alias Color = vec4<f32>;
alias Index = u32;

fn lerp(
  a: f32,
  b: f32,
  t: f32,
) -> f32 {
  return a + (b - a) * t;
}

fn nothing() {}

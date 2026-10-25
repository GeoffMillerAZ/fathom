// Entry points and attributes, with comments /* block /* nested */ still a comment */ in between.
struct VertexOut {
  @builtin(position) position: vec4<f32>,
  @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) index: u32) -> VertexOut {
  var out: VertexOut;
  let x = f32(index & 1u) * 2.0 - 1.0;
  let y = f32(index >> 1u) * 2.0 - 1.0;
  out.position = vec4<f32>(x, y, 0.0, 1.0);
  out.uv = vec2<f32>(x, y);
  return out;
}

@compute @workgroup_size(8, 8, 1)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
  workgroupBarrier();
  if (id.x < 4u) { return; }
}

struct Empty {
  pad: u32,
}

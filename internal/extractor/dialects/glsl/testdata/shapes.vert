#version 450
#define SCALE(x) ((x) * 2.0)
#define MAX_LIGHTS 4

struct VertexData {
    vec3 position;
    vec3 normal;
};

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inNormal;
layout(location = 0) out vec4 outColor;

layout(set = 0, binding = 0) uniform Scene {
    mat4 projection;
    vec4 tint;
} scene;

float diffuse(
    vec3 normal,
    vec3 lightDirection
) {
    return max(dot(normalize(normal), lightDirection), 0.0);
}

void main() {
    float light = diffuse(inNormal, vec3(0.0, 0.0, 1.0));
    gl_Position = scene.projection * vec4(inPosition, 1.0);
    outColor = scene.tint * light;
}

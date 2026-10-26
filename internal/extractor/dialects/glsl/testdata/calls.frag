#version 310 es
precision highp float;

layout(location = 0) in vec2 texCoord;
layout(location = 0) out vec4 fragColor;
uniform sampler2D albedo;

float luminance(vec3 color) {
    return dot(color, vec3(0.2126, 0.7152, 0.0722));
}

vec4 shade(vec2 uv) {
    float weights[3] = float[3](0.25, 0.5, 0.25);
    mat2 basis = mat2(1.0);
    vec3 color = texture(albedo, uv).rgb;
    float scale = weights[basis[0][1] == 0.0 ? 1 : 2];
    return vec4(color * luminance(color) * scale / float(weights.length()), 1.0);
}

void main() {
    fragColor = shade(texCoord);
}

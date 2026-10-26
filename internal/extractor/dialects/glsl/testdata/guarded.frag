#version 450
#if defined(USE_TINT)
uniform vec4 tint;
#ifdef USE_BLOCK
layout(std140) uniform Scene {
    vec4 color;
} scene;
#elif defined(USE_FALLBACK)
uniform vec4 fallback;
#else
uniform vec4 neutral;
#endif
#endif

void main() {
#define LOCAL_GAIN 2.0
    struct LocalData {
        float gain;
    };
#if defined(USE_LOCAL)
    vec4 localColor = tint;
#ifdef USE_NESTED_LOCAL
    float localStrength = 1.0;
#endif
#endif
    gl_FragColor = vec4(1.0);
}

#version 310 es
#define CUSTOM_PRECISION mediump
precision CUSTOM_PRECISION float;
precision highp int;
layout(location = 0) out vec4 fragColor;

struct Sample { float value; };

float sizedConstructor(Sample first, Sample second) {
    Sample data[2] = Sample[2](first, second);
    return data[0].value;
}

float unsizedConstructor(Sample first, Sample second) {
    Sample data[] = Sample[](first, second);
    return data[1].value;
}

float shade(float value) {
    return value * 0.5;
}

void main() {
    fragColor = vec4(shade(1.0));
}

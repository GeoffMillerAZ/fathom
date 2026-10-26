#version 450

float before(float x) {
    return x * 2.0;
}

float incomplete(float x) {
    return x + ;
}

struct Survivor {
    vec3 color;
};

float after(float y) {
    return before(y);
}

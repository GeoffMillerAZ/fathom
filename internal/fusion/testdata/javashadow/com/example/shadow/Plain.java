package com.example.shadow;

public class Plain {
    public boolean run(Object a, Object b) {
        return Objects.equal(a, b) && Ascii.lower(1) > 0;
    }
}

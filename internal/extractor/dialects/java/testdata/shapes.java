package com.example.shapes;

import com.example.util.Helper;

public class Widget {
    private int count;
    private String name, label;

    public Widget(String name) {
        this.name = name;
    }

    public int sum(
        int a,
        int b
    ) {
        int total = a + b;
        helper.doThing(total);
        Helper.staticThing();
        return total;
    }

    static class Inner {
        void innerMethod() {}
    }
}

interface Shape {
    int area();
}

enum Color {
    RED, GREEN, BLUE;
}

record Point(int x, int y) {
    int sum() { return x + y; }
}

@interface MyAnno {
    String value() default "";
}

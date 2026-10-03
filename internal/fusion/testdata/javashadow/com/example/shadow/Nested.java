package com.example.shadow;

public class Nested {
    static class Ascii {
        static int lower(int c) {
            return c;
        }
    }

    public int run(int c) {
        return Ascii.lower(c);
    }
}

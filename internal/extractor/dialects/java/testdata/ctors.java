package com.example.ctors;

import java.util.ArrayList;
import java.util.List;

public class Factory {
    private final List<String> seed = new ArrayList<>();

    public List<String> make() {
        List<String> out = new ArrayList<>();
        out.add("x");
        return out;
    }

    public Object nested(Outer outer) {
        Object a = new Outer.Inner(1);
        return outer.new Member();
    }

    public Object qualified() {
        return new java.util.HashMap<String, Integer>();
    }

    public Runnable anon() {
        return new Runnable() {
            @Override
            public void run() {
                helper(new Widget());
            }
        };
    }

    public int[] arrays() {
        return new int[3];
    }
}

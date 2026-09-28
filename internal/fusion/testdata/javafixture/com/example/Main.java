package com.example;

import com.example.Util;

public class Main {
    int local() {
        return 1;
    }

    int run() {
        local();
        return Util.helper(3);
    }
}

package com.example.broken;

public class Good {
    int good() {
        return helper();
    }
}

class Broken {
    int broken(int a, @#$) {
        return a;
    }
}

class Survivor {
    int ok;
}

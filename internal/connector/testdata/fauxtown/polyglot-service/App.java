package com.example;

public class App {

    public String greet(String name) {
        return "Hello " + name;
    }

    public interface Greeter {
        String greet(String name);
    }
}

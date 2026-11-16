package com.example;
import java.time.LocalDateTime;
import java.util.Arrays;
public class Main {
    public static void main(String[] args) {
        System.out.println("Hello from Java!");
        System.out.println("Time: " + LocalDateTime.now());
        System.out.println("Args: " + Arrays.toString(args));
    }
}

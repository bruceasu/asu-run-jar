package com.example;

import com.google.gson.Gson;
import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.Map;

public class MavenApp {
    public static void main(String[] args) {
        System.out.println("=== Maven Project Test ===");
        System.out.println("Time: " + LocalDateTime.now());
        
        // 使用 Gson 依赖测试
        Gson gson = new Gson();
        Map<String, Object> data = new HashMap<>();
        data.put("message", "Hello from Maven project!");
        data.put("timestamp", LocalDateTime.now().toString());
        data.put("hasGson", true);
        
        String json = gson.toJson(data);
        System.out.println("JSON output: " + json);
        
        System.out.println("\n=== Test Complete ===");
    }
}

package com.netcracker.it.controlplane.utils;

import io.fabric8.kubernetes.client.KubernetesClient;
import lombok.extern.slf4j.Slf4j;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;

import java.io.IOException;
import java.net.URL;
import java.util.concurrent.TimeUnit;

@Slf4j
public class RouteValidator {
    
    private final OkHttpClient httpClient;
    private final KubernetesClient kubernetesClient;
    
    public RouteValidator(KubernetesClient kubernetesClient) {
        this.kubernetesClient = kubernetesClient;
        this.httpClient = new OkHttpClient.Builder()
                .readTimeout(10, TimeUnit.SECONDS)
                .connectTimeout(10, TimeUnit.SECONDS)
                .build();
    }
    
    public boolean routeExists(URL gatewayUrl, String path) {
        String fullUrl = gatewayUrl.toString() + path;
        Request request = new Request.Builder()
                .url(fullUrl)
                .get()
                .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            log.debug("Route {} returned status {}", fullUrl, response.code());
            // Consider route exists if not connection refused
            return true;
        } catch (IOException e) {
            log.error("Failed to check route {}: {}", fullUrl, e.getMessage());
            return false;
        }
    }
    
    public int getRouteStatusCode(URL gatewayUrl, String path) throws IOException {
        String fullUrl = gatewayUrl.toString() + path;
        Request request = new Request.Builder()
                .url(fullUrl)
                .get()
                .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            return response.code();
        }
    }
    
    public boolean isRouteAccessible(URL gatewayUrl, String path) {
        try {
            int statusCode = getRouteStatusCode(gatewayUrl, path);
            return statusCode < 500; // Not a server error
        } catch (IOException e) {
            return false;
        }
    }
}
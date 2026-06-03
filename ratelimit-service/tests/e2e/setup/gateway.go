package setup

import (
    "fmt"
    "net/http"
    "time"
)

type GatewayClient struct {
    baseURL string
    client  *http.Client
}

func NewGatewayClient(port string) *GatewayClient {
    return &GatewayClient{
        baseURL: fmt.Sprintf("http://localhost:%s", port),
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (g *GatewayClient) SendRequest(endpoint, jwtToken string) (int, error) {
    url := g.baseURL + endpoint
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return 0, err
    }
    
    req.Header.Set("Authorization", "Bearer "+jwtToken)
    req.Header.Set("x-user-id", "e2e-test-user")
    
    resp, err := g.client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    
    return resp.StatusCode, nil
}

func (g *GatewayClient) Health() error {
    url := g.baseURL + "/health"
    resp, err := g.client.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("gateway health check failed: %d", resp.StatusCode)
    }
    return nil
}
// pkg/ratelimit/manager.go
package ratelimit

import (
    "context"
    "fmt"
    "regexp"
    "sync"
    "time"

    "k8s.io/klog/v2"
)

type Algorithm string

const (
    FixedWindow Algorithm = "fixed_window"
    SlidingLog  Algorithm = "sliding_log"
)

type Rule struct {
    Name      string
    Pattern   string
    Regex     *regexp.Regexp
    Limit     int
    Window    time.Duration
    Algorithm Algorithm
    Priority  int
}

type RateLimitManager struct {
    mu          sync.RWMutex
    rules       map[string]*Rule // key: rule name
    redisClient *RedisClient
}

func NewRateLimitManager(redisClient *RedisClient) *RateLimitManager {
    return &RateLimitManager{
        rules:       make(map[string]*Rule),
        redisClient: redisClient,
    }
}

func ValidatePattern(pattern string) error {
    _, err := regexp.Compile(pattern)
    return err
}

func (m *RateLimitManager) AddRule(rule *Rule) error {
    regex, err := regexp.Compile(rule.Pattern)
    if err != nil {
        return fmt.Errorf("invalid regex pattern '%s': %w", rule.Pattern, err)
    }
    rule.Regex = regex

    if rule.Algorithm == "" {
        rule.Algorithm = FixedWindow
    }
    
    if rule.Priority == 0 {
        rule.Priority = 50
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    m.rules[rule.Name] = rule
    
    klog.Infof("Rule added: %s (pattern: %s, limit: %d, window: %v, priority: %d)", 
        rule.Name, rule.Pattern, rule.Limit, rule.Window, rule.Priority)
    
    return nil
}

func (m *RateLimitManager) RemoveRule(name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.rules[name]; !exists {
        return fmt.Errorf("rule '%s' not found", name)
    }

    delete(m.rules, name)
    klog.Infof("Rule removed: %s", name)
    return nil
}

func (m *RateLimitManager) ClearRules() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.rules = make(map[string]*Rule)
    klog.Info("All rate limit rules cleared")
}

func (m *RateLimitManager) GetRule(name string) (*Rule, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    rule, exists := m.rules[name]
    return rule, exists
}

func (m *RateLimitManager) GetAllRules() []*Rule {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    rules := make([]*Rule, 0, len(m.rules))
    for _, rule := range m.rules {
        rules = append(rules, rule)
    }
    
    for i := 0; i < len(rules)-1; i++ {
        for j := i + 1; j < len(rules); j++ {
            if rules[i].Priority < rules[j].Priority {
                rules[i], rules[j] = rules[j], rules[i]
            }
        }
    }
    
    return rules
}

func (m *RateLimitManager) Check(ctx context.Context, key string) (allowed bool, current int, err error) {
    // Find matching rule
    rule := m.findMatchingRule(key)
    if rule == nil {
        // No rule matches - allow all
        return true, 0, nil
    }
    
    // Check rate limit in Redis
    allowed, current, err = m.redisClient.CheckRateLimit(ctx, key, rule)
    if err != nil {
        klog.Errorf("Failed to check rate limit: %v", err)
        return true, 0, err
    }
    
    return allowed, current, nil
}

type CheckResult struct {
    Allowed bool
    Current int
    Limit   int
    Window  time.Duration
    Rule    *Rule
}

func (m *RateLimitManager) CheckResult(ctx context.Context, key string) (*CheckResult, error) {
    // Find matching rule
    rule := m.findMatchingRule(key)
    if rule == nil {
        return &CheckResult{
            Allowed: true,
            Current: 0,
            Limit:   0,
        }, nil
    }
    
    // Check rate limit in Redis
    allowed, current, err := m.redisClient.CheckRateLimit(ctx, key, rule)
    if err != nil {
        return nil, err
    }
    
    return &CheckResult{
        Allowed: allowed,
        Current: current,
        Limit:   rule.Limit,
        Window:  rule.Window,
        Rule:    rule,
    }, nil
}

func (m *RateLimitManager) CheckWithComponents(ctx context.Context, components map[string]string, separator string) (map[string]interface{}, error) {
    key := m.buildKey(components, separator)
    
    rule := m.findMatchingRule(key)
    if rule == nil {
        return map[string]interface{}{
            "allowed": true,
            "limit":   0,
            "key":     key,
        }, nil
    }

    allowed, current, err := m.redisClient.CheckRateLimit(ctx, key, rule)
    if err != nil {
        klog.Errorf("Failed to check rate limit: %v", err)
        return nil, err
    }

    remaining := rule.Limit - current
    if remaining < 0 {
        remaining = 0
    }

    resetAt := time.Now().Add(rule.Window)

    return map[string]interface{}{
        "allowed":    allowed,
        "limit":      rule.Limit,
        "remaining":  remaining,
        "reset":      resetAt.Unix(),
        "key":        key,
        "rule_name":  rule.Name,
    }, nil
}

func (m *RateLimitManager) buildKey(components map[string]string, separator string) string {
    if components == nil {
        return ""
    }
    
    keys := make([]string, 0, len(components))
    for k := range components {
        keys = append(keys, k)
    }
    
    for i := 0; i < len(keys)-1; i++ {
        for j := i + 1; j < len(keys); j++ {
            if keys[i] > keys[j] {
                keys[i], keys[j] = keys[j], keys[i]
            }
        }
    }
    
    result := ""
    for i, k := range keys {
        if i > 0 {
            result += separator
        }
        result += k + "=" + components[k]
    }
    
    return result
}

func (m *RateLimitManager) findMatchingRule(key string) *Rule {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    var bestMatch *Rule
    highestPriority := -1
    
    for _, rule := range m.rules {
        if rule.Regex != nil && rule.Regex.MatchString(key) {
            if rule.Priority > highestPriority {
                highestPriority = rule.Priority
                bestMatch = rule
            }
        }
    }
    
    if bestMatch != nil {
        klog.V(4).Infof("Rule '%s' (priority=%d) matched key '%s' (pattern: %s)", 
            bestMatch.Name, bestMatch.Priority, key, bestMatch.Pattern)
    }
    
    return bestMatch
}
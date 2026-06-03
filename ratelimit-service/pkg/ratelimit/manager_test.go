package ratelimit

import (
    "testing"
    "time"
)

func TestValidatePattern(t *testing.T) {
    tests := []struct {
        name    string
        pattern string
        wantErr bool
    }{
        {"valid simple", "user_id=bad-user", false},
        {"valid with wildcard", ".*user_id=bad-user.*", false},
        {"valid with start", "^user_id=bad-user", false},
        {"valid with end", "user_id=bad-user$", false},
        {"invalid asterisk", "*user_id=bad-user*", true},
        {"invalid regex", "[invalid", true},
        {"empty", "", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePattern(tt.pattern)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePattern() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestRateLimitManager_AddRule(t *testing.T) {
    manager := NewRateLimitManager(nil)
    
    tests := []struct {
        name    string
        rule    *Rule
        wantErr bool
    }{
        {
            name: "valid rule",
            rule: &Rule{
                Name:      "test",
                Pattern:   ".*user_id=test.*",
                Limit:     10,
                Window:    60 * time.Second,
                Algorithm: FixedWindow,
            },
            wantErr: false,
        },
        {
            name: "invalid pattern",
            rule: &Rule{
                Name:      "invalid",
                Pattern:   "*invalid*",
                Limit:     10,
                Window:    60 * time.Second,
            },
            wantErr: true,
        },
        {
            name: "empty pattern",
            rule: &Rule{
                Name:      "empty",
                Pattern:   "",
                Limit:     10,
                Window:    60 * time.Second,
            },
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := manager.AddRule(tt.rule)
            if (err != nil) != tt.wantErr {
                t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
            }
            
            if !tt.wantErr {
                rule, exists := manager.GetRule(tt.rule.Name)
                if !exists {
                    t.Errorf("Rule not found after AddRule")
                }
                if rule.Regex == nil && tt.rule.Pattern != "" {
                    t.Errorf("Regex not compiled for pattern: %s", tt.rule.Pattern)
                }
            }
        })
    }
}

func TestRateLimitManager_RemoveRule(t *testing.T) {
    manager := NewRateLimitManager(nil)
    
    rule := &Rule{
        Name:    "test",
        Pattern: ".*test.*",
        Limit:   10,
        Window:  60 * time.Second,
    }
    
    manager.AddRule(rule)
    
    _, exists := manager.GetRule("test")
    if !exists {
        t.Fatal("Rule should exist after AddRule")
    }
    
    err := manager.RemoveRule("test")
    if err != nil {
        t.Errorf("RemoveRule() error = %v", err)
    }
    
    _, exists = manager.GetRule("test")
    if exists {
        t.Error("Rule should not exist after RemoveRule")
    }
    
    err = manager.RemoveRule("nonexistent")
    if err == nil {
        t.Error("RemoveRule() should return error for nonexistent rule")
    }
}

func TestRateLimitManager_FindMatchingRule(t *testing.T) {
    manager := NewRateLimitManager(nil)
    
    rules := []*Rule{
        {
            Name:    "bad-user",
            Pattern: ".*user_id=bad-user.*",
            Limit:   30,
            Window:  60 * time.Second,
        },
        {
            Name:    "admin",
            Pattern: ".*role=admin.*",
            Limit:   100,
            Window:  60 * time.Second,
        },
    }
    
    for _, rule := range rules {
        manager.AddRule(rule)
    }
    
    tests := []struct {
        name        string
        key         string
        expectedRule string
    }{
        {
            name:         "match bad user",
            key:          "path=/test&user_id=bad-user",
            expectedRule: "bad-user",
        },
        {
            name:         "match admin",
            key:          "user_id=admin&role=admin",
            expectedRule: "admin",
        },
        {
            name:         "no match",
            key:          "user_id=good-user",
            expectedRule: "",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            rule := manager.findMatchingRule(tt.key)
            if tt.expectedRule == "" && rule != nil {
                t.Errorf("Expected no match, got rule %s", rule.Name)
            }
            if tt.expectedRule != "" && (rule == nil || rule.Name != tt.expectedRule) {
                t.Errorf("Expected rule %s, got %v", tt.expectedRule, rule)
            }
        })
    }
}

func TestRateLimitManager_BuildKey(t *testing.T) {
    manager := NewRateLimitManager(nil)
    
    tests := []struct {
        name       string
        components map[string]string
        separator  string
        expected   string
    }{
        {
            name: "simple",
            components: map[string]string{
                "user_id": "test",
                "path":    "/api",
            },
            separator: "|",
            expected:  "path=/api|user_id=test",
        },
        {
            name: "single component",
            components: map[string]string{
                "user_id": "test",
            },
            separator: "&",
            expected:  "user_id=test",
        },
        {
            name:       "empty components",
            components: nil,
            separator:  "|",
            expected:   "",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := manager.buildKey(tt.components, tt.separator)
            if result != tt.expected {
                t.Errorf("buildKey() = %v, want %v", result, tt.expected)
            }
        })
    }
}
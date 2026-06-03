package ratelimit

import "time"

type Config struct {
	Domain      string                `yaml:"domain" json:"domain"`
	Separator   string                `yaml:"separator" json:"separator"`
	Descriptors []RateLimitDescriptor `yaml:"descriptors" json:"descriptors"`
}

type RateLimitDescriptor struct {
	Key         string                `yaml:"key" json:"key"`
	Value       string                `yaml:"value,omitempty" json:"value,omitempty"`
	ValueRegex  string                `yaml:"value_regex,omitempty" json:"value_regex,omitempty"`
	Algorithm   string                `yaml:"algorithm,omitempty" json:"algorithm,omitempty"`
	Priority    int                   `yaml:"priority,omitempty" json:"priority,omitempty"`
	RateLimit   *RateLimitValue       `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	Descriptors []RateLimitDescriptor `yaml:"descriptors,omitempty" json:"descriptors,omitempty"`
}

type RateLimitValue struct {
	Unit            string `yaml:"unit" json:"unit"`
	RequestsPerUnit int    `yaml:"requests_per_unit" json:"requests_per_unit"`
}

type LimitKey struct {
	FullKey    string
	Domain     string
	Components map[string]string
	Timestamp  int64
	LimitValue int
	Unit       string
	TTL        time.Duration
}

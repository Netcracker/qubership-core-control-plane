package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"ratelimit-service/pkg/metrics"
	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	sigsyaml "sigs.k8s.io/yaml"
)

type ConfigMapController struct {
	clientset        kubernetes.Interface
	redisClient      *ratelimit.RedisClient
	rateLimitManager *ratelimit.RateLimitManager
	configs          sync.Map
	// cmRuleNames tracks rule names registered per ConfigMap name, for cleanup on update/delete.
	cmRuleNames map[string][]string
	cmMu        sync.Mutex
	namespace   string
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

func NewConfigMapController(clientset kubernetes.Interface, redisClient *ratelimit.RedisClient, rateLimitManager *ratelimit.RateLimitManager) *ConfigMapController {
	namespace := utils.GetEnv("NAMESPACE", "core-1-core")

	return &ConfigMapController{
		clientset:        clientset,
		redisClient:      redisClient,
		rateLimitManager: rateLimitManager,
		namespace:        namespace,
		cmRuleNames:      make(map[string][]string),
		stopChan:         make(chan struct{}),
	}
}

func (c *ConfigMapController) Run(ctx context.Context) {
	klog.Info("Starting ConfigMap controller")

	if err := c.loadExistingConfigs(ctx); err != nil {
		klog.Errorf("Failed to load existing configs: %v", err)
	}

	c.wg.Add(1)
	go c.watchWithReconnect(ctx)

	<-c.stopChan
	c.wg.Wait()
	klog.Info("ConfigMap controller stopped")
}

func (c *ConfigMapController) loadExistingConfigs(ctx context.Context) error {
	cms, err := c.clientset.CoreV1().ConfigMaps(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "rate-limit-config=true",
	})
	if err != nil {
		return fmt.Errorf("failed to list configmaps: %w", err)
	}

	klog.Infof("Found %d existing ConfigMaps with rate-limit-config label", len(cms.Items))

	for _, cm := range cms.Items {
		if err := c.processConfigMap(ctx, &cm); err != nil {
			klog.Errorf("Failed to process ConfigMap %s: %v", cm.Name, err)
		}
	}

	return nil
}

func (c *ConfigMapController) watchWithReconnect(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		default:
			c.watchConfigMaps(ctx)
			klog.Warning("ConfigMap watcher disconnected, reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *ConfigMapController) watchConfigMaps(ctx context.Context) {
	watcher, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "rate-limit-config=true",
	})
	if err != nil {
		klog.Errorf("Failed to create watcher: %v", err)
		return
	}
	defer watcher.Stop()

	klog.Info("ConfigMap watcher started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			cm, ok := event.Object.(*v1.ConfigMap)
			if !ok {
				klog.Warning("Unexpected object type in watcher")
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				klog.Infof("ConfigMap %s: %s", cm.Name, event.Type)
				if err := c.processConfigMap(ctx, cm); err != nil {
					klog.Errorf("Failed to process ConfigMap %s: %v", cm.Name, err)
				}
			case watch.Deleted:
				klog.Infof("ConfigMap deleted: %s", cm.Name)
				c.deleteConfig(cm.Name)
			}
		}
	}
}

func (c *ConfigMapController) processConfigMap(_ context.Context, cm *v1.ConfigMap) error {
	configData, ok := cm.Data["config.yaml"]
	if !ok {
		configData, ok = cm.Data["app-config.yaml"]
		if !ok {
			return fmt.Errorf("no config.yaml or app-config.yaml found in ConfigMap %s", cm.Name)
		}
	}

	// Remove old rules from this ConfigMap before adding new ones.
	c.removeRulesForConfigMap(cm.Name)

	rules, err := c.parseConfig(configData, cm.Name)
	if err != nil {
		metrics.RecordConfigReload(false)
		return fmt.Errorf("failed to parse config: %w", err)
	}

	c.configs.Store(cm.Name, rules)

	ruleNames := make([]string, 0, len(rules))
	for _, rule := range rules {
		if err := c.rateLimitManager.AddRule(rule); err != nil {
			klog.Errorf("Failed to add rule %s: %v", rule.Name, err)
		} else {
			ruleNames = append(ruleNames, rule.Name)
		}
	}

	// Store rule names for later cleanup.
	c.cmMu.Lock()
	c.cmRuleNames[cm.Name] = ruleNames
	c.cmMu.Unlock()

	metrics.RecordConfigReload(true)
	klog.Infof("Processed ConfigMap %s with %d rules", cm.Name, len(rules))
	return nil
}

// removeRulesForConfigMap removes all rules previously registered from the given ConfigMap.
func (c *ConfigMapController) removeRulesForConfigMap(cmName string) {
	c.cmMu.Lock()
	names, exists := c.cmRuleNames[cmName]
	delete(c.cmRuleNames, cmName)
	c.cmMu.Unlock()

	if !exists {
		return
	}
	for _, name := range names {
		if err := c.rateLimitManager.RemoveRule(name); err != nil {
			klog.V(4).Infof("Could not remove rule %s (may already be absent): %v", name, err)
		}
	}
}

// ParseConfigYAML parses a ConfigMap "config.yaml" payload into a flat list
// of rate limit rules. configMapName is used to generate unique, stable rule
// names. Exported for use from tests in tests/integration/.
func ParseConfigYAML(configData string, configMapName string) ([]*ratelimit.Rule, error) {
	var cfg ratelimit.Config
	if err := sigsyaml.Unmarshal([]byte(configData), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML: %w", err)
	}

	separator := cfg.Separator
	if separator == "" {
		separator = "|"
	}

	var rules []*ratelimit.Rule
	if err := flattenDescriptors(cfg.Descriptors, nil, configMapName, separator, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// parseConfig is a thin wrapper retained for the receiver call sites
// inside this file. Tests should call ParseConfigYAML directly.
func (c *ConfigMapController) parseConfig(configData string, configMapName string) ([]*ratelimit.Rule, error) {
	return ParseConfigYAML(configData, configMapName)
}

// flattenDescriptors recursively walks the descriptor tree and emits a Rule for every
// descriptor that has a rate_limit block.
//
// pathComponents accumulates the (key, value/valueRegex) pairs from ancestor descriptors.
func flattenDescriptors(
	descriptors []ratelimit.RateLimitDescriptor,
	pathComponents []string,
	configMapName string,
	separator string,
	rules *[]*ratelimit.Rule,
) error {
	for i, desc := range descriptors {
		// Build the path component for this descriptor.
		component := buildComponent(desc, separator)

		currentPath := append(append([]string{}, pathComponents...), component)

		if desc.RateLimit != nil {
			rule, err := descriptorToRule(desc, currentPath, configMapName, separator, i)
			if err != nil {
				return err
			}
			*rules = append(*rules, rule)
		}

		// Recurse into children regardless of whether this level has a rate_limit.
		if len(desc.Descriptors) > 0 {
			if err := flattenDescriptors(desc.Descriptors, currentPath, configMapName, separator, rules); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildComponent returns the regex component string for a single descriptor level.
func buildComponent(desc ratelimit.RateLimitDescriptor, separator string) string {
	key := desc.Key
	if key == "" && desc.Value == "" && desc.ValueRegex == "" {
		// Catch-all descriptor.
		return ".*"
	}
	if desc.Value != "" {
		return regexp.QuoteMeta(key) + "=" + regexp.QuoteMeta(desc.Value)
	}
	if desc.ValueRegex != "" {
		return regexp.QuoteMeta(key) + "=" + desc.ValueRegex
	}
	// Key only — match any value (value may not contain the separator).
	escapedSep := regexp.QuoteMeta(separator)
	return regexp.QuoteMeta(key) + "=[^" + escapedSep + "]+"
}

// descriptorToRule converts a single descriptor with a rate_limit block into a Rule.
func descriptorToRule(
	desc ratelimit.RateLimitDescriptor,
	pathComponents []string,
	configMapName string,
	separator string,
	index int,
) (*ratelimit.Rule, error) {
	// Build pattern from path components.
	escapedSep := regexp.QuoteMeta(separator)
	pattern := ".*" + strings.Join(pathComponents, escapedSep) + ".*"

	// Compile the pattern — both validates it and provides the Regex field.
	compiledRegex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("generated pattern %q is invalid: %w", pattern, err)
	}

	// Parse window.
	var window time.Duration
	switch strings.ToLower(desc.RateLimit.Unit) {
	case "second":
		window = time.Second
	case "minute":
		window = time.Minute
	case "hour":
		window = time.Hour
	default:
		return nil, fmt.Errorf("unknown rate limit unit %q (must be one of: second, minute, hour)", desc.RateLimit.Unit)
	}

	// Parse algorithm.
	var algo ratelimit.Algorithm
	switch strings.ToLower(desc.Algorithm) {
	case "", "fixed_window":
		algo = ratelimit.FixedWindow
	case "sliding_log":
		algo = ratelimit.SlidingLog
	default:
		return nil, fmt.Errorf("unknown algorithm %q (must be one of: fixed_window, sliding_log)", desc.Algorithm)
	}

	// Build a unique rule name from the ConfigMap name and path.
	pathStr := strings.Join(pathComponents, "_")
	// Sanitize: remove characters that are unsafe in rule names.
	pathStr = strings.NewReplacer(
		"/", "_",
		".", "_",
		"*", "_",
		"^", "_",
		"$", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"|", "_",
		"+", "_",
		"?", "_",
		"\\", "_",
	).Replace(pathStr)
	ruleName := fmt.Sprintf("%s/%s#%d", configMapName, pathStr, index)

	return &ratelimit.Rule{
		Name:      ruleName,
		Pattern:   pattern,
		Regex:     compiledRegex,
		Limit:     desc.RateLimit.RequestsPerUnit,
		Window:    window,
		Algorithm: algo,
		Priority:  desc.Priority,
	}, nil
}

func (c *ConfigMapController) deleteConfig(name string) {
	c.configs.Delete(name)
	c.removeRulesForConfigMap(name)
	klog.Infof("ConfigMap %s removed and its rules deleted", name)
}

func (c *ConfigMapController) ReloadConfig(ctx context.Context) error {
	klog.Info("Manual config reload triggered")

	if err := c.loadExistingConfigs(ctx); err != nil {
		return fmt.Errorf("failed to reload configs: %w", err)
	}

	klog.Info("Config reload completed")
	return nil
}

func (c *ConfigMapController) getConfigCount() int {
	count := 0
	c.configs.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (c *ConfigMapController) Stop() {
	klog.Info("Stopping ConfigMap controller...")
	close(c.stopChan)
	c.wg.Wait()
	klog.Info("ConfigMap controller stopped")
}

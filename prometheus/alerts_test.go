package prometheus_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type alertRulesFile struct {
	Groups []alertGroup `yaml:"groups"`
}

type alertGroup struct {
	Name  string      `yaml:"name"`
	Rules []alertRule `yaml:"rules"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// TestAC001_AlertsYAMLIsValid verifies that alerts.yml is valid YAML
// with the expected Prometheus alerting rules structure.
func TestAC001_AlertsYAMLIsValid(t *testing.T) {
	data, err := os.ReadFile("alerts.yml")
	if err != nil {
		t.Fatalf("failed to read alerts.yml: %v", err)
	}

	var rules alertRulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		t.Fatalf("alerts.yml is not valid YAML: %v", err)
	}

	if len(rules.Groups) == 0 {
		t.Fatal("alerts.yml has no groups")
	}
	if rules.Groups[0].Name != "druggate" {
		t.Errorf("expected group name 'druggate', got %q", rules.Groups[0].Name)
	}
}

// TestAC002_HighErrorRateAlert verifies the DrugGateHighErrorRate alert exists
// with correct severity and expression referencing 5xx status codes.
func TestAC002_HighErrorRateAlert(t *testing.T) {
	rule := findRule(t, "DrugGateHighErrorRate")
	if rule.Labels["severity"] != "warning" {
		t.Errorf("expected severity=warning, got %q", rule.Labels["severity"])
	}
	if !strings.Contains(rule.Expr, "status_code") {
		t.Error("expression should reference status_code")
	}
	if !strings.Contains(rule.Expr, "5..") {
		t.Error("expression should match 5xx status codes")
	}
	if rule.For != "5m" {
		t.Errorf("expected for=5m, got %q", rule.For)
	}
}

// TestAC003_HighLatencyAlert verifies the DrugGateHighLatency alert.
func TestAC003_HighLatencyAlert(t *testing.T) {
	rule := findRule(t, "DrugGateHighLatency")
	if rule.Labels["severity"] != "warning" {
		t.Errorf("expected severity=warning, got %q", rule.Labels["severity"])
	}
	if !strings.Contains(rule.Expr, "histogram_quantile") {
		t.Error("expression should use histogram_quantile")
	}
	if !strings.Contains(rule.Expr, "0.95") {
		t.Error("expression should calculate p95")
	}
}

// TestAC004_RedisDownAlert verifies the DrugGateRedisDown alert.
func TestAC004_RedisDownAlert(t *testing.T) {
	rule := findRule(t, "DrugGateRedisDown")
	if rule.Labels["severity"] != "critical" {
		t.Errorf("expected severity=critical, got %q", rule.Labels["severity"])
	}
	if !strings.Contains(rule.Expr, "redis_up") {
		t.Error("expression should reference redis_up metric")
	}
	if rule.For != "1m" {
		t.Errorf("expected for=1m, got %q", rule.For)
	}
}

// TestAC005_HighRateLimitAlert verifies the DrugGateHighRateLimitRejections alert.
func TestAC005_HighRateLimitAlert(t *testing.T) {
	rule := findRule(t, "DrugGateHighRateLimitRejections")
	if rule.Labels["severity"] != "warning" {
		t.Errorf("expected severity=warning, got %q", rule.Labels["severity"])
	}
	if !strings.Contains(rule.Expr, "ratelimit_rejections_total") {
		t.Error("expression should reference ratelimit_rejections_total")
	}
}

// TestAC006_AllAlertsHaveAnnotations verifies all alerts have summary and description.
func TestAC006_AllAlertsHaveAnnotations(t *testing.T) {
	data, err := os.ReadFile("alerts.yml")
	if err != nil {
		t.Fatalf("failed to read alerts.yml: %v", err)
	}

	var rules alertRulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Annotations["summary"] == "" {
				t.Errorf("alert %q missing summary annotation", rule.Alert)
			}
			if rule.Annotations["description"] == "" {
				t.Errorf("alert %q missing description annotation", rule.Alert)
			}
		}
	}
}

// TestAC007_AllAlertsHaveSeverity verifies all alerts have a severity label.
func TestAC007_AllAlertsHaveSeverity(t *testing.T) {
	data, err := os.ReadFile("alerts.yml")
	if err != nil {
		t.Fatalf("failed to read alerts.yml: %v", err)
	}

	var rules alertRulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			sev := rule.Labels["severity"]
			if sev != "critical" && sev != "warning" {
				t.Errorf("alert %q has invalid severity %q (expected critical or warning)", rule.Alert, sev)
			}
		}
	}
}

// TestExpectedAlertCount verifies we have exactly 4 alert rules.
func TestExpectedAlertCount(t *testing.T) {
	data, err := os.ReadFile("alerts.yml")
	if err != nil {
		t.Fatalf("failed to read alerts.yml: %v", err)
	}

	var rules alertRulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}

	total := 0
	for _, group := range rules.Groups {
		total += len(group.Rules)
	}
	if total != 4 {
		t.Errorf("expected 4 alert rules, got %d", total)
	}
}

func findRule(t *testing.T, name string) alertRule {
	t.Helper()
	data, err := os.ReadFile("alerts.yml")
	if err != nil {
		t.Fatalf("failed to read alerts.yml: %v", err)
	}

	var rules alertRulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}

	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == name {
				return rule
			}
		}
	}
	t.Fatalf("alert %q not found in alerts.yml", name)
	return alertRule{}
}

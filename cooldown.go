package cooldown

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const hoursPerDay = 24

// Reason explains why a package version is allowed or blocked.
type Reason string

const (
	// ReasonDisabled means the selected cooldown duration is zero.
	ReasonDisabled Reason = "disabled"
	// ReasonElapsed means the package version has passed its cooldown period.
	ReasonElapsed Reason = "elapsed"
	// ReasonWaiting means the package version is still in its cooldown period.
	ReasonWaiting Reason = "waiting"
	// ReasonUnknownPublicationTime means the package version has no publication
	// time and is allowed by the default permissive policy.
	ReasonUnknownPublicationTime Reason = "unknown-publication-time"
)

// Decision describes the result of evaluating a package version.
type Decision struct {
	Allowed     bool
	Cooldown    time.Duration
	AvailableAt time.Time
	Reason      Reason
}

// Config holds cooldown settings for version filtering.
// Cooldown hides package versions published too recently, giving the community
// time to spot malicious releases before they're pulled into projects.
type Config struct {
	// Default is the global default cooldown duration (e.g., "3d", "48h").
	Default string `json:"default" yaml:"default"`

	// Ecosystems overrides the default for specific ecosystems.
	// Keys are ecosystem names (e.g., "npm", "pypi").
	Ecosystems map[string]string `json:"ecosystems" yaml:"ecosystems"`

	// Packages overrides the cooldown for specific packages.
	// Keys are PURLs (e.g., "pkg:npm/lodash", "pkg:npm/@babel/core").
	Packages map[string]string `json:"packages" yaml:"packages"`

	defaultDuration    time.Duration
	ecosystemDurations map[string]time.Duration
	packageDurations   map[string]time.Duration
	parsed             bool
}

// parse resolves all string durations into time.Duration values.
// Called lazily on first use.
func (c *Config) parse() {
	if c.parsed {
		return
	}
	c.parsed = true

	c.defaultDuration, _ = ParseDuration(c.Default)

	c.ecosystemDurations = make(map[string]time.Duration, len(c.Ecosystems))
	for k, v := range c.Ecosystems {
		d, _ := ParseDuration(v)
		c.ecosystemDurations[k] = d
	}

	c.packageDurations = make(map[string]time.Duration, len(c.Packages))
	for k, v := range c.Packages {
		d, _ := ParseDuration(v)
		c.packageDurations[k] = d
	}
}

// For returns the effective cooldown duration for a given ecosystem and package PURL.
// Resolution order: package override > ecosystem override > global default.
func (c *Config) For(ecosystem, packagePURL string) time.Duration {
	c.parse()

	if d, ok := c.packageDurations[packagePURL]; ok {
		return d
	}
	if d, ok := c.ecosystemDurations[ecosystem]; ok {
		return d
	}
	return c.defaultDuration
}

// IsAllowed returns true if a version with the given publish time has passed
// the cooldown period for this ecosystem/package.
func (c *Config) IsAllowed(ecosystem, packagePURL string, publishedAt time.Time) bool {
	return c.Evaluate(ecosystem, packagePURL, publishedAt, time.Now()).Allowed
}

// Evaluate returns the cooldown decision for a version at evaluatedAt.
// AvailableAt is zero when the publication time is unknown. Otherwise, it is
// the publication time plus the selected cooldown, including when the cooldown
// is disabled.
func (c *Config) Evaluate(ecosystem, packagePURL string, publishedAt, evaluatedAt time.Time) Decision {
	cooldown := c.For(ecosystem, packagePURL)
	decision := Decision{
		Cooldown: cooldown,
	}

	if !publishedAt.IsZero() {
		decision.AvailableAt = publishedAt.Add(cooldown)
	}

	if cooldown == 0 {
		decision.Allowed = true
		decision.Reason = ReasonDisabled
		return decision
	}
	if publishedAt.IsZero() {
		decision.Allowed = true
		decision.Reason = ReasonUnknownPublicationTime
		return decision
	}
	if !evaluatedAt.Before(decision.AvailableAt) {
		decision.Allowed = true
		decision.Reason = ReasonElapsed
		return decision
	}

	decision.Reason = ReasonWaiting
	return decision
}

// Enabled returns true if any cooldown is configured.
func (c *Config) Enabled() bool {
	c.parse()
	if c.defaultDuration > 0 {
		return true
	}
	for _, d := range c.ecosystemDurations {
		if d > 0 {
			return true
		}
	}
	for _, d := range c.packageDurations {
		if d > 0 {
			return true
		}
	}
	return false
}

// ParseDuration parses a duration string supporting days (e.g., "3d"),
// in addition to Go's standard time.ParseDuration formats ("48h", "30m").
// "0" means disabled (returns 0).
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	// Handle day suffix
	if numStr, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(days * float64(hoursPerDay*time.Hour)), nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

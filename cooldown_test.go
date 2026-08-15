package cooldown

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"3d", 3 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"14d", 14 * 24 * time.Hour, false},
		{"1.5d", 36 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1h30m", 90 * time.Minute, false},
		{"invalid", 0, true},
		{"d", 0, true},
		{"xd", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestConfigFor(t *testing.T) {
	c := &Config{
		Default: "3d",
		Ecosystems: map[string]string{
			"npm":   "7d",
			"cargo": "0",
		},
		Packages: map[string]string{
			"pkg:npm/lodash":      "0",
			"pkg:npm/@babel/core": "14d",
		},
	}

	tests := []struct {
		ecosystem   string
		packagePURL string
		want        time.Duration
	}{
		// Package override takes priority
		{"npm", "pkg:npm/lodash", 0},
		{"npm", "pkg:npm/@babel/core", 14 * 24 * time.Hour},
		// Ecosystem override
		{"npm", "pkg:npm/express", 7 * 24 * time.Hour},
		{"cargo", "pkg:cargo/serde", 0},
		// Global default
		{"pypi", "pkg:pypi/requests", 3 * 24 * time.Hour},
		{"pub", "pkg:pub/flutter", 3 * 24 * time.Hour},
	}

	for _, tt := range tests {
		got := c.For(tt.ecosystem, tt.packagePURL)
		if got != tt.want {
			t.Errorf("For(%q, %q) = %v, want %v", tt.ecosystem, tt.packagePURL, got, tt.want)
		}
	}
}

func TestConfigIsAllowed(t *testing.T) {
	c := &Config{
		Default: "3d",
		Packages: map[string]string{
			"pkg:npm/lodash": "0",
		},
	}

	now := time.Now()

	tests := []struct {
		name        string
		ecosystem   string
		packagePURL string
		publishedAt time.Time
		want        bool
	}{
		{"old enough", "npm", "pkg:npm/express", now.Add(-4 * 24 * time.Hour), true},
		{"too recent", "npm", "pkg:npm/express", now.Add(-1 * 24 * time.Hour), false},
		{"exactly at boundary", "npm", "pkg:npm/express", now.Add(-3 * 24 * time.Hour), true},
		{"exempt package", "npm", "pkg:npm/lodash", now.Add(-1 * time.Minute), true},
		{"zero time", "npm", "pkg:npm/express", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.IsAllowed(tt.ecosystem, tt.packagePURL, tt.publishedAt)
			if got != tt.want {
				t.Errorf("IsAllowed(%q, %q, %v) = %v, want %v",
					tt.ecosystem, tt.packagePURL, tt.publishedAt, got, tt.want)
			}
		})
	}
}

func TestConfigEvaluate(t *testing.T) {
	evaluatedAt := time.Date(2000, time.January, 15, 12, 0, 0, 0, time.UTC)
	c := &Config{
		Default: "3d",
		Packages: map[string]string{
			"pkg:npm/lodash": "0",
		},
	}

	tests := []struct {
		name        string
		packagePURL string
		publishedAt time.Time
		want        Decision
	}{
		{
			name:        "disabled",
			packagePURL: "pkg:npm/lodash",
			publishedAt: evaluatedAt.Add(-time.Minute),
			want: Decision{
				Allowed:     true,
				AvailableAt: evaluatedAt.Add(-time.Minute),
				Reason:      ReasonDisabled,
			},
		},
		{
			name:        "unknown publication time",
			packagePURL: "pkg:npm/express",
			want: Decision{
				Allowed:  true,
				Cooldown: 3 * 24 * time.Hour,
				Reason:   ReasonUnknownPublicationTime,
			},
		},
		{
			name:        "elapsed",
			packagePURL: "pkg:npm/express",
			publishedAt: evaluatedAt.Add(-4 * 24 * time.Hour),
			want: Decision{
				Allowed:     true,
				Cooldown:    3 * 24 * time.Hour,
				AvailableAt: evaluatedAt.Add(-24 * time.Hour),
				Reason:      ReasonElapsed,
			},
		},
		{
			name:        "waiting",
			packagePURL: "pkg:npm/express",
			publishedAt: evaluatedAt.Add(-24 * time.Hour),
			want: Decision{
				Cooldown:    3 * 24 * time.Hour,
				AvailableAt: evaluatedAt.Add(2 * 24 * time.Hour),
				Reason:      ReasonWaiting,
			},
		},
		{
			name:        "exactly at boundary",
			packagePURL: "pkg:npm/express",
			publishedAt: evaluatedAt.Add(-3 * 24 * time.Hour),
			want: Decision{
				Allowed:     true,
				Cooldown:    3 * 24 * time.Hour,
				AvailableAt: evaluatedAt,
				Reason:      ReasonElapsed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Evaluate("npm", tt.packagePURL, tt.publishedAt, evaluatedAt)
			if got != tt.want {
				t.Errorf("Evaluate() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConfigEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"empty config", Config{}, false},
		{"default only", Config{Default: "3d"}, true},
		{"ecosystem only", Config{Ecosystems: map[string]string{"npm": "7d"}}, true},
		{"package only", Config{Packages: map[string]string{"pkg:npm/x": "1d"}}, true},
		{"all zero", Config{Default: "0", Ecosystems: map[string]string{"npm": "0"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Enabled()
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

package server

import "testing"

func TestToSnake(t *testing.T) {
	cases := map[string]string{
		"NodeName":      "node_name",
		"IPv4":          "ipv4",
		"HostAddresses": "host_addresses",
		"HTTPServer":    "http_server",
		"LastSeenAt":    "last_seen_at",
		"AvgLatencyMs":  "avg_latency_ms",
		"name":          "name",
		"":              "",
		"X":             "x",
		"UTF8String":    "utf8_string",
	}
	for in, want := range cases {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

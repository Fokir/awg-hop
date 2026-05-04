package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr     string
	DataDir        string
	DatabasePath   string
	SessionSecret  string
	DevCORS        bool
	AllowedOrigins []string
	SecureCookies  bool
	WGQuickBin     string
	AWGShowBin     string
	IPTablesBin    string
	IPBin          string
	ExternalIface  string
	AutoApply      bool
	LogLevel       string
	LogFormat      string // "text" | "json"
}

func Load() Config {
	dataDir := getenv("AWGHOP_DATA", "./data")
	dbPath := getenv("AWGHOP_DATABASE", dataDir+"/awghop.db")
	return Config{
		ListenAddr:     getenv("AWGHOP_LISTEN", ":8080"),
		DataDir:        dataDir,
		DatabasePath:   dbPath,
		SessionSecret:  getenv("AWGHOP_SESSION_SECRET", ""),
		DevCORS:        truthy(os.Getenv("AWGHOP_DEV")),
		AllowedOrigins: splitCSV(os.Getenv("AWGHOP_ALLOWED_ORIGINS")),
		SecureCookies:  truthy(os.Getenv("AWGHOP_TLS")) || os.Getenv("AWGHOP_SECURE_COOKIES") == "1",
		WGQuickBin:     getenv("AWGHOP_WG_QUICK_BIN", "wg-quick"),
		AWGShowBin:     getenv("AWGHOP_AWG_BIN", "awg"),
		IPTablesBin:    getenv("AWGHOP_IPTABLES_BIN", "iptables"),
		IPBin:          getenv("AWGHOP_IP_BIN", "ip"),
		ExternalIface:  os.Getenv("AWGHOP_EXTERNAL_IFACE"),
		AutoApply:      !falsy(os.Getenv("AWGHOP_AUTO_APPLY")),
		LogLevel:       strings.ToLower(getenv("AWGHOP_LOG_LEVEL", "info")),
		LogFormat:      strings.ToLower(getenv("AWGHOP_LOG_FORMAT", "text")),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func truthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}

func falsy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "0" || s == "false" || s == "no" || s == "off"
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func ParseListenPort(addr string) int {
	addr = strings.TrimPrefix(addr, ":")
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		addr = addr[i+1:]
	}
	p, err := strconv.Atoi(addr)
	if err != nil {
		return 8080
	}
	return p
}

package adt

import (
	"bufio"
	"os"
	"strings"
)

// LoadCookiesFromFile loads cookies from a Netscape-format cookie file.
// The file format has 7 tab-separated fields per line:
// domain, flag, path, secure, expiration, name, value
//
// Also supports simple key=value format as fallback.
func LoadCookiesFromFile(cookieFile string) (map[string]string, error) {
	cookies := make(map[string]string)

	file, err := os.Open(cookieFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse Netscape format (7 fields separated by tabs)
		parts := strings.Split(line, "\t")
		if len(parts) >= 7 {
			// domain, flag, path, secure, expiration, name, value
			name := parts[5]
			value := parts[6]
			cookies[name] = value
		} else if strings.Contains(line, "=") {
			// Simple key=value format fallback
			kv := strings.SplitN(line, "=", 2)
			if len(kv) == 2 {
				cookies[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	return cookies, scanner.Err()
}

// ParseCookieString parses a cookie string in the format "key1=val1; key2=val2".
func ParseCookieString(cookieString string) map[string]string {
	cookies := make(map[string]string)
	for _, cookie := range strings.Split(cookieString, ";") {
		cookie = strings.TrimSpace(cookie)
		if strings.Contains(cookie, "=") {
			kv := strings.SplitN(cookie, "=", 2)
			if len(kv) == 2 {
				cookies[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}
	return cookies
}

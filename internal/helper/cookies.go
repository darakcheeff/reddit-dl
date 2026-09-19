package helper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
	"github.com/browserutils/kooky/browser/netscape"
)

var (
	// Default modern browser User-Agent
	DefaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

	// HTTPClient with active cookie jar
	Client *http.Client

	// Global cookie jar
	GlobalJar *cookiejar.Jar

	// Tracks if user explicitly passed cookies via flags
	CookiesExplicit bool

	// Total count of loaded cookies
	CookiesCount int
)

func init() {
	jar, _ := cookiejar.New(nil)
	GlobalJar = jar
	Client = &http.Client{
		Jar: GlobalJar,
	}
}

// addCookiesToJar stores cookies in the global cookie jar for appropriate domains
func addCookiesToJar(cookies []*http.Cookie) int {
	added := 0
	for _, c := range cookies {
		domain := strings.TrimPrefix(c.Domain, ".")
		if domain == "" {
			domain = "reddit.com"
		}
		u, err := url.Parse("https://" + domain)
		if err == nil {
			GlobalJar.SetCookies(u, []*http.Cookie{c})
			added++
		}
	}
	CookiesCount += added
	return added
}

// InitCookies is called when the user explicitly provides CLI flags
func InitCookies(browser, cookiesFile string) error {
	CookiesExplicit = true

	if cookiesFile != "" {
		count, err := LoadCookiesFromFile(cookiesFile)
		if err != nil {
			return err
		}
		InfoLog.Printf("Loaded %d cookies from file: %s\n", count, cookiesFile)
	}

	if browser != "" {
		count, err := LoadCookiesFromBrowser(browser)
		if err != nil {
			return err
		}
		InfoLog.Printf("Loaded %d cookies from browser '%s'\n", count, browser)
	}

	return nil
}

// LoadCookiesFromFile loads cookies from a Netscape cookies.txt file
func LoadCookiesFromFile(cookiesFile string) (int, error) {
	if _, err := os.Stat(cookiesFile); err != nil {
		return 0, fmt.Errorf("cookies file not found: %w", err)
	}

	cookies, _, err := netscape.ReadCookies(context.Background(), cookiesFile, kooky.DomainHasSuffix("reddit.com"))
	if err != nil {
		return 0, fmt.Errorf("reading cookies file: %w", err)
	}

	var httpCookies []*http.Cookie
	for _, c := range cookies {
		cookie := c.Cookie
		httpCookies = append(httpCookies, &cookie)
	}

	added := addCookiesToJar(httpCookies)
	return added, nil
}

// LoadCookiesFromBrowser extracts cookies matching reddit.com from a browser
func LoadCookiesFromBrowser(browser string) (int, error) {
	browserLower := strings.ToLower(strings.TrimSpace(browser))

	cookiesSeq := kooky.TraverseCookies(
		context.Background(),
		kooky.DomainHasSuffix("reddit.com"),
		kooky.FilterFunc(func(c *kooky.Cookie) bool {
			if c == nil || c.Browser == nil {
				return false
			}
			if browserLower == "all" || browserLower == "auto" {
				return true
			}
			return strings.Contains(strings.ToLower(c.Browser.Browser()), browserLower)
		}),
	).OnlyCookies()

	var httpCookies []*http.Cookie
	for c := range cookiesSeq {
		cookie := c.Cookie
		httpCookies = append(httpCookies, &cookie)
	}

	added := addCookiesToJar(httpCookies)
	return added, nil
}

// TryAutoLoadBraveCookies attempts to find cookies in Brave, falling back to other browsers if needed
func TryAutoLoadBraveCookies() (int, string, error) {
	// First: try Brave
	count, err := LoadCookiesFromBrowser("brave")
	if err == nil && count > 0 {
		return count, "Brave", nil
	}

	// Second: fallback to all other browsers (chrome, chromium, firefox, edge, opera, safari)
	allCount, allErr := LoadCookiesFromBrowser("all")
	if allErr == nil && allCount > 0 {
		return allCount, "detected browser", nil
	}

	return 0, "", fmt.Errorf("no Reddit cookies found in Brave or other browsers")
}

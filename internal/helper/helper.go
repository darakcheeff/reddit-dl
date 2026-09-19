package helper

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

func fetchJSON(targetURL string) ([]byte, int, error) {
	// Prepare request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := Client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("initial request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("unexpected status code for initial request: %s", resp.Status)
	}

	// Check if redirected to login
	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "/login/") {
		return nil, http.StatusForbidden, fmt.Errorf("redirected to login page: %s", finalURL)
	}

	// Determine final .json URL
	var jsonURL string
	if strings.HasSuffix(strings.TrimRight(targetURL, "/"), ".json") {
		jsonURL = targetURL
	} else {
		cleanPath := strings.TrimRight(resp.Request.URL.Path, "/") + ".json"
		jsonURL = fmt.Sprintf("https://www.reddit.com%s", cleanPath)
	}

	// Secondary request for the .json endpoint
	jsonReq, err := http.NewRequest("GET", jsonURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating json request: %w", err)
	}
	jsonReq.Header.Set("User-Agent", DefaultUserAgent)
	jsonReq.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	jsonReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	jsonReq.Header.Set("Sec-Fetch-Dest", "empty")
	jsonReq.Header.Set("Sec-Fetch-Mode", "cors")
	jsonReq.Header.Set("Sec-Fetch-Site", "same-origin")

	jsonResp, err := Client.Do(jsonReq)
	if err != nil {
		return nil, 0, fmt.Errorf("json request: %w", err)
	}
	defer jsonResp.Body.Close()

	if jsonResp.StatusCode != http.StatusOK {
		return nil, jsonResp.StatusCode, fmt.Errorf("unexpected status code for secondary request: %s", jsonResp.Status)
	}

	body, err := io.ReadAll(jsonResp.Body)
	if err != nil {
		return nil, jsonResp.StatusCode, fmt.Errorf("reading JSON body: %w", err)
	}

	return body, jsonResp.StatusCode, nil
}

// GetJSONBody fetches the JSON body from a secondary URL constructed from the response of the initial request.
// If access is restricted (403/401/login) and no cookies were passed, it automatically tries to load cookies from Brave.
func GetJSONBody(url string) ([]byte, error) {
	body, statusCode, err := fetchJSON(url)
	if err == nil {
		return body, nil
	}

	isAuthError := statusCode == http.StatusForbidden ||
		statusCode == http.StatusUnauthorized ||
		(err != nil && strings.Contains(err.Error(), "login"))

	if isAuthError && !CookiesExplicit {
		InfoLog.Printf("Access restricted (%s). Searching for Reddit cookies in Brave browser...\n", respStatusStr(statusCode, err))
		count, browserName, autoErr := TryAutoLoadBraveCookies()
		if autoErr == nil && count > 0 {
			InfoLog.Printf("Successfully loaded %d cookies from %s. Retrying request...\n", count, browserName)
			body, retryCode, retryErr := fetchJSON(url)
			if retryErr == nil {
				return body, nil
			}
			return nil, fmt.Errorf("request failed after applying %s cookies (status %d): %w", browserName, retryCode, retryErr)
		}

		return nil, fmt.Errorf("Reddit returned status %s and no cookies could be found in Brave browser.\n"+
			"Please log in to Reddit in Brave, or provide cookies explicitly via --cookies-from-browser <browser> or --cookies <file.txt>", respStatusStr(statusCode, err))
	}

	return nil, err
}

func respStatusStr(statusCode int, err error) string {
	if statusCode != 0 {
		return fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode))
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}

func GetHead(url string) (status_code int, content_type string) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, ""
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := Client.Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()

	status_code = resp.StatusCode
	content_type = resp.Header.Get("Content-Type")

	return status_code, content_type
}

func GetMediaUrl(url string) (media, audio string) {
	url = strings.ReplaceAll(url, "amp;", "")

	// checks if its a gif
	if strings.Contains(url, ".gif") {
		media = url
		audio = ""
		return media, audio
	}

	// normal video
	media = strings.Split(url, "?")[0]
	re, _ := regexp.Compile("_[0-9]+")
	audio = re.ReplaceAllString(media, "_audio")

	// this is for external videos/gif i.e. from gfycat
	// it wouldnt match the regex pattern
	if media == audio {
		return media, ""
	}

	return media, audio
}

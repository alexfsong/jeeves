package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (f *Fetcher) Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Jeeves/1.0 (Research Assistant)")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "text/plain") {
		// Read plain text directly
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	return extractText(resp.Body)
}

func extractText(r io.Reader) (string, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// Remove noise
	doc.Find("script, style, nav, footer, header, aside, .sidebar, .menu, .ad, .advertisement").Remove()

	var sb strings.Builder

	// Try article or main content first
	content := doc.Find("article, main, [role='main'], .content, .post-content, .entry-content")
	if content.Length() == 0 {
		content = doc.Find("body")
	}

	content.Find("h1, h2, h3, h4, h5, h6, p, li, td, th, blockquote, pre, code").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		tag := goquery.NodeName(s)
		switch {
		case strings.HasPrefix(tag, "h"):
			level := tag[1] - '0'
			sb.WriteString(strings.Repeat("#", int(level)))
			sb.WriteString(" ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		case tag == "li":
			sb.WriteString("- ")
			sb.WriteString(text)
			sb.WriteString("\n")
		case tag == "pre" || tag == "code":
			sb.WriteString("```\n")
			sb.WriteString(text)
			sb.WriteString("\n```\n\n")
		case tag == "blockquote":
			sb.WriteString("> ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		default:
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	result := sb.String()
	// Truncate very long content
	if len(result) > 50000 {
		result = result[:50000] + "\n\n[Content truncated]"
	}

	return result, nil
}

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"html/template"
	"time"
	"sync"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
)

// Add debug logger and statistics
type DebugStats struct {
	sync.Mutex
	Enabled        bool
	RequestCount   int
	BytesProcessed int64
	StartTime      time.Time
	RequestLog     []RequestLogEntry
}

type RequestLogEntry struct {
	Timestamp time.Time
	URL       string
	Duration  time.Duration
	Status    int
	Size      int64
}

var stats = DebugStats{
	StartTime:  time.Now(),
	RequestLog: make([]RequestLogEntry, 0),
}

func (ds *DebugStats) logRequest(entry RequestLogEntry) {
	if !ds.Enabled {
		return
	}
	ds.Lock()
	defer ds.Unlock()
	
	ds.RequestCount++
	ds.BytesProcessed += entry.Size
	ds.RequestLog = append(ds.RequestLog, entry)
	
	log.Printf("[DEBUG] Request: %s, Duration: %v, Status: %d, Size: %d bytes\n",
		entry.URL, entry.Duration, entry.Status, entry.Size)
}

func (ds *DebugStats) printStats() {
	if !ds.Enabled {
		return
	}
	ds.Lock()
	defer ds.Unlock()

	uptime := time.Since(ds.StartTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	log.Printf("\n=== Debug Statistics ===")
	log.Printf("Uptime: %v", uptime)
	log.Printf("Total Requests: %d", ds.RequestCount)
	log.Printf("Total Bytes Processed: %.2f MB", float64(ds.BytesProcessed)/1024/1024)
	log.Printf("Memory Usage: %.2f MB", float64(m.Alloc)/1024/1024)
	log.Printf("Goroutines: %d", runtime.NumGoroutine())
	log.Printf("====================\n")
}

type ResourceProcessor struct {
	baseURL string
	minifier *minify.M
	debugInfo struct {
		ResourcesProcessed int
		BytesProcessed    int64
	}
}

func NewResourceProcessor(baseURL string) *ResourceProcessor {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)

	return &ResourceProcessor{
		baseURL: baseURL,
		minifier: m,
	}
}

func (rp *ResourceProcessor) logDebug(format string, args ...interface{}) {
	if stats.Enabled {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func (rp *ResourceProcessor) processCSS(doc *goquery.Document) error {
	rp.logDebug("Processing CSS resources")
	// Process <link> tags for external CSS
	doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			absoluteURL := rp.makeAbsoluteURL(href)
			css, err := rp.fetchAndMinifyCSS(absoluteURL)
			if err == nil {
				// Replace external stylesheet with inline CSS
				s.ReplaceWithHtml(fmt.Sprintf("<style>%s</style>", css))
			}
		}
	})

	// Process inline CSS
	doc.Find("style").Each(func(i int, s *goquery.Selection) {
		css := s.Text()
		minified, err := rp.minifier.String("text/css", css)
		if err == nil {
			s.SetText(minified)
		}
	})

	rp.debugInfo.ResourcesProcessed++
	return nil
}

func (rp *ResourceProcessor) processJS(doc *goquery.Document) error {
	rp.logDebug("Processing JavaScript resources")
	// Process <script> tags for external JavaScript
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			// Keep YouTube player scripts and other essential external scripts
			if strings.Contains(src, "youtube.com") {
				s.SetAttr("src", rp.makeAbsoluteURL(src))
				return
			}

			absoluteURL := rp.makeAbsoluteURL(src)
			js, err := rp.fetchAndMinifyJS(absoluteURL)
			if err == nil {
				// Replace external script with inline JavaScript
				s.RemoveAttr("src")
				s.SetText(js)
			}
		}
	})

	// Process inline JavaScript
	doc.Find("script:not([src])").Each(func(i int, s *qqquery.Selection) {
		js := s.Text()
		// Skip if it contains YouTube player initialization
		if strings.Contains(js, "youtube.com") || 
		   strings.Contains(js, "YT.Player") {
			return
		}
		minified, err := rp.minifier.String("application/javascript", js)
		if err == nil {
			s.SetText(minified)
		}
	})

	rp.debugInfo.ResourcesProcessed++
	return nil
}

func (rp *ResourceProcessor) processImages(doc *goquery.Document) {
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			s.SetAttr("src", rp.makeAbsoluteURL(src))
		}
	})
}

func (rp *ResourceProcessor) processIframes(doc *goquery.Document) {
	doc.Find("iframe").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			// Handle YouTube embeds specially
			if strings.Contains(src, "youtube.com") || strings.Contains(src, "youtu.be") {
				s.SetAttr("src", src)
				s.SetAttr("allowfullscreen", "true")
				s.SetAttr("allow", "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture")
				return
			}
			s.SetAttr("src", rp.makeAbsoluteURL(src))
		}
	})
}

func (rp *ResourceProcessor) makeAbsoluteURL(resourceURL string) string {
	if strings.HasPrefix(resourceURL, "http://") || strings.HasPrefix(resourceURL, "https://") {
		return resourceURL
	}
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(rp.baseURL, "/"), strings.TrimPrefix(resourceURL, "/"))
}

func (rp *ResourceProcessor) fetchAndMinifyCSS(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	minified, err := rp.minifier.String("text/css", string(content))
	if err != nil {
		return string(content), nil // Return original if minification fails
	}
	return minified, nil
}

func (rp *ResourceProcessor) fetchAndMinifyJS(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	minified, err := rp.minifier.String("application/javascript", string(content))
	if err != nil {
		return string(content), nil // Return original if minification fails
	}
	return minified, nil
}

// Add these new helper functions
func extractVideoID(url string) string {
	if strings.Contains(url, "youtube.com/watch?v=") {
		return strings.Split(strings.Split(url, "watch?v=")[1], "&")[0]
	} else if strings.Contains(url, "youtu.be/") {
		return strings.Split(url, "youtu.be/")[1]
	}
	return ""
}

func (rp *ResourceProcessor) processMeta(doc *goquery.Document) {
	// Handle viewport meta
	doc.Find("meta[name='viewport']").Remove()
	doc.Find("head").PrependHtml(`<meta name="viewport" content="width=device-width, initial-scale=1.0">`)

	// Handle charset
	doc.Find("meta[charset]").Remove()
	doc.Find("head").PrependHtml(`<meta charset="UTF-8">`)

	// Handle CSP
	doc.Find("meta[http-equiv='Content-Security-Policy']").Remove()
}

func (rp *ResourceProcessor) processHead(doc *goquery.Document) error {
	// Process base tag
	doc.Find("base").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			s.SetAttr("href", rp.makeAbsoluteURL(href))
		}
	})

	// Process favicons
	doc.Find("link[rel='icon'], link[rel='shortcut icon']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			s.SetAttr("href", rp.makeAbsoluteURL(href))
		}
	})

	return nil
}

func main() {
	// Add debug flag
	debug := flag.Bool("debug", false, "Enable debug mode")
	debugShort := flag.Bool("d", false, "Enable debug mode")
	flag.Parse()

	// Enable debug if either flag is set
	stats.Enabled = *debug || *debugShort
	if stats.Enabled {
		log.Println("Debug mode enabled")
		// Print stats every minute
		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			for range ticker.C {
				stats.printStats()
			}
		}()
	}

	engine := html.New("./templates", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Add debug middleware
	app.Use(func(c *fiber.Ctx) error {
		if stats.Enabled {
			start := time.Now()
			err := c.Next()
			duration := time.Since(start)
			
			entry := RequestLogEntry{
				Timestamp: start,
				URL:       c.OriginalURL(),
				Duration:  duration,
				Status:    c.Response().StatusCode(),
				Size:      int64(len(c.Response().Body())),
			}
			stats.logRequest(entry)
			return err
		}
		return c.Next()
	})

	app.Static("/static", "./static")

	app.Get("/", func(c *fiber.Ctx) error {
		url := c.Query("url")
		if stats.Enabled {
			log.Printf("[DEBUG] Processing request for URL: %s", url)
		}

		if url == "" {
			return c.Render("index", fiber.Map{
				"Content": template.HTML(""),
			})
		}

		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}

		// Special handling for YouTube URLs
		if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
			videoID := extractVideoID(url)
			if videoID != "" {
				return c.Render("index", fiber.Map{
					"CurrentURL": url,
					"Content": template.HTML(`
						<div class="video-container">
							<iframe 
								src="https://www.youtube.com/embed/` + videoID + `" 
								frameborder="0" 
								allowfullscreen="true"
								allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture">
							</iframe>
						</div>
					`),
				})
			}
		}

		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		// Add headers to bypass some restrictions
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return c.Render("index", fiber.Map{
				"Error": fmt.Sprintf("Error creating request: %v", err),
			})
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err := client.Do(req)
		if err != nil {
			return c.Render("index", fiber.Map{
				"Error": fmt.Sprintf("Error fetching page: %v", err),
			})
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Render("index", fiber.Map{
				"Error": fmt.Sprintf("Error reading response: %v", err),
			})
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
		if err != nil {
			return c.Render("index", fiber.Map{
				"Error": fmt.Sprintf("Error parsing HTML: %v", err),
			})
		}

		processor := NewResourceProcessor(url)

		// Process everything
		processor.processMeta(doc)
		processor.processHead(doc)
		processor.processCSS(doc)
		processor.processJS(doc)
		processor.processImages(doc)
		processor.processIframes(doc)

		// Process forms
		doc.Find("form").Each(func(i int, s *goquery.Selection) {
			if action, exists := s.Attr("action"); exists {
				if !strings.HasPrefix(action, "http") {
					absoluteURL := processor.makeAbsoluteURL(action)
					s.SetAttr("action", fmt.Sprintf("/?url=%s", absoluteURL))
				} else {
					s.SetAttr("action", fmt.Sprintf("/?url=%s", action))
				}
			}
		})

		// Process links
		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				// Skip javascript: links and anchors
				if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "#") {
					return
				}
				if !strings.HasPrefix(href, "http") {
					absoluteURL := processor.makeAbsoluteURL(href)
					s.SetAttr("href", fmt.Sprintf("/?url=%s", absoluteURL))
				} else {
					s.SetAttr("href", fmt.Sprintf("/?url=%s", href))
				}
			}
		})

		// Get the full HTML content
		content, err := doc.Html()
		if err != nil {
			return c.Render("index", fiber.Map{
				"Error": fmt.Sprintf("Error extracting content: %v", err),
			})
		}

		if stats.Enabled {
			processor.logDebug("Resources processed: %d", processor.debugInfo.ResourcesProcessed)
			processor.logDebug("Bytes processed: %d", processor.debugInfo.BytesProcessed)
		}

		return c.Render("index", fiber.Map{
			"CurrentURL": url,
			"Content":    template.HTML(content),
		})
	})

	// Proxy route for YouTube API calls
	app.Get("/yt/*", func(c *fiber.Ctx) error {
		url := "https://www.youtube.com/" + c.Params("*")
		resp, err := http.Get(url)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		c.Set("Content-Type", resp.Header.Get("Content-Type"))
		return c.Send(body)
	})

	// Add debug endpoint
	if stats.Enabled {
		app.Get("/debug/stats", func(c *fiber.Ctx) error {
			stats.Lock()
			defer stats.Unlock()
			
			return c.JSON(fiber.Map{
				"uptime":          time.Since(stats.StartTime).String(),
				"requestCount":    stats.RequestCount,
				"bytesProcessed":  stats.BytesProcessed,
				"lastRequests":    stats.RequestLog[max(0, len(stats.RequestLog)-10):],
				"goroutines":      runtime.NumGoroutine(),
				"memoryUsageMB":   float64(runtime.MemStats{}.Alloc) / 1024 / 1024,
			})
		})
	}

	log.Printf("Server starting on :8080 (Debug mode: %v)", stats.Enabled)
	app.Listen(":8080")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
} 
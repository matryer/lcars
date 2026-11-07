// Package capture implements the LCARS browser verification and PNG capture loop.
package capture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/buildinfo"
)

// Options describes one viewport or one panel capture. Out must end in .png.
type Options struct {
	URL             string `json:"url"`
	Browser         string `json:"-"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Out             string `json:"-"`
	Panel           string `json:"panel,omitempty"`
	ExpectPanelHash string `json:"expectPanelHash,omitempty"`
	ExpectDataHash  string `json:"expectDataHash,omitempty"`
}

// Report includes the frontend's acknowledged hashes and verification evidence.
type Report map[string]any

type failedResource struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var panelPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}/[a-z0-9-]{1,64}$`)

func (o Options) validate() error {
	u, err := url.Parse(o.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("--url must be an absolute HTTP(S) URL")
	}
	if o.Width < 1 || o.Width > 16384 || o.Height < 1 || o.Height > 16384 {
		return errors.New("viewport width and height must be 1–16384 pixels")
	}
	if !strings.EqualFold(filepath.Ext(o.Out), ".png") {
		return errors.New("--out must name a .png file")
	}
	if o.Panel != "" && !panelPattern.MatchString(o.Panel) {
		return errors.New("--panel must be a tab/panel ID")
	}
	if (o.ExpectPanelHash != "" || o.ExpectDataHash != "") && o.Panel == "" {
		return errors.New("--expect-panel-hash and --expect-data-hash require --panel")
	}
	if o.ExpectPanelHash != "" && !hashPattern.MatchString(o.ExpectPanelHash) {
		return errors.New("--expect-panel-hash must be a lowercase SHA-256 hash")
	}
	if o.ExpectDataHash != "" && o.ExpectDataHash != "missing" && !hashPattern.MatchString(o.ExpectDataHash) {
		return errors.New("--expect-data-hash must be a lowercase SHA-256 hash or missing")
	}
	return nil
}

func browserPath(explicit string) (string, error) {
	if explicit != "" {
		p, err := exec.LookPath(explicit)
		if err != nil {
			return "", fmt.Errorf("browser %q is unavailable: %w", explicit, err)
		}
		return p, nil
	}
	for _, candidate := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", errors.New("Chrome/Chromium is not installed; install it or pass --browser /path/to/browser")
}

// Run captures the displayed revision and writes a PNG with a neighboring JSON
// report. Verification failures still produce the report and, when possible, a
// PNG of the failed display. Browser lifetime is bounded to 15 seconds.
func Run(parent context.Context, o Options) (report Report, retErr error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	report = Report{"captureBuild": buildinfo.Current(), "viewport": map[string]any{"width": o.Width, "height": o.Height, "deviceScaleFactor": 1}, "captureTime": time.Now().UTC().Format(time.RFC3339Nano), "url": o.URL, "errors": []string{}, "failedResources": []failedResource{}, "overflow": []string{}, "panels": []any{}, "pngWritten": false}
	var failures []string
	defer func() {
		if retErr != nil {
			failures = append(failures, retErr.Error())
		}
		// Frontend errors are kept alongside transport/browser errors.
		switch v := report["errors"].(type) {
		case []any:
			for _, e := range v {
				failures = append(failures, fmt.Sprint(e))
			}
		case []string:
			failures = append(failures, v...)
		}
		report["errors"] = failures
		if len(failures) > 0 && retErr == nil {
			retErr = fmt.Errorf("capture verification failed: %s", strings.Join(failures, "; "))
		}
		b, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			err = os.WriteFile(strings.TrimSuffix(o.Out, filepath.Ext(o.Out))+".json", append(b, '\n'), 0600)
		}
		if err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("write capture report: %w", err))
		}
	}()
	browserExe, err := browserPath(o.Browser)
	if err != nil {
		return report, err
	}
	timeout, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(timeout, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browserExe), chromedp.WindowSize(o.Width, o.Height))...)
	defer stopAllocator()
	var mu sync.Mutex
	requestURLs := map[network.RequestID]string{}
	resourceFailures := []failedResource{}
	var scriptErrors []string
	ctx, stopBrowser := chromedp.NewContext(allocator, chromedp.WithListener(func(event any) {
		mu.Lock()
		defer mu.Unlock()
		switch e := event.(type) {
		case *network.EventRequestWillBeSent:
			requestURLs[e.RequestID] = e.Request.URL
		case *network.EventLoadingFailed:
			if !e.Canceled {
				resourceFailures = append(resourceFailures, failedResource{requestURLs[e.RequestID], e.ErrorText})
			}
		case *network.EventResponseReceived:
			if e.Response.Status >= 400 {
				resourceFailures = append(resourceFailures, failedResource{e.Response.URL, fmt.Sprintf("HTTP %d", e.Response.Status)})
			}
		case *runtime.EventExceptionThrown:
			if e.ExceptionDetails != nil {
				msg := e.ExceptionDetails.Text
				if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
					msg = e.ExceptionDetails.Exception.Description
				}
				scriptErrors = append(scriptErrors, msg)
			}
		}
	}))
	defer stopBrowser()
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		report["failedResources"] = resourceFailures
		failures = append(failures, scriptErrors...)
		for _, r := range resourceFailures {
			failures = append(failures, fmt.Sprintf("resource %s: %s", r.URL, r.Error))
		}
	}()
	err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		v, e := cdp.Call(ctx, t, browser.GetVersion, cdp.Empty{})
		if e == nil {
			report["browserVersion"] = v.Product
		}
		return e
	}), chromedp.EmulateViewport(int64(o.Width), int64(o.Height), chromedp.EmulateScale(1)), chromedp.Navigate(o.URL))
	if err != nil {
		return report, fmt.Errorf("open capture page: %w", err)
	}
	// Reserve time for the failure screenshot and writing artifacts.
	readyCtx, stopReady := context.WithTimeout(ctx, 11*time.Second)
	defer stopReady()
	_, err = chromedp.Run(readyCtx, chromedp.Poll[bool](`typeof window.LCARSCapture === 'function'`, chromedp.WithPollingInterval(50*time.Millisecond)))
	if err == nil {
		payload, _ := json.Marshal(map[string]string{"panel": o.Panel, "expectPanelHash": o.ExpectPanelHash, "expectDataHash": o.ExpectDataHash})
		var frontend Report
		frontend, err = chromedp.Run(readyCtx, chromedp.Evaluate[Report](`window.LCARSCapture(`+string(payload)+`)`, chromedp.EvalAwaitPromise))
		if frontend != nil {
			for k, v := range frontend {
				report[k] = v
			}
		}
	}
	if err != nil {
		failures = append(failures, fmt.Sprintf("display verification: %v", err))
		// Preserve any partial display evidence the shell published before a
		// readiness failure. This uses the remaining overall deadline.
		partial, evidenceErr := chromedp.Run(ctx, chromedp.Evaluate[Report](`window.__lcarsCaptureReport || {errors: window.__lcarsCaptureErrors || []}`))
		if evidenceErr == nil {
			for k, v := range partial {
				report[k] = v
			}
		}
	}
	var png []byte
	var screenshotErr error
	if o.Panel != "" {
		selector := `[data-panel-id="` + o.Panel + `"]`
		png, screenshotErr = chromedp.Run(ctx, chromedp.Screenshot(chromedp.CSS(selector)))
	} else {
		png, screenshotErr = chromedp.Run(ctx, chromedp.CaptureScreenshot())
	}
	if screenshotErr != nil {
		return report, fmt.Errorf("take screenshot: %w", screenshotErr)
	}
	if err = os.WriteFile(o.Out, png, 0600); err != nil {
		return report, fmt.Errorf("write screenshot: %w", err)
	}
	report["pngWritten"] = true
	return report, nil
}

// lcars serves an agent-fed canvas, validates file publications, and verifies captures.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/matryer/lcars/internal/buildinfo"
	"github.com/matryer/lcars/internal/capture"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/scripts"
	"github.com/matryer/lcars/internal/server"
	authoring "github.com/matryer/lcars/skills/lcars-authoring"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "lcars:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) (err error) {
	defer func() {
		if errors.Is(err, flag.ErrHelp) {
			err = nil
		}
	}()
	if len(args) == 0 {
		usage(errOut)
		return errors.New("choose serve, validate, screenshot, snapshot, skill, or version")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "help", "-h", "--help":
		usage(out)
		return nil
	case "version", "--version":
		flags := flag.NewFlagSet("version", flag.ContinueOnError)
		flags.SetOutput(errOut)
		asJSON := flags.Bool("json", false, "print machine-readable build and contract versions")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected version arguments")
		}
		info := buildinfo.Current()
		if *asJSON {
			return json.NewEncoder(out).Encode(info)
		}
		fmt.Fprintf(out, "LCARS %s (authoring contract %s; bridge %d)\n%s; %s/%s\n", info.Version, info.ContractVersion, info.ProtocolVersion, info.GoVersion, info.OS, info.Arch)
		if info.Commit != "" {
			fmt.Fprintf(out, "Commit %s; modified %t\n", info.Commit, info.Modified)
		}
		if info.BuiltAt != "" {
			fmt.Fprintln(out, "Built", info.BuiltAt)
		}
		return nil
	case "skill":
		flags := flag.NewFlagSet("skill", flag.ContinueOnError)
		flags.SetOutput(errOut)
		guide := flags.Bool("guide", false, "print the complete authoring guide")
		starter := flags.Bool("starter", false, "print the starter HTML document")
		destination := flags.String("out", "", "export the complete skill to a new directory (no automatic installation)")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected skill arguments")
		}
		selected := 0
		if *guide {
			selected++
		}
		if *starter {
			selected++
		}
		if *destination != "" {
			selected++
		}
		if selected > 1 {
			return errors.New("choose only one of --guide, --starter, or --out")
		}
		if *destination != "" {
			if err := authoring.Export(*destination); err != nil {
				return err
			}
			_, err := fmt.Fprintln(out, "Exported lcars-authoring to", *destination)
			return err
		}
		name := "SKILL.md"
		if *guide {
			name = "references/guide.md"
		}
		if *starter {
			name = "assets/starter.html"
		}
		b, err := authoring.Files.ReadFile(name)
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		flags.SetOutput(errOut)
		data := flags.String("data", "./content", "existing content directory")
		listen := flags.String("listen", "127.0.0.1:8080", "HTTP listen address (use a private authenticated proxy for remote access)")
		allowedHosts := flags.String("allowed-host", "", "comma-separated hostnames your proxy forwards in the Host header (localhost and IP addresses are always allowed)")
		runScripts := flags.Bool("run-scripts", false, "execute trusted adjacent scheduled scripts")
		scriptTimeout := flags.Duration("script-timeout", 30*time.Second, "maximum script run duration")
		scriptConcurrency := flags.Int("script-concurrency", 4, "maximum concurrent scripts")
		scriptEnv := flags.String("script-env", "", "comma-separated additional environment variable names to pass to scripts")
		interpreters := map[string]*string{}
		for _, ext := range []string{"py", "js", "sh", "ps1"} {
			interpreters[ext] = flags.String("script-"+ext, "", "interpreter executable for ."+ext+" scripts")
		}
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected serve arguments")
		}
		store, err := content.New(*data)
		if err != nil {
			return err
		}
		listener, err := net.Listen("tcp", *listen)
		if err != nil {
			return err
		}
		defer listener.Close()
		runCtx, cancelRun := context.WithCancel(ctx)
		defer cancelRun()
		if *runScripts {
			configured := map[string]string{}
			for ext, path := range interpreters {
				configured[ext] = *path
			}
			runner, err := scripts.New(*data, store, scripts.Options{Timeout: *scriptTimeout, Concurrency: *scriptConcurrency, Interpreters: configured, PassEnv: list(*scriptEnv), Log: errOut})
			if err != nil {
				return err
			}
			done := make(chan struct{})
			go func() { defer close(done); runner.Run(runCtx) }()
			defer func() { cancelRun(); <-done; runner.Close() }()
		}
		srv := &http.Server{BaseContext: func(net.Listener) context.Context { return runCtx }, Addr: *listen, Handler: server.New(store, list(*allowedHosts)...), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
		go store.Run(runCtx)
		serverErrors := make(chan error, 1)
		go func() { serverErrors <- srv.Serve(listener) }()
		fmt.Fprintf(out, "LCARS listening on %s (data: %s)\n", listener.Addr(), *data)
		select {
		case err := <-serverErrors:
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		case <-ctx.Done():
			cancelRun()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		}
	case "validate":
		flags := flag.NewFlagSet("validate", flag.ContinueOnError)
		flags.SetOutput(errOut)
		data := flags.String("data", "./content", "existing content directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected validate arguments")
		}
		return content.Validate(*data, out)
	case "snapshot":
		flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
		flags.SetOutput(errOut)
		data := flags.String("data", "./content", "existing content directory")
		output := flags.String("out", "./captures", "directory for desktop, tablet, mobile, and narrow PNGs and reports")
		browserExe := flags.String("browser", "", "Chrome/Chromium executable (auto-detected by default)")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected snapshot arguments")
		}
		if err := content.Validate(*data, out); err != nil {
			return err
		}
		store, err := content.New(*data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(*output, 0700); err != nil {
			return err
		}
		// This listener exists only for this command and is always closed.
		srv := httptest.NewServer(server.New(store))
		defer srv.Close()
		tabs := []string{}
		for _, tab := range store.State().Tabs {
			tabs = append(tabs, tab.ID)
		}
		if len(tabs) == 0 {
			tabs = append(tabs, "")
		}
		viewports := []struct {
			name          string
			width, height int
		}{
			{"desktop", 1440, 900}, {"tablet", 1024, 768}, {"mobile", 390, 844}, {"narrow", 320, 844},
		}
		var failures []error
		for _, tab := range tabs {
			for _, viewport := range viewports {
				if err := ctx.Err(); err != nil {
					return errors.Join(append(failures, err)...)
				}
				pageURL, stem := srv.URL+"/", "welcome"
				if tab != "" {
					pageURL, stem = srv.URL+"/s/"+tab, tab
				}
				filename := filepath.Join(*output, stem+"-"+viewport.name+".png")
				_, err := capture.Run(ctx, capture.Options{URL: pageURL, Browser: *browserExe, Width: viewport.width, Height: viewport.height, Out: filename})
				if err != nil {
					failures = append(failures, fmt.Errorf("%s %s: %w", stem, viewport.name, err))
					fmt.Fprintf(errOut, "Failed %s: %v\n", filename, err)
				} else {
					fmt.Fprintln(out, "Verified", filename)
				}
			}
		}
		return errors.Join(failures...)
	case "screenshot":
		flags := flag.NewFlagSet("screenshot", flag.ContinueOnError)
		flags.SetOutput(errOut)
		var o capture.Options
		flags.StringVar(&o.URL, "url", "", "LCARS section URL")
		flags.StringVar(&o.Browser, "browser", "", "Chrome/Chromium executable (auto-detected by default)")
		flags.IntVar(&o.Width, "width", 1440, "viewport width in CSS pixels")
		flags.IntVar(&o.Height, "height", 900, "viewport height in CSS pixels")
		flags.StringVar(&o.Out, "out", "capture.png", "output PNG path (JSON report uses the same stem)")
		flags.StringVar(&o.Panel, "panel", "", "capture one panel, identified by tab/panel")
		flags.StringVar(&o.ExpectPanelHash, "expect-panel-hash", "", "require this exact displayed document SHA-256 (requires --panel)")
		flags.StringVar(&o.ExpectDataHash, "expect-data-hash", "", "require this exact displayed data SHA-256, or missing (requires --panel)")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected screenshot arguments")
		}
		_, err := capture.Run(ctx, o)
		if err == nil {
			fmt.Fprintln(out, "Captured", o.Out)
		}
		return err
	default:
		usage(errOut)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "LCARS %s · authoring contract %s\n", buildinfo.Current().Version, buildinfo.ContractVersion)
	fmt.Fprintln(w, `An agent-fed canvas

Usage:
  lcars serve --data ./content [--listen 127.0.0.1:8080] [--run-scripts]
  lcars validate --data ./content
  lcars screenshot --url URL --out desktop.png [--width 1440 --height 900]
  lcars snapshot --data ./content --out ./captures [--browser /path/to/chromium]
  lcars skill [--guide | --starter | --out ./lcars-authoring]
  lcars version [--json]

Agents: start with lcars skill, then lcars skill --guide. The skill and starter
are bundled in this binary. Export a portable copy with lcars skill --out DIR.

Run a command with --help for its options.`)
}

// list splits a comma-separated flag value, dropping blanks.
func list(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

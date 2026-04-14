package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/alexfsong/jeeves/internal/api"
	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/engine"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
	"github.com/alexfsong/jeeves/web"
	"github.com/spf13/cobra"
)

var (
	servePort int
	serveOpen bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Jeeves web dashboard",
	Long:  "Start a local web server providing the Jeeves research dashboard.",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(false)

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		st, err := store.Open()
		if err != nil {
			return fmt.Errorf("opening store: %w", err)
		}
		defer st.Close()

		search := buildSearchSource(cfg)
		router := llm.NewRouter(cfg)
		wiki := source.NewWikiSource(st)
		fetcher := source.NewFetcher()
		eng := engine.New(search, wiki, fetcher, router, st, cfg.Verify)

		// Warn if verification is enabled — tools incur separate metered charges.
		if cfg.Verify.Enabled {
			r.Warn("Verification pass is enabled. Web search and web_fetch tools incur separate charges (~$0.06/query). Disable with verify.enabled=false in ~/.jeeves/config.toml or unset JEEVES_VERIFY.")
		}

		a := api.New(st, eng, router, cfg)

		handler := a.HandlerWithStatic(web.Static())

		addr := fmt.Sprintf(":%d", servePort)
		srv := &http.Server{Addr: addr, Handler: handler}

		url := fmt.Sprintf("http://localhost:%d", servePort)
		r.Success(fmt.Sprintf("Jeeves dashboard: %s", url))
		r.Meta("Press Ctrl+C to stop, sir.")

		if serveOpen {
			openBrowser(url)
		}

		// Graceful shutdown
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
		}()

		<-stop
		r.Body("\nShutting down, sir...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	},
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return
	}
	exec.Command(cmd, url).Start()
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 7525, "Port to serve on")
	serveCmd.Flags().BoolVar(&serveOpen, "open", false, "Open dashboard in browser")
	rootCmd.AddCommand(serveCmd)
}

// Command xplan is a cockpit for many parallel projects: it keeps each
// project's intent, collects its docs and progress, and runs read-only
// external reviews. `xplan serve` starts the web UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ffqa/xplan/internal/app"
	"github.com/ffqa/xplan/internal/config"
	"github.com/ffqa/xplan/internal/insight"
	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/server"
	"github.com/ffqa/xplan/web"
)

const usage = `xplan — cockpit for parallel projects

Usage:
  xplan [--home DIR] <command> [args]

Commands:
  serve [--listen ADDR] [--no-schedule] [--open]   Start the web UI (default 127.0.0.1:7788)
  add <path> [--name NAME]                          Register a project directory
  discover <root> [--add]                           List (or register) git repos under root
  list                                              List registered projects
  scan [slug]                                       Rescan one project, or all
  brief <slug>                                      Print the re-entry brief
  park <slug> "<stopped> | <next> | <open>"         Leave a park note
  review <slug> <kind> [--question Q]               Run a read-only review and wait for it
  version                                           Print version

Home defaults to $XPLAN_HOME or ~/.xplan.
`

func main() {
	global := flag.NewFlagSet("xplan", flag.ExitOnError)
	home := global.String("home", "", "xplan home directory")
	global.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := global.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	args := global.Args()
	if len(args) == 0 {
		global.Usage()
		os.Exit(2)
	}
	if args[0] == "version" {
		fmt.Println(app.Version)
		return
	}

	resolvedHome, err := config.ResolveHome(*home)
	if err != nil {
		fatal(err)
	}
	logger := log.New(os.Stderr, "xplan ", log.LstdFlags)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application, err := app.New(ctx, resolvedHome, logger)
	if err != nil {
		fatal(err)
	}

	commands := map[string]func(context.Context, *app.App, []string) error{
		"serve":    cmdServe,
		"add":      cmdAdd,
		"discover": cmdDiscover,
		"list":     cmdList,
		"scan":     cmdScan,
		"brief":    cmdBrief,
		"park":     cmdPark,
		"review":   cmdReview,
	}
	run, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		global.Usage()
		os.Exit(2)
	}
	if err := run(ctx, application, args[1:]); err != nil {
		fatal(err)
	}
}

func cmdServe(ctx context.Context, application *app.App, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := flags.String("listen", application.Config().Listen, "listen address")
	noSchedule := flags.Bool("no-schedule", false, "do not run scheduled reviews")
	openBrowser := flags.Bool("open", false, "open the UI in the default browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	static := web.Dist()
	if static == nil {
		application.Logger.Printf("UI not embedded (run `make web` then rebuild); serving API only")
	}
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           server.New(application, static),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go application.RunBackground(ctx, !*noSchedule)

	errs := make(chan error, 1)
	go func() { errs <- httpServer.ListenAndServe() }()
	url := "http://" + strings.Replace(*listen, "0.0.0.0", "127.0.0.1", 1)
	application.Logger.Printf("xplan %s serving %s (home %s, scheduled reviews %v)", app.Version, url, application.Home, !*noSchedule)
	if *openBrowser {
		if err := exec.Command("open", url).Start(); err != nil {
			application.Logger.Printf("open browser: %v", err)
		}
	}
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

func cmdAdd(_ context.Context, application *app.App, args []string) error {
	flags := flag.NewFlagSet("add", flag.ExitOnError)
	name := flags.String("name", "", "display name")
	if err := flags.Parse(reorder(args)); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("usage: xplan add <path> [--name NAME]")
	}
	for _, path := range flags.Args() {
		created, err := application.Store.Add(path, *name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
			continue
		}
		if _, err := application.ScanProject(created.Slug); err != nil {
			fmt.Fprintf(os.Stderr, "scan %s: %v\n", created.Slug, err)
		}
		fmt.Println(created.Slug)
	}
	return nil
}

func cmdDiscover(_ context.Context, application *app.App, args []string) error {
	flags := flag.NewFlagSet("discover", flag.ExitOnError)
	add := flags.Bool("add", false, "register every candidate")
	if err := flags.Parse(reorder(args)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: xplan discover <root> [--add]")
	}
	candidates, err := application.Discover(flags.Arg(0))
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if !*add {
			fmt.Println(candidate)
			continue
		}
		created, err := application.Store.Add(candidate, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", candidate, err)
			continue
		}
		if _, err := application.ScanProject(created.Slug); err != nil {
			fmt.Fprintf(os.Stderr, "scan %s: %v\n", created.Slug, err)
		}
		fmt.Printf("%s\t%s\n", created.Slug, candidate)
	}
	return nil
}

func cmdList(_ context.Context, application *app.App, _ []string) error {
	cards, err := application.Overview()
	if err != nil {
		return err
	}
	for _, card := range cards {
		lastCommit := "-"
		if card.LastCommitAt != nil {
			lastCommit = age(*card.LastCommitAt)
		}
		fmt.Printf("%-24s %-8s %-5s commit %-6s %s\n", card.Slug, card.Status, card.HealthLevel, lastCommit, card.Path)
	}
	return nil
}

func cmdScan(_ context.Context, application *app.App, args []string) error {
	if len(args) == 1 {
		snapshot, err := application.ScanProject(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s: %d docs in %dms\n", args[0], len(snapshot.Docs), snapshot.DurationMs)
		return nil
	}
	for slug, err := range application.ScanAll() {
		fmt.Fprintf(os.Stderr, "%s: %v\n", slug, err)
	}
	return nil
}

func cmdBrief(_ context.Context, application *app.App, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: xplan brief <slug>")
	}
	brief, err := application.Brief(args[0])
	if err != nil {
		return err
	}
	printBrief(brief)
	return nil
}

func cmdPark(_ context.Context, application *app.App, args []string) error {
	if len(args) != 2 {
		return errors.New(`usage: xplan park <slug> "<stopped> | <next> | <open>"`)
	}
	parts := strings.Split(args[1], "|")
	entry := project.Entry{Stopped: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		entry.Next = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		entry.Open = strings.TrimSpace(strings.Join(parts[2:], "|"))
	}
	saved, err := application.Park(args[0], entry)
	if err != nil {
		return err
	}
	fmt.Println("parked at", saved.At.Local().Format("2006-01-02 15:04"))
	return nil
}

func cmdReview(ctx context.Context, application *app.App, args []string) error {
	flags := flag.NewFlagSet("review", flag.ExitOnError)
	question := flags.String("question", "", "question for kind=ask")
	if err := flags.Parse(reorder(args)); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("usage: xplan review <slug> <kind> [--question Q]")
	}
	slug := flags.Arg(0)
	if _, err := application.ScanProject(slug); err != nil {
		return err
	}
	job, err := application.Reviews.Enqueue(slug, flags.Arg(1), *question, "cli")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "review %s started with %s\n", job.ID, job.Reviewer)
	for {
		select {
		case <-ctx.Done():
			return application.Reviews.Cancel(slug, job.ID)
		case <-time.After(2 * time.Second):
		}
		current, err := application.Reviews.Get(slug, job.ID)
		if err != nil {
			return err
		}
		if current.Active() {
			continue
		}
		report, err := application.Reviews.Output(slug, job.ID)
		if err != nil {
			return err
		}
		fmt.Println(report)
		fmt.Fprintf(os.Stderr, "\nstatus=%s verdict=%s score=%d findings=%d %s\n", current.Status, current.Verdict, current.Score, len(current.Findings), current.Error)
		return nil
	}
}

func printBrief(brief *insight.Brief) {
	card := brief.Card
	fmt.Printf("# %s  [%s · health %s]\n%s\n\n", card.Name, card.Status, card.HealthLevel, card.Path)
	fmt.Println("## Intent")
	fmt.Println(strings.TrimSpace(brief.Intent))
	fmt.Println()
	if park := card.LastPark; park != nil {
		fmt.Printf("## Where you left off (%s)\n", age(park.At))
		for _, field := range [][2]string{{"Stopped", park.Stopped}, {"Next", park.Next}, {"Open", park.Open}, {"Note", park.Text}} {
			if field[1] != "" {
				fmt.Printf("- %s: %s\n", field[0], field[1])
			}
		}
		fmt.Println()
	}
	if brief.Since.At != nil {
		fmt.Printf("## Since %s (%s)\n", brief.Since.Reference, age(*brief.Since.At))
		for _, commit := range brief.Since.Commits {
			fmt.Printf("- %s %s\n", commit.Short, commit.Subject)
		}
		for _, doc := range brief.Since.Docs {
			fmt.Printf("- doc changed: %s\n", doc.Path)
		}
		fmt.Println()
	}
	if len(card.Health) > 0 {
		fmt.Println("## Health")
		for _, flag := range card.Health {
			fmt.Printf("- [%s] %s\n", flag.Level, flag.Message)
		}
	}
}

// reorder moves flags before positional args so `add PATH --name X` works.
func reorder(args []string) []string {
	flags, positional := []string{}, []string{}
	for index := 0; index < len(args); index++ {
		if strings.HasPrefix(args[index], "-") {
			flags = append(flags, args[index])
			if !strings.Contains(args[index], "=") && index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") && args[index] != "--add" && args[index] != "-add" {
				flags = append(flags, args[index+1])
				index++
			}
			continue
		}
		positional = append(positional, args[index])
	}
	return append(flags, positional...)
}

func age(at time.Time) string {
	elapsed := time.Since(at)
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 48*time.Hour:
		return fmt.Sprintf("%dh", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd", int(elapsed.Hours()/24))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "xplan:", err)
	os.Exit(1)
}

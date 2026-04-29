package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/runner"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/go-go-golems/go-go-agent/internal/evaljs"
	"github.com/go-go-golems/go-go-agent/internal/helpdb"
	"github.com/go-go-golems/go-go-agent/internal/helpdocs"
	"github.com/go-go-golems/go-go-agent/internal/logdb"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/spf13/cobra"
)

type settings struct {
	ConfigFile         string
	Profile            string
	ProfileRegistries  []string
	InputDBPath        string
	OutputDBPath       string
	EvalTimeout        time.Duration
	MaxOutputChars     int
	LogDBPath          string
	LogDBStrict        bool
	NoLogDB            bool
	LogDBKeepTemp      bool
	LogDBTurnSnapshots bool
	StreamStdout       bool
	PrintFinalTurn     bool
}

func main() {
	ctx := context.Background()
	var s settings
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Simple Geppetto chat REPL with an eval_js documentation tool",
		Long: `chat is a minimal stdin/stdout LLM chatbot.

It resolves standard Pinocchio profiles, embeds its own Glazed help entries into
an input SQLite database, exposes inputDB/outputDB globals to a go-go-goja
runtime, and registers a single Geppetto tool named eval_js.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.InitLoggerFromCobra(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(ctx, s, args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&s.ConfigFile, "config-file", "", "Explicit Pinocchio config/profile file")
	cmd.Flags().StringVar(&s.Profile, "profile", "", "Pinocchio profile to load")
	cmd.Flags().StringArrayVar(&s.ProfileRegistries, "profile-registries", nil, "Profile registry source (repeatable)")
	cmd.Flags().StringVar(&s.InputDBPath, "input-db", "", "Optional path for materialized embedded help input DB")
	cmd.Flags().StringVar(&s.OutputDBPath, "output-db", "", "Optional path for writable scratch output DB")
	cmd.Flags().DurationVar(&s.EvalTimeout, "eval-timeout", 5*time.Second, "eval_js execution timeout")
	cmd.Flags().IntVar(&s.MaxOutputChars, "max-output-chars", 16000, "maximum string/console output characters returned by eval_js")
	cmd.Flags().StringVar(&s.LogDBPath, "log-db", "", "Path for the private host-only logging DB (defaults to a temp SQLite DB)")
	cmd.Flags().BoolVar(&s.LogDBStrict, "log-db-strict", false, "Fail the chat run if private logging persistence fails")
	cmd.Flags().BoolVar(&s.NoLogDB, "no-log-db", false, "Disable private DB logging and eval_js persistence")
	cmd.Flags().BoolVar(&s.LogDBKeepTemp, "log-db-keep-temp", false, "Keep the default temporary log DB after exit")
	cmd.Flags().BoolVar(&s.LogDBTurnSnapshots, "log-db-turn-snapshots", false, "Persist intermediate turn snapshots in addition to final turns")
	cmd.Flags().BoolVar(&s.StreamStdout, "stream", true, "Stream assistant/tool progress to stdout while inference runs")
	cmd.Flags().BoolVar(&s.PrintFinalTurn, "print-final-turn", false, "Print the full final turn after completion, even when streaming")

	if err := logging.AddLoggingSectionToRootCommand(cmd, "chat"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	helpSystem := help.NewHelpSystem()
	if err := helpdocs.AddDocToHelpSystem(helpSystem); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	help_cmd.SetupCobraRootCommand(helpSystem, cmd)

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, s settings, args []string, in io.Reader, out io.Writer, errOut io.Writer) error {
	parsed, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        s.ConfigFile,
		Profile:           s.Profile,
		ProfileRegistries: s.ProfileRegistries,
	})
	if err != nil {
		return err
	}
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return fmt.Errorf("resolve Pinocchio profile settings: %w", err)
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}
	if resolved.FinalInferenceSettings == nil {
		return fmt.Errorf("resolved inference settings are nil")
	}

	input, err := helpdb.PrepareInputDB(ctx, helpdb.InputDBConfig{
		Path:    s.InputDBPath,
		HelpFS:  helpdocs.FS,
		HelpDir: helpdocs.Dir,
	})
	if err != nil {
		return fmt.Errorf("prepare input DB: %w", err)
	}
	defer input.Close()

	output, err := helpdb.PrepareOutputDB(ctx, s.OutputDBPath)
	if err != nil {
		return fmt.Errorf("prepare output DB: %w", err)
	}
	defer output.Close()

	scope := evaljs.Scope{InputDB: input.DB, OutputDB: output.DB}
	evalRuntimeFactory, err := evaljs.NewEngineFactory(scope)
	if err != nil {
		return fmt.Errorf("build eval_js engine factory: %w", err)
	}
	if s.NoLogDB {
		return fmt.Errorf("--no-log-db is incompatible with replapi-backed eval_js; omit it or provide --log-db")
	}
	logDB, err := logdb.Open(ctx, logdb.Config{
		Path:    s.LogDBPath,
		Strict:  s.LogDBStrict,
		Profile: s.Profile,
	}, evalRuntimeFactory)
	if err != nil {
		return fmt.Errorf("open private log DB: %w", err)
	}
	defer func() {
		path := logDB.Path
		_ = logDB.Close()
		if s.LogDBPath == "" && !s.LogDBKeepTemp {
			_ = os.Remove(path)
			_ = os.Remove(path + "-wal")
			_ = os.Remove(path + "-shm")
		}
	}()

	evalRuntime, err := evaljs.Build(ctx, scope, evaljs.Options{
		Timeout:        s.EvalTimeout,
		MaxOutputChars: s.MaxOutputChars,
	}, evaljs.WithEvalTool(logDB.EvalTool()))
	if err != nil {
		return fmt.Errorf("build eval_js runtime: %w", err)
	}
	defer evalRuntime.Close()

	r := runner.New()
	runtime := runner.Runtime{
		InferenceSettings: resolved.FinalInferenceSettings,
		ToolRegistrars:    []runner.ToolRegistrar{evalRuntime.Registrar()},
		ToolNames:         []string{evaljs.ToolName},
	}

	seed := initialTurn()
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		return runPrompt(ctx, r, runtime, logDB, s.LogDBTurnSnapshots, s.StreamStdout, s.PrintFinalTurn, &seed, prompt, out, errOut)
	}
	return repl(ctx, r, runtime, logDB, s.LogDBTurnSnapshots, s.StreamStdout, s.PrintFinalTurn, &seed, in, out, errOut)
}

func repl(ctx context.Context, r *runner.Runner, runtime runner.Runtime, logDB *logdb.DB, logDBTurnSnapshots bool, streamStdout bool, printFinalTurn bool, seed *turns.Turn, in io.Reader, out io.Writer, errOut io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	fmt.Fprintln(out, "chat REPL. Type :help for commands, :quit to exit.")
	for {
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "":
			continue
		case ":quit", ":exit":
			return nil
		case ":reset":
			*seed = initialTurn()
			fmt.Fprintln(out, "conversation reset")
			continue
		case ":help":
			printREPLHelp(out)
			continue
		}
		if err := runPrompt(ctx, r, runtime, logDB, logDBTurnSnapshots, streamStdout, printFinalTurn, seed, line, out, errOut); err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
		}
	}
}

func runPrompt(ctx context.Context, r *runner.Runner, runtime runner.Runtime, logDB *logdb.DB, logDBTurnSnapshots bool, streamStdout bool, printFinalTurn bool, seed *turns.Turn, prompt string, out io.Writer, errOut io.Writer) error {
	req := runner.StartRequest{
		SeedTurn: seed,
		Prompt:   prompt,
		Runtime:  runtime,
	}
	if streamStdout {
		req.EventSinks = append(req.EventSinks, newStdoutStreamSink(out, errOut, stdoutStreamOptions{}))
	}
	if logDB != nil {
		req.SessionID = logDB.ChatSessionID
		if logDBTurnSnapshots {
			req.SnapshotHook = logDB.SnapshotHook()
		}
		req.Persister = logDB.TurnPersister()
	}
	_, updated, err := r.Run(ctx, req)
	if err != nil {
		return err
	}
	if streamStdout {
		fmt.Fprintln(out)
	}
	if !streamStdout || printFinalTurn {
		fmt.Fprintln(out)
		turns.FprintfTurn(out, updated, turns.WithToolDetail(true))
		fmt.Fprintln(out)
	}
	*seed = *updated.Clone()
	return nil
}

func initialTurn() turns.Turn {
	seed := turns.Turn{}
	turns.AppendBlock(&seed, turns.NewSystemTextBlock(`You are the chat agent.
You have exactly one tool available: eval_js.
Use eval_js when the user asks about the embedded chat help entries, the JavaScript runtime APIs, inputDB/outputDB, or implementation details captured in the embedded help database.
The eval_js runtime exposes inputDB as a read-only SQLite facade over embedded help entries and outputDB as writable scratch space.
Prefer small SELECT queries against inputDB.docs or inputDB.sections, and cite help slugs/titles when answering.`))
	return seed
}

func printREPLHelp(out io.Writer) {
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  :help   show this help")
	fmt.Fprintln(out, "  :reset  clear in-memory conversation")
	fmt.Fprintln(out, "  :quit   exit")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Try: Use eval_js to list the embedded help entries.")
}

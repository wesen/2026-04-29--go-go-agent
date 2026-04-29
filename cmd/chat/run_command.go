package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type RunCommand struct {
	*cmds.CommandDescription
	ctx context.Context
}

type runCommandSettings struct {
	ConfigFile            string   `glazed:"config-file"`
	Profile               string   `glazed:"profile"`
	ProfileRegistries     []string `glazed:"profile-registries"`
	InputDBPath           string   `glazed:"input-db"`
	OutputDBPath          string   `glazed:"output-db"`
	EvalTimeout           string   `glazed:"eval-timeout"`
	MaxOutputChars        int      `glazed:"max-output-chars"`
	LogDBPath             string   `glazed:"log-db"`
	LogDBStrict           bool     `glazed:"log-db-strict"`
	NoLogDB               bool     `glazed:"no-log-db"`
	LogDBKeepTemp         bool     `glazed:"log-db-keep-temp"`
	LogDBTurnSnapshots    bool     `glazed:"log-db-turn-snapshots"`
	StreamStdout          bool     `glazed:"stream"`
	PrintFinalTurn        bool     `glazed:"print-final-turn"`
	StreamToolDetails     bool     `glazed:"stream-tool-details"`
	StreamMaxPreviewChars int      `glazed:"stream-max-preview-chars"`
	Prompt                []string `glazed:"prompt"`
}

var _ cmds.WriterCommand = &RunCommand{}

func NewRunCommand(ctx context.Context) (*RunCommand, error) {
	desc := cmds.NewCommandDescription(
		"run",
		cmds.WithShort("Run the chat REPL or a one-shot prompt"),
		cmds.WithLong(`Run starts the chat REPL when no prompt is provided, or executes a one-shot prompt when positional arguments are given.

Examples:
  chat run --profile gpt-5-nano-low
  chat run --log-db /tmp/chat.sqlite "List the embedded help topics"`),
		cmds.WithFlags(
			fields.New("config-file", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Explicit Pinocchio config/profile file")),
			fields.New("profile", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Pinocchio profile to load")),
			fields.New("profile-registries", fields.TypeStringList, fields.WithDefault([]string{}), fields.WithHelp("Profile registry source (repeatable)")),
			fields.New("input-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Optional path for materialized embedded help input DB")),
			fields.New("output-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Optional path for writable scratch output DB")),
			fields.New("eval-timeout", fields.TypeString, fields.WithDefault("5s"), fields.WithHelp("eval_js execution timeout")),
			fields.New("max-output-chars", fields.TypeInteger, fields.WithDefault(16000), fields.WithHelp("maximum string/console output characters returned by eval_js")),
			fields.New("log-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Path for the private host-only logging DB (defaults to a temp SQLite DB)")),
			fields.New("log-db-strict", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Fail the chat run if private logging persistence fails")),
			fields.New("no-log-db", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Disable private DB logging and eval_js persistence")),
			fields.New("log-db-keep-temp", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Keep the default temporary log DB after exit")),
			fields.New("log-db-turn-snapshots", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Persist intermediate turn snapshots in addition to final turns")),
			fields.New("stream", fields.TypeBool, fields.WithDefault(true), fields.WithHelp("Stream assistant/tool/thinking progress to stdout while inference runs")),
			fields.New("print-final-turn", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Print the full final turn after completion, even when streaming")),
			fields.New("stream-tool-details", fields.TypeBool, fields.WithDefault(true), fields.WithHelp("Print expanded streaming tool call code and tool results")),
			fields.New("stream-max-preview-chars", fields.TypeInteger, fields.WithDefault(4000), fields.WithHelp("Maximum characters for streamed tool args/results previews")),
		),
		cmds.WithArguments(
			fields.New("prompt", fields.TypeStringList, fields.WithDefault([]string{}), fields.WithHelp("Optional one-shot prompt"), fields.WithIsArgument(true)),
		),
	)
	return &RunCommand{CommandDescription: desc, ctx: ctx}, nil
}

func (c *RunCommand) RunIntoWriter(ctx context.Context, vals *values.Values, w io.Writer) error {
	settings_, err := decodeRunSettings(vals)
	if err != nil {
		return err
	}
	runCtx := ctx
	if c.ctx != nil {
		runCtx = c.ctx
	}
	return run(runCtx, settings_, settings_.Prompt, os.Stdin, w, os.Stderr)
}

func decodeRunSettings(vals *values.Values) (settings, error) {
	raw := &runCommandSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, raw); err != nil {
		return settings{}, err
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(raw.EvalTimeout))
	if err != nil {
		return settings{}, fmt.Errorf("parse --eval-timeout: %w", err)
	}
	return settings{
		ConfigFile:            raw.ConfigFile,
		Profile:               raw.Profile,
		ProfileRegistries:     raw.ProfileRegistries,
		InputDBPath:           raw.InputDBPath,
		OutputDBPath:          raw.OutputDBPath,
		EvalTimeout:           timeout,
		MaxOutputChars:        raw.MaxOutputChars,
		LogDBPath:             raw.LogDBPath,
		LogDBStrict:           raw.LogDBStrict,
		NoLogDB:               raw.NoLogDB,
		LogDBKeepTemp:         raw.LogDBKeepTemp,
		LogDBTurnSnapshots:    raw.LogDBTurnSnapshots,
		StreamStdout:          raw.StreamStdout,
		PrintFinalTurn:        raw.PrintFinalTurn,
		StreamToolDetails:     raw.StreamToolDetails,
		StreamMaxPreviewChars: raw.StreamMaxPreviewChars,
		Prompt:                raw.Prompt,
	}, nil
}

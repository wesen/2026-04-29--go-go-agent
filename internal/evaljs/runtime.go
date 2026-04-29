package evaljs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/runner"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/go-go-agent/internal/jsdb"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
)

const ToolName = "eval_js"

type Scope struct {
	InputDB  *sql.DB
	OutputDB *sql.DB
}

type Meta struct {
	Globals []string
}

type EvalInput = scopedjs.EvalInput
type EvalOutput = scopedjs.EvalOutput
type ConsoleLine = scopedjs.ConsoleLine

type EvalTool interface {
	Eval(ctx context.Context, in EvalInput) (EvalOutput, error)
}

type Runtime struct {
	Spec scopedjs.EnvironmentSpec[Scope, Meta]
	Tool EvalTool
}

type Options struct {
	Timeout        time.Duration
	MaxOutputChars int
}

type BuildOption func(*buildConfig)

type buildConfig struct {
	evalTool EvalTool
}

func WithEvalTool(tool EvalTool) BuildOption {
	return func(c *buildConfig) { c.evalTool = tool }
}

func Build(ctx context.Context, scope Scope, opts Options, buildOpts ...BuildOption) (*Runtime, error) {
	_ = ctx
	cfg := buildConfig{}
	for _, opt := range buildOpts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.evalTool == nil {
		return nil, fmt.Errorf("eval_js requires a replapi-backed EvalTool")
	}
	return &Runtime{Spec: NewSpec(opts), Tool: cfg.evalTool}, nil
}

func (r *Runtime) Registrar() runner.ToolRegistrar {
	return func(ctx context.Context, reg geptools.ToolRegistry) error {
		_ = ctx
		if r == nil || r.Tool == nil {
			return fmt.Errorf("eval_js tool is not configured")
		}
		def, err := geptools.NewToolFromFunc(
			r.Spec.Tool.Name,
			scopedjs.BuildDescription(r.Spec.Tool.Description, Manifest(), "Calls execute as persistent replapi/replsession cells; top-level declarations and evaluation history persist across calls."),
			func(ctx context.Context, in EvalInput) (EvalOutput, error) {
				return r.Tool.Eval(ctx, in)
			},
		)
		if err != nil {
			return fmt.Errorf("create %s tool: %w", r.Spec.Tool.Name, err)
		}
		def.Tags = append([]string(nil), r.Spec.Tool.Tags...)
		def.Version = r.Spec.Tool.Version
		if err := reg.RegisterTool(def.Name, *def); err != nil {
			return fmt.Errorf("register %s tool: %w", r.Spec.Tool.Name, err)
		}
		return nil
	}
}

func (r *Runtime) Close() error { return nil }

func NewSpec(opts Options) scopedjs.EnvironmentSpec[Scope, Meta] {
	evalOpts := scopedjs.DefaultEvalOptions()
	if opts.Timeout > 0 {
		evalOpts.Timeout = opts.Timeout
	}
	if opts.MaxOutputChars > 0 {
		evalOpts.MaxOutputChars = opts.MaxOutputChars
	}

	return scopedjs.EnvironmentSpec[Scope, Meta]{
		RuntimeLabel: "chat-help-runtime",
		Tool: scopedjs.ToolDefinitionSpec{
			Name: ToolName,
			Description: scopedjs.ToolDescription{
				Summary: "Execute JavaScript against the chat agent's embedded help SQLite database and writable scratch database.",
				Notes: []string{
					"The runtime exposes inputDB, outputDB, input, globalThis, window, and global globals.",
					"inputDB is read-only and contains help entries embedded into this chat binary.",
					"The canonical help table is sections; docs is a compatibility view over sections.",
					"outputDB is writable scratch space with a notes table.",
					"Use parameterized SQL with ? placeholders when incorporating input values.",
					"Write code as a persistent REPL cell; top-level declarations persist across calls.",
					"The tool result is the final expression value; do not use top-level return.",
					"Use globalThis for explicit global state; window and global are aliases of globalThis.",
					"The final expression must be JSON-serializable for tool output.",
				},
				StarterSnippets: []string{
					`const rows = inputDB.query("SELECT slug, title, short FROM docs ORDER BY title LIMIT 10"); rows`,
					`const matches = inputDB.query("SELECT slug, title FROM docs WHERE content LIKE ? LIMIT 10", "%outputDB%"); matches`,
					`outputDB.exec("INSERT INTO notes(key, value) VALUES (?, ?)", "summary", "important finding"); outputDB.query("SELECT * FROM notes")`,
					`function summarizeDoc(row) { return row.slug + ": " + row.title; }
const rows = inputDB.query("SELECT slug, title FROM docs ORDER BY title LIMIT 3");
rows.map(summarizeDoc)`,
				},
			},
			Tags:    []string{"chat", "javascript", "sqlite", "help"},
			Version: "0.1.0",
		},
		DefaultEval: evalOpts,
		Configure:   configureRuntime,
	}
}

func NewEngineFactory(scope Scope) (*gojengine.Factory, error) {
	return gojengine.NewBuilder().WithRuntimeInitializers(scopeInitializer{scope: scope}).Build()
}

func Manifest() scopedjs.EnvironmentManifest {
	return scopedjs.EnvironmentManifest{
		Globals: []scopedjs.GlobalDoc{
			{Name: "inputDB", Type: "object", Description: "Read-only SQLite facade for embedded chat help entries. Methods: query(sql, ...args), exec(sql, ...args) which errors, schema()."},
			{Name: "outputDB", Type: "object", Description: "Writable scratch SQLite facade. Methods: query(sql, ...args), exec(sql, ...args), schema(). Default table: notes."},
			{Name: "input", Type: "object", Description: "Per-call input object passed in the eval_js tool request."},
			{Name: "globalThis", Type: "object", Description: "Canonical persistent JavaScript global object."},
			{Name: "window", Type: "object", Description: "Alias of globalThis for browser-style snippets; DOM APIs are not implied."},
			{Name: "global", Type: "object", Description: "Alias of globalThis for Node-style snippets; Node built-ins are not implied."},
		},
		Helpers: []scopedjs.HelperDoc{
			{Name: "parameterized SQL", Signature: `inputDB.query("SELECT * FROM docs WHERE slug = ?", slug)`, Description: "Use ? placeholders and pass bind arguments after the SQL string."},
			{Name: "list help entries", Signature: `inputDB.query("SELECT slug, title, short FROM docs ORDER BY title")`, Description: "List embedded help entries available to the agent."},
		},
	}
}

type scopeInitializer struct{ scope Scope }

func (s scopeInitializer) ID() string { return "chat-evaljs-scope" }

func (s scopeInitializer) InitRuntime(ctx *gojengine.RuntimeContext) error {
	input := &jsdb.Facade{Name: "inputDB", DB: s.scope.InputDB, Readonly: true, Tables: []string{"sections", "docs"}}
	if err := bindFacade(ctx, "inputDB", input); err != nil {
		return err
	}
	output := &jsdb.Facade{Name: "outputDB", DB: s.scope.OutputDB, Readonly: false, Tables: []string{"notes"}}
	return bindFacade(ctx, "outputDB", output)
}

func bindFacade(ctx *gojengine.RuntimeContext, globalName string, f *jsdb.Facade) error {
	if ctx == nil || ctx.VM == nil {
		return fmt.Errorf("runtime context is nil")
	}
	obj := ctx.VM.NewObject()
	if err := obj.Set("query", f.Query); err != nil {
		return err
	}
	if err := obj.Set("exec", f.Exec); err != nil {
		return err
	}
	if err := obj.Set("schema", f.Schema); err != nil {
		return err
	}
	return ctx.VM.Set(globalName, obj)
}

func configureRuntime(ctx context.Context, b *scopedjs.Builder, scope Scope) (Meta, error) {
	_ = ctx
	input := &jsdb.Facade{
		Name:     "inputDB",
		DB:       scope.InputDB,
		Readonly: true,
		Tables:   []string{"sections", "docs"},
	}
	name, bind, doc := input.BindGlobal("inputDB", scopedjs.GlobalDoc{
		Type:        "object",
		Description: "Read-only SQLite facade for embedded chat help entries. Methods: query(sql, ...args), exec(sql, ...args) which errors, schema().",
	})
	if err := b.AddGlobal(name, bind, doc); err != nil {
		return Meta{}, err
	}

	output := &jsdb.Facade{
		Name:     "outputDB",
		DB:       scope.OutputDB,
		Readonly: false,
		Tables:   []string{"notes"},
	}
	name, bind, doc = output.BindGlobal("outputDB", scopedjs.GlobalDoc{
		Type:        "object",
		Description: "Writable scratch SQLite facade. Methods: query(sql, ...args), exec(sql, ...args), schema(). Default table: notes.",
	})
	if err := b.AddGlobal(name, bind, doc); err != nil {
		return Meta{}, err
	}

	if err := b.AddHelper("parameterized SQL", `inputDB.query("SELECT * FROM docs WHERE slug = ?", slug)`, "Use ? placeholders and pass bind arguments after the SQL string."); err != nil {
		return Meta{}, err
	}
	if err := b.AddHelper("list help entries", `inputDB.query("SELECT slug, title, short FROM docs ORDER BY title")`, "List embedded help entries available to the agent."); err != nil {
		return Meta{}, err
	}

	return Meta{Globals: []string{"inputDB", "outputDB"}}, nil
}

func trimPreview(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "…"
}

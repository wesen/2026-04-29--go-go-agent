package evaljs

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/runner"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/go-go-agent/internal/jsdb"
)

const ToolName = "eval_js"

type Scope struct {
	InputDB  *sql.DB
	OutputDB *sql.DB
}

type Meta struct {
	Globals []string
}

type Runtime struct {
	Spec   scopedjs.EnvironmentSpec[Scope, Meta]
	Handle *scopedjs.BuildResult[Meta]
}

type Options struct {
	Timeout        time.Duration
	MaxOutputChars int
}

func Build(ctx context.Context, scope Scope, opts Options) (*Runtime, error) {
	spec := NewSpec(opts)
	handle, err := scopedjs.BuildRuntime(ctx, spec, scope)
	if err != nil {
		return nil, err
	}
	return &Runtime{Spec: spec, Handle: handle}, nil
}

func (r *Runtime) Registrar() runner.ToolRegistrar {
	return func(ctx context.Context, reg geptools.ToolRegistry) error {
		if r == nil || r.Handle == nil {
			return fmt.Errorf("eval_js runtime is not built")
		}
		return scopedjs.RegisterPrebuilt(reg, r.Spec, r.Handle, scopedjs.EvalOptionOverrides{})
	}
}

func (r *Runtime) Close() error {
	if r == nil || r.Handle == nil || r.Handle.Cleanup == nil {
		return nil
	}
	return r.Handle.Cleanup()
}

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
					"The runtime exposes inputDB and outputDB globals.",
					"inputDB is read-only and contains help entries embedded into this chat binary.",
					"The canonical help table is sections; docs is a compatibility view over sections.",
					"outputDB is writable scratch space with a notes table.",
					"Use parameterized SQL with ? placeholders when incorporating input values.",
					"Return a JSON-serializable value from the script.",
				},
				StarterSnippets: []string{
					`const rows = inputDB.query("SELECT slug, title, short FROM docs ORDER BY title LIMIT 10"); return rows;`,
					`const matches = inputDB.query("SELECT slug, title FROM docs WHERE content LIKE ? LIMIT 10", "%outputDB%"); return matches;`,
					`outputDB.exec("INSERT INTO notes(key, value) VALUES (?, ?)", "summary", "important finding"); return outputDB.query("SELECT * FROM notes");`,
				},
			},
			Tags:    []string{"chat", "javascript", "sqlite", "help"},
			Version: "0.1.0",
		},
		DefaultEval: evalOpts,
		Configure:   configureRuntime,
	}
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

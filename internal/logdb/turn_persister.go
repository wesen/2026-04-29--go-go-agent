package logdb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type TurnLogger struct {
	DB *DB
}

func (d *DB) TurnPersister() *TurnLogger {
	if d == nil {
		return nil
	}
	return &TurnLogger{DB: d}
}

func (d *DB) SnapshotHook() toolloop.SnapshotHook {
	if d == nil {
		return nil
	}
	return d.TurnPersister().SnapshotHook()
}

func (l *TurnLogger) PersistTurn(ctx context.Context, t *turns.Turn) error {
	return l.SaveSnapshot(ctx, t, "final")
}

func (l *TurnLogger) SnapshotHook() toolloop.SnapshotHook {
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if err := l.SaveSnapshot(ctx, t, phase); err != nil && l.DB != nil && l.DB.Strict {
			// SnapshotHook cannot return an error. Final-turn persistence and eval
			// persistence still surface errors directly when strict mode is enabled.
			_ = err
		}
	}
}

func (l *TurnLogger) SaveSnapshot(ctx context.Context, t *turns.Turn, phase string) error {
	if l == nil || l.DB == nil || l.DB.TurnStore == nil || t == nil {
		return nil
	}
	turnID := t.ID
	if turnID == "" {
		turnID = newID("turn")
	}
	sessionID := l.DB.ChatSessionID
	if got, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok && got != "" {
		sessionID = got
	}
	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return fmt.Errorf("serialize turn snapshot: %w", err)
	}
	return l.DB.TurnStore.Save(ctx, l.DB.ConvID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  runtimeKey(t),
		InferenceID: inferenceID(t),
	})
}

func runtimeKey(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if got, ok, err := turns.KeyTurnMetaRuntime.Get(t.Metadata); err == nil && ok && got != nil {
		return fmt.Sprint(got)
	}
	return ""
}

func inferenceID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if got, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		return got
	}
	return ""
}

package logdb_test

import (
	"context"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
)

func TestTurnLoggerPersistsSnapshotIntoChatstoreTables(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer db.Close()

	turn := &turns.Turn{ID: "turn-test"}
	if err := turns.KeyTurnMetaSessionID.Set(&turn.Metadata, db.ChatSessionID); err != nil {
		t.Fatalf("set session metadata: %v", err)
	}
	if err := turns.KeyTurnMetaInferenceID.Set(&turn.Metadata, "inference-test"); err != nil {
		t.Fatalf("set inference metadata: %v", err)
	}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("world"))

	if err := db.TurnPersister().SaveSnapshot(ctx, turn, "final"); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	var turnCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM turns WHERE session_id = ? AND turn_id = ?`, db.ChatSessionID, "turn-test").Scan(&turnCount); err != nil {
		t.Fatalf("query turns: %v", err)
	}
	if turnCount != 1 {
		t.Fatalf("expected one turn row, got %d", turnCount)
	}

	var blockCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM turn_block_membership WHERE session_id = ? AND turn_id = ?`, db.ChatSessionID, "turn-test").Scan(&blockCount); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if blockCount != 2 {
		t.Fatalf("expected two block memberships, got %d", blockCount)
	}
}

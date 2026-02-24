package chatstore

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkSQLiteTurnStore_ListByConversation(b *testing.B) {
	dir := b.TempDir()
	dsn, err := SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	if err != nil {
		b.Fatal(err)
	}
	store, err := NewSQLiteTurnStore(dsn)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	for i := 0; i < 2000; i++ {
		convID := "conv-bench"
		sessionID := fmt.Sprintf("sess-%d", i%8)
		turnID := fmt.Sprintf("turn-%d", i)
		phase := "final"
		payload := validTurnPayload(turnID, fmt.Sprintf("text-%d", i))
		if err := store.Save(ctx, convID, sessionID, turnID, phase, int64(1_000+i), payload, TurnSaveOptions{}); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.List(ctx, TurnQuery{ConvID: "conv-bench", Limit: 200})
		if err != nil {
			b.Fatal(err)
		}
	}
}

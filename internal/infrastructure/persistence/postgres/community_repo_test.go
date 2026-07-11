package postgres

import (
	"strings"
	"testing"
)

func TestBuildPublishedDataSQLAllowsComputedTrendingOrder(t *testing.T) {
	const trendingOrder = "(pc.like_count * 10 + EXTRACT(EPOCH FROM pc.published_at) / 86400) DESC"

	query := buildPublishedDataSQL("", trendingOrder, 1)

	if strings.Contains(strings.ToUpper(query), "SELECT DISTINCT") {
		t.Fatal("trending query must not use SELECT DISTINCT with a computed ORDER BY expression")
	}
	if !strings.Contains(query, "ORDER BY "+trendingOrder) {
		t.Fatalf("trending ORDER BY expression missing from query: %s", query)
	}
	if !strings.Contains(query, "LIMIT $1 OFFSET $2") {
		t.Fatalf("pagination placeholders are incorrect: %s", query)
	}
}

func TestBuildPublishedDataSQLUsesDeterministicTieBreaker(t *testing.T) {
	query := buildPublishedDataSQL("WHERE pc.title ILIKE $1", "pc.published_at DESC", 2)

	if !strings.Contains(query, "ORDER BY pc.published_at DESC, pc.canvas_id DESC") {
		t.Fatalf("deterministic canvas ID tie-breaker missing: %s", query)
	}
	if !strings.Contains(query, "WHERE pc.title ILIKE $1") {
		t.Fatalf("filter clause missing: %s", query)
	}
}

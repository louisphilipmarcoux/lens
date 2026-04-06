package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSpanTree(t *testing.T) {
	spans := []TraceSpan{
		{SpanID: "root", ParentID: "", Operation: "GET /api", Duration: 100 * time.Millisecond},
		{SpanID: "child1", ParentID: "root", Operation: "db.query", Duration: 50 * time.Millisecond},
		{SpanID: "child2", ParentID: "root", Operation: "cache.get", Duration: 5 * time.Millisecond},
		{SpanID: "grandchild", ParentID: "child1", Operation: "db.connect", Duration: 10 * time.Millisecond},
	}

	root := buildSpanTree(spans)
	require.NotNil(t, root)
	assert.Equal(t, "root", root.SpanID)
	assert.Len(t, root.Children, 2)
	assert.Equal(t, "child1", root.Children[0].SpanID)
	assert.Equal(t, "child2", root.Children[1].SpanID)
	assert.Len(t, root.Children[0].Children, 1)
	assert.Equal(t, "grandchild", root.Children[0].Children[0].SpanID)
}

func TestBuildSpanTree_NoRoot(t *testing.T) {
	spans := []TraceSpan{
		{SpanID: "a", ParentID: "missing", Operation: "op1"},
		{SpanID: "b", ParentID: "a", Operation: "op2"},
	}

	root := buildSpanTree(spans)
	require.NotNil(t, root)
	// Falls back to first span.
	assert.Equal(t, "a", root.SpanID)
}

package tracing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildSpanTree_NestedLevels guards the recursive build against the
// value-copy regression: an earlier version appended *detail into roots
// before walking children, which dropped any grandchild (root.Children was
// captured before child spans were attached to it). This must stay green.
func TestBuildSpanTree_NestedLevels(t *testing.T) {
	spans := []OtelSpan{
		{SpanID: "root", ParentSpanID: "", Name: "generate"},
		{SpanID: "child", ParentSpanID: "root", Name: "cluster_tags"},
		{SpanID: "grandchild", ParentSpanID: "child", Name: "Router.Chat"},
	}
	tree := BuildSpanTree(spans)

	require.Len(t, tree, 1)
	require.Equal(t, "root", tree[0].SpanID)
	require.Len(t, tree[0].Children, 1)

	child := tree[0].Children[0]
	require.Equal(t, "child", child.SpanID)
	require.Len(t, child.Children, 1)
	require.Equal(t, "grandchild", child.Children[0].SpanID)
}

func TestBuildSpanTree_Empty(t *testing.T) {
	require.Empty(t, BuildSpanTree(nil))
}

// TestBuildSpanTree_OrphanParentBecomesRoot: a span whose ParentSpanID is not
// in the set becomes a root (not silently dropped).
func TestBuildSpanTree_OrphanParentBecomesRoot(t *testing.T) {
	spans := []OtelSpan{
		{SpanID: "orphan", ParentSpanID: "missing", Name: "x"},
	}
	tree := BuildSpanTree(spans)
	require.Len(t, tree, 1)
	require.Equal(t, "orphan", tree[0].SpanID)
}

func TestBuildSpanTree_AllZeroParentIsRoot(t *testing.T) {
	spans := []OtelSpan{
		{SpanID: "r", ParentSpanID: "0000000000000000", Name: "r"},
		{SpanID: "c", ParentSpanID: "r", Name: "c"},
	}
	tree := BuildSpanTree(spans)
	require.Len(t, tree, 1)
	require.Equal(t, "r", tree[0].SpanID)
	require.Len(t, tree[0].Children, 1)
}

func TestBuildSpanTree_MultipleRoots(t *testing.T) {
	spans := []OtelSpan{
		{SpanID: "r1", ParentSpanID: "", Name: "r1"},
		{SpanID: "r2", ParentSpanID: "", Name: "r2"},
		{SpanID: "c1", ParentSpanID: "r1", Name: "c1"},
	}
	tree := BuildSpanTree(spans)
	require.Len(t, tree, 2)
}

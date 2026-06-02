package datamigrate

import (
	"testing"
)

func TestValidateImportColumnsRejectsUnexpectedSourceColumns(t *testing.T) {
	spec := TableSpec{Name: "feeds", PrimaryKey: "id"}
	_, err := validateImportColumns(spec, []string{"id", "title", "legacy_only"}, []string{"id", "title"})
	if err == nil {
		t.Fatal("expected schema drift error")
	}
}

func TestValidateImportColumnsAllowsExplicitMissingTargetColumns(t *testing.T) {
	spec := TableSpec{Name: "topic_tag_embeddings", PrimaryKey: "id", AllowedMissingTargetColumns: []string{"legacy_col"}}
	shared, err := validateImportColumns(spec, []string{"id", "topic_tag_id", "legacy_col"}, []string{"id", "topic_tag_id"})
	if err != nil {
		t.Fatalf("expected allowed column drift to pass: %v", err)
	}
	if len(shared) != 2 {
		t.Fatalf("expected 2 shared columns, got %d", len(shared))
	}
}

func TestNormalizeInsertValueConvertsSQLiteBoolNumerics(t *testing.T) {
	trueValue, err := normalizeInsertValue(int64(1), "boolean")
	if err != nil {
		t.Fatalf("normalize true bool: %v", err)
	}
	trueBool, ok := trueValue.(bool)
	if !ok || !trueBool {
		t.Fatalf("expected int64(1) to normalize to true, got %#v", trueValue)
	}

	falseValue, err := normalizeInsertValue(int64(0), "bool")
	if err != nil {
		t.Fatalf("normalize false bool: %v", err)
	}
	falseBool, ok := falseValue.(bool)
	if !ok || falseBool {
		t.Fatalf("expected int64(0) to normalize to false, got %#v", falseValue)
	}
}

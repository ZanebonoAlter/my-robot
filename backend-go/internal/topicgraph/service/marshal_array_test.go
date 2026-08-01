package service

import (
	"bytes"
	"testing"
)

// Regression guard for the section-timeline 500 bug: a nil slice marshalled to
// the JSON literal "null" (a JSON null scalar) instead of "[]". Stored in a
// jsonb column that value later broke jsonb_array_elements_text with
// SQLSTATE 22023 ("cannot extract elements from a scalar").
func TestMarshalJSONArray(t *testing.T) {
	cases := []struct {
		name string
		v    interface{}
		want []byte
	}{
		{"untyped nil", nil, []byte("[]")},
		{"typed nil slice", []uint(nil), []byte("[]")},
		{"empty slice", []uint{}, []byte("[]")},
		{"non-empty slice", []uint{1, 2, 3}, []byte("[1,2,3]")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := marshalJSONArray(c.v); !bytes.Equal(got, c.want) {
				t.Fatalf("marshalJSONArray(%v) = %s, want %s", c.v, got, c.want)
			}
		})
	}
}

package service

import (
	"encoding/json"
	"reflect"
)

// marshalJSONArray marshals v as a JSON array, normalizing nil and empty slices
// to the literal "[]" so the result is safe to store in a jsonb column consumed
// by jsonb_array_elements_text (a bare "null" scalar raises SQLSTATE 22023).
// Regression guard: TestMarshalJSONArray.
func marshalJSONArray(v interface{}) []byte {
	if v == nil {
		return []byte("[]")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Len() == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	return b
}

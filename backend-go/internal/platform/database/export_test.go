package database

// export_test.go exposes unexported migration helpers to the external test
// package (database_test) so integration tests can drive them directly with
// seeded data. These vars exist only in the test build.
var ExportedRunAuxLabelDupMerge = runAuxLabelDupMerge

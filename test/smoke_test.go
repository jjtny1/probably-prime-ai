package smoke_test

import "testing"

func TestModuleCompiles(t *testing.T) {
	// Intentionally empty. This test exists to force `go test ./...` to compile
	// the module before any package exists. Once Phase 0 (go mod init + go get)
	// is done, this test passes trivially. Do not delete it — it is the
	// canary that the module remains in a buildable state.
}

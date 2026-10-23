Copy into the project root keeping the folders:
  *_test.go                          -> project root (next to app.go)
  cmd/gothermo-admin/main_test.go    -> cmd/gothermo-admin/
  .github/workflows/ci.yml           -> .github/workflows/

Then:
  go get github.com/alicebob/miniredis/v2
  go mod tidy
  go test ./...

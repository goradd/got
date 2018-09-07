package build

//go:generate got -t got -i -o ../../template -I ../inc -d ..
//go:generate go run ../../runner/runner.go ../../expected

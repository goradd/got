package build

//go:generate got -t got -i -o ../../template -I ../inc2:../inc:../inc/testInclude4.inc -d ..
//go:generate go run ../../runner/runner.go ../../expected

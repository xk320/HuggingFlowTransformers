package engine

import (
	"slices"
	"testing"
)

func TestCommandStartsRuntimeDirectly(t *testing.T) {
	cmd := command(t.Context(), "/tmp/HuggingFlowTransformers-runtime", []string{"--worker", "host'g0"})
	if cmd.Path != "/tmp/HuggingFlowTransformers-runtime" {
		t.Fatalf("path = %q", cmd.Path)
	}
	if !slices.Contains(cmd.Args, "--worker") || !slices.Contains(cmd.Args, "host'g0") {
		t.Fatalf("args = %#v", cmd.Args)
	}
}

func TestWrappedEnvironmentAddsBrandedProcessWrapper(t *testing.T) {
	env := wrappedEnvironment("/tmp/HuggingFlowTransformers-process-wrapper.so")
	if !slices.Contains(env, "LD_PRELOAD=/tmp/HuggingFlowTransformers-process-wrapper.so") {
		t.Fatalf("LD_PRELOAD missing from child environment")
	}
	if !slices.Contains(env, "HFT_PROCESS_TITLE=HuggingFlowTransformers-runtime") {
		t.Fatalf("process title missing from child environment")
	}
}

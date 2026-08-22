package forge

import "testing"

func TestParseWorkflowDefinitionGitHubFormat(t *testing.T) {
	definition, err := ParseWorkflowDefinition(`name: Build
on:
  - push
  - workflow_dispatch
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Compile
        run: go test ./...
`)
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	if len(definition.On) != 2 || len(definition.Steps) != 1 || definition.Steps[0].Run != "go test ./..." {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestParseWorkflowDefinitionLegacyShape(t *testing.T) {
	definition, err := ParseWorkflowDefinition(`{"on":["push"],"steps":[{"run":"echo ok"}]}`)
	if err != nil || len(definition.On) != 1 || len(definition.Steps) != 1 {
		t.Fatalf("legacy config was not accepted: %#v, %v", definition, err)
	}
}

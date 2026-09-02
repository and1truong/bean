package agentprotocol_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
)

func TestSequenceVocabularyReferencesAndSemanticDiffAreAgentDiscoverable(t *testing.T) {
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", agentprotocol.APIVersion)
	if !reflect.DeepEqual(capabilities.SequenceProfiles, []string{"presentation"}) || !reflect.DeepEqual(capabilities.SequenceAspectRatios, []string{"standard", "wide"}) || len(capabilities.SequenceFrameLayouts) != 14 || len(capabilities.ContentElementTypes) != 8 {
		t.Fatalf("Sequence capabilities=%+v", capabilities)
	}
	current := appir.Empty()
	current.Blocks["opening"] = appir.Block{Name: "opening", Type: "content", Content: []appir.ContentElement{{Type: "heading", Text: "Bean"}}}
	current.Panels["opening"] = appir.Panel{Name: "opening", Regions: []appir.Region{{Name: "main", Blocks: []string{"opening"}}}}
	current.Sequences["bean"] = appir.Sequence{Name: "bean", Route: "/presentations/bean", Frames: []appir.SequenceFrame{{Name: "opening", Title: "Bean", Panel: "opening"}}}
	_, references, exists := compiler.InspectDefinition(current, "Sequence", "bean")
	if !exists || !containsReference(references, "frames.0.panel", "Panel", "opening") {
		t.Fatalf("Sequence references=%+v", references)
	}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	block := candidate.Blocks["opening"]
	block.Content[0].Text = "Introducing Bean"
	candidate.Blocks["opening"] = block
	sequence := candidate.Sequences["bean"]
	sequence.Frames[0].Title = "Introducing Bean"
	candidate.Sequences["bean"] = sequence
	changes := agentprotocol.SemanticDiff(current, candidate)
	foundContent, foundFrame := false, false
	for _, change := range changes {
		foundContent = foundContent || change.Path == "blocks.opening.content"
		foundFrame = foundFrame || change.Path == "sequences.bean.frames"
	}
	if !foundContent || !foundFrame {
		t.Fatalf("changes=%+v", changes)
	}
}

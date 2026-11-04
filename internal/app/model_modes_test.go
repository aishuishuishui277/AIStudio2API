package app

import (
	"reflect"
	"testing"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

func TestModelAliasesDoNotInventAccessOrMutateCatalog(t *testing.T) {
	service := &trackedService{modelMapping: []config.ModelAlias{
		{Alias: "gemini-3.8-flash-fake", Model: "gemini-3.8-flash"},
		{Alias: "gemini-2.5-pro-fake", Model: "gemini-2.5-pro"},
		{Alias: "gemini-3-pro-preview-fake", Model: "gemini-3-pro-preview"},
		{Alias: "gemini-3.1-flash-lite-fake", Model: "gemini-3.1-flash-lite"},
	}}
	for _, alias := range service.modelMapping {
		service.fakeStreamModels = append(service.fakeStreamModels, alias.Alias)
	}
	for _, target := range []string{"gemini-3.8-flash", "gemini-2.5-pro", "gemini-3-pro-preview", "gemini-3.1-flash-lite"} {
		original := aistudio.Model{ID: target, Name: "real", Methods: []string{"generateContent", "streamGenerateContent"}, InputTokenLimit: 1000000, OutputTokenLimit: 8192, Paid: true, Capabilities: map[string]bool{"chat_model": true}}
		catalog := []aistudio.Model{original}
		public := service.withAliases(catalog)
		if len(public) != 2 || public[0].ID != target || public[1].ID != target+"-fake" {
			t.Fatalf("wrong public IDs for %s: %v", target, public)
		}
		alias := public[1]
		alias.ID = target
		if !reflect.DeepEqual(alias, original) {
			t.Fatal("alias lost upstream capabilities or access metadata")
		}
		if !reflect.DeepEqual(catalog, []aistudio.Model{original}) {
			t.Fatal("real catalog was mutated")
		}
		if got := service.mappedModel("models/" + target + "-fake"); got != target {
			t.Fatalf("upstream model = %s", got)
		}
		if got := service.mappedModel(target + "-fake(low)"); got != target+"(low)" {
			t.Fatalf("thinking suffix = %s", got)
		}
	}
	if got := service.withAliases(nil); len(got) != 0 {
		t.Fatal("aliases invented an unavailable model")
	}
	latest := []aistudio.Model{{ID: "gemini-pro-latest", CapabilityOptions: map[string][]string{"aliases": {"gemini-2.5-pro", "gemini-3-pro-preview"}}}}
	if got := service.withAliases(latest); len(got) != 1 {
		t.Fatal("capability synonyms invented fake variants for absent real model IDs")
	}
}

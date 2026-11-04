package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFakeStreamModelsConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	cfg := Default()
	cfg.FakeStreamModels = "gemini-3.8-flash-fake,models/gemini-2.5-pro-fake"
	cfg.ModelMapping = "gemini-3.8-flash-fake:gemini-3.8-flash,gemini-2.5-pro-fake:gemini-2.5-pro"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FakeStream || loaded.FakeStreamModels != cfg.FakeStreamModels || loaded.ModelMapping != cfg.ModelMapping {
		t.Fatal("model modes did not survive config save/load")
	}
	if got := ModelList(loaded.FakeStreamModels); !slices.Equal(got, []string{"gemini-3.8-flash-fake", "gemini-2.5-pro-fake"}) {
		t.Fatalf("model list = %v", got)
	}
	t.Setenv("FAKE_STREAM_MODELS", "custom-fake")
	loaded, err = Load(path)
	if err != nil || loaded.FakeStreamModels != "custom-fake" || !slices.Contains(loaded.Environment, "FAKE_STREAM_MODELS") {
		t.Fatal("environment override not applied")
	}
	if err := loaded.Save(path); err != nil {
		t.Fatal(err)
	}
	values, _, err := readEnvFile(path)
	if err != nil || values["FAKE_STREAM_MODELS"] != cfg.FakeStreamModels {
		t.Fatal("environment override overwrote saved selection")
	}
}

func TestAndroidModelModesPreserveExistingDataAndOptOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	original := "# existing settings\r\nPROXY=http://127.0.0.1:7890\r\nPROXY_API_KEY=test-only-key\r\nFAKE_STREAM=false\r\nCUSTOM_UNKNOWN=preserve-me\r\nMODEL_MAPPING=custom:gemini-3.8-flash,gemini-3.8-flash-fake:custom-target\r\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAndroidModelModes(path); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".before-model-modes-v1")
	if err != nil || string(backup) != original {
		t.Fatal("private original backup differs")
	}
	data, _ := os.ReadFile(path)
	for _, unchanged := range []string{"PROXY=http://127.0.0.1:7890\r\n", "PROXY_API_KEY=test-only-key\r\n", "FAKE_STREAM=false\r\n", "CUSTOM_UNKNOWN=preserve-me\r\n"} {
		if !strings.Contains(string(data), unchanged) {
			t.Fatal("unrelated configuration changed")
		}
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := ParseModelMapping(cfg.ModelMapping)
	if err != nil {
		t.Fatal(err)
	}
	if aliases[0].Alias != "custom" || aliases[1].Model != "custom-target" {
		t.Fatal("user mappings overwritten")
	}
	if slices.Contains(ModelList(cfg.FakeStreamModels), "gemini-3.8-flash-fake") {
		t.Fatal("conflicting user alias silently changed mode")
	}
	for _, target := range []string{"gemini-2.5-pro", "gemini-3-pro-preview", "gemini-3.1-flash-lite"} {
		if !slices.Contains(ModelList(cfg.FakeStreamModels), target+"-fake") {
			t.Fatalf("missing main model seed %s", target)
		}
	}
	if err := EnsureAndroidModelModes(path); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != string(data) {
		t.Fatal("migration is not idempotent")
	}
	cfg.FakeStreamModels = ""
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAndroidModelModes(path); err != nil {
		t.Fatal(err)
	}
	cleared, err := Load(path)
	if err != nil || cleared.FakeStreamModels != "" {
		t.Fatal("explicit opt-out was re-enabled")
	}
}

func TestAndroidModelModesKeepExplicitSelection(t *testing.T) {
	for _, selection := range []string{"", "my-alias"} {
		t.Run(selection, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".env")
			original := "MODEL_MAPPING=my-alias:gemini-3.8-flash\nFAKE_STREAM_MODELS=" + selection + "\n"
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := EnsureAndroidModelModes(path); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(path)
			if string(data) != original {
				t.Fatal("explicit selection changed")
			}
		})
	}
}

package config

import (
	"errors"
	"os"
	"strings"
)

// AndroidFakeStreamTargets seeds ordinary text models, including older main
// models. A seed is only a mapping, never a claim of upstream availability:
// trackedService.withAliases advertises it only if its target is in the real
// account-filtered catalog. Users can extend MODEL_MAPPING to any other model.
func AndroidFakeStreamTargets() []string {
	return []string{
		"gemini-3.8-flash", "gemini-3.6-flash", "gemini-3.5-flash-lite",
		"gemini-3.1-pro-preview", "gemini-3.1-flash", "gemini-3.1-flash-lite",
		"gemini-3-pro-preview", "gemini-3-flash-preview", "gemini-3.0-pro",
		"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite",
		"gemini-pro-latest", "gemini-flash-latest", "gemini-flash-lite-latest",
		"gemini-4-argon", "gemma-4-26b-a4b-it", "gemma-4-31b-it",
	}
}

// EnsureAndroidModelModes is a one-time, Android-entry-only migration. It
// changes only MODEL_MAPPING/FAKE_STREAM_MODELS and keeps a private original.
// A marker lets users subsequently clear the defaults without re-enabling them.
func EnsureAndroidModelModes(path string) error {
	marker := path + ".android-model-modes-v1"
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	values, _, err := readEnvFile(path)
	if err != nil {
		return err
	}
	if _, configured := values["FAKE_STREAM_MODELS"]; configured {
		return atomicWrite(marker, []byte("1\n"), 0o600)
	}
	mapping, err := ParseModelMapping(values["MODEL_MAPPING"])
	if err != nil {
		return err
	}
	used := make(map[string]string)
	for _, alias := range mapping {
		if _, exists := used[alias.Alias]; !exists {
			used[alias.Alias] = alias.Model
		}
	}
	merged := strings.TrimSpace(values["MODEL_MAPPING"])
	var fake []string
	for _, target := range AndroidFakeStreamTargets() {
		alias := target + "-fake"
		if previous, exists := used[alias]; exists {
			if previous == target {
				fake = append(fake, alias)
			}
			continue
		}
		if merged != "" {
			merged += ","
		}
		merged += alias + ":" + target
		fake = append(fake, alias)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backup, err := os.OpenFile(path+".before-model-modes-v1", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, writeErr := backup.Write(original)
		if err := errors.Join(writeErr, backup.Close()); err != nil {
			return err
		}
	} else if !os.IsExist(err) {
		return err
	}
	var output strings.Builder
	for _, line := range strings.SplitAfter(string(original), "\n") {
		key, _, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(key) == "MODEL_MAPPING" {
			continue
		}
		output.WriteString(line)
	}
	if output.Len() > 0 && !strings.HasSuffix(output.String(), "\n") {
		output.WriteByte('\n')
	}
	output.WriteString("# Android model modes: aliases are advertised only for available targets.\n")
	output.WriteString("MODEL_MAPPING=" + formatEnvValue(merged) + "\n")
	output.WriteString("FAKE_STREAM_MODELS=" + formatEnvValue(strings.Join(fake, ",")) + "\n")
	if err := atomicWrite(path, []byte(output.String()), 0o600); err != nil {
		return err
	}
	return atomicWrite(marker, []byte("1\n"), 0o600)
}

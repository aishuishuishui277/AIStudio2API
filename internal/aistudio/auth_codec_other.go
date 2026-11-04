//go:build !android

package aistudio

func encodeStoredState(data []byte) ([]byte, error) { return data, nil }
func decodeStoredState(data []byte) ([]byte, error) { return data, nil }

//go:build android && authpoc

package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/api"
	"github.com/Mag1cFall/AIStudio2API/internal/chromeauth"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
	"github.com/Mag1cFall/AIStudio2API/internal/i18n"
)

// RunAuthProbe is compiled only for the isolated :pgprobe process. All errors
// cross JNI as fixed classifications, never as upstream text or Cookie data.
func RunAuthProbe(directory string, key, raw []byte) (result map[string]any) {
	return runPGProbe(directory, key, raw, false, true)
}

// RunSavedAccountProbe never calls the cookie parser/importer or GeckoView.
func RunSavedAccountProbe(directory string, key []byte, generate bool) map[string]any {
	return runPGProbe(directory, key, nil, true, generate)
}

type pgProbeOptions struct {
	context         context.Context
	runDirectory    string
	expectedAccount string
}

func runPGProbe(directory string, key, raw []byte, restore, generate bool, options ...pgProbeOptions) (result map[string]any) {
	var option pgProbeOptions
	if len(options) > 0 {
		option = options[0]
	}
	parent := option.context
	if parent == nil {
		parent = context.Background()
	}
	started := time.Now()
	result = map[string]any{"stage": "input", "success": false, "real_upstream": true, "saved_account_restore": restore}
	defer func() {
		result["elapsed_ms"] = time.Since(started).Milliseconds()
		classifyProbeResult(result)
	}()
	defer clear(raw)
	defer clear(key)
	defer func() {
		if recover() != nil {
			result["error"] = "INTERNAL_FAILURE"
			result["success"] = false
		}
	}()
	i18n.Stdout, i18n.Stderr = io.Discard, io.Discard
	log.SetOutput(io.Discard)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(parent, 180*time.Second)
	defer cancel()
	fail := func(err error) map[string]any {
		result["error"] = probeError(err)
		result["category"] = classifyProbeError(err)
		types := []string{}
		for next := err; next != nil && len(types) < 8; next = errors.Unwrap(next) {
			types = append(types, reflect.TypeOf(next).String())
		}
		result["error_types"] = types
		return result
	}
	if !filepath.IsAbs(directory) || len(key) != 32 || len(raw) > 256*1024 {
		result["error"] = "INVALID_INPUT"
		return
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fail(err)
	}
	var runDir string
	var err error
	if option.runDirectory != "" {
		runDir = option.runDirectory
	} else if restore {
		runDir = filepath.Join(directory, "pg-auth-3b1")
		err = os.MkdirAll(runDir, 0700)
	} else {
		runDir, err = os.MkdirTemp(directory, "run-")
	}
	if err != nil {
		return fail(err)
	}
	if err = os.Chdir(runDir); err != nil {
		return fail(err)
	}
	if err = os.MkdirAll("tmp", 0700); err != nil {
		return fail(err)
	}
	os.Setenv("TMPDIR", filepath.Join(runDir, "tmp"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(runDir, "cache"))
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(runDir, "config"))
	if err = aistudio.SetAndroidStorageKey(key); err != nil {
		return fail(err)
	}
	var state aistudio.StorageState
	if restore {
		result["stage"] = "saved_state_loading"
		if err = prepareSavedAccount(directory, runDir, result); err != nil {
			return fail(err)
		}
		result["keystore_restored"] = true
	} else {
		// The existing parser discards unknown partition fields. Reject them first.
		var document struct {
			Cookies []struct {
				Name             string          `json:"name"`
				Value            string          `json:"value"`
				Domain           string          `json:"domain"`
				Path             string          `json:"path"`
				HostOnly         bool            `json:"hostOnly"`
				Secure           bool            `json:"secure"`
				HTTPOnly         bool            `json:"httpOnly"`
				SameSite         string          `json:"sameSite"`
				Session          bool            `json:"session"`
				Expires          float64         `json:"expires"`
				PartitionKey     json.RawMessage `json:"partitionKey"`
				FirstPartyDomain string          `json:"firstPartyDomain"`
			} `json:"cookies"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&document); err != nil {
			result["error"] = "COOKIE_SCHEMA_INVALID"
			return
		}
		if len(document.Cookies) == 0 {
			result["error"] = "COOKIE_FIELDS_MISSING"
			return
		}
		for _, c := range document.Cookies {
			if len(c.PartitionKey) > 0 && string(c.PartitionKey) != "null" || c.FirstPartyDomain != "" {
				result["error"] = "PARTITION_UNSUPPORTED"
				return
			}
			if c.Domain != ".google.com" && c.Domain != "google.com" && c.Domain != "accounts.google.com" && c.Domain != ".accounts.google.com" && c.Domain != "aistudio.google.com" && c.Domain != ".aistudio.google.com" {
				result["error"] = "COOKIE_DOMAIN_INVALID"
				return
			}
			if c.Name == "" || c.Value == "" || !strings.HasPrefix(c.Path, "/") {
				result["error"] = "COOKIE_FIELDS_MISSING"
				return
			}
			if c.HostOnly && strings.HasPrefix(c.Domain, ".") {
				result["error"] = "COOKIE_HOST_ONLY_CONFLICT"
				return
			}
			if c.SameSite != "" && c.SameSite != "None" && c.SameSite != "Lax" && c.SameSite != "Strict" {
				result["error"] = "COOKIE_SAMESITE_INVALID"
				return
			}
			if !c.Session && c.Expires <= float64(time.Now().Unix()) {
				result["error"] = "COOKIE_EXPIRED"
				return
			}
			if c.Session && c.Expires != -1 {
				result["error"] = "COOKIE_EXPIRY_INVALID"
				return
			}
		}
		state, err = aistudio.ParseCookieText(string(raw))
		if err != nil {
			return fail(err)
		}
		result["format_converted"] = true
		result["cookie_count"] = len(state.Cookies)
		if _, err = aistudio.NewSigner().Authorization(state); err != nil {
			return fail(err)
		}
		result["core_accepted"] = true
	}
	var savedBefore [32]byte
	if restore {
		saved, loadErr := aistudio.NewAccountStore(filepath.Join(runDir, "auth")).Load()
		if loadErr != nil || len(saved) != 1 {
			return fail(errSavedStateInvalid)
		}
		savedBefore = savedCookieDigest(saved[0].StorageState)
	}
	result["stage"] = "runtime_initialization"
	cfg := config.Default()
	cfg.AuthStates = filepath.Join(runDir, "auth")
	cfg.WAABackend = config.WAABackendSpiderMonkey
	cfg.Proxy = "" // Use the existing per-app VPN route; no proxy or TLS changes.
	cfg.UpstreamChannels = []string{"playground"}
	cfg.LogFile = false
	cfg.RequestBodyLog = false
	cfg.WarmWorkerLimit = 1
	cfg.MaxActiveWorkers = 1
	cfg.WarmStartupConcurrency = 1
	cfg.InitTimeout = 60 * time.Second
	cfg.RequestTimeout = 60 * time.Second
	cfg.FirstEventHedge = "off"
	requests := newRequestRegistry(ctx)
	requests.silent = true
	service, admin, closeRuntime, err := newRuntime(ctx, ctx, cfg, requests)
	if err != nil {
		return fail(err)
	}
	var closeOnce sync.Once
	closeProbe := func() { closeOnce.Do(func() { cancel(); closeRuntime() }) }
	defer closeProbe()
	if !restore {
		result["stage"] = "account_identification_and_import"
		account, err := admin.ImportAccountState(ctx, api.AccountStateInput{StorageState: state, AuthUser: 0})
		if err != nil {
			return fail(err)
		}
		if option.expectedAccount != "" && account.ID != option.expectedAccount {
			result["error"] = "ACCOUNT_MISMATCH"
			result["same_account"] = false
			return
		}
		if option.expectedAccount != "" {
			result["same_account"] = true
		}
		result["account_identified"] = account.ID != "" // Never return account email.
		result["encrypted_state_saved"] = true
		catalog, _ := admin.CatalogModels(ctx)
		catalogIDs := []string{}
		for _, m := range catalog {
			catalogIDs = append(catalogIDs, m.ID)
		}
		result["models_after_import"] = catalogIDs
	} else {
		loaded, loadErr := admin.Accounts(ctx)
		if loadErr != nil {
			return fail(loadErr)
		}
		if len(loaded) != 1 {
			return fail(errSavedAuthRequired)
		}
		result["account_identified"] = loaded[0].ID != ""
		result["core_accepted"] = true
	}
	result["stage"] = "model_catalog_and_waa"
	models, _, err := service.Start(ctx, func() {})
	if err != nil {
		return fail(err)
	}
	result["waa_initialized"] = true
	ids := make([]string, 0, len(models))
	found := false
	for _, m := range models {
		ids = append(ids, m.ID)
		if m.ID == "gemini-3.8-flash" {
			found = true
		}
	}
	result["models"] = ids
	result["model_count"] = len(ids)
	result["model_present"] = found
	if !found {
		result["error"] = "MODEL_NOT_AVAILABLE"
		return
	}
	if restore {
		statuses, statusErr := admin.Accounts(ctx)
		if statusErr != nil {
			return fail(statusErr)
		}
		ready := len(statuses) == 1 && statuses[0].State == "ready"
		result["account_ready"] = ready
		if !ready {
			if len(statuses) == 1 && statuses[0].State == "auth_required" {
				return fail(errSavedAuthRequired)
			}
			result["error"] = "ACCOUNT_NOT_READY"
			return
		}
		defer func() {
			// Close happens before this audit, so last Set-Cookie writes have settled.
			closeProbe()
			auditSavedAccount(runDir, savedBefore, result)
		}()
		if !generate {
			result["success"] = true
			result["stage"] = "ready_check_complete"
			result["generation_requested"] = false
			return
		}
	}
	if !generate {
		statuses, e := admin.Accounts(ctx)
		if e != nil {
			return fail(e)
		}
		if len(statuses) != 1 || statuses[0].State != "ready" {
			result["error"] = "ACCOUNT_NOT_READY"
			return
		}
		result["account_ready"] = true
		result["success"] = true
		result["generation_requested"] = false
		result["stage"] = "candidate_validated"
		return
	}
	result["generation_requested"] = true
	result["stage"] = "single_generation"
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fail(err)
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		listener.Close()
		return fail(err)
	}
	token := hex.EncodeToString(secret)
	clear(secret)
	handler := api.NewHandler(service, api.Config{APIKey: token})
	// Only expose generation and models. No admin/account export routes.
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go server.Serve(listener)
	defer server.Close()
	result["listen_address"] = listener.Addr().String()
	body := `{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"Reply with exactly OK."}],"stream":false,"max_tokens":2048,"reasoning_effort":"minimal"}`
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://"+listener.Addr().String()+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 90 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer response.Body.Close()
	result["http_status"] = response.StatusCode
	encoded, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return fail(err)
	}
	defer clear(encoded)
	if response.StatusCode != 200 {
		result["error"] = classifyProbeResponse(response.StatusCode, encoded)
		result["upstream_diagnostic"] = savedErrorDiagnostic(encoded)
		result["category"] = classifyProbeResponse(response.StatusCode, encoded)
		return
	}
	var completion struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	decodeErr := json.Unmarshal(encoded, &completion)
	result["json_valid"] = decodeErr == nil
	result["choices_count"] = len(completion.Choices)
	if len(completion.Choices) > 0 {
		finish := completion.Choices[0].FinishReason
		switch finish {
		case "stop", "length", "content_filter", "tool_calls", "function_call":
			result["finish_reason"] = finish
		default:
			result["finish_reason"] = "other"
		}
	}
	if decodeErr != nil || len(completion.Choices) == 0 || completion.Choices[0].FinishReason != "stop" || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		result["error"] = "EMPTY_OR_INVALID_GENERATION"
		return
	}
	reply := strings.TrimSpace(completion.Choices[0].Message.Content)
	result["valid_body"] = true
	result["reply_bytes"] = len(reply)
	if reply == "OK" {
		result["reply"] = "OK"
	} // Whitelist expected output only.
	result["success"] = true
	result["stage"] = "complete"
	return
}

func probeError(err error) string {
	if errors.Is(err, errSavedAuthRequired) {
		return "AUTH_REQUIRED"
	}
	if errors.Is(err, errSavedSourceAmbiguous) {
		return "SAVED_SOURCE_AMBIGUOUS"
	}
	if errors.Is(err, errSavedStateInvalid) {
		return "SAVED_STATE_INVALID"
	}
	if errors.Is(err, aistudio.ErrLoginCookieMissing) {
		return "COOKIE_FIELDS_MISSING"
	}
	if errors.Is(err, chromeauth.ErrSignedOut) {
		return "GOOGLE_SESSION_INVALID"
	}
	if errors.Is(err, os.ErrPermission) {
		return "PRIVATE_STORAGE_PERMISSION"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "PRIVATE_STORAGE_MISSING"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "PRIVATE_STORAGE_FAILURE"
	}
	return classifyProbeError(err)
}

// Finite vocabulary only; never forward arbitrary server text, codes, or headers.
func savedErrorDiagnostic(body []byte) map[string]any {
	var failure struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	output := map[string]any{"json_error": json.Unmarshal(body, &failure) == nil}
	switch failure.Error.Code {
	case "account_unavailable", "account_required", "upstream_error", "rate_limit_exceeded", "model_not_found", "invalid_request":
		output["code"] = failure.Error.Code
	default:
		output["code"] = "other"
	}
	message := strings.ToLower(failure.Error.Message)
	flags := []string{}
	for _, item := range []struct {
		flag     string
		patterns []string
	}{
		{"region_restriction", []string{"region not supported"}},
		{"account_restriction", []string{"account not supported"}},
		{"permission_denied", []string{"permission_denied", "permission denied", "does not have permission"}},
		{"authentication", []string{"unauthenticated", "signed_out", "invalid authentication", "expired"}},
		{"bot_challenge", []string{"botguard", "recaptcha", "unusual traffic", "suspicious", "waa"}},
		{"drive", []string{"drive", "generateaccesstoken"}},
		{"generate_content", []string{"generatecontent"}},
		{"generate_interaction", []string{"generateinteraction"}},
		{"oauth", []string{"oauth"}},
		{"quota", []string{"quota", "resource_exhausted"}},
	} {
		for _, pattern := range item.patterns {
			if strings.Contains(message, pattern) {
				flags = append(flags, item.flag)
				break
			}
		}
	}
	output["categories"] = flags
	return output
}

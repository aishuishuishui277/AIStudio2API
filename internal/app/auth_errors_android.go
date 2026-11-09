//go:build android && authpoc

package app

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/authpoc"
	"github.com/Mag1cFall/AIStudio2API/internal/camoufoxnative"
	"github.com/Mag1cFall/AIStudio2API/internal/chromeauth"
)

func classifyProbeError(err error) string {
	e := authpoc.Evidence{SignedOut: errors.Is(err, chromeauth.ErrSignedOut) || errors.Is(err, camoufoxnative.ErrAuthenticationRequired), MissingLoginCookie: errors.Is(err, aistudio.ErrLoginCookieMissing) || errors.Is(err, errSavedAuthRequired)}
	var admin *adminOperationError
	if errors.As(err, &admin) && admin.code == "signed_out" {
		e.SignedOut = true
	}
	var path *os.PathError
	var network net.Error
	// os.PathError also implements net.Error; filesystem failures are not network evidence.
	if !errors.As(err, &path) && errors.As(err, &network) {
		e.Network = true
	}
	var rpc *aistudio.RPCError
	if errors.As(err, &rpc) {
		e.HTTPStatus = rpc.StatusCode
		e.AccountRestricted = rpc.Code == 7 && rpc.Message == "Account not supported."
		e.Quota = rpc.Code == 8
	}
	// Only concrete runtime failure phrases, not arbitrary permission/403 strings.
	message := strings.ToLower(err.Error())
	e.WAA = strings.Contains(message, "spidermonkey") || strings.Contains(message, "初始化 waa") || strings.Contains(message, "生成 fresh waa proof")
	return authpoc.Classify(e)
}

func classifyProbeResponse(status int, raw []byte) string {
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	e := authpoc.Evidence{HTTPStatus: status}
	if json.Unmarshal(raw, &body) == nil {
		e.SignedOut = body.Error.Code == "signed_out"
		e.Quota = body.Error.Code == "rate_limit_exceeded"
		// This exact PG RPC phrase names an account restriction, not an expired login.
		e.AccountRestricted = strings.Contains(body.Error.Message, "协议错误码 7: Account not supported.")
	}
	return authpoc.Classify(e)
}

func classifyProbeResult(result map[string]any) {
	if result["success"] == true {
		result["needs_reauthentication"] = false
		return
	}
	if _, exists := result["category"]; !exists {
		category := "UNKNOWN_ERROR"
		if result["error"] == "AUTH_REQUIRED" || result["error"] == "COOKIE_FIELDS_MISSING" || result["error"] == "GOOGLE_SESSION_INVALID" {
			category = "AUTH_REQUIRED"
		}
		result["category"] = category
	}
	category, _ := result["category"].(string)
	result["needs_reauthentication"] = authpoc.NeedsLogin(category)
	// Categories are hints only. Never erase credentials or fetch browser cookies here.
}

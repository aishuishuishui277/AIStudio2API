// Package authpoc contains only the isolated experiment's offline policies.
package authpoc

type Evidence struct {
	SignedOut, MissingLoginCookie, Network, AccountRestricted, Quota, WAA bool
	HTTPStatus                                                            int
}

// An HTTP status alone never proves that a Google login session has expired.
func Classify(e Evidence) string {
	switch {
	case e.SignedOut || e.MissingLoginCookie:
		return "AUTH_REQUIRED"
	case e.Network:
		return "NETWORK_FAILURE"
	case e.AccountRestricted:
		return "ACCOUNT_RESTRICTED"
	case e.Quota || e.HTTPStatus == 429:
		return "QUOTA_EXCEEDED"
	case e.WAA:
		return "WAA_FAILURE"
	case e.HTTPStatus == 403:
		return "UPSTREAM_FORBIDDEN"
	default:
		return "UNKNOWN_ERROR"
	}
}

// NeedsLogin is a UI hint, not a request to erase, overwrite, or auto-import state.
func NeedsLogin(category string) bool { return category == "AUTH_REQUIRED" }

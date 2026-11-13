package handlers

import (
	"net/http"

	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

// AndroidAssetLinks lets the Android app use passkeys for this domain (Digital Asset Links).
func (h *Handler) AndroidAssetLinks(w http.ResponseWriter, r *http.Request) {
	if h.mobile.AndroidPackage == "" || len(h.mobile.AndroidCertFingerprints) == 0 {
		http.NotFound(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, []map[string]any{{
		"relation": []string{
			"delegate_permission/common.handle_all_urls",
			"delegate_permission/common.get_login_creds",
		},
		"target": map[string]any{
			"namespace":                "android_app",
			"package_name":             h.mobile.AndroidPackage,
			"sha256_cert_fingerprints": h.mobile.AndroidCertFingerprints,
		},
	}})
}

// AppleAppSiteAssociation lets the iOS app use passkeys for this domain (webcredentials).
func (h *Handler) AppleAppSiteAssociation(w http.ResponseWriter, r *http.Request) {
	if len(h.mobile.IOSAppIDs) == 0 {
		http.NotFound(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"webcredentials": map[string]any{"apps": h.mobile.IOSAppIDs},
	})
}

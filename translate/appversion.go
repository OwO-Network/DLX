/*
 * @Author: Vincent Young
 * @Date: 2026-10-08 00:00:00
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2026-10-08 00:00:00
 * @FilePath: /DLX/translate/appversion.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package translate

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DeepL's iOS app on the US App Store. Apple's public lookup endpoint is
// key-less and returns, in results[0].version, the marketing version the iOS
// client reports as CFBundleShortVersionString in its User-Agent and
// app_information.app_version.
//
// DeepL rate-limits clients whose iOS TLS fingerprint and advertised app
// version are inconsistent (see OwO-Network/DLX#236): once the pinned version
// goes stale, *every* request is answered with HTTP 429. Resolving the version
// at runtime keeps that profile coherent without a rebuild.
const (
	deeplAppStoreTrackID  = 1552407475
	deeplAppStoreBundleID = "com.linguee.DeepLMobileTranslator"
	deeplAppStoreSeller   = "DeepL SE"
)

// deeplAppStoreLookupURL is a var rather than a const so tests can point it at
// a local server.
var deeplAppStoreLookupURL = fmt.Sprintf(
	"https://itunes.apple.com/lookup?id=%d&country=us", deeplAppStoreTrackID,
)

// Lookup policy, mirroring the approach of tisfeng/Easydict#1341. The App Store
// is only contacted lazily, on the first translation after the cache expires —
// never at startup — so an unreachable App Store cannot delay or prevent the
// process from serving.
const (
	appVersionCacheTTL      = 24 * time.Hour
	appVersionRetryBackoff  = 15 * time.Minute
	appVersionLookupTimeout = 3 * time.Second
)

// appVersionHTTPClient is shared so repeat lookups reuse the connection pool.
// It is separate from the translation client: no TLS fingerprint, no cookies,
// no proxy.
var appVersionHTTPClient = &http.Client{Timeout: appVersionLookupTimeout}

// appVersionProvider resolves and caches the DeepL iOS version this client
// advertises. The zero value is ready to use.
type appVersionProvider struct {
	mu          sync.Mutex
	version     string    // last successfully resolved version, "" if none
	fetchedAt   time.Time // when version was resolved
	attemptedAt time.Time // when a lookup was last started
}

var iosAppVersionProvider appVersionProvider

// currentIOSAppVersion returns the DeepL iOS marketing version to advertise in
// both the User-Agent and app_information. A successful lookup is cached for
// appVersionCacheTTL; a failed one is not retried for appVersionRetryBackoff.
// Any failure falls back to the last resolved version, or to the pinned
// iosAppVersion if none was ever resolved, so behaviour degrades to the
// previous hardcoded value instead of failing.
func currentIOSAppVersion() string {
	p := &iosAppVersionProvider
	// The lock is held across the lookup so a burst of cold requests performs
	// at most one App Store query; callers block for no longer than
	// appVersionLookupTimeout and then share the result.
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if isValidAppVersion(p.version) && now.Sub(p.fetchedAt) < appVersionCacheTTL {
		return p.version
	}
	if !p.attemptedAt.IsZero() && now.Sub(p.attemptedAt) < appVersionRetryBackoff {
		return p.cachedOrFallback()
	}

	p.attemptedAt = now
	version, err := fetchLatestIOSAppVersion()
	if err == nil && !isValidAppVersion(version) {
		err = fmt.Errorf("implausible version %q", version)
	}
	if err != nil {
		fallback := p.cachedOrFallback()
		log.Printf("DeepL iOS app version lookup failed, falling back to %s: %v", fallback, err)
		return fallback
	}
	p.version = version
	p.fetchedAt = now
	return version
}

// cachedOrFallback prefers a previously resolved version over the pinned
// constant: a version that was current recently is a better guess than one
// hardcoded at build time.
func (p *appVersionProvider) cachedOrFallback() string {
	if isValidAppVersion(p.version) {
		return p.version
	}
	return iosAppVersion
}

// appStoreLookup is the subset of Apple's lookup response we consume.
type appStoreLookup struct {
	ResultCount int              `json:"resultCount"`
	Results     []appStoreResult `json:"results"`
}

type appStoreResult struct {
	TrackID    int64  `json:"trackId"`
	BundleID   string `json:"bundleId"`
	ArtistName string `json:"artistName"`
	Version    string `json:"version"`
}

// fetchLatestIOSAppVersion queries Apple's lookup API for the DeepL iOS app.
func fetchLatestIOSAppVersion() (string, error) {
	resp, err := appVersionHTTPClient.Get(deeplAppStoreLookupURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("app store lookup: unexpected status %d", resp.StatusCode)
	}
	// Cap the read so a misbehaving endpoint cannot make us allocate without
	// bound.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var lookup appStoreLookup
	if err := json.Unmarshal(body, &lookup); err != nil {
		return "", fmt.Errorf("app store lookup: %w", err)
	}
	return lookup.deepLVersion()
}

// deepLVersion validates that the lookup result really is the DeepL iOS app
// and returns its marketing version. A wrong or unexpected answer is rejected
// rather than advertised, which keeps the fallback path honest.
func (l appStoreLookup) deepLVersion() (string, error) {
	if l.ResultCount != 1 || len(l.Results) != 1 {
		return "", fmt.Errorf("app store lookup: expected 1 result, got %d", len(l.Results))
	}
	r := l.Results[0]
	if r.TrackID != deeplAppStoreTrackID || r.BundleID != deeplAppStoreBundleID || r.ArtistName != deeplAppStoreSeller {
		return "", fmt.Errorf("app store lookup: unexpected app %d/%q/%q", r.TrackID, r.BundleID, r.ArtistName)
	}
	if !isValidAppVersion(r.Version) {
		return "", fmt.Errorf("app store lookup: implausible version %q", r.Version)
	}
	return r.Version, nil
}

// isValidAppVersion accepts the dotted numeric forms DeepL ships ("26.42",
// "26.52.1", "1.2.3.4") and rejects everything else.
func isValidAppVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

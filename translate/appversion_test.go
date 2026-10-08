/*
 * @Author: Vincent Young
 * @Date: 2026-10-08 00:00:00
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2026-10-08 00:00:00
 * @FilePath: /DLX/translate/appversion_test.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestIsValidAppVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"26.42", true},
		{"26.53", true},
		{"26.52.1", true},
		{"1.2.3.4", true},
		{"", false},
		{"26", false},
		{"26.42.1.2.3", false},
		{"26..42", false},
		{".26.42", false},
		{"26.42.", false},
		{"26.42-beta", false},
		{"v26.42", false},
		{"26.4 2", false},
	}
	for _, tt := range tests {
		if got := isValidAppVersion(tt.version); got != tt.want {
			t.Errorf("isValidAppVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestAppStoreLookupDeepLVersion(t *testing.T) {
	result := func(trackID int64, bundleID, artist, version string) appStoreResult {
		return appStoreResult{TrackID: trackID, BundleID: bundleID, ArtistName: artist, Version: version}
	}
	valid := result(deeplAppStoreTrackID, deeplAppStoreBundleID, deeplAppStoreSeller, "26.53")

	tests := []struct {
		name    string
		lookup  appStoreLookup
		want    string
		wantErr bool
	}{
		{"valid", appStoreLookup{ResultCount: 1, Results: []appStoreResult{valid}}, "26.53", false},
		{"no results", appStoreLookup{ResultCount: 0}, "", true},
		{"two results", appStoreLookup{ResultCount: 2, Results: []appStoreResult{valid, valid}}, "", true},
		{"wrong track id", appStoreLookup{ResultCount: 1, Results: []appStoreResult{result(1, deeplAppStoreBundleID, deeplAppStoreSeller, "26.53")}}, "", true},
		{"wrong bundle", appStoreLookup{ResultCount: 1, Results: []appStoreResult{result(deeplAppStoreTrackID, "com.example.app", deeplAppStoreSeller, "26.53")}}, "", true},
		{"wrong seller", appStoreLookup{ResultCount: 1, Results: []appStoreResult{result(deeplAppStoreTrackID, deeplAppStoreBundleID, "Someone Else", "26.53")}}, "", true},
		{"implausible version", appStoreLookup{ResultCount: 1, Results: []appStoreResult{result(deeplAppStoreTrackID, deeplAppStoreBundleID, deeplAppStoreSeller, "latest")}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.lookup.deepLVersion()
			if (err != nil) != tt.wantErr {
				t.Fatalf("deepLVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("deepLVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// lookupServer serves an App Store lookup response with the given version and
// counts how many times it was queried.
func lookupServer(t *testing.T, version string, status int) (*httptest.Server, func() int) {
	t.Helper()
	var (
		mu       sync.Mutex
		requests int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			return
		}
		_ = json.NewEncoder(w).Encode(appStoreLookup{
			ResultCount: 1,
			Results: []appStoreResult{{
				TrackID:    deeplAppStoreTrackID,
				BundleID:   deeplAppStoreBundleID,
				ArtistName: deeplAppStoreSeller,
				Version:    version,
			}},
		})
	}))
	t.Cleanup(srv.Close)
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return requests
	}
	return srv, count
}

// resetProvider points the resolver at url and clears its cache for the test.
func resetProvider(t *testing.T, url string) {
	t.Helper()
	originalURL := deeplAppStoreLookupURL
	iosAppVersionProvider = appVersionProvider{}
	t.Cleanup(func() {
		deeplAppStoreLookupURL = originalURL
		iosAppVersionProvider = appVersionProvider{}
	})
	deeplAppStoreLookupURL = url
}

func TestCurrentIOSAppVersionUsesLookupAndCaches(t *testing.T) {
	srv, count := lookupServer(t, "26.99", http.StatusOK)
	resetProvider(t, srv.URL)

	for i := 0; i < 3; i++ {
		if got := currentIOSAppVersion(); got != "26.99" {
			t.Fatalf("currentIOSAppVersion() = %q, want %q", got, "26.99")
		}
	}
	if got := count(); got != 1 {
		t.Fatalf("lookup served %d requests, want 1 (result must be cached)", got)
	}
}

func TestCurrentIOSAppVersionFallsBackAndBacksOff(t *testing.T) {
	srv, count := lookupServer(t, "", http.StatusInternalServerError)
	resetProvider(t, srv.URL)

	for i := 0; i < 3; i++ {
		if got := currentIOSAppVersion(); got != iosAppVersion {
			t.Fatalf("currentIOSAppVersion() = %q, want fallback %q", got, iosAppVersion)
		}
	}
	if got := count(); got != 1 {
		t.Fatalf("lookup served %d requests, want 1 (failure must back off)", got)
	}
}

func TestCurrentIOSAppVersionFallsBackOnMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	t.Cleanup(srv.Close)
	resetProvider(t, srv.URL)

	if got := currentIOSAppVersion(); got != iosAppVersion {
		t.Fatalf("currentIOSAppVersion() = %q, want fallback %q", got, iosAppVersion)
	}
}

func TestCurrentIOSAppVersionSingleFlight(t *testing.T) {
	srv, count := lookupServer(t, "26.99", http.StatusOK)
	resetProvider(t, srv.URL)

	const goroutines = 16
	results := make([]string, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = currentIOSAppVersion()
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		if got != "26.99" {
			t.Fatalf("goroutine %d got %q, want %q", i, got, "26.99")
		}
	}
	if got := count(); got != 1 {
		t.Fatalf("lookup served %d requests, want 1 (concurrent callers must share one lookup)", got)
	}
}

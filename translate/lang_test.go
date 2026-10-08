/*
 * @Author: Vincent Young
 * @Date: 2026-10-08 00:00:00
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2026-10-08 00:00:00
 * @FilePath: /DLX/translate/lang_test.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package translate

import "testing"

func TestResolveSourceLang(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"auto", ""},
		{"AUTO", ""},
		{"EN", "en"},
		{"en-us", "en-US"},
		{"PT", "pt"},
		{"PT-BR", "pt-BR"},
		// DeepL only accepts the generic "zh" as a source hint; the
		// script variants are target-only (#239).
		{"ZH", "zh"},
		{"zh", "zh"},
		{"ZH-HANS", "zh"},
		{"zh-hant", "zh"},
		{"JA", "ja"},
		{"ES-419", "es-419"},
		{"DE-CH", "de-CH"},
	}
	for _, c := range cases {
		got, err := resolveSourceLang(c.in)
		if err != nil {
			t.Errorf("resolveSourceLang(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveSourceLang(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := resolveSourceLang("xx"); err == nil {
		t.Errorf("resolveSourceLang(%q) expected error", "xx")
	}
}

func TestResolveTargetLang(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"EN", "en-US"},
		{"en-gb", "en-GB"},
		{"PT", "pt-BR"},
		{"ZH", "zh-Hans"},
		{"ZH-HANS", "zh-Hans"},
		{"zh-hant", "zh-Hant"},
	}
	for _, c := range cases {
		got, err := resolveTargetLang(c.in)
		if err != nil {
			t.Errorf("resolveTargetLang(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveTargetLang(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "auto", "xx"} {
		if _, err := resolveTargetLang(bad); err == nil {
			t.Errorf("resolveTargetLang(%q) expected error", bad)
		}
	}
}

/*
 * @Author: Vincent Young
 * @Date: 2024-09-16 11:59:24
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2026-08-04 00:00:00
 * @FilePath: /DLX/translate/types.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package translate

// DLXTranslationResult is the public response shape consumed by the HTTP
// handlers in the service package. The structure predates the migration to
// the iOS oneshot endpoint; Alternatives is now always empty because oneshot
// does not return alternative translations, and ID is synthesized from time.
//
// Data holds one translation per requested text, in request order. The
// handlers render it as a bare string when the caller sent a bare string in
// `text` and as an array of the same length when the caller sent an array.
type DLXTranslationResult struct {
	Code         int      `json:"code"`
	ID           int64    `json:"id"`
	Message      string   `json:"message,omitempty"`
	Data         []string `json:"data"`
	Alternatives []string `json:"alternatives"`
	SourceLang   string   `json:"source_lang"`
	TargetLang   string   `json:"target_lang"`
	Method       string   `json:"method"`
}

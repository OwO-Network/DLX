/*
 * @Author: Vincent Yang
 * @Date: 2023-07-01 21:45:34
 * @LastEditors: Jason Lyu
 * @LastEditTime: 2025-04-08 13:45:00
 * @FilePath: /DLX/service/service.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/OwO-Network/DLX/translate"
)

func authMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Token != "" {
			providedTokenInQuery := c.Query("token")
			providedTokenInHeader := c.GetHeader("Authorization")

			// Compatability with the Bearer token format
			if providedTokenInHeader != "" {
				parts := strings.Split(providedTokenInHeader, " ")
				if len(parts) == 2 {
					if parts[0] == "Bearer" || parts[0] == "DeepL-Auth-Key" {
						providedTokenInHeader = parts[1]
					} else {
						providedTokenInHeader = ""
					}
				} else {
					providedTokenInHeader = ""
				}
			}

			if providedTokenInHeader != cfg.Token && providedTokenInQuery != cfg.Token {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    http.StatusUnauthorized,
					"message": "Invalid access token",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

type PayloadFree struct {
	TransText   string `json:"text"`
	SourceLang  string `json:"source_lang"`
	TargetLang  string `json:"target_lang"`
	TagHandling string `json:"tag_handling"`
}

type PayloadAPI struct {
	Text        []string `json:"text"`
	TargetLang  string   `json:"target_lang"`
	SourceLang  string   `json:"source_lang"`
	TagHandling string   `json:"tag_handling"`
}

// PayloadTranslate is the request body of POST /translate. Unlike PayloadFree
// (shared with the Pro endpoint), `text` accepts either a single string or an
// array of strings.
type PayloadTranslate struct {
	Text        PayloadText `json:"text"`
	SourceLang  string      `json:"source_lang"`
	TargetLang  string      `json:"target_lang"`
	TagHandling string      `json:"tag_handling"`
}

// PayloadText is the `text` field of POST /translate, in either documented
// shape: a bare JSON string or an array of strings. Both are normalized to
// Texts, and Batch records which shape the caller used so the response can
// mirror it. Any other shape (a number, an object, an array holding
// non-strings) fails the payload decode, which the handler reports as
// 400 Invalid request payload; null names no text, like a body without the key.
type PayloadText struct {
	Texts []string
	Batch bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *PayloadText) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var texts []string
		if err := json.Unmarshal(trimmed, &texts); err != nil {
			return err
		}
		p.Texts, p.Batch = texts, true
		return nil
	}

	// A JSON null names no text: leave the zero value, so Texts stays nil.
	if string(trimmed) == "null" {
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	p.Texts, p.Batch = []string{text}, false
	return nil
}

func Router(cfg *Config) *gin.Engine {
	// Set Proxy
	proxyURL := os.Getenv("PROXY")
	if proxyURL == "" {
		proxyURL = cfg.Proxy
	}
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			log.Fatalf("Failed to parse proxy URL: %v", err)
		}
		http.DefaultTransport = &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
	}

	if cfg.Token != "" {
		fmt.Println("Access token is set.")
	}

	r := gin.Default()
	r.Use(cors.Default())

	// Defining the root endpoint which returns the project details
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "DLX Translation API, Developed by sjlleo and missuo. Go to /translate with POST. https://github.com/OwO-Network/DLX",
		})
	})

	// Free API endpoint, No Pro Account required
	r.POST("/translate", authMiddleware(cfg), func(c *gin.Context) {
		req := PayloadTranslate{}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid request payload",
			})
			return
		}

		// A body without `text` never named a text to translate, which is a
		// payload error. `"text": []` names a list that happens to be empty,
		// and is answered with an empty list, so it is not caught here.
		if req.Text.Texts == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid request payload",
			})
			return
		}

		sourceLang := req.SourceLang
		targetLang := req.TargetLang
		texts := req.Text.Texts
		tagHandling := req.TagHandling

		proxyURL := cfg.Proxy

		if tagHandling != "" && tagHandling != "html" && tagHandling != "xml" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid tag_handling value. Allowed values are 'html' and 'xml'.",
			})
			return
		}

		result, err := translate.TranslateByDLX(sourceLang, targetLang, texts, tagHandling, proxyURL, "")
		if err != nil {
			log.Printf("Translation failed: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "Translation failed",
			})
			return
		}

		if result.Code == http.StatusOK {
			response := gin.H{
				"code":         http.StatusOK,
				"id":           result.ID,
				"alternatives": result.Alternatives,
				"source_lang":  result.SourceLang,
				"target_lang":  result.TargetLang,
				"method":       result.Method,
			}
			// Mirror the request: a string in `text` answers with a string
			// in `data`, an array answers with a position-aligned array.
			if req.Text.Batch {
				response["data"] = result.Data
			} else {
				response["data"] = result.Data[0]
			}
			c.JSON(http.StatusOK, response)
		} else {
			c.JSON(result.Code, gin.H{
				"code":    result.Code,
				"message": result.Message,
			})

		}
	})

	// Pro API endpoint, Pro Account required
	r.POST("/v1/translate", authMiddleware(cfg), func(c *gin.Context) {
		req := PayloadFree{}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid request payload",
			})
			return
		}

		sourceLang := req.SourceLang
		targetLang := req.TargetLang
		translateText := req.TransText
		tagHandling := req.TagHandling
		proxyURL := cfg.Proxy

		dlSession := cfg.DlSession

		if tagHandling != "" && tagHandling != "html" && tagHandling != "xml" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid tag_handling value. Allowed values are 'html' and 'xml'.",
			})
			return
		}

		cookie := c.GetHeader("Cookie")
		if cookie != "" {
			dlSession = strings.Replace(cookie, "dl_session=", "", -1)
		}

		if dlSession == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "No dl_session Found",
			})
			return
		} else if strings.Contains(dlSession, ".") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "Your account is not a Pro account. Please upgrade your account or switch to a different account.",
			})
			return
		}

		result, err := translate.TranslateByDLX(sourceLang, targetLang, []string{translateText}, tagHandling, proxyURL, dlSession)
		if err != nil {
			log.Printf("Translation failed: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "Translation failed",
			})
			return
		}

		if result.Code == http.StatusOK {
			c.JSON(http.StatusOK, gin.H{
				"code":         http.StatusOK,
				"id":           result.ID,
				"data":         result.Data[0],
				"alternatives": result.Alternatives,
				"source_lang":  result.SourceLang,
				"target_lang":  result.TargetLang,
				"method":       result.Method,
			})
		} else {
			c.JSON(result.Code, gin.H{
				"code":    result.Code,
				"message": result.Message,
			})

		}
	})

	// Free API endpoint, Consistent with the official API format
	r.POST("/v2/translate", authMiddleware(cfg), func(c *gin.Context) {
		proxyURL := cfg.Proxy

		var translateText string
		var sourceLang string
		var targetLang string

		translateText = c.PostForm("text")
		sourceLang = c.PostForm("source_lang")
		targetLang = c.PostForm("target_lang")

		if translateText == "" || targetLang == "" {
			var jsonData struct {
				Text       []string `json:"text"`
				SourceLang string   `json:"source_lang"`
				TargetLang string   `json:"target_lang"`
			}

			if err := c.BindJSON(&jsonData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "Invalid request payload",
				})
				return
			}

			translateText = strings.Join(jsonData.Text, "\n")
			sourceLang = jsonData.SourceLang
			targetLang = jsonData.TargetLang
		}

		result, err := translate.TranslateByDLX(sourceLang, targetLang, []string{translateText}, "", proxyURL, "")
		if err != nil {
			log.Printf("Translation failed: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "Translation failed",
			})
			return
		}

		if result.Code == http.StatusOK {
			c.JSON(http.StatusOK, gin.H{
				"translations": []map[string]interface{}{
					{
						"detected_source_language": result.SourceLang,
						"text":                     result.Data[0],
					},
				},
			})
		} else {
			c.JSON(result.Code, gin.H{
				"code":    result.Code,
				"message": result.Message,
			})
		}
	})

	// Catch-all route to handle undefined paths
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Path not found",
		})
	})

	return r
}

<!--
 * @Author: Vincent Young
 * @Date: 2022-10-18 07:32:29
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2026-07-08 00:00:00
 * @FilePath: /DLX/README.md
 * @Telegram: https://t.me/missuo
 * 
 * Copyright © 2022 by Vincent, All Rights Reserved. 
-->

# DLX

[![GitHub Workflow][1]](https://github.com/OwO-Network/DLX/actions)
[![Go Version][2]](https://github.com/OwO-Network/DLX/blob/main/go.mod)
[![Go Report][3]](https://goreportcard.com/badge/github.com/OwO-Network/DLX)
[![GitHub License][4]](https://github.com/OwO-Network/DLX/blob/main/LICENSE)
[![Docker Pulls][5]](https://hub.docker.com/r/missuo/deeplx)
[![Releases][6]](https://github.com/OwO-Network/DLX/releases)

[1]: https://img.shields.io/github/actions/workflow/status/OwO-Network/DLX/release.yaml?logo=github
[2]: https://img.shields.io/github/go-mod/go-version/OwO-Network/DLX?logo=go
[3]: https://goreportcard.com/badge/github.com/OwO-Network/DLX
[4]: https://img.shields.io/github/license/OwO-Network/DLX
[5]: https://img.shields.io/docker/pulls/missuo/deeplx?logo=docker
[6]: https://img.shields.io/github/v/release/OwO-Network/DLX?logo=smartthings

> [!IMPORTANT]
> **Disclaimer:** DLX is an independent, open-source project. It is **not** an official DeepL product, and it is **not** affiliated with, endorsed by, or sponsored by DeepL SE in any way. "DeepL" is a registered trademark of DeepL SE. Any reference to DeepL in this repository is made solely to describe interoperability with the DeepL translation service.

## Why was this project renamed?

In July 2026, we received a trademark notice forwarded by GitHub Trust & Safety, submitted on behalf of DeepL SE. The notice stated that this project's former name, "DeepLX", contained the registered trademark "DeepL" and might cause confusion about whether the project is authorized or endorsed by DeepL SE.

It never was, and it never claimed to be. To resolve the matter and remove any possible confusion, we renamed the repository to **DLX** and removed DeepL branding from the project. To state it plainly one more time: **this project is not an official DeepL project and has no relationship with DeepL SE whatsoever.**

DLX is a self-hosted translation API server written in Go. It exposes a simple HTTP API on port `1188`.

## Usage

### Docker

Docker Hub and GHCR image names remain `deeplx` (only the GitHub repository was renamed to DLX):

```bash
docker run -d -p 1188:1188 ghcr.io/owo-network/deeplx:latest
# or: docker run -d -p 1188:1188 missuo/deeplx:latest
```

Or use the provided [`compose.yaml`](compose.yaml):

```bash
docker compose up -d
```

### Binary

Download the binary for your platform from [Releases](https://github.com/OwO-Network/DLX/releases) and run it (artifact names remain `deeplx_*`):

```bash
./deeplx
```

### Translate

`text` accepts either a single string or an array of strings. A string is answered with a string in `data`; an array is answered with a position-aligned array of the same length, and the whole batch travels in a single upstream request.

```bash
# Single text
curl -X POST http://localhost:1188/translate \
  -H "Content-Type: application/json" \
  -d '{"text": "Hello, world!", "source_lang": "EN", "target_lang": "ZH"}'
```

```json
{"alternatives":null,"code":200,"data":"你好，世界！","id":123,"method":"Free","source_lang":"EN","target_lang":"ZH"}
```

```bash
# Batch: one entry per text, answers come back in the same order
curl -X POST http://localhost:1188/translate \
  -H "Content-Type: application/json" \
  -d '{"text": ["Hello, world!", "Good morning"], "source_lang": "EN", "target_lang": "ZH"}'
```

```json
{"alternatives":null,"code":200,"data":["你好，世界！","早上好"],"id":123,"method":"Free","source_lang":"EN","target_lang":"ZH"}
```

Batch limits and failure modes:

- The anonymous limit is **1500 characters for the whole request**, counted in Unicode characters (runes) across every text in the array — not 1500 per text. A batch over that total is rejected with `413` and `text exceeds maximum length: N characters (anonymous oneshot limit is 1500)`, so a large batch has to be chunked by total length rather than by segment count. There is no separate cap on the number of texts.
- A batch is all-or-nothing: if DeepL returns a different number of translations or leaves one of them empty, the request fails with `503 {"code":503,"message":"Translation failed"}` instead of answering with placeholders or a shorter, misaligned array.
- `"text": []`, and any array containing an empty string, are rejected with the same `404 {"code":404,"message":"No text to translate"}` as `"text": ""`.

## Discussion Group
[Telegram Group](https://t.me/+8KDGHKJCxEVkNzll)

## Acknowledgements

### Contributors

<a href="https://github.com/OwO-Network/DLX/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=OwO-Network/DLX&anon=0" />
</a>

## Activity
![Alt](https://repobeats.axiom.co/api/embed/5f473f85db27cb30028a2f3db7a560f3577a4860.svg "Repobeats analytics image")

## License
[MIT](LICENSE)

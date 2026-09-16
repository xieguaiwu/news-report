// sources_tgweb.go — Telegram 公开频道网页预览采集器。
//
// # 为什么用网页预览而不是 Bot API
//
// Bot API 需要走 @BotFather 建 bot 拿 token。而 `https://t.me/s/<channel>` 是
// Telegram 给公开频道提供的**网页预览**，返回完整 HTML：
//
//	· 每条消息一个 `data-post="<channel>/<msgid>"` 锚点
//	· `<time datetime="...">` ISO8601 时间戳
//	· 消息全文、浏览量
//	· `data-before="<msgid>"` 翻页游标
//
// **不需要 bot、不需要账号、不需要 API key**。实测（2026-09-16）：三个公开频道
// 均 http=200，各 20 条消息，字段可完整抽取。
//
// # 与 Bot API 的取舍
//
// | 维度 | 网页预览（本文件） | Bot API |
// |:--|:--|:--|
// | 凭据 | 无 | Bot token（@BotFather） |
// | 覆盖 | 仅**公开**频道 | bot 所在的任意会话 |
// | 历史 | 网页可见范围内（约最近 20 条/页，可 `?before=` 翻） | getUpdates 仅最近 24h |
// | 稳定性 | 依赖 Telegram 不改版式（**会坏**） | 官方契约 |
//
// 结论：只要监控目标都是公开频道，网页预览是更优解——零凭据、能翻历史。
// 需要读私有群时才回退 Bot API（`sources_attention.go`）。
//
// # 纪律
//
// 出网必须经代理（本机 Telegram 直连被阻断）。频道列表由使用者配置，本包不内置。
package crypto

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const tgWebBase = "https://t.me/s/"

// tgWebBrowserUA Telegram 网页预览对非浏览器 UA 也可用，但用浏览器型更稳。
const tgWebBrowserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// TgWebMessage 是从公开频道网页抽出的单条消息。
type TgWebMessage struct {
	Channel string
	MsgID   int64
	At      int64 // UTC 秒
	Text    string
	Views   int
}

// TelegramWebChannel 拉取公开频道最近一页消息。
// channel 传不带 @ 也不带 https://t.me/ 的名字，例如 "whale_alert_io"。
func TelegramWebChannel(ctx context.Context, c *http.Client, channel string) ([]AttentionItem, error) {
	msgs, _, err := telegramWebPage(ctx, c, channel, 0)
	if err != nil {
		return nil, err
	}
	out := make([]AttentionItem, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.toItem())
	}
	return out, nil
}

// TelegramWebChannelBefore 用 `?before=<msgID>` 翻页取更早的消息。
// before=0 表示取最新一页。
func TelegramWebChannelBefore(ctx context.Context, c *http.Client, channel string, before int64) ([]AttentionItem, int64, error) {
	msgs, next, err := telegramWebPage(ctx, c, channel, before)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AttentionItem, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.toItem())
	}
	return out, next, nil
}

func (m TgWebMessage) toItem() AttentionItem {
	return AttentionItem{
		ObservedAt: m.At,
		Symbol:     m.Channel,
		Source:     "telegram_web",
		Kind:       KindAttention,
		Title:      m.Channel,
		Text:       m.Text,
		URL:        fmt.Sprintf("https://t.me/%s/%d", m.Channel, m.MsgID),
		Metrics: map[string]float64{
			"msg_id": float64(m.MsgID),
			"views":  float64(m.Views),
		},
	}
}

func telegramWebPage(ctx context.Context, c *http.Client, channel string, before int64) ([]TgWebMessage, int64, error) {
	channel = strings.TrimPrefix(strings.TrimSpace(channel), "@")
	if channel == "" {
		return nil, 0, fmt.Errorf("telegram_web: 频道名为空")
	}
	u := tgWebBase + url.PathEscape(channel)
	if before > 0 {
		u += "?before=" + strconv.FormatInt(before, 10)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", tgWebBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("telegram_web: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("telegram_web: http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("telegram_web: read: %w", err)
	}
	msgs, err := parseTgWeb(body, channel)
	if err != nil {
		return nil, 0, err
	}
	return msgs, tgWebNextBefore(body), nil
}

// parseTgWeb 从网页预览 HTML 抽取消息。可注入字节，便于离线测试。
func parseTgWeb(htmlBytes []byte, channel string) ([]TgWebMessage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram_web: parse html: %w", err)
	}
	var out []TgWebMessage
	doc.Find("div.tgme_widget_message[data-post]").Each(func(_ int, sel *goquery.Selection) {
		post, ok := sel.Attr("data-post")
		if !ok {
			return
		}
		_, idStr, found := strings.Cut(post, "/")
		if !found {
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		m := TgWebMessage{Channel: channel, MsgID: id}

		if dt, ok := sel.Find("time[datetime]").First().Attr("datetime"); ok {
			if t, err := time.Parse(time.RFC3339, dt); err == nil {
				m.At = t.UTC().Unix()
			}
		}
		// 正文可能在多个 tgme_widget_message_text 块（图文/引用），拼接。
		var parts []string
		sel.Find("div.tgme_widget_message_text").Each(func(_ int, t *goquery.Selection) {
			// 换行：<br> 转成 \n，否则会粘连
			t.Find("br").ReplaceWithHtml("\n")
			if s := strings.TrimSpace(t.Text()); s != "" {
				parts = append(parts, s)
			}
		})
		m.Text = strings.Join(parts, "\n")

		if v := strings.TrimSpace(sel.Find("span.tgme_widget_message_views").First().Text()); v != "" {
			m.Views = parseViews(v)
		}
		if m.At == 0 && m.Text == "" {
			return // 既无时间又无正文：不是有效消息（如服务消息）
		}
		out = append(out, m)
	})
	return out, nil
}

// tgWebNextBefore 抽取 `data-before` 翻页游标。无则返回 0。
func tgWebNextBefore(htmlBytes []byte) int64 {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return 0
	}
	v, ok := doc.Find("a[data-before]").First().Attr("data-before")
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseViews 把 "9.05K" / "1.2M" / "934" 解析成整数。
func parseViews(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "K"):
		mult, s = 1e3, strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult, s = 1e6, strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "B"):
		mult, s = 1e9, strings.TrimSuffix(s, "B")
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int(f * mult)
}

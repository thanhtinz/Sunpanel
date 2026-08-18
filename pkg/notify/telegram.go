package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// telegramChannel gửi thông báo qua bot Telegram.
//
// Telegram được hỗ trợ vì đây là kênh cảnh báo mà quản trị viên Việt Nam thực sự
// đọc: tin nhắn tới thẳng điện thoại, không lẫn vào hộp thư rác như email.
type telegramChannel struct {
	token  string
	chatID string
	client *http.Client
	// apiBase cho phép trỏ sang máy chủ khác trong test.
	apiBase string
}

func newTelegram(config map[string]string) (Channel, error) {
	token, err := require(config, "token")
	if err != nil {
		return nil, err
	}
	chatID, err := require(config, "chatId")
	if err != nil {
		return nil, err
	}

	apiBase := strings.TrimRight(config["apiBase"], "/")
	if apiBase == "" {
		apiBase = "https://api.telegram.org"
	}

	return &telegramChannel{
		token: token, chatID: chatID, apiBase: apiBase,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Kind trả về loại kênh.
func (c *telegramChannel) Kind() Kind { return Telegram }

// Send gửi thông báo tới Telegram.
func (c *telegramChannel) Send(ctx context.Context, message Message) error {
	text := fmt.Sprintf("%s *%s*\n\n%s",
		message.Level.Icon(), escapeMarkdown(message.Title), escapeMarkdown(message.Body))

	if message.Hostname != "" {
		text += "\n\nMáy chủ: `" + escapeMarkdown(message.Hostname) + "`"
	}
	for _, key := range sortedKeys(message.Fields) {
		text += fmt.Sprintf("\n%s: `%s`", escapeMarkdown(key), escapeMarkdown(message.Fields[key]))
	}

	payload, err := json.Marshal(map[string]any{
		"chat_id": c.chatID,
		"text":    text,
		// MarkdownV2 cho phép in đậm tiêu đề; đổi lại mọi ký tự đặc biệt phải
		// được thoát, nếu không Telegram từ chối cả tin nhắn.
		"parse_mode": "MarkdownV2",
	})
	if err != nil {
		return fmt.Errorf("%w: dựng nội dung: %s", ErrSendFailed, err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.apiBase, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: dựng yêu cầu: %s", ErrSendFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: gọi Telegram: %s", ErrSendFailed, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// Telegram nói rõ vì sao trong thân phản hồi: sai token, sai chat, hoặc
		// bot chưa được thêm vào nhóm.
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 1000))
		return fmt.Errorf("%w: Telegram trả %s: %s", ErrSendFailed, res.Status, strings.TrimSpace(string(detail)))
	}
	return nil
}

// markdownSpecial là các ký tự MarkdownV2 bắt buộc phải thoát.
const markdownSpecial = "_*[]()~`>#+-=|{}.!\\"

// escapeMarkdown thoát các ký tự đặc biệt của MarkdownV2.
//
// Bỏ sót một ký tự khiến Telegram từ chối cả tin nhắn với lỗi khó hiểu, nên phải
// thoát toàn bộ danh sách chứ không chỉ vài ký tự hay gặp.
func escapeMarkdown(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, char := range value {
		if strings.ContainsRune(markdownSpecial, char) {
			builder.WriteByte('\\')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

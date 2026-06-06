package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type notificationServiceImpl struct{}

// NotificationService 通知服务单例
var NotificationService = &notificationServiceImpl{}

// WeChatMarkdownMessage 企业微信 Markdown 消息体
type WeChatMarkdownMessage struct {
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Content string `json:"content"`
	} `json:"markdown"`
}

// SendWeChatMarkdown 发送企业微信 Markdown 消息
func (s *notificationServiceImpl) SendWeChatMarkdown(content string) error {
	message := WeChatMarkdownMessage{MsgType: "markdown"}
	message.Markdown.Content = content

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("构建消息失败: %v", err)
	}

	resp, err := http.Post(
		AppConfig.WeChatWebhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("发送消息失败，状态码: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if errCode, ok := result["errcode"].(float64); ok && errCode != 0 {
		errMsg := result["errmsg"].(string)
		return fmt.Errorf("企业微信返回错误: %s (code: %.0f)", errMsg, errCode)
	}

	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WeChatService 微信服务结构体
type WeChatService struct {
	appID     string
	appSecret string
}

// NewWeChatService 创建微信服务实例
func NewWeChatService(appID, appSecret string) *WeChatService {
	return &WeChatService{
		appID:     appID,
		appSecret: appSecret,
	}
}

// WXSessionResp 微信会话响应结构
type WXSessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid,omitempty"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2Session 通过 code 换取 session
func (w *WeChatService) Code2Session(code string) (*WXSessionResp, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		w.appID, w.appSecret, code,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr WXSessionResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	if sr.ErrCode != 0 {
		return nil, fmt.Errorf("wx err: %d %s", sr.ErrCode, sr.ErrMsg)
	}
	return &sr, nil
}

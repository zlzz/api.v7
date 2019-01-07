package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/qiniu/api.v7/conf"
	"github.com/qiniu/x/bytes.v7/seekable"
)

//  七牛鉴权类，用于生成Qbox, Qiniu, Upload签名
// AK/SK可以从 https://portal.qiniu.com/user/key 获取。
type Authorization struct {
	AccessKey string
	SecretKey []byte
}

// 构建一个Authorization对象
func New(accessKey, secretKey string) *Authorization {
	return &Authorization{accessKey, []byte(secretKey)}
}

// Sign 对数据进行签名，一般用于私有空间下载用途
func (ath *Authorization) Sign(data []byte) (token string) {
	h := hmac.New(sha1.New, ath.SecretKey)
	h.Write(data)

	sign := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s:%s", ath.AccessKey, sign)
}

// SignWithData 对数据进行签名，一般用于上传凭证的生成用途
func (ath *Authorization) SignWithData(b []byte) (token string) {
	encodedData := base64.URLEncoding.EncodeToString(b)
	sign := ath.Sign(b)
	return fmt.Sprintf("%s:%s:%s", ath.AccessKey, sign, encodedData)
}

// SignRequest 对数据进行签名，一般用于管理凭证的生成
func (ath *Authorization) SignRequest(req *http.Request) (token string, err error) {
	u := req.URL
	s := u.Path
	if u.RawQuery != "" {
		s += "?" + u.RawQuery
	}
	s += "\n"

	data := []byte(s)
	if incBody(req) {
		s2, err2 := seekable.New(req)
		if err2 != nil {
			return "", err2
		}
		data = append(data, s2.Bytes())
	}
	token = ath.Sign(data)
	return
}

// SignRequestV2 对数据进行签名，一般用于高级管理凭证的生成
func (ath *Authorization) SignRequestV2(req *http.Request) (token string, err error) {
	u := req.URL

	//write method path?query
	s := fmt.Sprintf("%s %s", req.Method, u.Path)
	if u.RawQuery != "" {
		s += "?"
		s += u.RawQuery
	}

	//write host and post
	s += "\nHost: "
	s += req.Host

	//write content type
	contentType := req.Header.Get("Content-Type")
	if contentType != "" {
		s += "\n"
		s += fmt.Sprintf("Content-Type: %s", contentType)
	}
	s += "\n\n"

	data := []byte(s)
	//write body
	if incBodyV2(req) {
		s2, err2 := seekable.New(req)
		if err2 != nil {
			return "", err2
		}
		data = append(data, s2.Bytes())
	}

	token = ath.Sign(data)
	return
}

// 管理凭证生成时，是否同时对request body进行签名
func incBody(req *http.Request) bool {
	return req.Body != nil && req.Header.Get("Content-Type") == conf.CONTENT_TYPE_FORM
}

func incBodyV2(req *http.Request) bool {
	contentType := req.Header.Get("Content-Type")
	return req.Body != nil && (contentType == conf.CONTENT_TYPE_FORM || contentType == conf.CONTENT_TYPE_JSON)
}

// VerifyCallback 验证上传回调请求是否来自七牛
func (ath *Authorization) VerifyCallback(req *http.Request) (bool, error) {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		return false, nil
	}

	token, err := ath.SignRequest(req)
	if err != nil {
		return false, err
	}

	return auth == "QBox "+token, nil
}
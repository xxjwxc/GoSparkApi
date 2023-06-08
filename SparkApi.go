package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"github.com/xxjwxc/public/mylog"
	"github.com/xxjwxc/public/tools"
	"golang.org/x/net/websocket"
)

const (
	_xfHost = "ws://spark-api.xf-yun.com/v1.1/chat"
	ORIGIN  = "http://spark-api.xf-yun.com"

	_xfAPIID     = "xxx"
	_xfAPIKey    = "xxxx"
	_xfAPISecret = "xxxxx"
)

type SparkResp struct {
	Header       SparkHead    `json:"header"`
	SparkPayload SparkPayload `json:"payload"`
}

type SparkHead struct {
	Code    int    `json:"code"`    // 错误码，0表示正常，非0表示出错；详细释义可在接口说明文档最后的错误码说明了解
	Message string `json:"message"` // 会话是否成功的描述信息
	Sid     string `json:"SID"`     // 会话的唯一id，用于讯飞技术人员查询服务端会话日志使用,出现调用错误时建议留存该字段
	Status  int    `json:"status"`  // 会话状态，取值为[0,1,2]；0代表首次结果；1代表中间结果；2代表最后一个结果
}

type SparkPayload struct {
	SparkChoices SparkChoices `json:"choices"`
	SparkUsage   SparkUsage   `json:"usage"`
}

type SparkChoices struct {
	Status int `json:"status"` // 	文本响应状态，取值为[0,1,2]; 0代表首个文本结果；1代表中间文本结果；2代表最后一个文本结果
	Seq    int `json:"seq"`    // 	返回的数据序号，取值为[0,9999999]
	Text   []struct {
		Content string `json:"content"` // 	AI的回答内容
		Role    string `json:"role"`    // 角色标识，固定为assistant，标识角色为AI
		Index   int    `json:"index"`   // 	结果序号，取值为[0,10]; 当前为保留字段，开发者可忽略
	} `json:"text"`
}

type SparkUsage struct {
	Text struct {
		QuestionTokens   int `json:"question_tokens"`   //	保留字段，可忽略
		PromptTokens     int `json:"prompt_tokens"`     //包含历史问题的总tokens大小
		CompletionTokens int `json:"completion_tokens"` //回答的tokens大小
		TotalTokens      int `json:"total_tokens"`      //prompt_tokens和completion_tokens的和，也是本次交互计费的tokens大小
	} `json:"test"`
}

// 获取答案
// getWxMessage 主入口
func GetXfAnswer(username, msg string) (string, error) {
	url, err := CreatUrl()
	if err != nil {
		return "", err
	}
	config, _ := websocket.NewConfig(url, ORIGIN)
	conn, err := websocket.DialConfig(config)
	if err != nil {
		mylog.Errorf("websocket dial err: %v", err)
		return "", err
	}
	defer conn.Close()

	req := fmt.Sprintf(`
	{
        "header": {
            "app_id": "%v",
            "uid": "%v"
        },
        "parameter": {
            "chat": {
                "domain": "general",
                "random_threshold": 0.5,
                "max_tokens": 512,
                "auditing": "default"
            }
        },
        "payload": {
            "message": {
                "text": [
                    {"role": "user", "content": "%v"}
                ]
            }
        }
    }
	`, _xfAPIID, username, msg)
	// 分帧发送音频数据
	if err := websocket.Message.Send(conn, []byte(req)); err != nil {
		mylog.Errorf("send data err: %v", err)

		return "", err
	}
	var ans string
	// 接收数据
	for {
		var msg string
		if err := websocket.Message.Receive(conn, &msg); err != nil {
			if err.Error() == "EOF" {
				mylog.Info("receive msg end")
			} else {
				mylog.Errorf("receive msg error: %v", msg)
				return "", err
			}
			break
		}
		fmt.Println(msg)
		var out SparkResp
		tools.JSONEncode(msg, &out)
		if out.Header.Code != 0 { // 请求错误
			mylog.Error(out.Header.Message)
			break
		} else {
			if len(out.SparkPayload.SparkChoices.Text) > 0 {
				ans += out.SparkPayload.SparkChoices.Text[0].Content
			}
			if out.SparkPayload.SparkUsage.Text.TotalTokens > 0 {
				break
			}
		}
	}

	return ans, nil
}

func CreatUrl() (string, error) {

	// # 生成RFC1123格式的时间戳
	date := time.Now().Add(-8 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 GMT")
	_url, err := url.Parse(_xfHost)
	if err != nil {
		return "", err
	}

	host := _url.Host
	// # 拼接字符串
	signature_origin := "host: " + host + "\n"
	signature_origin += "date: " + date + "\n"
	signature_origin += "GET " + _url.Path + " HTTP/1.1"

	// # 进行hmac-sha256进行加密
	signature_sha_base64 := ComputeHmacSha256(signature_origin, _xfAPISecret)

	authorization_origin := fmt.Sprintf(`api_key="%v", algorithm="hmac-sha256", headers="host date request-line", signature="%v"`, _xfAPIKey, signature_sha_base64)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorization_origin))

	// # 将请求的鉴权参数组合为字典
	// mp := map[string]string{
	// 	"authorization": authorization,
	// 	"date":          date,
	// 	"host":          host,
	// }

	// # 拼接鉴权参数，生成url
	_url1 := fmt.Sprintf(`%v?authorization=%v&date=%v&host=%v`, _xfHost, url.QueryEscape(authorization), url.QueryEscape(date), url.QueryEscape(host))
	// # 此处打印出建立连接时候的url,参考本demo的时候可取消上方打印的注释，比对相同参数时生成的url与自己代码生成的url是否一致
	return _url1, nil
}

func ComputeHmacSha256(data string, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(data))

	return tools.Base64Encode(mac.Sum(nil))
}

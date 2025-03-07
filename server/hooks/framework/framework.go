package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/utils/context"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EmailHook interface {
	// SendBefore 邮件发送前的数据 同步执行
	SendBefore(ctx *context.Context, email *parsemail.Email)
	// SendAfter 邮件发送后的数据，err是每个收信服务器的错误信息 异步执行
	SendAfter(ctx *context.Context, email *parsemail.Email, err map[string]error)
	// ReceiveParseBefore 接收到邮件，解析之前的原始数据 同步执行
	ReceiveParseBefore(ctx *context.Context, email *[]byte)
	// ReceiveParseAfter 接收到邮件，解析之后的结构化数据 (收信规则前，写数据库前执行)  同步执行
	ReceiveParseAfter(ctx *context.Context, email *parsemail.Email)
	// ReceiveSaveAfter 邮件落库以后执行（收信规则后执行） 异步执行
	ReceiveSaveAfter(ctx *context.Context, email *parsemail.Email, ue []*models.UserEmail)
	// GetName 获取插件名称
	GetName(ctx *context.Context) string
	// SettingsHtml 插件页面
	SettingsHtml(ctx *context.Context, url string, requestData string) string
}

// HookDTO PMail 主程序和插件通信的结构体
type HookDTO struct {
	ServerVersion string           // 服务端程序版本
	Ctx           *context.Context // 上下文
	Email         *parsemail.Email // 邮件内容
	EmailByte     *[]byte          // 未解析前的文件内容
	ErrMap        map[string]error // 错误信息
	UserEmail     []*models.UserEmail
}

type SettingsHtmlRequest struct {
	Ctx         *context.Context // 上下文
	URL         string
	RequestData string
}

type Plugin struct {
	name string
	hook EmailHook
}

func CreatePlugin(name string, hook EmailHook) *Plugin {
	return &Plugin{
		name: name,
		hook: hook,
	}
}

type logFormatter struct {
}

// Format 定义日志输出格式
func (l *logFormatter) Format(entry *log.Entry) ([]byte, error) {
	b := bytes.Buffer{}

	b.WriteString(fmt.Sprintf("[%s]", entry.Level.String()))
	b.WriteString(fmt.Sprintf("[%s]", entry.Time.Format("2006-01-02 15:04:05")))
	if entry.Context != nil {
		ctx := entry.Context.(*context.Context)
		if ctx != nil {
			b.WriteString(fmt.Sprintf("[%s]", ctx.GetValue(context.LogID)))
		}
	}
	b.WriteString(fmt.Sprintf("[%s:%d]", entry.Caller.File, entry.Caller.Line))
	b.WriteString(entry.Message)

	b.WriteString("\n")
	return b.Bytes(), nil
}

func (p *Plugin) Run() {

	// 设置日志格式为json格式
	log.SetFormatter(&logFormatter{})
	log.SetReportCaller(true)

	// 设置将日志输出到标准输出（默认的输出为stderr,标准错误）
	// 日志消息输出可以是任意的io.writer类型
	log.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		panic("Command Params Error!")
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/SendBefore", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] SendBefore Start", p.name)
		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		p.hook.SendBefore(hookDTO.Ctx, hookDTO.Email)
		body, _ = json.Marshal(hookDTO)
		writer.Write(body)
		log.Debugf("[%s] SendBefore End", p.name)
	})
	mux.HandleFunc("/SendAfter", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] SendAfter Start", p.name)

		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		p.hook.SendAfter(hookDTO.Ctx, hookDTO.Email, hookDTO.ErrMap)
		body, _ = json.Marshal(hookDTO)
		writer.Write(body)
		log.Debugf("[%s] SendAfter End", p.name)

	})
	mux.HandleFunc("/ReceiveParseBefore", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] ReceiveParseBefore Start", p.name)
		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		p.hook.ReceiveParseBefore(hookDTO.Ctx, hookDTO.EmailByte)
		body, _ = json.Marshal(hookDTO)
		writer.Write(body)
		log.Debugf("[%s] ReceiveParseBefore End", p.name)
	})
	mux.HandleFunc("/ReceiveParseAfter", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] ReceiveParseAfter Start", p.name)
		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		p.hook.ReceiveParseAfter(hookDTO.Ctx, hookDTO.Email)
		body, _ = json.Marshal(hookDTO)
		writer.Write(body)
		log.Debugf("[%s] ReceiveParseAfter End", p.name)
	})
	mux.HandleFunc("/ReceiveSaveAfter", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] ReceiveSaveAfter Start", p.name)
		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		p.hook.ReceiveSaveAfter(hookDTO.Ctx, hookDTO.Email, hookDTO.UserEmail)
		body, _ = json.Marshal(hookDTO)
		writer.Write(body)
		log.Debugf("[%s] ReceiveSaveAfter End", p.name)
	})
	mux.HandleFunc("/GetName", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] GetName Start", p.name)
		var hookDTO HookDTO
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		name := strings.ReplaceAll(p.hook.GetName(hookDTO.Ctx), " ", "")
		writer.Write([]byte(name))
		log.Debugf("[%s] GetName End", p.name)
	})
	mux.HandleFunc("/SettingsHtml", func(writer http.ResponseWriter, request *http.Request) {
		log.Debugf("[%s] SettingsHtml Start", p.name)
		var hookDTO SettingsHtmlRequest
		body, _ := io.ReadAll(request.Body)
		err := json.Unmarshal(body, &hookDTO)
		if err != nil {
			log.Errorf("params error %+v", err)
			return
		}
		html := p.hook.SettingsHtml(hookDTO.Ctx, hookDTO.URL, hookDTO.RequestData)
		writer.Write([]byte(html))
		log.Debugf("[%s] SettingsHtml End", p.name)
	})

	server := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      mux,
	}

	filePath := getExePath() + "/" + os.Args[1]
	unixListener, err := net.Listen("unix", filePath)
	if err != nil {
		panic(err)
	}
	err = server.Serve(unixListener)
	if err != nil {
		panic(err)
	}
}

func getExePath() string {
	//return "/PMail/server/hooks/spam_block" //debug socket path

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exePath := filepath.Dir(ex)
	return exePath
}

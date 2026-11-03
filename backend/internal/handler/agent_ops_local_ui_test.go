package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 服务端形态（DesktopTrust 为 nil）下，浏览器工作台自身的同源请求可以写画布。
func TestServerProfileBrowserSameOriginCanCommitDocument(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	code, body := postOps(t, env.router, "canvas.document.commit", "local-ui-doc", map[string]string{
		"Origin": "http://127.0.0.1:18090",
	}, `{"canvasId":"`+env.canvasID+`","expectedRevision":1,"document":{"title":"本地界面提交","nodes":[{"id":"n1","type":"text","title":"本地"}],"connections":[]}}`)
	if code != http.StatusOK {
		t.Fatalf("同源浏览器提交失败: %d %s", code, body)
	}
	if !strings.Contains(body, `"caller":"manual"`) {
		t.Fatalf("本地界面写入应记录为 manual: %s", body)
	}
}

// 同源 GET 不一定带 Origin，浏览器会用 Sec-Fetch-Site 标注同源来源。
func TestServerProfileBrowserSameSiteHeaderCanWrite(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	code, body := postOps(t, env.router, "canvas.document.commit", "same-site-doc", map[string]string{
		"Sec-Fetch-Site": "same-origin",
	}, `{"canvasId":"`+env.canvasID+`","expectedRevision":1,"document":{"title":"同源头提交","nodes":[{"id":"n1","type":"text","title":"同源"}],"connections":[]}}`)
	if code != http.StatusOK {
		t.Fatalf("Sec-Fetch-Site 同源提交失败: %d %s", code, body)
	}
}

// 不带 Origin 的本机进程仍需 owner 凭据或已登记客户端：放行只覆盖浏览器工作台。
func TestServerProfileLocalProcessWithoutOriginStillRejected(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	code, body := postOps(t, env.router, "canvas.document.commit", "local-process-doc", nil,
		`{"canvasId":"`+env.canvasID+`","expectedRevision":1,"document":{"title":"命令行提交"}}`)
	if code != http.StatusForbidden {
		t.Fatalf("无 Origin 的本机进程不应获得写身份: %d %s", code, body)
	}
}

// 跨站 Origin 不满足同源要求，不能借服务端形态放行。
func TestServerProfileForeignOriginRejected(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	code, body := postOps(t, env.router, "canvas.document.commit", "foreign-origin-doc", map[string]string{
		"Origin": "http://evil.example",
	}, `{"canvasId":"`+env.canvasID+`","expectedRevision":1,"document":{"title":"跨站提交"}}`)
	if code != http.StatusForbidden {
		t.Fatalf("跨站 Origin 不应获得写身份: %d %s", code, body)
	}
}

// 服务端形态下同源浏览器可以签发 UI 会话，不再依赖 BEEFTV_UI_BOOTSTRAP。
func TestServerProfileSameOriginCanIssueUISession(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	router := gin.New()
	router.Use(RuntimeDependenciesMiddleware(RuntimeDependencies{AssistantHost: env.assistantHost}))
	RegisterAgentUISessionRoutes(router.Group("/api"), env.service, newUISessionStore(), nil)

	issued := uiSessionRequest(router, "http://127.0.0.1:18090")
	if issued.Code != http.StatusOK {
		t.Fatalf("同源浏览器应能签发 UI 会话: %d %s", issued.Code, issued.Body.String())
	}
	if !strings.Contains(issued.Body.String(), `"localUI":true`) {
		t.Fatalf("响应应标明 localUI 引导: %s", issued.Body.String())
	}

	noOrigin := uiSessionRequest(router, "")
	if noOrigin.Code != http.StatusForbidden {
		t.Fatalf("无 Origin 的本机进程不应签发 UI 会话: %d %s", noOrigin.Code, noOrigin.Body.String())
	}

	foreign := uiSessionRequest(router, "http://evil.example")
	if foreign.Code != http.StatusForbidden {
		t.Fatalf("跨站 Origin 不应签发 UI 会话: %d %s", foreign.Code, foreign.Body.String())
	}
}

func uiSessionRequest(router http.Handler, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18090/api/assistant/ui-session", strings.NewReader(`{}`))
	request.Host = "127.0.0.1:18090"
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

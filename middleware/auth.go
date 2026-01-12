package middleware

import (
	"log"
	"net/http"
	"regexp"
	"strings"
	"cas-gateway/auth"
	"cas-gateway/proxy"

	"github.com/gorilla/sessions"
)

const (
	SessionName        = "cas_gateway_session"
	UserKey            = "user"
	IsAuthenticatedKey = "authenticated"
)

var (
	// staticFileRegex 静态文件扩展名正则表达式（参考原 Node.js 版本）
	staticFileRegex = regexp.MustCompile(`\.(ico|jpg|jpeg|png|gif|svg|js|css|swf|eot|ttf|otf|woff|woff2)$`)
)

// isStaticFile 判断是否为静态文件
func isStaticFile(path string) bool {
	return staticFileRegex.MatchString(path)
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	store        *sessions.CookieStore
	proxyManager *proxy.ProxyManager
	authProvider auth.Provider
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(sessionKey string, pm *proxy.ProxyManager, authProvider auth.Provider) *AuthMiddleware {
	store := sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7天
		HttpOnly: true,
		Secure:   false, // 在生产环境建议设为true（HTTPS）
		SameSite: http.SameSiteLaxMode,
	}

	return &AuthMiddleware{
		store:        store,
		proxyManager: pm,
		authProvider: authProvider,
	}
}

// Handler 认证处理函数（参考原 Node.js 版本的逻辑）
func (am *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 打印请求日志
		log.Printf("[请求] %s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

		// 特殊路径直接处理（不转发到后端）
		if r.URL.Path == "/health" || r.URL.Path == "/logout" {
			next.ServeHTTP(w, r)
			return
		}

		// 获取路由配置（单个路由，所有请求都转发到同一个后端）
		route := am.proxyManager.GetRoute()
		if route == nil {
			log.Printf("[路由] 路由配置不存在")
			http.NotFound(w, r)
			return
		}

		// 静态文件直接转发到后端系统，不进行认证检查（参考原 Node.js 版本的逻辑）
		if isStaticFile(r.URL.Path) {
			log.Printf("[静态文件] 直接转发: %s -> %s", r.URL.Path, route.Target)
			// 直接使用代理转发，不剥离路径前缀
			proxy := am.proxyManager.GetProxy()
			proxy.ServeHTTP(w, r)
			return
		}

		// 获取session
		session, _ := am.store.Get(r, SessionName)

		// 检查是否已认证（参考原代码：检查cookie中的token）
		authenticated, ok := session.Values[IsAuthenticatedKey].(bool)
		if ok && authenticated {
			// 已认证，继续处理（参考原代码：设置请求头并转发）
			user := am.GetUser(r)
			if user != "" {
				r.Header.Set("X-User", user)
				if employeeName, ok := session.Values["employeeName"].(string); ok && employeeName != "" {
					r.Header.Set("X-Employee-Name", employeeName)
				}
			}
			log.Printf("[认证] 已认证用户: %s, 转发请求: %s", user, r.URL.Path)
			// 如果请求路径包含路由前缀，需要剥离前缀
			if route.Path != "" && route.Path != "/" && strings.HasPrefix(r.URL.Path, route.Path) {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, route.Path)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		// 检查是否为登录回调（包含ticket）
		if am.authProvider.IsLoginPath(r.URL.String()) {
			ticket, err := am.authProvider.ExtractTicket(r.URL.String())
			if err == nil {
				// 验证ticket（使用路由路径构建service URL）
				servicePath := route.Path
				if servicePath == "" {
					servicePath = "/"
				}
				serviceURL := am.authProvider.BuildServiceURL(r, servicePath)
				userInfo, err := am.authProvider.ValidateTicket(ticket, serviceURL)
				if err == nil {
					// 验证成功，保存session（使用oaid作为用户标识）
					session.Values[UserKey] = userInfo.Oaid
					if userInfo.EmployeeName != "" {
						session.Values["employeeName"] = userInfo.EmployeeName
					}
					session.Values[IsAuthenticatedKey] = true
					if err := session.Save(r, w); err == nil {
						// 重定向到路由路径（去除ticket参数）
						redirectPath := servicePath
						if redirectPath == "/" {
							redirectPath = ""
						}
						log.Printf("[认证] 认证成功，重定向到: %s", redirectPath)
						http.Redirect(w, r, redirectPath, http.StatusFound)
						return
					}
				} else {
					log.Printf("[认证] Ticket验证失败: %v", err)
				}
			}
		}

		// 未认证，跳转到登录页（参考原代码逻辑）
		servicePath := route.Path
		if servicePath == "" {
			servicePath = "/"
		}
		serviceURL := am.authProvider.BuildServiceURL(r, servicePath)
		loginURL := am.authProvider.GetLoginURL(serviceURL)
		log.Printf("[认证] 未认证，跳转到登录页: %s", loginURL)
		http.Redirect(w, r, loginURL, http.StatusFound)
	})
}

// GetUser 从请求中获取当前用户
func (am *AuthMiddleware) GetUser(r *http.Request) string {
	session, _ := am.store.Get(r, SessionName)
	if user, ok := session.Values[UserKey].(string); ok {
		return user
	}
	return ""
}

// Logout 登出
func (am *AuthMiddleware) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := am.store.Get(r, SessionName)
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	session.Save(r, w)
}


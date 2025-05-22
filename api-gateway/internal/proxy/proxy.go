package proxy

import (
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// CreateProxy создает прокси с модификацией пути
func CreateProxy(targetHost, stripPrefix, addPrefix string) gin.HandlerFunc {
	target, _ := url.Parse(targetHost)
	proxy := httputil.NewSingleHostReverseProxy(target)

	return func(c *gin.Context) {
		// Сохраняем оригинальный путь
		originalPath := c.Request.URL.Path

		// Удаляем stripPrefix
		path := strings.TrimPrefix(originalPath, stripPrefix)

		// Добавляем addPrefix, сохраняя оригинальное окончание пути
		if strings.HasSuffix(addPrefix, "/") && strings.HasPrefix(path, "/") {
			path = addPrefix + strings.TrimPrefix(path, "/")
		} else if !strings.HasSuffix(addPrefix, "/") && !strings.HasPrefix(path, "/") && path != "" {
			path = addPrefix + "/" + path
		} else {
			path = addPrefix + path
		}

		c.Request.URL.Path = path

		// Обновляем заголовки
		c.Request.Header.Set("X-Forwarded-Host", c.Request.Host)
		c.Request.Header.Del("X-Forwarded-For")

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

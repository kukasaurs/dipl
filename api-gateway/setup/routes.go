package setup

import (
	"cleaning-app/api-gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

func ConfigureServiceProxies(router *gin.RouterGroup) {
	services := []struct {
		path        string
		target      string
		stripPrefix string
		addPrefix   string
	}{
		{"/orders", "http://order-service:8001", "/api/orders", "/orders"},
		{"/notifications", "http://notification-service:8009", "/api/notifications", "/notifications"},
		{"/support", "http://support-service:8008", "/api/support", "/support"},
		{"/subscriptions", "http://subscription-service:8004", "/api/subscriptions", "/subscriptions"},
		{"/services", "http://cleaning-details-service:8003", "/api/services", "/api/services"},
		{"/review", "http://review-media-service:8007", "/api/review", "/reviews"},
		{"/users", "http://user-management-service:8006", "/api/users", "/users"},
	}

	for _, service := range services {
		router.Any(service.path+"/*proxyPath", proxy.CreateProxy(
			service.target,
			service.stripPrefix,
			service.addPrefix,
		))
	}
}

func ConfigureAdminProxies(router *gin.RouterGroup) {
	adminServices := []struct {
		path        string
		target      string
		stripPrefix string
		addPrefix   string
	}{
		{"/services", "http://cleaning-details-service:8003", "/api/admin/services", "/admin/services"},
		{"/users", "http://user-management-service:8006", "/api/admin/users", "/api/admin/users"},
	}

	for _, service := range adminServices {
		router.Any(service.path, proxy.CreateProxy(
			service.target,
			service.stripPrefix,
			service.addPrefix,
		))

		router.Any(service.path+"/*proxyPath", proxy.CreateProxy(
			service.target,
			service.stripPrefix,
			service.addPrefix,
		))
	}
}

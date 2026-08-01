package app

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupStaticFiles(r *gin.Engine) {
	staticDir := "frontend"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		return
	}

	r.Use(spaFallback(staticDir))
	r.Static("/assets", staticDir+"/assets")
	r.StaticFile("/favicon.png", staticDir+"/favicon.png")
	// /icons is owned by the backend icon store (registered in SetupRoutes) —
	// it must not be re-registered here or gin would panic on the duplicate
	// route prefix.
	for _, dir := range []string{"_nuxt"} {
		if _, err := os.Stat(staticDir + "/" + dir); err == nil {
			r.Static("/"+dir, staticDir+"/"+dir)
		}
	}
}

// iconsFileHandler serves the icon storage directory (registered at
// /icons/*filepath) with security headers that neutralize stored SVG XSS:
// X-Content-Type-Options: nosniff forces the browser to honor the served
// Content-Type instead of sniffing, and Content-Security-Policy: sandbox
// strips script execution from any SVG rendered outside an <img> context.
func iconsFileHandler(dir string) gin.HandlerFunc {
	fileServer := http.StripPrefix("/icons", http.FileServer(http.Dir(dir)))
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "sandbox")
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

func spaFallback(staticDir string) gin.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/icons" || strings.HasPrefix(path, "/icons/") || path == "/ws" || path == "/health" {
			c.Next()
			return
		}
		f, err := os.Stat(staticDir + path)
		if err == nil && !f.IsDir() {
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

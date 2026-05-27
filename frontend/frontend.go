package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

//go:embed all:dist
var assets embed.FS

func Register(e *echo.Echo) {
	distFS, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	e.GET("/*", func(c *echo.Context) error {
		p := c.Request().URL.Path
		if strings.HasPrefix(p, "/api") {
			return echo.ErrNotFound
		}
		if p == "/" {
			return c.FileFS("index.html", distFS)
		}

		name := strings.TrimPrefix(p, "/")
		_, err := fs.Stat(distFS, name)
		if err == nil {
			return echo.WrapHandler(fileServer)(c)
		}
		return c.FileFS("index.html", distFS)
	})
}

package curator

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

//
// server to list and serve curated articles
//

type errorMsg struct {
	Error string `json:"error"`
}

func (c *Curator) middleWare(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}

func (c *Curator) articleList(e echo.Context) error {
	list, err := c.ListArticles()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, list)
}

func (c *Curator) articleGet(e echo.Context) error {
	article, err := c.GetArticle(e.Param("name"))

	if err != nil {
		e.Logger().Error(err)
		if err.Error() == "files/read: file does not exist" {
			return e.JSON(http.StatusNotFound, &errorMsg{Error: "article not found"})
		} else {
			return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
		}
	}

	return e.JSON(http.StatusOK, article)
}

func NewCuratorServer(config *Config) *echo.Echo {

	server := echo.New()

	curator := NewCurator(config)
	server.Use(curator.middleWare)

	server.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	server.GET("/article/list", curator.articleList)
	server.GET("/article/:name", curator.articleGet)
	server.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "not found")
	})

	return server
}

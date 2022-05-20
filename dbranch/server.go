package dbranch

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

func (c *Config) middleWare(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}

func (c *Config) articleIndex(e echo.Context) error {
	index, err := c.LoadArticleIndex()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, index)
}

type articleListResponse struct {
	Articles []string `json:"articles"`
}

func (c *Config) articleList(e echo.Context) error {
	list, err := c.ListArticles()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, &articleListResponse{Articles: list})
}

func (c *Config) articleGet(e echo.Context) error {
	record, article, err := c.GetArticle(e.Param("name"))

	if err != nil {
		e.Logger().Error(err)
		if err.Error() == "files/read: file does not exist" {
			return e.JSON(http.StatusNotFound, &errorMsg{Error: "article not found"})
		} else {
			return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
		}
	}

	return e.JSON(http.StatusOK, &FullArticle{Article: article, Record: record})
}

func NewCuratorServer(config *Config) *echo.Echo {

	server := echo.New()

	server.Use(config.middleWare)

	server.GET("/article/:name", config.articleGet)
	server.GET("/article/list", config.articleList)
	server.GET("/article/index", config.articleIndex)
	server.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "not found")
	})

	return server
}

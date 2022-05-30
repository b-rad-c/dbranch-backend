package dbranch

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

//
// server to list and serve curated articles
//

type errorMsg struct {
	Error string `json:"error"`
}

func middleWare(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}

//
// article endpoints
//

func articleIndex(e echo.Context) error {
	index, err := LoadArticleIndex()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, index)
}

type articleListResponse struct {
	Articles []string `json:"articles"`
}

func articleList(e echo.Context) error {
	list, err := ListArticles()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, &articleListResponse{Articles: list})
}

func articleGet(e echo.Context) error {
	article, err := GetArticle(e.Param("name"))

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

//
// db status endpoints
//

func dbMeta(e echo.Context) error {
	meta, err := CardanoDBMeta()

	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, meta)
}

func dbSyncStatus(e echo.Context) error {
	sync_status, err := CardanoDBSyncStatus()

	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, sync_status)
}

func dbBlockStatus(e echo.Context) error {
	block_status, err := CardanoDBBlockStatus()

	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, block_status)
}

func dbOverview(e echo.Context) error {
	overview, err := CardanoDBOverview()

	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, &errorMsg{Error: "internal server error"})
	}

	return e.JSON(http.StatusOK, overview)
}

//
// server / router
//

func CuratorServer() error {

	port := os.Getenv("DBRANCH_SERVER_PORT")
	if port == "" {
		port = "1323"
	}

	server := echo.New()

	server.Use(middleWare)

	server.GET("/article/:name", articleGet)
	server.GET("/article/list", articleList)
	server.GET("/article/index", articleIndex)
	server.GET("/db/meta", dbMeta)
	server.GET("/db/sync", dbSyncStatus)
	server.GET("/db/block", dbBlockStatus)
	server.GET("/db/overview", dbOverview)
	server.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "not found")
	})

	err := server.Start(":" + port)
	server.Logger.Fatal(err)
	return err
}

package dbranch

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/labstack/echo/v4/middleware"
)

//
// server to list and serve curated articles
//

type errorMsg struct {
	Error string `json:"error"`
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

func articleGetByCid(e echo.Context) error {
	// init request
	article_cid := e.Param("cid")
	load_record := false
	err := echo.QueryParamsBinder(e).Bool("load_record", &load_record).BindError()
	if err != nil {
		e.Logger().Error(err)
		return e.JSON(http.StatusBadRequest, &errorMsg{Error: "invalud request: " + err.Error()})
	}

	// load article
	article, err := GetArticleByCID(article_cid, load_record)
	if err != nil {
		e.Logger().Error(err)
		if err.Error() == "article not found" {
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

	server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	prefix := "/api/v0"

	server.GET(prefix+"/article/index", articleIndex)
	server.GET(prefix+"/article/cid/:cid", articleGetByCid)

	server.GET(prefix+"/db/meta", dbMeta)
	server.GET(prefix+"/db/sync", dbSyncStatus)
	server.GET(prefix+"/db/block", dbBlockStatus)
	server.GET(prefix+"/db/overview", dbOverview)

	server.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "not found")
	})

	err := server.Start(":" + port)
	server.Logger.Fatal(err)
	return err
}

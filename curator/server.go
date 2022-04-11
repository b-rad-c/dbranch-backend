package curator

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/labstack/echo/v4"
)

//
// article models
//

type ArticleMetadata struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	SubTitle string `json:"subTitle"`
	Author   string `json:"author"`
}

type Article struct {
	Metadata ArticleMetadata `json:"metadata"`
}

type ArticleListItem struct {
	Name     string          `json:"name"`
	Size     uint64          `json:"size"`
	Hash     string          `json:"hash"`
	Metadata ArticleMetadata `json:"metadata"`
}

type ArticleList struct {
	Items []*ArticleListItem `json:"entries"`
}

//
// server
//

type errorMsg struct {
	Error string `json:"error"`
}

type CuratorServer struct {
	Config *Config
	Shell  *ipfs.Shell
}

func NewCurator(config *Config) *CuratorServer {
	return &CuratorServer{
		Config: config,
		Shell:  ipfs.NewShell(config.IpfsHost),
	}
}

func (curator *CuratorServer) MiddleWare(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}

func (c *CuratorServer) ArticleList(e echo.Context) error {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	e.Logger().Info("listing articles from: " + c.Config.CuratedDir)

	ls, err := c.Shell.FilesLs(ctx, c.Config.CuratedDir, ipfs.FilesLs.Stat(true))
	if err != nil {
		e.Logger().Error(err)
		return err
	}

	list := &ArticleList{}
	for _, entry := range ls {

		articlePath := path.Join(c.Config.CuratedDir, entry.Name)

		e.Logger().Info("reading article for list: " + articlePath)

		content, err := c.Shell.FilesRead(ctx, articlePath, ipfs.FilesLs.Stat(true))
		if err != nil {
			e.Logger().Error(err)
			return err
		}

		article := Article{}
		err = json.NewDecoder(content).Decode(&article)
		if err != nil {
			e.Logger().Error(err)
			return err
		}

		list.Items = append(list.Items, &ArticleListItem{
			Name:     entry.Name,
			Size:     entry.Size,
			Hash:     entry.Hash,
			Metadata: article.Metadata,
		})
	}

	return e.JSON(http.StatusOK, list)
}

func (c *CuratorServer) ArticleGet(e echo.Context) error {
	articlePath := path.Join(c.Config.CuratedDir, e.Param("name"))

	e.Logger().Info("reading article for list: " + articlePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	content, err := c.Shell.FilesRead(ctx, articlePath, ipfs.FilesLs.Stat(true))
	if err != nil {
		e.Logger().Error(err)
		if err.Error() == "files/read: file does not exist" {
			return e.JSON(http.StatusNotFound, &errorMsg{Error: "article not found: " + e.Param("name")})
		}
		return err
	}

	var article map[string]interface{}
	err = json.NewDecoder(content).Decode(&article)
	if err != nil {
		e.Logger().Error(err)
		return err
	}

	return e.JSON(http.StatusOK, article)
}

func NewCuratorServer(config *Config) *echo.Echo {

	server := echo.New()

	curator := NewCurator(config)
	server.Use(curator.MiddleWare)

	server.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	server.GET("/article/list", curator.ArticleList)
	server.GET("/article/:name", curator.ArticleGet)
	server.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "not found")
	})

	return server
}

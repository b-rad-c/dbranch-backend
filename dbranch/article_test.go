package dbranch

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

var (
	sampleArticle = `{
		"metadata": {
			"type": "information",
			"title": "Sample Article",
			"subTitle": "A Sample Subtitle",
			"author": "John Doe"
		},
		"contents": {
			"ops": [
				{
					"insert": "hello.world"
				}
			]
		}
	}`
	sampleArticleName = "test.news"
	sampleArticleCID  = "Qmafd1poTdPLa6VAWGDunquRWZa3BYNZEyCnzFdURHm4hm"
)

func TestMain(m *testing.M) {
	config := DefaultConfig()
	cid, err := config.ipfsShell().Add(strings.NewReader(sampleArticle))
	fmt.Printf("sample id: %s\n", cid)
	if err != nil {
		panic(err)
	}

	result := m.Run()

	os.Exit(result)
}

/*func TestAddArticle(t *testing.T) {
	err := curator.AddToCurated(&IncomingArticle{Name: sampleArticleName, CID: sampleArticleCID})
	if err != nil {
		t.Errorf("error adding to curated: %s", err.Error())
	}
}*/

/*func TestGetArticle(t *testing.T) {
	article, err := curator.GetArticle(sampleArticleName)

	if err != nil {
		t.Errorf("error getting article: %s", err.Error())
	}

	if article.Metadata.Title != "Sample Article" {
		t.Errorf("article title is incorrect: %s", article.Metadata.Title)
	}

	if article.Metadata.SubTitle != "A Sample Subtitle" {
		t.Errorf("article subTitle is incorrect: %s", article.Metadata.SubTitle)
	}

	if article.Metadata.Author != "John Doe" {
		t.Errorf("article author is incorrect: %s", article.Metadata.Author)
	}
}

func TestListArticles(t *testing.T) {
	list, err := curator.ListArticles()
	if err != nil {
		t.Errorf("error listing articles: %s", err.Error())
	}

	found := false

	for _, item := range list.Items {
		if item.Name == sampleArticleName {
			found = true

			if item.CID != sampleArticleCID {
				t.Errorf("got article cid: %s but expected: %s", item.CID, sampleArticleCID)
			}

			if item.Metadata.Title != "Sample Article" {
				t.Errorf("article title is incorrect: %s", item.Metadata.Title)
			}

			if item.Metadata.SubTitle != "A Sample Subtitle" {
				t.Errorf("article subTitle is incorrect: %s", item.Metadata.SubTitle)
			}

			if item.Metadata.Author != "John Doe" {
				t.Errorf("article author is incorrect: %s", item.Metadata.Author)
			}
		}
	}

	if !found {
		t.Errorf("did not get sample fle in list: %s", sampleArticleName)
	}
}

/*func TestRemoveArticle(t *testing.T) {
	err := curator.RemoveFromCurated(sampleArticleName)
	if err != nil {
		t.Errorf("error removing article: %s", err.Error())
	}

	_, err = curator.GetArticle(sampleArticleName)

	if err == nil {
		t.Errorf("article was not removed")
	}
}*/

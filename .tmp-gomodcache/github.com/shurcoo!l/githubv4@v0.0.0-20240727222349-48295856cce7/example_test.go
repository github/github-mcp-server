package githubv4_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func Example() {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)

	var q struct {
		Viewer struct {
			Login     string
			CreatedAt time.Time
			AvatarURL string `graphql:"avatarUrl(size: 72)"`
		}
	}
	err := client.Query(context.Background(), &q, nil)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(q.Viewer.Login)
	fmt.Println(q.Viewer.CreatedAt)
	fmt.Println(q.Viewer.AvatarURL)
}

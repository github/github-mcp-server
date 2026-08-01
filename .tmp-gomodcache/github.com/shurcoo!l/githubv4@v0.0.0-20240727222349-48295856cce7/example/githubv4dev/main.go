// githubv4dev is a test program currently being used for developing githubv4 package.
//
// Warning: It performs some queries and mutations against real GitHub API.
//
// It's not meant to be a clean or readable example. But it's functional.
// Better, actual examples will be created in the future.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func main() {
	flag.Parse()

	err := run()
	if err != nil {
		log.Println(err)
	}
}

func run() error {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_GRAPHQL_TEST_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	// Query some details about a repository, an issue in it, and its comments.
	{
		type githubV4Actor struct {
			Login     githubv4.String
			AvatarURL githubv4.URI `graphql:"avatarUrl(size:72)"`
			URL       githubv4.URI
		}

		var q struct {
			Repository struct {
				DatabaseID githubv4.Int
				URL        githubv4.URI

				Issue struct {
					Author         githubV4Actor
					PublishedAt    githubv4.DateTime
					LastEditedAt   *githubv4.DateTime
					Editor         *githubV4Actor
					Body           githubv4.String
					ReactionGroups []struct {
						Content githubv4.ReactionContent
						Users   struct {
							Nodes []struct {
								Login githubv4.String
							}

							TotalCount githubv4.Int
						} `graphql:"users(first:10)"`
						ViewerHasReacted githubv4.Boolean
					}
					ViewerCanUpdate githubv4.Boolean

					Comments struct {
						Nodes []struct {
							Body   githubv4.String
							Author struct {
								Login githubv4.String
							}
							Editor struct {
								Login githubv4.String
							}
						}
						PageInfo struct {
							EndCursor   githubv4.String
							HasNextPage githubv4.Boolean
						}
					} `graphql:"comments(first:$commentsFirst,after:$commentsAfter)"`
				} `graphql:"issue(number:$issueNumber)"`
			} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
			Viewer struct {
				Login      githubv4.String
				CreatedAt  githubv4.DateTime
				ID         githubv4.ID
				DatabaseID githubv4.Int
			}
			RateLimit struct {
				Cost      githubv4.Int
				Limit     githubv4.Int
				Remaining githubv4.Int
				ResetAt   githubv4.DateTime
			}
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String("shurcooL-test"),
			"repositoryName":  githubv4.String("test-repo"),
			"issueNumber":     githubv4.Int(1),
			"commentsFirst":   githubv4.NewInt(1),
			"commentsAfter":   githubv4.NewString("Y3Vyc29yOjE5NTE4NDI1Ng=="),
		}
		err := client.Query(context.Background(), &q, variables)
		if err != nil {
			return err
		}
		printJSON(q)
		//goon.Dump(out)
		//fmt.Println(github.Stringify(out))
	}

	// Toggle a 👍 reaction on an issue.
	//
	// That involves first doing a query (and determining whether the reaction already exists),
	// then either adding or removing it.
	{
		var q struct {
			Repository struct {
				Issue struct {
					ID        githubv4.ID
					Reactions struct {
						ViewerHasReacted githubv4.Boolean
					} `graphql:"reactions(content:$reactionContent)"`
				} `graphql:"issue(number:$issueNumber)"`
			} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String("shurcooL-test"),
			"repositoryName":  githubv4.String("test-repo"),
			"issueNumber":     githubv4.Int(2),
			"reactionContent": githubv4.ReactionContentThumbsUp,
		}
		err := client.Query(context.Background(), &q, variables)
		if err != nil {
			return err
		}
		fmt.Println("already reacted:", q.Repository.Issue.Reactions.ViewerHasReacted)

		if !q.Repository.Issue.Reactions.ViewerHasReacted {
			// Add reaction.
			var m struct {
				AddReaction struct {
					Subject struct {
						ReactionGroups []struct {
							Content githubv4.ReactionContent
							Users   struct {
								TotalCount githubv4.Int
							}
						}
					}
				} `graphql:"addReaction(input:$input)"`
			}
			input := githubv4.AddReactionInput{
				SubjectID: q.Repository.Issue.ID,
				Content:   githubv4.ReactionContentThumbsUp,
			}
			err := client.Mutate(context.Background(), &m, input, nil)
			if err != nil {
				return err
			}
			printJSON(m)
			fmt.Println("Successfully added reaction.")
		} else {
			// Remove reaction.
			var m struct {
				RemoveReaction struct {
					Subject struct {
						ReactionGroups []struct {
							Content githubv4.ReactionContent
							Users   struct {
								TotalCount githubv4.Int
							}
						}
					}
				} `graphql:"removeReaction(input:$input)"`
			}
			input := githubv4.RemoveReactionInput{
				SubjectID: q.Repository.Issue.ID,
				Content:   githubv4.ReactionContentThumbsUp,
			}
			err := client.Mutate(context.Background(), &m, input, nil)
			if err != nil {
				return err
			}
			printJSON(m)
			fmt.Println("Successfully removed reaction.")
		}
	}

	return nil
}

// printJSON prints v as JSON encoded with indent to stdout. It panics on any error.
func printJSON(v interface{}) {
	w := json.NewEncoder(os.Stdout)
	w.SetIndent("", "\t")
	err := w.Encode(v)
	if err != nil {
		panic(err)
	}
}

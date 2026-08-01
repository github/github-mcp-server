githubv4
========

[![Go Reference](https://pkg.go.dev/badge/github.com/shurcooL/githubv4.svg)](https://pkg.go.dev/github.com/shurcooL/githubv4)

Package `githubv4` is a client library for accessing GitHub GraphQL API v4 (https://docs.github.com/en/graphql).

If you're looking for a client library for GitHub REST API v3, the recommended package is [`github`](https://github.com/google/go-github#installation) (also known as `go-github`).

Focus
-----

-	Friendly, simple and powerful API.
-	Correctness, high performance and efficiency.
-	Support all of GitHub GraphQL API v4 via code generation from schema.

Installation
------------

```sh
go get github.com/shurcooL/githubv4
```

Usage
-----

### Authentication

GitHub GraphQL API v4 [requires authentication](https://docs.github.com/en/graphql/guides/forming-calls-with-graphql#authenticating-with-graphql). The `githubv4` package does not directly handle authentication. Instead, when creating a new client, you're expected to pass an `http.Client` that performs authentication. The easiest and recommended way to do this is to use the [`golang.org/x/oauth2`](https://golang.org/x/oauth2) package. You'll need an OAuth token from GitHub (for example, a [personal API token](https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/)) with the right scopes. Then:

```Go
import "golang.org/x/oauth2"

func main() {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)
	// Use client...
}
```

If you are using GitHub Enterprise, use [`githubv4.NewEnterpriseClient`](https://godoc.org/github.com/shurcooL/githubv4#NewEnterpriseClient):

```Go
client := githubv4.NewEnterpriseClient(os.Getenv("GITHUB_ENDPOINT"), httpClient)
// Use client...
```

### Simple Query

To make a query, you need to define a Go type that corresponds to the GitHub GraphQL schema, and contains the fields you're interested in querying. You can look up the GitHub GraphQL schema at https://docs.github.com/en/graphql/reference/queries.

For example, to make the following GraphQL query:

```GraphQL
query {
	viewer {
		login
		createdAt
	}
}
```

You can define this variable:

```Go
var query struct {
	Viewer struct {
		Login     githubv4.String
		CreatedAt githubv4.DateTime
	}
}
```

Then call `client.Query`, passing a pointer to it:

```Go
err := client.Query(context.Background(), &query, nil)
if err != nil {
	// Handle error.
}
fmt.Println("    Login:", query.Viewer.Login)
fmt.Println("CreatedAt:", query.Viewer.CreatedAt)

// Output:
//     Login: gopher
// CreatedAt: 2017-05-26 21:17:14 +0000 UTC
```

### Scalar Types

For each scalar in the GitHub GraphQL schema listed at https://docs.github.com/en/graphql/reference/scalars, there is a corresponding Go type in package `githubv4`.

You can use these types when writing queries:

```Go
var query struct {
	Viewer struct {
		Login          githubv4.String
		CreatedAt      githubv4.DateTime
		IsBountyHunter githubv4.Boolean
		BioHTML        githubv4.HTML
		WebsiteURL     githubv4.URI
	}
}
// Call client.Query() and use results in query...
```

However, depending on how you're planning to use the results of your query, it's often more convenient to use other Go types.

The `encoding/json` rules are used for converting individual JSON-encoded fields from a GraphQL response into Go values. See https://godoc.org/encoding/json#Unmarshal for details. The [`json.Unmarshaler`](https://godoc.org/encoding/json#Unmarshaler) interface is respected.

That means you can simplify the earlier query by using predeclared Go types:

```Go
// import "time"

var query struct {
	Viewer struct {
		Login          string    // E.g., "gopher".
		CreatedAt      time.Time // E.g., time.Date(2017, 5, 26, 21, 17, 14, 0, time.UTC).
		IsBountyHunter bool      // E.g., true.
		BioHTML        string    // E.g., `I am learning <a href="https://graphql.org">GraphQL</a>!`.
		WebsiteURL     string    // E.g., "https://golang.org".
	}
}
// Call client.Query() and use results in query...
```

The [`DateTime`](https://docs.github.com/en/graphql/reference/scalars#datetime) scalar is described as "an ISO-8601 encoded UTC date string". If you wanted to fetch in that form without parsing it into a `time.Time`, you can use the `string` type. For example, this would work:

```Go
// import "html/template"

type MyBoolean bool

var query struct {
	Viewer struct {
		Login          string        // E.g., "gopher".
		CreatedAt      string        // E.g., "2017-05-26T21:17:14Z".
		IsBountyHunter MyBoolean     // E.g., MyBoolean(true).
		BioHTML        template.HTML // E.g., template.HTML(`I am learning <a href="https://graphql.org">GraphQL</a>!`).
		WebsiteURL     template.URL  // E.g., template.URL("https://golang.org").
	}
}
// Call client.Query() and use results in query...
```

### Arguments and Variables

Often, you'll want to specify arguments on some fields. You can use the `graphql` struct field tag for this.

For example, to make the following GraphQL query:

```GraphQL
{
	repository(owner: "octocat", name: "Hello-World") {
		description
	}
}
```

You can define this variable:

```Go
var q struct {
	Repository struct {
		Description string
	} `graphql:"repository(owner: \"octocat\", name: \"Hello-World\")"`
}
```

Then call `client.Query`:

```Go
err := client.Query(context.Background(), &q, nil)
if err != nil {
	// Handle error.
}
fmt.Println(q.Repository.Description)

// Output:
// My first repository on GitHub!
```

However, that'll only work if the arguments are constant and known in advance. Otherwise, you will need to make use of variables. Replace the constants in the struct field tag with variable names:

```Go
// fetchRepoDescription fetches description of repo with owner and name.
func fetchRepoDescription(ctx context.Context, owner, name string) (string, error) {
	var q struct {
		Repository struct {
			Description string
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
```

When sending variables to GraphQL, you need to use exact types that match GraphQL scalar types, otherwise the GraphQL server will return an error.

So, define a `variables` map with their values that are converted to GraphQL scalar types:

```Go
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(name),
	}
```

Finally, call `client.Query` providing `variables`:

```Go
	err := client.Query(ctx, &q, variables)
	return q.Repository.Description, err
}
```

### Inline Fragments

Some GraphQL queries contain inline fragments. You can use the `graphql` struct field tag to express them.

For example, to make the following GraphQL query:

```GraphQL
{
	repositoryOwner(login: "github") {
		login
		... on Organization {
			description
		}
		... on User {
			bio
		}
	}
}
```

You can define this variable:

```Go
var q struct {
	RepositoryOwner struct {
		Login        string
		Organization struct {
			Description string
		} `graphql:"... on Organization"`
		User struct {
			Bio string
		} `graphql:"... on User"`
	} `graphql:"repositoryOwner(login: \"github\")"`
}
```

Alternatively, you can define the struct types corresponding to inline fragments, and use them as embedded fields in your query:

```Go
type (
	OrganizationFragment struct {
		Description string
	}
	UserFragment struct {
		Bio string
	}
)

var q struct {
	RepositoryOwner struct {
		Login                string
		OrganizationFragment `graphql:"... on Organization"`
		UserFragment         `graphql:"... on User"`
	} `graphql:"repositoryOwner(login: \"github\")"`
}
```

Then call `client.Query`:

```Go
err := client.Query(context.Background(), &q, nil)
if err != nil {
	// Handle error.
}
fmt.Println(q.RepositoryOwner.Login)
fmt.Println(q.RepositoryOwner.Description)
fmt.Println(q.RepositoryOwner.Bio)

// Output:
// github
// How people build software.
//
```

### Pagination

Imagine you wanted to get a complete list of comments in an issue, and not just the first 10 or so. To do that, you'll need to perform multiple queries and use pagination information. For example:

```Go
type comment struct {
	Body   string
	Author struct {
		Login     string
		AvatarURL string `graphql:"avatarUrl(size: 72)"`
	}
	ViewerCanReact bool
}
var q struct {
	Repository struct {
		Issue struct {
			Comments struct {
				Nodes    []comment
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"comments(first: 100, after: $commentsCursor)"` // 100 per page.
		} `graphql:"issue(number: $issueNumber)"`
	} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
}
variables := map[string]interface{}{
	"repositoryOwner": githubv4.String(owner),
	"repositoryName":  githubv4.String(name),
	"issueNumber":     githubv4.Int(issue),
	"commentsCursor":  (*githubv4.String)(nil), // Null after argument to get first page.
}

// Get comments from all pages.
var allComments []comment
for {
	err := client.Query(ctx, &q, variables)
	if err != nil {
		return err
	}
	allComments = append(allComments, q.Repository.Issue.Comments.Nodes...)
	if !q.Repository.Issue.Comments.PageInfo.HasNextPage {
		break
	}
	variables["commentsCursor"] = githubv4.NewString(q.Repository.Issue.Comments.PageInfo.EndCursor)
}
```

There is more than one way to perform pagination. Consider additional fields inside [`PageInfo`](https://docs.github.com/en/graphql/reference/objects#pageinfo) object.

### Mutations

Mutations often require information that you can only find out by performing a query first. Let's suppose you've already done that.

For example, to make the following GraphQL mutation:

```GraphQL
mutation($input: AddReactionInput!) {
	addReaction(input: $input) {
		reaction {
			content
		}
		subject {
			id
		}
	}
}
variables {
	"input": {
		"subjectId": "MDU6SXNzdWUyMTc5NTQ0OTc=",
		"content": "HOORAY"
	}
}
```

You can define:

```Go
var m struct {
	AddReaction struct {
		Reaction struct {
			Content githubv4.ReactionContent
		}
		Subject struct {
			ID githubv4.ID
		}
	} `graphql:"addReaction(input: $input)"`
}
input := githubv4.AddReactionInput{
	SubjectID: targetIssue.ID, // ID of the target issue from a previous query.
	Content:   githubv4.ReactionContentHooray,
}
```

Then call `client.Mutate`:

```Go
err := client.Mutate(context.Background(), &m, input, nil)
if err != nil {
	// Handle error.
}
fmt.Printf("Added a %v reaction to subject with ID %#v!\n", m.AddReaction.Reaction.Content, m.AddReaction.Subject.ID)

// Output:
// Added a HOORAY reaction to subject with ID "MDU6SXNzdWUyMTc5NTQ0OTc="!
```

Directories
-----------

| Path                                                                                       | Synopsis                                                                            |
|--------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| [example/githubv4dev](https://pkg.go.dev/github.com/shurcooL/githubv4/example/githubv4dev) | githubv4dev is a test program currently being used for developing githubv4 package. |

License
-------

-	[MIT License](LICENSE)

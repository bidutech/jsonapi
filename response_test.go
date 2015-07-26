package jsonapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"
)

type Blog struct {
	Id               int               `jsonapi:"primary,blogs"`
	Links            map[string]string `jsonapi:"links,top"`
	Title            string            `jsonapi:"attr,title"`
	Posts            []*Post           `jsonapi:"relation,posts"`
	PostsLinks       map[string]string `jsonapi:"links,posts"`
	CurrentPost      *Post             `jsonapi:"relation,current_post"`
	CurrentPostLinks map[string]string `jsonapi:"links,current_post"`
	CurrentPostId    int               `jsonapi:"attr,current_post_id"`
	CreatedAt        time.Time         `jsonapi:"attr,created_at"`
	ViewCount        int               `jsonapi:"attr,view_count"`
}

type Post struct {
	Blog
	Id                 int               `jsonapi:"primary,posts"`
	Links              map[string]string `jsonapi:"links,top"`
	BlogId             int               `jsonapi:"attr,blog_id"`
	Title              string            `jsonapi:"attr,title"`
	Body               string            `jsonapi:"attr,body"`
	Comments           []*Comment        `jsonapi:"relation,comments"`
	CommentsLinks      map[string]string `jsonapi:"links,comments"`
	LatestComment      *Comment          `jsonapi:"relation,latest_comment"`
	LatestCommentLinks map[string]string `jsonapi:"links,latest_comment`
}

type Comment struct {
	Id     int    `jsonapi:"primary,comments"`
	PostId int    `jsonapi:"attr,post_id"`
	Body   string `jsonapi:"attr,body"`
}

const linkTemplateBlogs = "https://localhost:8080/api/v1/blogs"
const linkTemplateBlogsPosts = "https://localhost:8080/api/v1/blogs/posts?blog_id=%s"

func TestMarshalOnePayloadWithExtras(t *testing.T) {
	testModel := testBlog()
	buf := bytes.NewBuffer(nil)

	nextPage := "2"

	if err := MarshalOnePayloadWithExtras(buf, testModel, func(c *ApiExtras) {
		c.AddRootLink("current", linkTemplateBlogs)
		c.AddRootLink("next", linkTemplateBlogs+"?page="+nextPage)
		c.AddRelationshipLink("current", "posts", "posts", "blogs", fmt.Sprintf(linkTemplateBlogsPosts, "{blogs.id}"))
		c.AddRelationshipLink("next", "posts", "posts", "blogs", fmt.Sprintf(linkTemplateBlogsPosts+"&page=%s", "{blogs.id}", nextPage))
		c.AddRelationshipLink("current", "comments", "comments", "posts", "https://localhost:3000/api/v1/blogs/posts/comments?blog_id={blogs.id}&post_id={posts.id}")
	}); err != nil {
		t.Fatal(err)
	}

	payload := new(OnePayload)

	if err := json.NewDecoder(buf).Decode(payload); err != nil {
		t.Fatal(err)
	}

	if len(payload.Links) != 2 {
		t.Fatalf("wrong number of links")
	}

	if !regexp.MustCompile(`api/v1`).Match([]byte(payload.Links["current"])) {
		t.Fatalf("did not assign current link to correct name")
	}

	links := make(map[string]string)

	posts := payload.Data.Relationships["posts"].(map[string]interface{})
	lnks, ok := posts["links"].(map[string]interface{})
	if !ok {
		t.Fatalf("posts is missing links: %#v", posts)
	}
	for linkName, link := range lnks {
		links[linkName] = link.(string)
	}

	if len(links) != 2 {
		t.Fatalf("relationship links not serialized")
	}

	if links["current"] != "https://localhost:8080/api/v1/blogs/posts?blog_id%3D5" {
		t.Fatalf("posts relationship current link not set")
	}

	if links["next"] != "https://localhost:8080/api/v1/blogs/posts?blog_id%3D5%26page%3D2" {
		t.Fatalf("posts relationship next link not set")
	}
}

func TestMalformedTagResposne(t *testing.T) {
	testModel := &BadModel{}
	out := bytes.NewBuffer(nil)
	err := MarshalOnePayload(out, testModel)

	if err == nil {
		t.Fatalf("Did not error out with wrong number of arguments in tag")
	}

	r := regexp.MustCompile(`two few arguments`)

	if !r.Match([]byte(err.Error())) {
		t.Fatalf("The err was not due two two few arguments in a tag")
	}
}

func TestHasPrimaryAnnotation(t *testing.T) {
	testModel := &Blog{
		Id:        5,
		Title:     "Title 1",
		CreatedAt: time.Now(),
	}

	out := bytes.NewBuffer(nil)
	if err := MarshalOnePayload(out, testModel); err != nil {
		t.Fatal(err)
	}

	resp := new(OnePayload)

	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	data := resp.Data

	if data.Type != "blogs" {
		t.Fatalf("type should have been blogs, got %s", data.Type)
	}

	if data.Id != "5" {
		t.Fatalf("Id not transfered")
	}
}

func TestSupportsAttributes(t *testing.T) {
	testModel := &Blog{
		Id:        5,
		Title:     "Title 1",
		CreatedAt: time.Now(),
	}

	out := bytes.NewBuffer(nil)
	if err := MarshalOnePayload(out, testModel); err != nil {
		t.Fatal(err)
	}

	resp := new(OnePayload)
	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	data := resp.Data

	if data.Attributes == nil {
		t.Fatalf("Expected attributes")
	}

	if data.Attributes["title"] != "Title 1" {
		t.Fatalf("Attributes hash not populated using tags correctly")
	}
}

func TestOmitsZeroTimes(t *testing.T) {
	testModel := &Blog{
		Id:        5,
		Title:     "Title 1",
		CreatedAt: time.Time{},
	}

	out := bytes.NewBuffer(nil)
	if err := MarshalOnePayload(out, testModel); err != nil {
		t.Fatal(err)
	}

	resp := new(OnePayload)
	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	data := resp.Data

	if data.Attributes == nil {
		t.Fatalf("Expected attributes")
	}

	if data.Attributes["created_at"] != nil {
		t.Fatalf("Created at was serialized even though it was a zero Time")
	}
}

func TestRelations(t *testing.T) {
	testModel := testBlog()

	out := bytes.NewBuffer(nil)
	if err := MarshalOnePayload(out, testModel); err != nil {
		t.Fatal(err)
	}

	resp := new(OnePayload)
	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	relations := resp.Data.Relationships

	if relations == nil {
		t.Fatalf("Relationships were not materialized")
	}

	if relations["posts"] == nil {
		t.Fatalf("Posts relationship was not materialized")
	}

	if relations["current_post"] == nil {
		t.Fatalf("Current post relationship was not materialized")
	}

	if len(relations["posts"].(map[string]interface{})["data"].([]interface{})) != 2 {
		t.Fatalf("Did not materialize two posts")
	}
}

func TestNoRelations(t *testing.T) {
	testModel := &Blog{Id: 1, Title: "Title 1", CreatedAt: time.Now()}

	out := bytes.NewBuffer(nil)
	if err := MarshalOnePayload(out, testModel); err != nil {
		t.Fatal(err)
	}

	resp := new(OnePayload)
	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	if resp.Included != nil {
		t.Fatalf("Encoding json response did not omit included")
	}
}

func TestMarshalMany(t *testing.T) {
	data := []interface{}{
		&Blog{
			Id:        5,
			Title:     "Title 1",
			CreatedAt: time.Now(),
			Posts: []*Post{
				&Post{
					Id:    1,
					Title: "Foo",
					Body:  "Bar",
				},
				&Post{
					Id:    2,
					Title: "Fuubar",
					Body:  "Bas",
				},
			},
			CurrentPost: &Post{
				Id:    1,
				Title: "Foo",
				Body:  "Bar",
			},
		},
		&Blog{
			Id:        6,
			Title:     "Title 2",
			CreatedAt: time.Now(),
			Posts: []*Post{
				&Post{
					Id:    3,
					Title: "Foo",
					Body:  "Bar",
				},
				&Post{
					Id:    4,
					Title: "Fuubar",
					Body:  "Bas",
				},
			},
			CurrentPost: &Post{
				Id:    4,
				Title: "Foo",
				Body:  "Bar",
			},
		},
	}

	out := bytes.NewBuffer(nil)
	if err := MarshalManyPayload(out, data); err != nil {
		t.Fatal(err)
	}

	resp := new(ManyPayload)
	if err := json.NewDecoder(out).Decode(resp); err != nil {
		t.Fatal(err)
	}

	d := resp.Data

	if len(d) != 2 {
		t.Fatalf("data should have two elements")
	}
}

func testBlog() *Blog {
	return &Blog{
		Id:        5,
		Title:     "Title 1",
		CreatedAt: time.Now(),
		Posts: []*Post{
			&Post{
				Id:    1,
				Title: "Foo",
				Body:  "Bar",
				Comments: []*Comment{
					&Comment{
						Id:   1,
						Body: "foo",
					},
					&Comment{
						Id:   2,
						Body: "bar",
					},
				},
				LatestComment: &Comment{
					Id:   1,
					Body: "foo",
				},
			},
			&Post{
				Id:    2,
				Title: "Fuubar",
				Body:  "Bas",
				Comments: []*Comment{
					&Comment{
						Id:   1,
						Body: "foo",
					},
					&Comment{
						Id:   3,
						Body: "bas",
					},
				},
				LatestComment: &Comment{
					Id:   1,
					Body: "foo",
				},
			},
		},
		CurrentPost: &Post{
			Id:    1,
			Title: "Foo",
			Body:  "Bar",
			Comments: []*Comment{
				&Comment{
					Id:   1,
					Body: "foo",
				},
				&Comment{
					Id:   2,
					Body: "bar",
				},
			},
			LatestComment: &Comment{
				Id:   1,
				Body: "foo",
			},
		},
	}
}

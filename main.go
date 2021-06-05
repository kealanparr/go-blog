package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgconn" // SQL driver
	"github.com/jackc/pgx/v4" // SQL driver
)

type Post struct {
	Header  string // The header the Post
	Content string // The content of the Post
	Slug    string // The url we access this Post on
}

// Type used to parse templates on the homepage
type HomePage struct {
	Posts []Post
}

// Type used for templating to alert the user if a CRUD operation failed or succeeded
type CRUDResult struct {
	Message string
}

const (
	HOME         = "/home/"
	POST         = "/post/"
	EDIT         = "/edit/"
	NEW          = "/new/"
	SAVE         = "/save/"
	DELETE       = "/delete/"
	DATABASE_URL = "postgres://postgres:shush@localhost:5432/blog" //postgres://username:password@localhost:5432/database_name
)

var (
	HomePageData        = HomePage{}
	needsToPollDataBase = true // Set to true for first load

	routingWhiteList = map[string]func(http.ResponseWriter, *http.Request){
		HOME:   homeHandler,
		NEW:    newPostHandler,
		SAVE:   saveHandler,
		EDIT:   editHandler,
		DELETE: deleteHandler,
		POST:   postHandler,
	}
)

func init() {
	initialiseDBConnection()
}

func initialiseDBConnection() (conn *pgx.Conn) {
	conn, err := pgx.Connect(context.Background(), DATABASE_URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	return conn
}

func main() {
	http.HandleFunc("/", makeHandler(homeHandler))
	fmt.Println("Server starting on port:8080....")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func makeHandler(handlerFn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		re := regexp.MustCompile(`\/(.*?)\/`)
		endPoint := re.FindStringSubmatch(r.URL.Path)

		if len(endPoint) > 0 && routingWhiteList[endPoint[0]] != nil {
			routingWhiteList[endPoint[0]](w, r)
		} else {
			// Redirect the user back to the homepage if they're going to 404
			http.Redirect(w, r, HOME, http.StatusFound)
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {

	if needsToPollDataBase {
		HomePageData.Posts = HomePageData.Posts[:0]
		// Need to poll as we've added new posts, or loaded for the first time
		conn := initialiseDBConnection()
		defer conn.Close(context.Background())

		rows, err := conn.Query(context.Background(), "SELECT header, content, slug FROM posts;")
		if err != nil {
			log.Fatal(err)
		}

		for rows.Next() {
			var p Post
			err := rows.Scan(&p.Header, &p.Content, &p.Slug)
			if err != nil {
				log.Fatal(err)
			}
			HomePageData.Posts = append(HomePageData.Posts, p)
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
		needsToPollDataBase = false
	}

	t, tmplerr := template.ParseFiles("views/home.html")
	if tmplerr != nil {
		log.Fatal(tmplerr)
		return
	}

	t.Execute(w, HomePageData)
}

func newPostHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/newPost.html")
}

func saveHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		r.ParseForm()
		header := r.PostFormValue("header")
		content := r.PostFormValue("content")
		slug := r.PostFormValue("slug")

		updateDatabase(w, r.URL.Path, Post{Header: header, Content: content, Slug: slug})
	}
}

func updateDatabase(w http.ResponseWriter, urlPath string, post Post) {

	conn := initialiseDBConnection()
	defer conn.Close(context.Background())

	var rows pgconn.CommandTag
	var err error

	if strings.Contains(urlPath, "update") {
		rows, err = conn.Exec(context.Background(), "UPDATE posts SET (header, content) = ($1, $2) WHERE slug = $3;", post.Header, post.Content, post.Slug)
	} else if strings.Contains(urlPath, "add") {
		rows, err = conn.Exec(context.Background(), "INSERT INTO posts (header, content, slug) VALUES ($1, $2, $3) ON CONFLICT (slug) DO NOTHING;", post.Header, post.Content, post.Slug) // On Conflict used to ensure we dont dupe our slugs
	} else if strings.Contains(urlPath, "del") {
		rows, err = conn.Exec(context.Background(), "DELETE FROM posts WHERE slug=$1;", post.Slug)
	}

	resultHTML(w, rows, err)
}

func resultHTML(w http.ResponseWriter, rows pgconn.CommandTag, err error) {
	if rows.RowsAffected() == 0 {
		http.Error(w, "Failed to save the post.", http.StatusInternalServerError)
		generateResulTemplate(w, &CRUDResult{Message: "Sorry! This attempt to add a new post failed"})
		log.Fatal(err)
	}

	if err != nil {
		http.Error(w, "Failed to save the post.", http.StatusInternalServerError)
		generateResulTemplate(w, &CRUDResult{Message: "Sorry! This attempt to add a new post failed"})
		log.Fatal(err)
	} else {
		// We succesfully added/updated/deleted posts, we need to poll the DB
		generateResulTemplate(w, &CRUDResult{Message: "Thanks for editing the blog, and sharing your expertise!"})
		needsToPollDataBase = true
	}
}

func generateResulTemplate(w http.ResponseWriter, result *CRUDResult) {
	t, tmplerr := template.ParseFiles("views/result.html")
	if tmplerr != nil {
		log.Fatal(tmplerr)
	}
	t.Execute(w, result)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/edit.html")
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/delete.html")
}

func postHandler(w http.ResponseWriter, r *http.Request) {

	conn := initialiseDBConnection()
	defer conn.Close(context.Background())

	slug := strings.Split(strings.ToLower(r.URL.Path), "/post/")[1]
	rows, err := conn.Query(context.Background(), "SELECT header, content, slug FROM POSTS WHERE slug = $1;", slug)
	if err != nil {
		log.Fatal(err)
	}

	var p Post
	for rows.Next() {
		err := rows.Scan(&p.Header, &p.Content, &p.Slug)
		if err != nil {
			log.Fatal(err)
		}
	}

	t, tmplerr := template.ParseFiles("views/post.html")
	if tmplerr != nil {
		log.Fatal(tmplerr)
		return
	}

	t.Execute(w, p)
}

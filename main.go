package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./forum.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		log.Fatalf("Tablolar oluşturulamadı: %v", err)
	}

	log.Println("Database Tables Created Successfully!")

	http.HandleFunc("/", logInPageHandler(db))
	http.HandleFunc("/register", registerPageHandler(db))
	http.HandleFunc("/guestLogin", guestLoginHandler(db))
	http.HandleFunc("/forum", forumPageHandler(db))
	http.HandleFunc("/createPost", authorize(createPostHandler(db)))
	http.HandleFunc("/like", authorize(likeHandler(db)))
	http.HandleFunc("/dislike", authorize(dislikeHandler(db)))
	http.HandleFunc("/comment", authorize(commentHandler(db)))
	http.ListenAndServe(":8080", nil)
}

func createTables(db *sql.DB) error {
	createUsersTable := `
    CREATE TABLE IF NOT EXISTS Users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT UNIQUE NOT NULL,
        username TEXT UNIQUE NOT NULL,
        password TEXT NOT NULL
    );`

	createPostsTable := `
    CREATE TABLE IF NOT EXISTS Posts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        content TEXT NOT NULL,
        user_id INTEGER NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES Users(id)
    );`

	createCommentsTable := `
    CREATE TABLE IF NOT EXISTS Comments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        user_id INTEGER NOT NULL,
        post_id INTEGER NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES Users(id),
        FOREIGN KEY (post_id) REFERENCES Posts(id)
    );`

	createLikesTable := `
    CREATE TABLE IF NOT EXISTS Likes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        post_id INTEGER NOT NULL,
        UNIQUE (user_id, post_id),
        FOREIGN KEY (user_id) REFERENCES Users(id),
        FOREIGN KEY (post_id) REFERENCES Posts(id)
    );`

	createDislikesTable := `
    CREATE TABLE IF NOT EXISTS Dislikes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        post_id INTEGER NOT NULL,
        UNIQUE (user_id, post_id),
        FOREIGN KEY (user_id) REFERENCES Users(id),
        FOREIGN KEY (post_id) REFERENCES Posts(id)
    );`

	_, err := db.Exec(createUsersTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createPostsTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createCommentsTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createLikesTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createDislikesTable)
	if err != nil {
		return err
	}

	return nil
}

func registerUser(db *sql.DB, email, username, password string) error {
	query := "INSERT INTO Users (email, username, password) VALUES (?, ?, ?)"
	_, err := db.Exec(query, email, username, password)
	return err
}

func registerPageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tmpl := template.Must(template.ParseFiles("register.html"))
			tmpl.Execute(w, nil)
			return
		}

		if r.Method == http.MethodPost {
			email := r.FormValue("email")
			username := r.FormValue("username")
			password := r.FormValue("password")

			err := registerUser(db, email, username, password)
			if err != nil {
				http.Error(w, "Unable to register user", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}
}

func logInPageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tmpl := template.Must(template.ParseFiles("login.html"))
			tmpl.Execute(w, nil)
			return
		}

		if r.Method == http.MethodPost {
			email := r.FormValue("email")
			password := r.FormValue("password")

			valid, err := authenticateUser(db, email, password)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			if valid {
				setSession(w, "user", email)
				http.Redirect(w, r, "/forum", http.StatusSeeOther)
			} else {
				http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
			}
		}
	}
}

func authenticateUser(db *sql.DB, email, password string) (bool, error) {
	var dbEmail, dbPassword string
	err := db.QueryRow("SELECT email, password FROM Users WHERE email = ?", email).Scan(&dbEmail, &dbPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("user not found")
		}
		return false, err
	}

	if password == dbPassword {
		return true, nil
	}
	return false, fmt.Errorf("invalid password")
}

type Post struct {
	ID        int
	Title     string
	Content   string
	Username  string
	CreatedAt string
	Likes     int
	Dislikes  int
	Comments  []Comment
}

type Comment struct {
	Content   string
	Username  string
	CreatedAt string
}

func forumPageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			posts, err := fetchPosts(db)
			if err != nil {
				http.Error(w, "Unable to load posts", http.StatusInternalServerError)
				return
			}

			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, struct{ Posts []Post }{Posts: posts})
		}
	}
}

func fetchPosts(db *sql.DB) ([]Post, error) {
	rows, err := db.Query(`
        SELECT Posts.id, Posts.title, Posts.content, Users.username, Posts.created_at,
               (SELECT COUNT(*) FROM Likes WHERE Likes.post_id = Posts.id) as likes,
               (SELECT COUNT(*) FROM Dislikes WHERE Dislikes.post_id = Posts.id) as dislikes
        FROM Posts
        JOIN Users ON Posts.user_id = Users.id
        ORDER BY Posts.created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Content, &post.Username, &post.CreatedAt, &post.Likes, &post.Dislikes); err != nil {
			return nil, err
		}

		comments, err := fetchComments(db, post.ID)
		if err != nil {
			return nil, err
		}
		post.Comments = comments

		posts = append(posts, post)
	}
	return posts, nil
}

func fetchComments(db *sql.DB, postID int) ([]Comment, error) {
	rows, err := db.Query(`
        SELECT Comments.content, Users.username, Comments.created_at
        FROM Comments
        JOIN Users ON Comments.user_id = Users.id
        WHERE Comments.post_id = ?
        ORDER BY Comments.created_at ASC
    `, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.Content, &comment.Username, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func createPostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			title := r.FormValue("title")
			content := r.FormValue("content")
			userID := 1

			err := createPost(db, title, content, userID)
			if err != nil {
				http.Error(w, "Unable to create post", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/forum", http.StatusSeeOther)
		}
	}
}

func createPost(db *sql.DB, title, content string, userID int) error {
	query := "INSERT INTO Posts (title, content, user_id) VALUES (?, ?, ?)"
	_, err := db.Exec(query, title, content, userID)
	return err
}

func likeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postID := r.FormValue("post_id")
			userID := 1

			err := likePost(db, postID, userID)
			if err != nil {
				http.Error(w, "Unable to like post", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/forum", http.StatusSeeOther)
		}
	}
}

func likePost(db *sql.DB, postID string, userID int) error {
	var existingID int
	err := db.QueryRow("SELECT id FROM Likes WHERE post_id = ? AND user_id = ?", postID, userID).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if existingID == 0 {
		_, err = db.Exec("INSERT INTO Likes (post_id, user_id) VALUES (?, ?)", postID, userID)
		if err != nil {
			return err
		}

		_, err = db.Exec("DELETE FROM Dislikes WHERE post_id = ? AND user_id = ?", postID, userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func dislikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postID := r.FormValue("post_id")
			userID := 1

			err := dislikePost(db, postID, userID)
			if err != nil {
				http.Error(w, "Unable to dislike post", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/forum", http.StatusSeeOther)
		}
	}
}

func dislikePost(db *sql.DB, postID string, userID int) error {
	var existingID int
	err := db.QueryRow("SELECT id FROM Dislikes WHERE post_id = ? AND user_id = ?", postID, userID).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if existingID == 0 {
		_, err = db.Exec("INSERT INTO Dislikes (post_id, user_id) VALUES (?, ?)", postID, userID)
		if err != nil {
			return err
		}

		_, err = db.Exec("DELETE FROM Likes WHERE post_id = ? AND user_id = ?", postID, userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func commentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postID := r.FormValue("post_id")
			content := r.FormValue("comment")
			userID := 1

			err := createComment(db, content, postID, userID)
			if err != nil {
				http.Error(w, "Unable to add comment", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/forum", http.StatusSeeOther)
		}
	}
}

func createComment(db *sql.DB, content, postID string, userID int) error {
	query := "INSERT INTO Comments (content, post_id, user_id) VALUES (?, ?, ?)"
	_, err := db.Exec(query, content, postID, userID)
	return err
}

func guestLoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {

			_ = db.Ping()

			setSession(w, "user", "guest")
			http.Redirect(w, r, "/forum", http.StatusSeeOther)
		}
	}
}

func authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getSession(r, "user")
		if user == "guest" {
			http.Error(w, "Guest users cannot perform this action", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func setSession(w http.ResponseWriter, key, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:  key,
		Value: value,
		Path:  "/",
	})
}

func getSession(r *http.Request, key string) string {
	cookie, err := r.Cookie(key)
	if err != nil {
		return ""
	}
	return cookie.Value
}

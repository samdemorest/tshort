package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

/*
 * This is the configuration for the application.
 * Everything should come from a json configuration file.
 * The json configuration file will be loaded automatically as "config.json"
 */
type Config struct {
	Db_host     string
	Db_port     string
	Db_name     string
	Db_user     string
	Db_pass     string
	Listen_port string
	Hash_len    int
}

/*
 * Type used by net/template
 */
type Page struct {
	Title string
	Body  string
}

/*
 * Response to requests submitted via API
 */
type API_response struct {
	URL string
}

/*
 * Kicks off the program, just serves HTTP.
 */
func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)

}

/*
 * This function handles incoming http requests.
 */
func handler(w http.ResponseWriter, r *http.Request) {

	// Get and read configuration from json file.
	conf := read_config("config.json")

	// Prepare to open database
	db_string := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		conf.Db_user,
		conf.Db_pass,
		conf.Db_host,
		conf.Db_name)

	// Open database using Sprintf'd string from above
	db, err := sql.Open("postgres", db_string)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	/*
	 * Switch will handle GET/POST. For this implementation, PUT and DELETE are
	 * things that never need to face the user.
	 */
	switch r.Method {
	case "GET":
		switch r.URL.Path {
		// Case for the root
		case "/":
			p := &Page{Title: "t-short: the link un-longerer", Body: ""}
			t, err := template.ParseFiles("index.html")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			t.Execute(w, p)
			break
		// Case for any path requested
		default:
			path := r.URL.Path[1:]
			serve_redirect(path, db, w, r)
		}
		break

	// Case for POST (submission) request
	case "POST":
		// Gets fields from submitted data
		url := r.PostFormValue("url")
		method := r.PostFormValue("method")
		ip := strings.Split(r.RemoteAddr, ":")[0]

		// Prepend 'http://' if not submitted with a link. This may cause some pages
		// to be viewed without TLS, but prepending 'https://' programmatically is
		// likely to cause certificate errors in the browser.
		if strings.Index(url, "http") != 0 {
			s := []string{"http://", url}
			url = strings.Join(s, "")
		}

		// Create an ID based on the hash of the URL
		id := create_link(url, ip, conf.Hash_len, db)

		// Join strings together to form a complete URL
		s := []string{"http://", r.Host, "/", id}
		link := strings.Join(s, "")

		// If posted from web form
		if method == "web" {
			// Write http response (right now very minimal) to be viewed in a browser
			// instead of the JSON that is returned by an API call.
			p := &Page{Title: link}
			t, err := template.ParseFiles("response.html")
			if err != nil {
				fmt.Println(err)
			}
			t.Execute(w, p)
		} else { // API call not web interaction

			// Format response as JSON string
			response := API_response{
				URL: link,
			}
			// Write the formatted JSON string to the caller
			formatted, err := json.Marshal(response)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Fprintf(w, string(formatted))
		}
	} // end switch
	db.Close()
}

/*
 * This function insets a link into the database, generating a stort string by
 * which the URL will be looked up in the future.
 */
func create_link(full_url string, ipaddr string, leng int, db *sql.DB) string {
	var link string

	// Default length of a link. Incremented in case of collission.
	var hash_len int = leng

	// Initialize and get the shasum of the URL as passed to the function
	hash := sha256.New()
	hash.Write([]byte(full_url))
	shasum := base64.URLEncoding.EncodeToString(hash.Sum(nil))

	// Shorten the shasum to a manageable length
	shortsum := string(shasum[0:hash_len])
	//fmt.Println("About to make query")
	if url_exists(full_url, db) {
		link = query_id_by_url(full_url, db)
	} else {
		// Make sure the truncated hash doesn't collide
		for !check_uniqueness(shortsum, full_url, db) {
			// truncated shasum collided.
			hash_len += 1
			shortsum = string(shasum[0:hash_len])
		}
		// Is unique
		fmt.Println("Inserting")
		link = shortsum
		insert_link(shortsum, full_url, ipaddr, db)
	}

	return link
}

/*
 * This function determines whether a given URL has been submitted to the
 * application and added to the database already
 */
func url_exists(url string, db *sql.DB) bool {
	row, err := db.Query("SELECT id FROM pages WHERE url = $1", url)
	defer row.Close()
	if row.Next() {
		return true
	} // end row.Next()
	if err != nil {
		fmt.Println(err)
	}
	return false
}

/*
 * This function determines whether a given shortener string exists, or if a
 * status of 404 should be returned. If the string exists, the function returns
 * true.
 */
func id_exists(id string, db *sql.DB) bool {
	row, err := db.Query("SELECT url FROM pages WHERE id = $1", id)
	defer row.Close()
	if row.Next() {
		return true
	} // end row.Next()
	if err != nil {
		fmt.Println(err)
	}
	return false
}

/*
 * This function queries the database to find the shortened string associated
 * with a particular URL.
 *
 * Returns the string that is the ID associated with the URL.
 */
func query_id_by_url(url string, db *sql.DB) string {
	var link string
	// Make the query
	row, err := db.Query("SELECT id FROM pages WHERE url = $1", url)
	defer row.Close()

	// If exists a result from the query
	if row.Next() {
		err := row.Scan(&link)
		if err != nil {
			fmt.Println(err)
		}
	} else { // end row.Next()

		// The program somehow got to this function without verifying the existence
		// of this particular ID. Return nil string to avoid crash in this case.
		return ""
	}

	// If error with db query, handle that here.
	if err != nil {
		fmt.Println(err)
	}
	return link
}

/*
 * This function queries the database for the URL associated with a particular
 * ID string.
 *
 * Returns the string that is the URL associated with the ID.
 */
func query_url_by_id(id string, db *sql.DB) string {
	var link string

	// Make the query
	row, err := db.Query("SELECT url FROM pages WHERE id = $1", id)
	defer row.Close()

	// If a row is returned by the db query, process it.
	if row.Next() {
		err := row.Scan(&link)
		if err != nil {
			fmt.Println(err)
		}
	} else { // end row.Next()

		// Handle the condition that there may not be a url associated. Program
		// should never make it to this point, but it shouldn't crash if it does.
		return ""
	}
	// If error with db query, handle that here.
	if err != nil {
		fmt.Println(err)
	}
	return link
}

/*
 * This function checks whether an entry already exists in the database for a
 * given ID string. This could be the case in two circumstances:
 * 1) There was a collision in the truncated hash values
 * 2) The URL has already been submitted
 *
 * Returns a boolean, true if unique, false if not.
 */
func check_uniqueness(id string, url string, db *sql.DB) bool {

	// Make the database query here.
	row, err := db.Query("SELECT url FROM pages WHERE id = $1", id)
	defer row.Close()

	// If rows are returned, then this is not unique
	if row.Next() {
		return false
	} // end row.Next()

	// Handle error with query here.
	if err != nil {
		os.Exit(1)
	}
	return true
}

/*
 * This function inserts a link associated with an ID into the database for
 * later recall.
 */
func insert_link(id string, url string, origin string, db *sql.DB) {

	// Prepare the insert statement to help protect against SQL injection.
	trans, err := db.Prepare(
		"INSERT INTO pages (ID,URL,ORIGIN) VALUES($1, $2, $3)")

	// Handle error with insert transaction preparation
	if err != nil {
		fmt.Println(err)
	}

	// Execute the prepared query
	_, err = trans.Exec(id, url, origin)
	if err != nil {
		fmt.Println(err)
	}
}

/*
 * This function serves the redirect response from the web application when you
 * follow the link provided in the response. If the link exists, it redirects to
 * the URL associated with the ID. If the link doesn't exist, a 404 is served.
 */
func serve_redirect(id string, db *sql.DB, w http.ResponseWriter,
	r *http.Request) {

	// Check if the url for the provided ID exists. Redirect or 404.
	if id_exists(id, db) {
		url := query_url_by_id(id, db)
		http.Redirect(w, r, url, 307)
	} else { // ID doesn't exist
		http.NotFound(w, r)
	}
}

/*
 * This function reads the json configuration file in the project directory.
 * There can only be "config.json". At this time, changing the names of
 * configuration files is not supported.
 */
func read_config(conffile string) *Config {
	// Allocate the struct
	configuration := new(Config)

	// Read the contents of the config file. Assuming enough RAM for short file.
	contents, err := ioutil.ReadFile(conffile)
	if err != nil {
		fmt.Printf("Could not read configuration file: ")
		fmt.Println(err)
		os.Exit(1)
	}

	// Read the file contents into a Config struct.
	err = json.Unmarshal(contents, &configuration)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return configuration
}

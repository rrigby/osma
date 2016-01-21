package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/schema"
	"golang.org/x/crypto/bcrypt"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Globals
var templatePath = "tmpl/"
var templates = template.Must(
	template.ParseGlob(templatePath + "*"))

var adminUsername = "admin"
var adminPassword = "secretpass"

var dbName = "osma"
var dbSession = &mgo.Session{}

var websiteConfig = &Config{}

// Config Struct stores site information
type Config struct {
	Ident     int
	FullName  string
	ShortName string
	Footer    string
}

type Admin struct {
	Username       string
	HashedPassword []byte
}

// Page Data Structure
type Page struct {
	URL        string
	ShortTitle string
	FullTitle  string
	Subtitle   string
	Items      []Item
	NavBar     []NavEntry
	Footer     string
	Image      string
	Body       string
	Visible    bool
}

// Item Data Structure
type Item struct {
	Category    string
	Title       string
	Description string
	RunnersUp   string
	Image       string
}

// NavEntry Helper Struct for making navigation menu
type NavEntry struct {
	URL        string
	ShortTitle string
	Visible    bool
}

// Database Structure
// Used to allow typed functions
type Database struct {
	Session *mgo.Session
}

// Utils
func dataBase() *Database {
	return &Database{dbSession.Copy()}
}

func renderTemplate(n string, w http.ResponseWriter, p *Page) {
	err := templates.ExecuteTemplate(w, n+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loadPage(url string) (*Page, error) {
	p := Page{}
	s := dataBase()
	defer s.Session.Close()
	db := s.Session.DB(dbName).C("pages")
	err := db.Find(bson.M{"url": url}).One(&p)
	err = db.Find(nil).Select(bson.M{"url": 1, "shorttitle": 1, "visible": 1, "_id": 0}).All(&p.NavBar)
	p.Footer = websiteConfig.Footer
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Page) save() error {
	db := dbSession.DB(dbName).C("pages")
	_, err := db.Upsert(bson.M{"url": p.URL}, bson.M{"$set": p})
	if err != nil {
		log.Fatal(err)
		return err
	}
	if p.URL == "index" {
		_, err = db.Upsert(bson.M{"url": p.URL}, bson.M{"$set": bson.M{"shorttitle": p.ShortTitle, "fulltitle": p.FullTitle}})
		if err != nil {
			log.Fatal(err)
			return err
		}
	}
	return nil
}

// Handlers
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	s := dataBase()
	defer s.Session.Close()
	db := s.Session.DB(dbName).C("pages")
	err := db.Remove(bson.M{"url": r.URL.Query().Get("p")})
	if err != nil {
		return
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		fmt.Println("Parsing form failed.")
	}

	p := new(Page)
	decoder := schema.NewDecoder()
	decoder.Decode(p, r.Form)
	fmt.Println(p)

	err = p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+p.URL, http.StatusFound)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	fmt.Println(username)
	fmt.Println(password)
	admin := Admin{}
	s := dataBase()
	defer s.Session.Close()
	db := s.Session.DB(dbName).C("admin")
	err := db.Find(bson.M{"username": username}).One(&admin)
	err = bcrypt.CompareHashAndPassword(admin.HashedPassword, []byte(password))
	if err != nil {
		fmt.Println("err")
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path[1:]
	if url == "auth" {
		authHandler(w, r)
	} else {
		saveHandler(w, r)
	}
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path[1:]
	if url == "" || url == "index" {
		p, err := loadPage("index")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			renderTemplate("index", w, p)
		}
	} else if url == "admin" {
		// p, err := loadPage("admin")
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// } else {
		p := &Page{ShortTitle: "Admin", FullTitle: websiteConfig.FullName, Subtitle: "Admin Panel"}
		renderTemplate("admin", w, p)
		// }
	} else if url == "login" {
		p := &Page{ShortTitle: "Login", FullTitle: websiteConfig.FullName, Subtitle: "Admin Panel"}
		renderTemplate("login", w, p)
	} else if url == "edit" {
		p, err := loadPage(r.URL.Query().Get("p"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			renderTemplate("edit", w, p)
		}
	} else if url == "new" {
		renderTemplate("edit", w, &Page{ShortTitle: "New Page"})
	} else {
		p, err := loadPage(url)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			if p.Visible != true {
				p = &Page{ShortTitle: "Coming soon.", FullTitle: "Coming soon.", NavBar: p.NavBar}
			}
			renderTemplate("view", w, p)
		}
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Method)
	switch r.Method {
	case "DELETE":
		deleteHandler(w, r)
	case "GET":
		getHandler(w, r)
	case "POST":
		postHandler(w, r)
	case "PUT":
		saveHandler(w, r)
	default:
		fmt.Println("default")
	}
}

func rescHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func main() {
	fmt.Printf("Starting up...\n")

	var err error

	dbSession, err = mgo.Dial("127.0.0.1")
	if err != nil {
		fmt.Println("Panic: Dialing failed.")
		panic(err)
	}
	defer dbSession.Close()
	// Optional. Switch the session to a monotonic behavior.
	//dbSession.SetMode(mgo.Monotonic, true)

	// Setup config collection
	db := dbSession.DB(dbName).C("config")
	config := &Config{1, "Default Full Website Name", "Short Name", "Default Footer"}
	_, err = db.Upsert(bson.M{"ident": 1}, bson.M{"$setOnInsert": config})
	if err != nil {
		fmt.Println("Upsert failed.")
		fmt.Println(err)
	}

	err = db.Find(bson.M{"ident": 1}).One(websiteConfig)
	if err != nil {
		fmt.Println("Config retrieval failed.")
		fmt.Println(err)
	}

	// Setup pages collection
	db = dbSession.DB(dbName).C("pages")
	index := mgo.Index{
		Key: []string{"$text:url"},
	}
	err = db.EnsureIndex(index)
	if err != nil {
		fmt.Println("Panic: Index failed.")
		panic(err)
	}
	home := &Page{"index", websiteConfig.ShortName, websiteConfig.FullName, "Default Subtitle", nil, nil, "", "", "", true}
	admin := &Page{"admin", websiteConfig.ShortName, websiteConfig.FullName, "Admin Panel", nil, nil, "", "", "", true}
	_, err = db.Upsert(bson.M{"url": home.URL}, bson.M{"$setOnInsert": home})
	_, err = db.Upsert(bson.M{"url": admin.URL}, bson.M{"$setOnInsert": admin})
	if err != nil {
		fmt.Println("Upsert failed.")
		fmt.Println(err)
	}

	// Setup admin info collection
	db = dbSession.DB(dbName).C("admin")
	user := &Admin{"admin", []byte("password")}
	_, err = db.Upsert(bson.M{"username": user.Username}, bson.M{"$setOnInsert": user})
	if err != nil {
		fmt.Println("Upsert failed.")
		fmt.Println(err)
	}

	// Setup handlers then listen
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/data/", rescHandler)
	fmt.Print("Listening.\n")
	http.ListenAndServe(":8080", nil)
}

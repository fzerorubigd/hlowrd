package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"fmt"
	"sync"
	"flag"
	"os"
	"log"
	"encoding/csv"
	"io"
	"html/template"
	"path/filepath"
)

type items map[int]string

type scopes map[int]items

type data struct {
	Url string
}

var issues map[int]scopes

var lock *sync.RWMutex

var theme *template.Template

var (
	address = flag.String("address", "127.0.0.1:8080", "The address to bind")
	home    = flag.String("home", "", "The home contain all link files")
	fcgi    = flag.Bool("fcgi", false, "Run fastcgi server instead of http server")
)

func loadFile(issue int) error {
	lock.Lock()
	defer lock.Unlock()

	csvFile, err := os.Open(*home + strconv.Itoa(issue) + ".csv")
	defer csvFile.Close()

	if err != nil {
		return err
	}

	var localScopes scopes
	localScopes = make(scopes)

	csvReader := csv.NewReader(csvFile)
	for {
		fields, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Println(err) // Log error to fix.
			return err
		}
		// scope, item, url
		if len(fields) != 3 {
			log.Println(fields, " Is not usable! please verify")
			return fmt.Errorf("Invalid row")
		}

		scope, err := strconv.Atoi(fields[0])
		if err != nil {
			log.Println(err)
			return err
		}

		item, err := strconv.Atoi(fields[1])
		if err != nil {
			log.Print(err)
			return err
		}

		//localItems[item] = fields[2]
		currentScope , ok := localScopes[scope]
		if !ok {
			currentScope = make(items)
		}

		currentScope[item] = fields[2]
		localScopes[scope] = currentScope
	}

	issues[issue] = localScopes
	return nil
}

func handler(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	issue, _ := strconv.Atoi(vars["issue"])
	scope, _ := strconv.Atoi(vars["scope"])
	item, _ := strconv.Atoi(vars["item"])
	issueScopes, ok := issues[issue]
	if !ok {
		err := loadFile(issue)
		if err != nil {
			log.Print(err)
			http.Redirect(rw, r, "/", http.StatusNotFound)
			return
		}
		issueScopes, _ = issues[issue]
	}

	scopeItems, ok := issueScopes[scope]
	if !ok {
		log.Printf("The scope is not valid %d", scope)
		http.Redirect(rw, r, "/", http.StatusNotFound)
		return
	}

	target, ok := scopeItems[item]
	if !ok {
		log.Printf("The item is not valid %d", item)
		http.Redirect(rw, r, "/", http.StatusNotFound)
		return
	}

	if theme != nil {
		theme.Execute(rw, data{target})
	} else {
		http.Redirect(rw, r, target, http.StatusFound)
	}
}

func loadTemplate() {
	var err error
	theme, err = template.ParseFiles(*home + "templates/redirect.html")
	if err != nil {
		log.Println(err)
		log.Println("Load template failed, just using redirect method.")
	}
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, string, error) {
	full, _ := filepath.Abs(path)
	_, err := os.Stat(full)
	if err == nil { return true, full, nil }
	if os.IsNotExist(err) { return false, "", nil }
	return false, "", err
}

func main() {
	flag.Parse()
	loadTemplate()
	r := mux.NewRouter()
	r.HandleFunc("/{issue:[0-9]+}/{scope:[0-9]+}/{item:[0-9]+}", handler)
	if ok , full, _ := exists(*home + "templates/assets/"); ok {
		log.Println("Serving assets folder: " + full)
		r.PathPrefix("/assets").Handler(http.StripPrefix("/assets/",http.FileServer(http.Dir(full))))
	}
	http.Handle("/", r)

	lock = new(sync.RWMutex)
	issues = make(map[int]scopes)
	http.ListenAndServe(*address, nil)
}


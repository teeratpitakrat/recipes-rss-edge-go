package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/mmcdole/gofeed"
	"github.com/teeratpitakrat/gokieker"
)

var (
	edgeAddrPort       string
	middletierAddrPort string
)

type Subscription struct {
	Feeds []*gofeed.Feed
	User  string
}

func GetRequest(w http.ResponseWriter, req *http.Request) {
	username := req.FormValue("username")
	feedUrl := req.FormValue("url")
	delFeedUrl := req.FormValue("delFeedUrl")
	if username == "" {
		username = "default"
	}
	if delFeedUrl != "" {
		DeleteFeed(w, req, username, delFeedUrl)
	} else if feedUrl != "" {
		AddFeed(w, req, username, feedUrl)
	} else {
		ViewFeed(w, req, username)
	}
}

func ViewFeed(w http.ResponseWriter, req *http.Request, username string) {
	k := gokieker.BeginFunction()
	defer k.EndFunction()
	resp, err := http.Get("http://" + middletierAddrPort + "/middletier/rss/user/" + username)
	if err != nil {
		ReturnErrorPage(w, req, err)
		return
	}
	defer resp.Body.Close()
	var subscription Subscription
	//ParseRSSFeeds(resp, &feeds)
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &subscription)
	subscription.User = username
	t, _ := template.ParseFiles("view.html")
	t.Execute(w, subscription)
}

func AddFeed(w http.ResponseWriter, req *http.Request, username string, url string) {
	k := gokieker.BeginFunction()
	defer k.EndFunction()
	resp, err := http.PostForm("http://"+middletierAddrPort+"/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		ReturnErrorPage(w, req, err)
		return
	}
	defer resp.Body.Close()
	http.Redirect(w, req, "/jsp/rss.jsp?username="+username, 302)
}

func DeleteFeed(w http.ResponseWriter, req *http.Request, username string, url string) {
	k := gokieker.BeginFunction()
	defer k.EndFunction()
	req, err := http.NewRequest("DELETE", "http://"+middletierAddrPort+"/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		ReturnErrorPage(w, req, err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("error")
	}
	defer resp.Body.Close()
	http.Redirect(w, req, "/jsp/rss.jsp?username="+username, 302)
}

func Healthcheck(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<h1>Healthcheck page</h1>")
}

func ReturnErrorPage(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func main() {
	gokieker.StartMonitoring()
	k := gokieker.BeginFunction()
	defer k.EndFunction()

	edgeAddrPort = os.Getenv("EDGE_LISTEN_ADDR_PORT")
	middletierAddrPort = os.Getenv("MIDDLETIER_ADDR_PORT")
	fmt.Println("edge addr:", edgeAddrPort)
	fmt.Println("middletier addr:", middletierAddrPort)

	r := mux.NewRouter()
	r.HandleFunc("/jsp/rss.jsp", GetRequest)
	r.HandleFunc("/healthcheck", Healthcheck)
	srv := &http.Server{
		Handler:      r,
		Addr:         ":9090",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

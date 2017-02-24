package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/teeratpitakrat/gokieker"
)

type FeedItem struct {
	Title       string `json:title`
	Description string `json:description`
	Link        string `json:link`
}

type RSSFeed struct {
	Title string     `json:title`
	URL   string     `json:url`
	Items []FeedItem `json:items`
}

type RSSFeedSubscription struct {
	Subscriptions []RSSFeed `json:subscriptions`
	User          string    `json:user`
}

type RSSFeedSubscriptionOneFeed struct {
	Subscriptions RSSFeed `json:subscriptions`
	User          string  `json:user`
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

func DeleteFeed(w http.ResponseWriter, req *http.Request, username string, url string) {
	r := gokieker.BeginFunction()
	defer r.EndFunction()
	req, err := http.NewRequest("DELETE", "http://middletier:9191/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		fmt.Println(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("error")
	}
	defer resp.Body.Close()
	http.Redirect(w, req, "/jsp/rss.jsp?username="+username, 302)
}

func AddFeed(w http.ResponseWriter, req *http.Request, username string, url string) {
	r := gokieker.BeginFunction()
	defer r.EndFunction()
	resp, err := http.PostForm("http://middletier:9191/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		fmt.Println("error")
	}
	defer resp.Body.Close()
	http.Redirect(w, req, "/jsp/rss.jsp?username="+username, 302)
}

func ViewFeed(w http.ResponseWriter, req *http.Request, username string) {
	r := gokieker.BeginFunction()
	defer r.EndFunction()
	resp, err := http.Get("http://middletier:9191/middletier/rss/user/" + username)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	var feeds RSSFeedSubscription
	ParseRSSFeeds(resp, &feeds)
	t, _ := template.ParseFiles("view.html")
	t.Execute(w, &feeds)
}

func ParseRSSFeeds(resp *http.Response, feeds *RSSFeedSubscription) {
	r := gokieker.BeginFunction()
	defer r.EndFunction()
	body, err := ioutil.ReadAll(resp.Body)
	bodycopy := make([]byte, len(body))
	copy(bodycopy, body)
	if err != nil {
		fmt.Println(err)
	}
	err = json.Unmarshal(body, &feeds)
	if err != nil {
		var feed RSSFeedSubscriptionOneFeed
		err = json.Unmarshal(bodycopy, &feed)
		if err != nil {
			fmt.Println(err)
		}
		feeds.Subscriptions = make([]RSSFeed, 1)
		feeds.Subscriptions[0] = feed.Subscriptions
	}
}

func Healthcheck(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<h1>Healthcheck page</h1>")
}

func main() {
	gokieker.StartMonitoring()
	r := gokieker.BeginFunction()
	defer r.EndFunction()
	http.HandleFunc("/jsp/rss.jsp", GetRequest)
	http.HandleFunc("/healthcheck", Healthcheck)
	log.Fatal(http.ListenAndServe(":9090", nil))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/mmcdole/gofeed"
	opentracing "github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
	appdashtracer "sourcegraph.com/sourcegraph/appdash/opentracing"
	"sourcegraph.com/sourcegraph/appdash/traceapp"
)

const CtxSpanID = 0

var collector appdash.Collector

var (
	edgeAddrPort       string
	middletierAddrPort string
)

type Subscription struct {
	Feeds []*gofeed.Feed
	User  string
}

func GetRequest(w http.ResponseWriter, req *http.Request) {
	span := opentracing.StartSpan(req.URL.Path)
	defer span.Finish()

	span.SetTag("Request.Host", req.Host)
	span.SetTag("Request.Address", req.RemoteAddr)
	addHeaderTags(span, req.Header)

	username := req.FormValue("username")
	feedUrl := req.FormValue("url")
	delFeedUrl := req.FormValue("delFeedUrl")
	if username == "" {
		username = "default"
	}

	span.SetBaggageItem("User", username)
	span.SetBaggageItem("feedUrl", feedUrl)
	span.SetBaggageItem("delFeedUrl", delFeedUrl)

	ctx := context.Background()
	ctx = opentracing.ContextWithSpan(ctx, span)

	if delFeedUrl != "" {
		DeleteFeed(ctx, w, req, username, delFeedUrl)
	} else if feedUrl != "" {
		AddFeed(ctx, w, req, username, feedUrl)
	} else {
		ViewFeed(ctx, w, req, username)
	}
}

func ViewFeed(ctx context.Context, w http.ResponseWriter, req *http.Request, username string) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "edge:ViewFeed")
	defer span.Finish()

	r, err := http.NewRequest("GET", "http://"+middletierAddrPort+"/middletier/rss/user/"+username, nil)
	if err != nil {
		log.Println("/middletier/rss/user/:", err)
	}
	carrier := opentracing.HTTPHeadersCarrier(r.Header)
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Println("edge:ViewFeed", err)
		span.LogEvent(err.Error())
	}

	span.SetTag("Response.Status", resp.Status)
	defer resp.Body.Close()

	var subscription Subscription
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &subscription)
	subscription.User = username
	t, _ := template.ParseFiles("view.html")
	t.Execute(w, subscription)
}

func AddFeed(ctx context.Context, w http.ResponseWriter, req *http.Request, username string, url string) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "edge:AddFeed")
	defer span.Finish()

	r, err := http.NewRequest("POST", "http://"+middletierAddrPort+"/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		log.Println("/middletier/rss/user/:", err)
	}
	carrier := opentracing.HTTPHeadersCarrier(r.Header)
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Println("edge:AddFeed", err)
		span.LogEvent(err.Error())
	}

	span.SetTag("Response.Status", resp.Status)
	defer resp.Body.Close()

	http.Redirect(w, req, "/jsp/rss.jsp?username="+username, 302)
}

func DeleteFeed(ctx context.Context, w http.ResponseWriter, req *http.Request, username string, url string) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "edge:DeleteFeed")
	defer span.Finish()

	r, err := http.NewRequest("DELETE", "http://"+middletierAddrPort+"/middletier/rss/user/"+username+"?url="+url, nil)
	if err != nil {
		log.Println("/middletier/rss/user/:", err)
	}
	carrier := opentracing.HTTPHeadersCarrier(r.Header)
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Println("edge:DeleteFeed", err)
		span.LogEvent(err.Error())
	}

	span.SetTag("Response.Status", resp.Status)
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

const headerTagPrefix = "Request.Header."

func addHeaderTags(span opentracing.Span, h http.Header) {
	for k, v := range h {
		span.SetTag(headerTagPrefix+k, strings.Join(v, ", "))
	}
}

func main() {
	collector = appdash.NewRemoteCollector("localhost:7701")
	collector = appdash.NewChunkedCollector(collector)
	tracer := appdashtracer.NewTracer(collector)
	opentracing.InitGlobalTracer(tracer)

	edgeAddrPort = os.Getenv("EDGE_LISTEN_ADDR_PORT")
	middletierAddrPort = os.Getenv("MIDDLETIER_ADDR_PORT")
	fmt.Println("edge addr:", edgeAddrPort)
	fmt.Println("middletier addr:", middletierAddrPort)

	router := mux.NewRouter()
	router.HandleFunc("/jsp/rss.jsp", GetRequest)
	router.HandleFunc("/healthcheck", Healthcheck)

	n := negroni.Classic()
	n.UseHandler(router)
	n.Run(":9090")
}

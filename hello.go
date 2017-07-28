package guestbook

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"google.golang.org/appengine/user"
)

type Vote struct {
	Topic  *datastore.Key
	Author string
}

// [START greeting_struct]
type Greeting struct {
	Author  string
	Title   string
	Content string
	Key     int64
	Date    time.Time
	Score   int
}

// [END greeting_struct]

func init() {
	http.HandleFunc("/", root)
	http.HandleFunc("/sign", sign)
	http.HandleFunc("/vote", vote)
}

// guestbookKey returns the key used for all guestbook entries.
func guestbookKey(c context.Context) *datastore.Key {
	// The string "default_guestbook" here could be varied to have multiple guestbooks.
	return datastore.NewKey(c, "Guestbook", "default_guestbook", 0, nil)
}

func countVotes(c context.Context, topic *datastore.Key) int {
	count, _ := datastore.NewQuery("Vote").Filter("Topic =", topic).Distinct().Count(c)
	return count
}

// [START func_root]
func root(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	// Ancestor queries, as shown here, are strongly consistent with the High
	// Replication Datastore. Queries that span entity groups are eventually
	// consistent. If we omitted the .Ancestor from this query there would be
	// a slight chance that Greeting that had just been written would not
	// show up in a query.
	// [START query]
	q := datastore.NewQuery("Greeting").Ancestor(guestbookKey(c)).Order("-Date").Limit(10)
	// [END query]
	// [START getall]
	greetings := make([]Greeting, 0, 10)
	keys, err := q.GetAll(c, &greetings)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i, k := range keys {
		greetings[i].Score = countVotes(c, k)
		greetings[i].Key = k.IntID()
	}

	loginURL, logoutURL := "", ""
	if user.Current(c) == nil {
		loginURL, _ = user.LoginURL(c, "/")
	} else {
		logoutURL, _ = user.LogoutURL(c, "/")
	}
	// [END getall]
	if err := guestbookTemplate.Execute(w, guestbookTemplateContext{greetings, loginURL, logoutURL}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// [END func_root]

type guestbookTemplateContext struct {
	Greetings           []Greeting
	LoginURL, LogoutURL string
}

var guestbookTemplate = template.Must(template.ParseFiles("book.html"))

func vote(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	topicQuery := r.URL.Query()["greeting"]
	if topicQuery == nil || len(topicQuery) != 1 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	topicID, err := strconv.ParseInt(topicQuery[0], 10, 64)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	u := user.Current(c)
	if u == nil {
		loginURL, _ := user.LoginURL(c, fmt.Sprintf("/vote?greeting=%d", topicID))
		http.Redirect(w, r, loginURL, http.StatusFound)
		return
	}
	tk := datastore.NewKey(c, "Greeting", "", topicID, guestbookKey(c))
	num, err := datastore.NewQuery("Vote").Filter("Topic =", tk).Filter("Author =", u.String()).Count(c)
	if err != nil || num > 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	v := Vote{
		Author: u.String(),
		Topic:  tk,
	}
	k := datastore.NewIncompleteKey(c, "Vote", guestbookKey(c))
	if _, err := datastore.Put(c, k, &v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Debugf(c, "User %s submitted vote on %s", v.Author, v.Topic)
	http.Redirect(w, r, "/", http.StatusFound)
}

// [START func_sign]
func sign(w http.ResponseWriter, r *http.Request) {
	// [START new_context]
	c := appengine.NewContext(r)
	// [END new_context]
	g := Greeting{
		Content: r.FormValue("content"),
		Date:    time.Now(),
	}
	// [START if_user]
	if u := user.Current(c); u != nil {
		g.Author = u.String()
	}
	// We set the same parent key on every Greeting entity to ensure each Greeting
	// is in the same entity group. Queries across the single entity group
	// will be consistent. However, the write rate to a single entity group
	// should be limited to ~1/second.
	key := datastore.NewIncompleteKey(c, "Greeting", guestbookKey(c))
	key, err := datastore.Put(c, key, &g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Debugf(c, "User %s submitted post %s", g.Author, key)
	http.Redirect(w, r, "/", http.StatusFound)
	// [END if_user]
}

// [END func_sign

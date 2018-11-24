// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/build/cmd/pubsubhelper/pubsubtypes"
)

func (c *Corpus) activityChan(topic string) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.activityChans[topic]; ok {
		return ch
	}
	if c.activityChans == nil {
		c.activityChans = map[string]chan struct{}{}
	}
	ch := make(chan struct{}) // unbuffered
	c.activityChans[topic] = ch
	return ch
}

func (c *Corpus) fire(topic string) {
	ch := c.activityChan(topic)
	select {
	case ch <- struct{}{}:
		log.Printf("Pubsub woke up sync for topic %q", topic)
	default:
		log.Printf("Pubsub event on topic %q discarded; already syncing?", topic)
	}
}

// StartPubSubHelperSubscribe starts subscribing to a
// golang.org/x/build/cmd/pubsubhelper server, such
// as https://pubsubhelper.golang.org
func (c *Corpus) StartPubSubHelperSubscribe(urlBase string) {
	go c.subscribeLoop(urlBase)
}

func (c *Corpus) subscribeLoop(urlBase string) {
	var after time.Time
	for {
		newAfter, err := c.getEvent(urlBase, after)
		if err != nil {
			log.Printf("pubsub subscribe: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		after = newAfter
	}
}

var zt time.Time // a zero time.Time

func (c *Corpus) getEvent(urlBase string, after time.Time) (newAfter time.Time, err error) {
	var afterStr string
	if !after.IsZero() {
		afterStr = after.UTC().Format(time.RFC3339Nano)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, _ := http.NewRequest("GET", urlBase+"/waitevent?after="+afterStr, nil)
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return zt, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return zt, errors.New(res.Status)
	}
	var evt pubsubtypes.Event
	if err := json.NewDecoder(res.Body).Decode(&evt); err != nil {
		return zt, err
	}
	if !evt.LongPollTimeout {
		got, _ := json.MarshalIndent(evt, "", "\t")
		log.Printf("Got pubsubhelper event: %s", got)
		if gh := evt.GitHub; gh != nil {
			topic := "github:" + gh.RepoOwner + "/" + gh.Repo
			c.fire(topic)
		}
		if gr := evt.Gerrit; gr != nil {
			c.fire(gerritTopicOfEvent(gr))
		}
	}
	return evt.Time.Time(), nil
}

// Return topics like "gerrit:go.googlesource.com/build"
func gerritTopicOfEvent(gr *pubsubtypes.GerritEvent) string {
	server := gr.URL // "https://code-review.googlesource.com/11970
	if i := strings.Index(server, "//"); i != -1 {
		server = server[i+2:] // code-review.googlesource.com/11970
	}
	if i := strings.Index(server, "/"); i != -1 {
		server = server[:i] // code-review.googlesource.com
	}
	server = strings.Replace(server, "-review.googlesource.com", ".googlesource.com", 1)
	return fmt.Sprintf("gerrit:%s/%s", server, gr.Project)
}

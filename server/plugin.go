package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// this regexp matches links to Wikipedia pages with or without scheme
var wikiURLPattern = regexp.MustCompile(`((http|https):\/\/)?[^\s]+\.wikipedia\.org[^\s]*`)

// this regexp captures the page name and anchor
// the title is \0 and the anchor is \1
var wikiPagePattern = regexp.MustCompile(`wiki\/[^\s\#]+(\#[^\s]+)?`)

// // matches the pattern of wikilinks from inside of the body text after
// // they have been converted to Markdown by pandoc
// var wikilinkPattern = regexp.MustCompile(`\[[\w\s]+\]\((\w+) "wikilink"\)`)
var wikilinkPattern = regexp.MustCompile(`(\w+) "wikilink"`)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	botId        string
	profileImage []byte
}

// types for unmarshaling data from wikipedia
type WikiResponse struct {
	Query `json:"query"`
}

type Query struct {
	Pages map[string]WikiResult `json:"pages"`
}

type WikiResult struct {
	Title   string `json:"title"`
	Extract string `json:"extract"`
	PageId  int    `json:"pageid"`
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if post.UserId == p.botId {
		return
	}

	for _, link := range wikiURLPattern.FindAllString(post.Message, -1) {
		titleAndAnchor := strings.Split(wikiPagePattern.FindStringSubmatch(link)[0], "#")

		var title string
		//var anchor string
		if len(titleAndAnchor) == 1 {
			title = titleAndAnchor[0]
			//anchor = ""
		} else if len(titleAndAnchor) == 2 {
			title = titleAndAnchor[0]
			//anchor = titleAndAnchor[1]
		} else {
			p.API.LogWarn(fmt.Sprintf("Couldn't find title in Wikipedia link %s", link))
			continue
		}

		// this hack hacks off a bad prefix
		// probably a better regexp or something is better
		// hackity hack hack hack ⚔
		if strings.HasPrefix(title, "wiki/") {
			title = title[5:]
		}

		resp, err := http.Get(
			fmt.Sprintf(
				"https://en.wikipedia.org/w/api.php?action=query&titles="+
					"%s"+
					"&format=json"+
					"&prop=extracts"+
					"&exintro=true"+
					"&explaintext=true"+
					"&exsentences=1"+
					"&exlimit=1", title))

		if err != nil {
			p.API.LogWarn(fmt.Sprintf("Something went wrong getting %s: %s", link, err.Error()))
			continue
		}

		if resp == nil {
			p.API.LogError("response was empty")
			continue
		}

		message, err := p.messageContentFromResponse(resp)
		if err != nil {
			p.API.LogError(err.Error())
			continue
		}

		p.API.CreatePost(&model.Post{
			UserId:    p.botId,
			Message:   message,
			ParentId:  post.ParentId,
			ChannelId: post.ChannelId,
		})
	}
}

func (p *Plugin) messageContentFromResponse(resp *http.Response) (string, error) {
	response, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var w WikiResponse
	if err := json.Unmarshal(response, &w); err != nil {
		return "", err
	}

	for _, page := range w.Pages {
		return strings.SplitN(page.Extract, "\n", 2)[0], nil
	}

	return "", errors.New("shit")
}

func (p *Plugin) OnActivate() (err error) {
	p.botId, err = p.Helpers.EnsureBot(&model.Bot{
		Username:    "wikipedia",
		DisplayName: "Wikipedia",
		Description: "The Wikipedia Bot",
	})

	if err != nil {
		return err
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return err
	}

	p.profileImage, err = ioutil.ReadFile(filepath.Join(bundlePath, "assets", "wiki.png"))
	if err != nil {
		p.API.LogError(fmt.Sprintf(err.Error(), "couldn't read profile image: %s"))
		return err
	}

	appErr := p.API.SetProfileImage(p.botId, p.profileImage)
	if appErr != nil {
		return appErr
	}

	return nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Write(p.profileImage)
}

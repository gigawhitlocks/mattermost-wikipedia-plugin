package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// this regexp matches links to Wikipedia pages with or without scheme
var wikiLinkPattern = regexp.MustCompile(`((http|https):\/\/)?[^\s]+\.wikipedia\.org[^\s]*`)

// this regexp captures the page name and anchor
// the title is \0 and the anchor is \1
var wikiPagePattern = regexp.MustCompile(`wiki\/[^\s\#]+(\#[^\s]+)?`)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	for _, link := range wikiLinkPattern.FindAllString(post.Message, -1) {
		titleAndAnchor := strings.Split(wikiPagePattern.FindStringSubmatch(link)[0], "#")

		var title string
		var anchor string
		if len(titleAndAnchor) == 1 {
			title = titleAndAnchor[0]
			anchor = ""
		} else if len(titleAndAnchor) == 2 {
			title = titleAndAnchor[0]
			anchor = titleAndAnchor[1]
		} else {
			p.API.LogWarn(fmt.Sprintf("Couldn't find title in Wikipedia link %s", link))
			continue
		}

		p.API.LogDebug(fmt.Sprintf("%s, %s", title, anchor))
	}
}

// See https://developers.mattermost.com/extend/plugins/server/reference/

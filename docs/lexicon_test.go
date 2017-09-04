package docs

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("lexicon_test")

type ruleArgs struct {
	Functions map[string]struct {
		Args []struct {
			Deprecated bool     `json:"deprecated"`
			Required   bool     `json:"required"`
			Name       string   `json:"name"`
			Types      []string `json:"types"`
		} `json:"args"`
	} `json:"functions"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func loadRuleArgs() map[string]map[string][]string {
	b, err := ioutil.ReadFile("src/parse/rule_args.json")
	must(err)
	args := &ruleArgs{}
	must(json.Unmarshal(b, &args))
	ret := map[string]map[string][]string{}
	for name, f := range args.Functions {
		a := map[string][]string{}
		for _, arg := range f.Args {
			a[arg.Name] = arg.Types
		}
		ret[name] = a
	}
	return ret
}

// loadHTML loads lexicon.html and returns a mapping of function name -> arg name -> declared types.
func loadHTML() map[string]map[string][]string {
	ret := map[string]map[string][]string{}
	f, err := os.Open("docs/lexicon.html")
	must(err)
	defer f.Close()
	tree, err := html.Parse(f)
	must(err)
	// This assumes fairly specific knowledge about the structure of the HTML.
	// Note that it's not a trivial parse since the rules & their tables are all siblings.
	lastAName := ""
	tree = tree.FirstChild.FirstChild.NextSibling // Walk through structural elements that parser inserts
	for node := tree.FirstChild; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode && node.Data == "h3" {
			if a := node.FirstChild; a != nil && a.Type == html.ElementNode && a.Data == "a" {
				for _, attr := range a.Attr {
					if attr.Key == "name" {
						lastAName = attr.Val
						break
					}
				}
			}
		} else if node.Type == html.ElementNode && node.Data == "table" {
			args := map[string][]string{}
			for tr := node.FirstChild.NextSibling.FirstChild; tr != nil; tr = tr.NextSibling {
				if tr.Type == html.ElementNode && tr.Data == "tr" {
					tds := allTDs(tr)
					log.Warning("%d", len(tds))
					nameTD := tr.FirstChild
					typeTD := nameTD.NextSibling.NextSibling
					args[nameTD.FirstChild.Data] = strings.Split(typeTD.FirstChild.Data, " or ")
				}
			}
			ret[lastAName] = args
		}
	}
	return ret
}

func allTDs(node *html.Node) []*html.Node {
	ret := []*html.Node{}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		log.Warning("%s %s", child.Data, child.FirstChild)
		if child.Type == html.ElementNode && child.Data == "td" {
			ret = append(ret, child)
		}
	}
	return ret
}

func TestAllArgsArePresentInHTML(t *testing.T) {
	args := loadRuleArgs()
	html := loadHTML()
	assert.True(t, false)
	return
	for name, arg := range args {
		htmlArg, present := html[name]
		assert.True(t, present, "Built-in function %s is not documented in lexicon", name)
		for argName, types := range arg {
			htmlTypes, present := htmlArg[argName]
			assert.True(t, present, "Built-in function %s is lacking documentation for argument %s", name, argName)
			assert.Equal(t, types, htmlTypes, "Built-in function %s, argument %s declares different types to documentation", name, argName)
		}
	}
}

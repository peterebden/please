package maven

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A Fetch fetches files for us from Maven.
// It memoises requests internally so we don't re-request the same file.
type Fetch struct {
	// URL of the Maven repo we're fetching from
	url string
	// HTTP client to fetch with
	client *http.Client
	// Request cache
	cache map[string][]byte
	mutex sync.Mutex
	// Excluded & optional artifacts; this isn't a great place for them but they need to go somewhere.
	exclude, optional map[string]bool
}

// NewFetch constructs & returns a new Fetch instance.
func NewFetch(url string, exclude, optional []string) *Fetch {
	if !strings.HasSuffix(url, "/") {
		url = url + "/"
	}
	if strings.HasPrefix(url, "http:") {
		log.Warning("Repo URL is not secure, you should really be using https")
	}
	return &Fetch{
		url:      url,
		client:   &http.Client{Timeout: 30 * time.Second},
		cache:    map[string][]byte{},
		exclude:  toMap(exclude),
		optional: toMap(optional),
	}
}

// toMap converts a slice of strings to a map.
func toMap(sl []string) map[string]bool {
	m := make(map[string]bool, len(sl))
	for _, s := range sl {
		m[s] = true
	}
	return m
}

// Pom fetches the POM XML for a package.
// Note that this may invoke itself recursively to fetch parent artifacts and dependencies.
func (f *Fetch) Pom(a artifact) *pomXml {
	pom := &pomXml{}
	pom.Unmarshal(f, f.mustFetch(a.PomPath()))
	return pom
}

// Metadata returns the metadata XML for a package.
// This contains some information, typically the main useful thing is the latest available version of the package.
func (f *Fetch) Metadata(a artifact) *mavenMetadataXml {
	metadata := &mavenMetadataXml{Group: a.GroupId, Artifact: a.ArtifactId}
	metadata.Unmarshal(f.mustFetch(a.MetadataPath()))
	return metadata
}

// HasSources returns true if the given artifact has any sources available.
// Unfortunately there's no way of determining this other than making a request, and lots of servers
// don't seem to support HEAD requests to just find out if the artifact is there.
func (f *Fetch) HasSources(a artifact) bool {
	_, err := f.fetch(a.SourcePath(), false)
	return err == nil
}

// IsExcluded returns true if this artifact should be excluded from the download.
func (f *Fetch) IsExcluded(artifact string) bool {
	return f.exclude[artifact]
}

// ShouldInclude returns true if we should include an optional dependency.
func (f *Fetch) ShouldInclude(artifact string) bool {
	return f.optional[artifact]
}

// buildURL returns a URL for the given artifact.
func (f *Fetch) buildURL(a artifact, suffix string) string {
	if a.Version == "" {
		// This is kind of exciting - we just assume the latest version if one isn't available.
		a.Version = f.Metadata(a).LatestVersion()
		log.Notice("Version not specified for %s:%s, decided to use %s", a.GroupId, a.ArtifactId, a.Version)
	}
	return a.PomPath()
}

// mustFetch fetches a URL and returns the content, dying if it can't be downloaded.
func (f *Fetch) mustFetch(url string) []byte {
	b, err := f.fetch(url, true)
	if err != nil {
		log.Fatalf("Error downloading %s: %s\n", f.url+url, err)
	}
	return b
}

// fetch fetches a URL and returns the content.
func (f *Fetch) fetch(url string, readBody bool) ([]byte, error) {
	f.mutex.Lock()
	contents, present := f.cache[url]
	f.mutex.Unlock()
	if present {
		log.Debug("Retrieved %s from cache", url)
		return contents, nil
	}
	url = f.url + url
	log.Notice("Downloading %s...", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := f.client.Do(req)
	if err != nil {
		return nil, err
	} else if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("Bad response code: %s", response.Status)
	}
	defer response.Body.Close()
	if readBody {
		contents, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.cache[url] = contents
	return contents, nil
}

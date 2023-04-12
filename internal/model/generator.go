package model

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/go-github/v51/github"
)

type CachedDataSet[Data any] struct {
	FileName string
	Retrieve func() (*Data, error)
	d        *Data
	err      error
}

func (c *CachedDataSet[Data]) Get() (*Data, error) {
	if c.d != nil || c.err != nil {
		return c.d, c.err
	}

	if _, err := os.Stat(c.FileName); err == nil {
		f, err := os.Open(c.FileName)
		if err != nil {
			return nil, err
		}
		c.d = new(Data)
		err = json.NewDecoder(f).Decode(c.d)
		if err != nil {
			return c.d, err
		}

		return c.d, nil
	}

	c.d, c.err = c.Retrieve()
	f, err := os.Create(c.FileName)
	if err != nil {
		return nil, err
	}

	err = json.NewEncoder(f).Encode(c.d)
	if err != nil {
		return c.d, err
	}

	return c.d, nil
}

var EndOfList = fmt.Errorf("EndOfList")

type ListGenerator[ItemType any] struct {
	Retrieve func(github.ListOptions) ([]*ItemType, *github.Response, error)
	pg       *PagedListGenerator[string, ItemType]
}

func (g *ListGenerator[ItemType]) init() {
	if g.pg != nil {
		return
	}
	g.pg = &PagedListGenerator[string, ItemType]{
		Retrieve: func(opts github.ListOptions) (*string, []*ItemType, *github.Response, error) {
			v, res, err := g.Retrieve(opts)
			return nil, v, res, err
		},
	}
}

func (g *ListGenerator[GithubType]) HasNext() bool {
	g.init()
	return g.pg.HasNext()
}

func (g *ListGenerator[GithubType]) Next() (*GithubType, error) {
	g.init()
	_, v, err := g.pg.Next()
	return v, err
}

type PagedListGenerator[GithubType any, ItemType any] struct {
	Retrieve  func(github.ListOptions) (*GithubType, []*ItemType, *github.Response, error)
	index     int
	nextPage  int
	page      *GithubType
	items     []*ItemType
	opts      github.ListOptions
	endOfList bool
	lastPage  bool
}

func (g *PagedListGenerator[GithubType, ItemType]) HasNext() bool {
	if !g.endOfList {
		g.getNextPage()
	}
	return !g.endOfList
}

func (g *PagedListGenerator[GithubType, ItemType]) getNextPage() error {
	var (
		res *github.Response
		err error
	)

	if !g.lastPage && g.index >= len(g.items) {
		// reset page index to 0
		g.index = 0

		// retrieve the next page
		g.opts.Page = g.nextPage
		g.page, g.items, res, err = g.Retrieve(g.opts)
		if err != nil {
			return err
		}
		// update the last page and next page
		g.lastPage = res.NextPage == 0
		g.nextPage = res.NextPage // this will be 0 for the last page

		// Update whether it has reached the end of the list
		g.endOfList = g.index >= len(g.items) && g.lastPage
	}

	return nil
}

func (g *PagedListGenerator[GithubType, ItemType]) Next() (*GithubType, *ItemType, error) {
	// End immediately if this is at the end of the list
	if g.endOfList {
		return nil, nil, EndOfList
	}

	// Get the next page if needed
	err := g.getNextPage()
	if err != nil {
		return nil, nil, err
	}

	if g.index >= len(g.items) && g.lastPage {
		g.endOfList = true
		return nil, nil, EndOfList
	}

	// Pick the index for this item
	thisIndex := g.index

	// increment the index for the next call to Next()
	g.index++

	// Update whether it has reached the end of the list
	g.endOfList = g.index >= len(g.items) && g.lastPage

	return g.page, g.items[thisIndex], nil
}

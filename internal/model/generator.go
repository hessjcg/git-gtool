package model

import (
	"fmt"

	"github.com/google/go-github/v51/github"
)

var EndOfList = fmt.Errorf("EndOfList")

// ListGenerator handles logic for iterating through github client
// paged lists.
type ListGenerator[ItemType any] struct {
	// Retrieve should call the github client with the provided ListOptions
	// to retrieve the next page.
	Retrieve func(github.ListOptions) ([]*ItemType, *github.Response, error)
	pg       *PagedListGenerator[string, ItemType]
}

// init ensures that this ListGenerator's hidden internal PagedListGenerator
// is initialized before any operations may proceed.
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

// HasNext Returns true when the next item in the list exists.
func (g *ListGenerator[GithubType]) HasNext() bool {
	g.init()
	return g.pg.HasNext()
}

// Next returns the next item in the list or error if the next page
// cannot be retrieved or there are no more items.
func (g *ListGenerator[GithubType]) Next() (*GithubType, error) {
	g.init()
	_, v, err := g.pg.Next()
	return v, err
}

// PagedListGenerator handles logic for iterating through a Github Client paged
// list where your code needs both the page object and the item object. GithubType
// is the page object, ItemType is the item object.
type PagedListGenerator[GithubType any, ItemType any] struct {
	// Retrieve should call the github client with the provided ListOptions
	// to retrieve the next page.
	Retrieve  func(github.ListOptions) (*GithubType, []*ItemType, *github.Response, error)
	index     int
	nextPage  int
	page      *GithubType
	items     []*ItemType
	opts      github.ListOptions
	endOfList bool
	lastPage  bool
}

// HasNext returns true when there are more items in the list.
func (g *PagedListGenerator[GithubType, ItemType]) HasNext() bool {
	if !g.endOfList {
		g.getNextPage()
	}
	return !g.endOfList
}

// getNextPage is an internal method that attempts to load the next page
// before HasNext() or Next() may return.
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

// Next returns a pointer the next item in the list, along with a pointer to
// the current page. Returns error when the end of the list is reached, or when
// there was a problem retrieving the next page.
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

	// Pick the index for this item
	thisIndex := g.index

	// increment the index for the next call to Next()
	g.index++

	// Update whether it has reached the end of the list
	g.endOfList = g.index >= len(g.items) && g.lastPage

	return g.page, g.items[thisIndex], nil
}

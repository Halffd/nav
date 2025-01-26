package actions

import (
	"github.com/gobuffalo/buffalo"
)

func HomeHandler(c buffalo.Context) error {
	url := c.Param("url")
	if url == "" {
		return c.Render(200, r.HTML("index.html"))
	}
	// ... rest of your rendering logic
	return c.Render(200, r.HTML("index.html", "Content": content))
} 
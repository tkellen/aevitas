package domain

import "fmt"

type websiteSpec struct {
	Title       string
	Description string
	Author      string
	//Aliases     []string
}

func (ws *websiteSpec) Validate() error {
	if ws.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if ws.Description == "" {
		return fmt.Errorf("description must be defined")
	}
	if ws.Author == "" {
		return fmt.Errorf("author must be defined")
	}
	return nil
}
